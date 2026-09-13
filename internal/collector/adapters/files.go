package adapters

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/regutierrez/traicr/internal/archive"
	"github.com/regutierrez/traicr/internal/domain"
)

type jsonlAdapter struct {
	name          string
	format        string
	defaultRoots  func() []string
	companionJSON bool
}

func (a jsonlAdapter) Name() string { return a.name }

func (a jsonlAdapter) Discover(_ context.Context, configured []string) Source {
	roots := chooseRoots(configured, a.defaultRoots())
	files, warnings := findFiles(roots, ".jsonl")
	return Source{Harness: a.name, Location: strings.Join(roots, string(os.PathListSeparator)), Traces: len(files), Warnings: warnings}
}

func (a jsonlAdapter) Collect(ctx context.Context, configured []string, progress Progress) (Result, error) {
	roots := chooseRoots(configured, a.defaultRoots())
	files, warnings := findFiles(roots, ".jsonl")
	base, err := os.MkdirTemp("", "traicr-"+a.name+"-*")
	if err != nil {
		return Result{}, err
	}
	result := Result{Warnings: warnings, Cleanup: func() { os.RemoveAll(base) }}
	for i, path := range files {
		if err := ctx.Err(); err != nil {
			result.Cleanup()
			return Result{}, err
		}
		progress.report(i+1, len(files))
		dir := filepath.Join(base, fmt.Sprintf("%06d", i+1))
		if err := os.MkdirAll(filepath.Join(dir, "source"), 0o700); err != nil {
			result.Cleanup()
			return Result{}, err
		}
		output := filepath.Join(dir, "source", "records.jsonl")
		complete, changed, invalid, first, err := snapshotCompleteLines(ctx, path, output)
		if err != nil {
			result.Warnings = append(result.Warnings, warning("unreadable_source", path+": "+err.Error()))
			continue
		}
		info, _ := os.Stat(path)
		metadata, metadataWarnings := jsonlMetadata(a.name, output, path)
		descriptor := domain.Descriptor{Harness: a.name, Adapter: a.format, NativeTraceID: metadata.ID, ParentNativeTraceID: metadata.ParentID, Title: metadata.Title, WorkingDirectory: metadata.CWD, Warnings: metadataWarnings}
		if info != nil {
			descriptor.NativeUpdatedAt = info.ModTime().UTC().Format(time.RFC3339Nano)
		}
		if !complete {
			descriptor.Warnings = append(descriptor.Warnings, warning("partial_tail", "ignored an incomplete final JSONL record in "+path))
		}
		if changed {
			descriptor.Warnings = append(descriptor.Warnings, warning("live_source_changed", "source changed while it was being copied: "+path))
		}
		if invalid {
			descriptor.Warnings = append(descriptor.Warnings, warning("unknown_schema", "one or more complete records are not valid JSON; bytes were preserved"))
		}
		if len(first) == 0 {
			descriptor.Warnings = append(descriptor.Warnings, warning("empty_source", "source contains no complete JSONL records"))
		}
		if a.companionJSON {
			entries, readErr := os.ReadDir(filepath.Dir(path))
			if readErr != nil {
				descriptor.Warnings = append(descriptor.Warnings, warning("metadata_copy_failed", readErr.Error()))
			}
			for _, entry := range entries {
				if !entry.Type().IsRegular() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
					continue
				}
				sourceMetadata := filepath.Join(filepath.Dir(path), entry.Name())
				before, _ := os.Stat(sourceMetadata)
				if copyErr := copyFile(ctx, sourceMetadata, filepath.Join(dir, "source", entry.Name())); copyErr != nil {
					descriptor.Warnings = append(descriptor.Warnings, warning("metadata_copy_failed", copyErr.Error()))
					continue
				}
				after, _ := os.Stat(sourceMetadata)
				if before == nil || after == nil || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
					descriptor.Warnings = append(descriptor.Warnings, warning("live_source_changed", "cursor-agent metadata changed while it was being copied: "+sourceMetadata))
				}
			}
		}
		if a.name == "claude-code" && strings.Contains(filepath.ToSlash(path), "/subagents/agent-") {
			metadataPath := strings.TrimSuffix(path, ".jsonl") + ".meta.json"
			if _, statErr := os.Stat(metadataPath); statErr == nil {
				if copyErr := copyFile(ctx, metadataPath, filepath.Join(dir, "source", filepath.Base(metadataPath))); copyErr != nil {
					descriptor.Warnings = append(descriptor.Warnings, warning("metadata_copy_failed", copyErr.Error()))
				}
			}
		}
		if descriptor.WorkingDirectory != "" {
			root, remote := gitRepository(descriptor.WorkingDirectory)
			descriptor.Repository = domain.Repository{Root: root, Remote: remote}
		} else {
			populateRepository(&descriptor, first)
		}
		result.Inputs = append(result.Inputs, archive.Input{Descriptor: descriptor, Directory: dir})
	}
	return result, nil
}

func piRoots() []string {
	if value := os.Getenv("PI_CODING_AGENT_DIR"); value != "" {
		return []string{filepath.Join(value, "sessions")}
	}
	home, _ := os.UserHomeDir()
	return []string{filepath.Join(home, ".pi", "agent", "sessions")}
}

func claudeRoots() []string {
	if value := os.Getenv("CLAUDE_CONFIG_DIR"); value != "" {
		return []string{filepath.Join(value, "projects")}
	}
	home, _ := os.UserHomeDir()
	return []string{filepath.Join(home, ".claude", "projects")}
}

func chooseRoots(configured, defaults []string) []string {
	if len(configured) > 0 {
		return configured
	}
	return defaults
}

func findFiles(roots []string, suffixes ...string) ([]string, []domain.Warning) {
	var files []string
	var warnings []domain.Warning
	for _, root := range roots {
		info, err := os.Stat(root)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			warnings = append(warnings, warning("unreadable_source", root+": "+err.Error()))
			continue
		}
		if !info.IsDir() {
			files = append(files, root)
			continue
		}
		err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				warnings = append(warnings, warning("unreadable_source", path+": "+walkErr.Error()))
				return nil
			}
			if entry.Type().IsRegular() {
				for _, suffix := range suffixes {
					if strings.HasSuffix(strings.ToLower(entry.Name()), suffix) {
						files = append(files, path)
						break
					}
				}
			}
			return nil
		})
		if err != nil {
			warnings = append(warnings, warning("unreadable_source", root+": "+err.Error()))
		}
	}
	sort.Strings(files)
	return files, warnings
}

func snapshotCompleteLines(ctx context.Context, source, destination string) (complete, changed, invalid bool, first []byte, err error) {
	before, err := os.Stat(source)
	if err != nil {
		return false, false, false, nil, err
	}
	in, err := os.Open(source)
	if err != nil {
		return false, false, false, nil, err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return false, false, false, nil, err
	}
	buffer := make([]byte, 64<<10)
	var total, lastNewline int64
	remaining := before.Size()
	complete = true
	if err := ctx.Err(); err != nil {
		out.Close()
		return false, false, false, nil, err
	}
	for remaining > 0 {
		if err := ctx.Err(); err != nil {
			out.Close()
			return false, false, false, nil, err
		}
		chunk := buffer
		if int64(len(chunk)) > remaining {
			chunk = chunk[:remaining]
		}
		read, readErr := in.Read(chunk)
		if read > 0 {
			if _, err = out.Write(chunk[:read]); err != nil {
				out.Close()
				return false, false, false, nil, err
			}
			for index, value := range chunk[:read] {
				if value == '\n' {
					lastNewline = total + int64(index) + 1
				}
			}
			total += int64(read)
			remaining -= int64(read)
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			out.Close()
			return false, false, false, nil, readErr
		}
	}
	complete = total == 0 || lastNewline == total
	if !complete {
		if err := out.Truncate(lastNewline); err != nil {
			out.Close()
			return false, false, false, nil, err
		}
	}
	if err := out.Close(); err != nil {
		return false, false, false, nil, err
	}
	snapshot, err := os.Open(destination)
	if err != nil {
		return false, false, false, nil, err
	}
	if err := ctx.Err(); err != nil {
		snapshot.Close()
		return false, false, false, nil, err
	}
	scanner := bufio.NewScanner(snapshot)
	scanner.Buffer(make([]byte, 64<<10), 16<<20)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			snapshot.Close()
			return false, false, false, nil, err
		}
		trimmed := bytes.TrimSpace(scanner.Bytes())
		if len(first) == 0 {
			first = append([]byte(nil), trimmed...)
		}
		if !json.Valid(trimmed) {
			invalid = true
		}
	}
	if scanner.Err() != nil {
		invalid = true
	}
	snapshot.Close()
	after, err := os.Stat(source)
	if err != nil {
		return complete, true, invalid, first, nil
	}
	changed = before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime())
	return complete, changed, invalid, first, nil
}

type traceMetadata struct {
	ID       string
	ParentID string
	CWD      string
	Title    string
}

// Pi session names use the last session_info in file order, including explicit clears.
func readPiSessionTitle(snapshot string) (string, error) {
	file, err := os.Open(snapshot)
	if err != nil {
		return "", err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	var title string
	for {
		// Messages can contain large images. Do not stop before a later name entry.
		line, err := reader.ReadBytes('\n')
		if errors.Is(err, io.EOF) {
			return title, nil
		}
		if err != nil {
			return "", err
		}
		var entry struct {
			Type string `json:"type"`
			Name string `json:"name"`
		}
		if json.Unmarshal(line, &entry) == nil && entry.Type == "session_info" {
			title = strings.TrimSpace(entry.Name)
		}
	}
}

func jsonlMetadata(harness, snapshot, source string) (traceMetadata, []domain.Warning) {
	fallback := strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))
	switch harness {
	case "pi":
		var header struct {
			Type          string `json:"type"`
			ID            string `json:"id"`
			CWD           string `json:"cwd"`
			ParentSession string `json:"parentSession"`
		}
		if !decodeFirstRecord(snapshot, &header) || header.Type != "session" || header.ID == "" {
			return traceMetadata{ID: fallback}, []domain.Warning{warning("unknown_schema", "Pi JSONL has no valid session header; filename identity was used")}
		}
		title, err := readPiSessionTitle(snapshot)
		metadata := traceMetadata{ID: header.ID, CWD: header.CWD, Title: title}
		if err != nil {
			return metadata, []domain.Warning{warning("metadata_read_failed", "read Pi session name: "+err.Error())}
		}
		if header.ParentSession != "" {
			parentPath := header.ParentSession
			if !filepath.IsAbs(parentPath) {
				parentPath = filepath.Join(filepath.Dir(source), parentPath)
			}
			var parent struct {
				Type string `json:"type"`
				ID   string `json:"id"`
			}
			if decodeFirstRecord(parentPath, &parent) && parent.Type == "session" && parent.ID != "" {
				metadata.ParentID = parent.ID
			} else {
				return metadata, []domain.Warning{warning("parent_unresolved", "Pi parentSession did not resolve to a valid session header")}
			}
		}
		return metadata, nil
	case "claude-code":
		file, err := os.Open(snapshot)
		if err != nil {
			return traceMetadata{ID: fallback}, nil
		}
		defer file.Close()
		reader := bufio.NewReader(file)
		metadata := traceMetadata{ID: fallback}
		subagent := filepath.Base(filepath.Dir(source)) == "subagents" && strings.HasPrefix(filepath.Base(source), "agent-")
		foundIdentity := false
		for {
			line, readErr := reader.ReadBytes('\n')
			if readErr != nil && !errors.Is(readErr, io.EOF) {
				return metadata, []domain.Warning{warning("metadata_read_failed", "read Claude metadata: "+readErr.Error())}
			}
			if errors.Is(readErr, io.EOF) {
				break
			}
			var record struct {
				SessionID string `json:"sessionId"`
				AgentID   string `json:"agentId"`
				CWD       string `json:"cwd"`
				Type      string `json:"type"`
				AITitle   string `json:"aiTitle"`
			}
			if json.Unmarshal(line, &record) != nil {
				continue
			}
			if !foundIdentity && record.SessionID != "" && (!subagent || record.AgentID != "") {
				foundIdentity = true
				if subagent {
					metadata.ID, metadata.ParentID = record.AgentID, record.SessionID
				} else {
					metadata.ID = record.SessionID
				}
			}
			if metadata.CWD == "" && filepath.IsAbs(strings.TrimSpace(record.CWD)) {
				metadata.CWD = strings.TrimSpace(record.CWD)
			}
			if record.Type == "ai-title" && strings.TrimSpace(record.AITitle) != "" {
				metadata.Title = strings.TrimSpace(record.AITitle)
			}
		}
		if foundIdentity {
			return metadata, nil
		}
		return metadata, []domain.Warning{warning("unknown_schema", "Claude JSONL has no top-level sessionId; filename identity was used")}
	case "cursor-agent":
		metadata := traceMetadata{ID: fallback}
		if filepath.Base(filepath.Dir(source)) == "subagents" {
			metadata.ParentID = filepath.Base(filepath.Dir(filepath.Dir(source)))
		}
		return metadata, nil
	case "codex":
		var record struct {
			Type    string `json:"type"`
			Payload struct {
				ID             string `json:"id"`
				CWD            string `json:"cwd"`
				ParentThreadID string `json:"parent_thread_id"`
				ForkedFromID   string `json:"forked_from_id"`
			} `json:"payload"`
		}
		if decodeFirstRecord(snapshot, &record) && record.Type == "session_meta" && record.Payload.ID != "" {
			parentID := record.Payload.ParentThreadID
			if parentID == "" {
				parentID = record.Payload.ForkedFromID
			}
			return traceMetadata{ID: record.Payload.ID, ParentID: parentID, CWD: record.Payload.CWD}, nil
		}
		return traceMetadata{ID: fallback}, []domain.Warning{warning("unknown_schema", "Codex rollout has no valid session_meta header; filename identity was used")}
	default:
		return traceMetadata{ID: fallback}, nil
	}
}

func decodeFirstRecord(path string, target any) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	reader := bufio.NewReaderSize(file, 64<<10)
	line, err := reader.ReadSlice('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false
	}
	return json.Unmarshal(bytes.TrimSpace(line), target) == nil
}

func populateRepository(descriptor *domain.Descriptor, record []byte) {
	var object map[string]any
	if json.Unmarshal(record, &object) != nil {
		return
	}
	for _, key := range []string{"cwd", "workingDirectory", "working_directory", "projectPath"} {
		if value, ok := object[key].(string); ok && filepath.IsAbs(value) {
			descriptor.WorkingDirectory = value
			root, remote := gitRepository(value)
			descriptor.Repository = domain.Repository{Root: root, Remote: remote}
			return
		}
	}
}

func gitRepository(directory string) (string, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	rootBytes, err := exec.CommandContext(ctx, "git", "-C", directory, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", ""
	}
	root := strings.TrimSpace(string(rootBytes))
	remoteBytes, err := exec.CommandContext(ctx, "git", "-C", root, "remote", "get-url", "origin").Output()
	if err != nil {
		return root, ""
	}
	return root, sanitizeRemote(strings.TrimSpace(string(remoteBytes)))
}

func sanitizeRemote(remote string) string {
	remote = strings.TrimSpace(remote)
	if parsed, err := url.Parse(remote); err == nil && parsed.Scheme != "" && parsed.Hostname() != "" {
		host := strings.ToLower(parsed.Hostname())
		port := parsed.Port()
		if port != "" && !((parsed.Scheme == "ssh" && port == "22") || (parsed.Scheme == "https" && port == "443") || (parsed.Scheme == "http" && port == "80")) {
			host = net.JoinHostPort(host, port)
		}
		return canonicalRemote(host, parsed.Path)
	}
	remote, _, _ = strings.Cut(remote, "#")
	remote, _, _ = strings.Cut(remote, "?")
	hostPath := remote
	if at := strings.LastIndex(hostPath, "@"); at >= 0 {
		hostPath = hostPath[at+1:]
	}
	if host, repository, ok := strings.Cut(hostPath, ":"); ok && host != "" && repository != "" {
		return canonicalRemote(strings.ToLower(host), repository)
	}
	return strings.TrimSuffix(strings.TrimSuffix(hostPath, ".git"), "/")
}

func canonicalRemote(host, repository string) string {
	repository = strings.Trim(repository, "/")
	repository = strings.TrimSuffix(repository, ".git")
	if repository == "" {
		return host
	}
	return host + "/" + repository
}

func copyFile(ctx context.Context, source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	buffer := make([]byte, 64<<10)
	remaining := info.Size()
	var copyErr error
	for remaining > 0 {
		if copyErr = ctx.Err(); copyErr != nil {
			break
		}
		chunk := buffer
		if int64(len(chunk)) > remaining {
			chunk = chunk[:remaining]
		}
		var read int
		read, copyErr = in.Read(chunk)
		if read > 0 {
			var written int
			written, copyErr = out.Write(chunk[:read])
			if copyErr == nil && written != read {
				copyErr = io.ErrShortWrite
			}
			if copyErr != nil {
				break
			}
			remaining -= int64(read)
		}
		if copyErr != nil {
			break
		}
	}
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func warning(code, message string) domain.Warning {
	return domain.Warning{Code: code, Message: message}
}

func homePath(parts ...string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(append([]string{home}, parts...)...)
}

func cursorConfigRoot() string {
	if runtime.GOOS == "darwin" {
		return homePath("Library", "Application Support", "Cursor", "User")
	}
	if value := os.Getenv("XDG_CONFIG_HOME"); value != "" {
		return filepath.Join(value, "Cursor", "User")
	}
	return homePath(".config", "Cursor", "User")
}
