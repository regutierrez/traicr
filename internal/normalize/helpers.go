package normalize

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"time"

	"github.com/regutierrez/traicr/internal/domain"
)

const maxSourceBytes = 64 << 20

var errSourceLimit = fmt.Errorf("normalization source exceeds %d MiB limit", maxSourceBytes>>20)

type limitedFS struct {
	ctx       context.Context
	source    fs.FS
	remaining int64
}

type limitedFile struct {
	fs.File
	input *limitedFS
}

type limitedFileInfo struct {
	fs.FileInfo
	size int64
}

func newLimitedFS(ctx context.Context, source fs.FS) fs.FS {
	return &limitedFS{ctx: ctx, source: source, remaining: maxSourceBytes}
}

func (source *limitedFS) Open(name string) (fs.File, error) {
	if err := source.ctx.Err(); err != nil {
		return nil, err
	}
	file, err := source.source.Open(name)
	if err != nil {
		return nil, err
	}
	return &limitedFile{File: file, input: source}, nil
}

func (file *limitedFile) Read(p []byte) (int, error) {
	if err := file.input.ctx.Err(); err != nil {
		return 0, err
	}
	if file.input.remaining == 0 {
		var probe [1]byte
		n, err := file.File.Read(probe[:])
		if n > 0 {
			return 0, errSourceLimit
		}
		return 0, err
	}
	if int64(len(p)) > file.input.remaining {
		p = p[:file.input.remaining]
	}
	n, err := file.File.Read(p)
	file.input.remaining -= int64(n)
	return n, err
}

func (file *limitedFile) Stat() (fs.FileInfo, error) {
	info, err := file.File.Stat()
	if err != nil {
		return nil, err
	}
	size := info.Size()
	if size > file.input.remaining {
		size = file.input.remaining
	}
	return limitedFileInfo{FileInfo: info, size: size}, nil
}

func (info limitedFileInfo) Size() int64 {
	return info.size
}

type sourceRecord struct {
	Value map[string]any
	Path  string
	Line  int
}

func readJSON(source fs.FS, path string) (map[string]any, error) {
	b, err := fs.ReadFile(source, path)
	if err != nil {
		return nil, err
	}
	var value map[string]any
	if err := json.Unmarshal(b, &value); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return value, nil
}

func readJSONL(source fs.FS, path string) ([]sourceRecord, []domain.Warning, error) {
	f, err := source.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	var records []sourceRecord
	var warnings []domain.Warning
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), maxSourceBytes)
	line := 0
	for scanner.Scan() {
		line++
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		var value map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &value); err != nil {
			warnings = append(warnings, warning("malformed_record", fmt.Sprintf("%s line %d is not valid JSON", path, line)))
			continue
		}
		records = append(records, sourceRecord{Value: value, Path: path, Line: line})
	}
	if err := scanner.Err(); err != nil {
		return nil, warnings, err
	}
	return records, warnings, nil
}

func complete(events []domain.Event, warnings []domain.Warning) domain.Normalization {
	status := "normalized"
	if len(warnings) > 0 {
		status = "partially_parsed"
	}
	if len(events) == 0 {
		status = "unsupported"
	}
	return domain.Normalization{Status: status, Events: events, Warnings: warnings}
}

func warning(code, message string) domain.Warning {
	return domain.Warning{Code: code, Message: message}
}

func unknown(record sourceRecord, detail string) domain.Warning {
	return warning("unsupported_record", fmt.Sprintf("%s line %d: %s", record.Path, record.Line, detail))
}

func source(path string, line int) []domain.SourceRef {
	return []domain.SourceRef{{Path: path, Line: line}}
}

func nativeKey(kind, id string, stable any) string {
	if id != "" {
		return kind + ":" + id
	}
	b, _ := json.Marshal(stable)
	sum := sha256.Sum256(append([]byte(kind+"\x00"), b...))
	return kind + ":sha256:" + hex.EncodeToString(sum[:])
}

func object(value any) map[string]any {
	v, _ := value.(map[string]any)
	return v
}

func objects(value any) []map[string]any {
	items, _ := value.([]any)
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if value := object(item); value != nil {
			result = append(result, value)
		}
	}
	return result
}

func stringValue(value any) string {
	s, _ := value.(string)
	return s
}

func numberString(value any) string {
	switch value := value.(type) {
	case string:
		return value
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case int:
		return strconv.Itoa(value)
	default:
		return ""
	}
}

func timestamp(value any) string {
	switch value := value.(type) {
	case string:
		return value
	case float64:
		seconds := int64(value)
		if seconds > 10_000_000_000 {
			return time.UnixMilli(seconds).UTC().Format(time.RFC3339Nano)
		}
		return time.Unix(seconds, 0).UTC().Format(time.RFC3339Nano)
	default:
		return ""
	}
}

func readableJSON(value any) string {
	if value == nil {
		return ""
	}
	b, err := json.Marshal(searchableValue("", value))
	if err != nil {
		return ""
	}
	return string(b)
}

func searchableValue(key string, value any) any {
	switch value := value.(type) {
	case string:
		if binaryField(key) || strings.HasPrefix(strings.ToLower(value), "data:") && strings.Contains(value, ";base64,") {
			return nil
		}
		return value
	case []any:
		result := make([]any, 0, len(value))
		for _, item := range value {
			if item := searchableValue("", item); item != nil {
				result = append(result, item)
			}
		}
		return result
	case map[string]any:
		result := make(map[string]any, len(value))
		encodedData := strings.EqualFold(stringValue(value["type"]), "base64") || strings.EqualFold(stringValue(value["encoding"]), "base64")
		for field, item := range value {
			if encodedData && strings.EqualFold(field, "data") {
				continue
			}
			if item := searchableValue(field, item); item != nil {
				result[field] = item
			}
		}
		return result
	default:
		return value
	}
}

func binaryField(key string) bool {
	switch strings.ToLower(key) {
	case "base64", "blob", "bytes", "encryptedcontent":
		return true
	default:
		return false
	}
}

func contentText(value any) string {
	switch value := value.(type) {
	case string:
		if filtered := searchableValue("", value); filtered == nil {
			return ""
		}
		return value
	case []any:
		var parts []string
		for _, item := range value {
			if text := contentText(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	case map[string]any:
		var parts []string
		for _, key := range []string{"text", "thinking", "content", "output", "result", "error", "summary", "title"} {
			if text := contentText(value[key]); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

func indexedID(id string, index int) string {
	if id == "" {
		return ""
	}
	return id + ":" + strconv.Itoa(index)
}
