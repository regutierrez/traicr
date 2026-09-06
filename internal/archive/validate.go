package archive

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"reflect"
	"strings"

	"github.com/regutierrez/traicr/internal/domain"
)

type Limits struct {
	ArchiveBytes  int64
	ExpandedBytes int64
	FileBytes     int64
	Files         int
	Depth         int
	MetadataBytes int64
}

func DefaultLimits() Limits {
	return Limits{ArchiveBytes: 8 << 30, ExpandedBytes: 32 << 30, FileBytes: 4 << 30, Files: 100_000, Depth: 32, MetadataBytes: 16 << 20}
}

type Validated struct {
	Manifest domain.Manifest
	ZIP      *zip.Reader
}

func Validate(ctx context.Context, reader io.ReaderAt, size int64, limits Limits) (*Validated, error) {
	if limits.ArchiveBytes <= 0 || limits.ExpandedBytes <= 0 || limits.FileBytes <= 0 || limits.Files <= 0 || limits.Depth <= 0 || limits.MetadataBytes <= 0 {
		return nil, errors.New("archive limits must be positive")
	}
	if size <= 0 || size > limits.ArchiveBytes {
		return nil, errors.New("archive size exceeds allowed range")
	}
	if err := checkDirectorySize(ctx, reader, size, limits.Files); err != nil {
		return nil, err
	}
	zipped, err := zip.NewReader(reader, size)
	if err != nil {
		return nil, fmt.Errorf("invalid ZIP: %w", err)
	}
	if len(zipped.File) > limits.Files {
		return nil, errors.New("archive contains too many files")
	}
	files := make(map[string]*zip.File, len(zipped.File))
	var expanded uint64
	for _, file := range zipped.File {
		if !safePath(file.Name, limits.Depth) || !file.Mode().IsRegular() {
			return nil, errors.New("archive contains an unsafe path or non-regular file")
		}
		if _, exists := files[file.Name]; exists {
			return nil, errors.New("archive contains a duplicate path")
		}
		if file.UncompressedSize64 > uint64(limits.FileBytes) || file.UncompressedSize64 > uint64(limits.ExpandedBytes)-expanded {
			return nil, errors.New("archive expansion limit exceeded")
		}
		expanded += file.UncompressedSize64
		files[file.Name] = file
	}
	var manifest domain.Manifest
	if err := readMetadata(files["manifest.json"], limits.MetadataBytes, &manifest); err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}
	if manifest.FormatVersion != domain.FormatVersion || manifest.SourceMachine.ID == "" || manifest.SourceMachine.Hostname == "" || len(manifest.Traces) == 0 {
		return nil, errors.New("unsupported or incomplete manifest")
	}
	used := map[string]bool{"manifest.json": true}
	identities := map[string]bool{}
	for _, descriptor := range manifest.Traces {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := validateTrace(ctx, descriptor, files, used, limits); err != nil {
			return nil, err
		}
		key := descriptor.Harness + "\x00" + descriptor.NativeTraceID
		if identities[key] {
			return nil, errors.New("manifest contains a duplicate trace")
		}
		identities[key] = true
	}
	if len(used) != len(files) {
		return nil, errors.New("archive contains unlisted files")
	}
	return &Validated{Manifest: manifest, ZIP: zipped}, nil
}

func validateTrace(ctx context.Context, descriptor domain.Descriptor, files map[string]*zip.File, used map[string]bool, limits Limits) error {
	if !safePath(descriptor.Path, 2) || !strings.HasPrefix(descriptor.Path, "traces/") || descriptor.Harness == "" || descriptor.NativeTraceID == "" || len(descriptor.Files) == 0 {
		return errors.New("incomplete trace descriptor")
	}
	name := descriptor.Path + "/descriptor.json"
	if used[name] {
		return errors.New("duplicate trace path")
	}
	var copy domain.Descriptor
	if err := readMetadata(files[name], limits.MetadataBytes, &copy); err != nil {
		return fmt.Errorf("trace descriptor: %w", err)
	}
	if !reflect.DeepEqual(copy, descriptor) || descriptor.RevisionDigest != RevisionDigest(descriptor.Files) {
		return errors.New("trace descriptor or revision digest mismatch")
	}
	used[name] = true
	for _, source := range descriptor.Files {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !safePath(source.Path, limits.Depth-2) || !strings.HasPrefix(source.Path, "source/") || source.Size < 0 || source.Size > limits.FileBytes {
			return errors.New("invalid source file descriptor")
		}
		name := descriptor.Path + "/" + source.Path
		file := files[name]
		if file == nil || used[name] || file.UncompressedSize64 != uint64(source.Size) {
			return errors.New("missing, duplicate, or size-mismatched source file")
		}
		content, err := file.Open()
		if err != nil {
			return err
		}
		hash := sha256.New()
		size, copyErr := io.Copy(hash, contextReader{ctx, io.LimitReader(content, source.Size+1)})
		closeErr := content.Close()
		if err := errors.Join(copyErr, closeErr); err != nil {
			return fmt.Errorf("corrupt source file: %w", err)
		}
		if size != source.Size || hex.EncodeToString(hash.Sum(nil)) != source.SHA256 {
			return errors.New("source digest mismatch")
		}
		used[name] = true
	}
	return nil
}

func safePath(name string, depth int) bool {
	return fs.ValidPath(name) && name != "." && !strings.ContainsAny(name, "\\:\x00") && strings.Count(name, "/") < depth
}

func readMetadata(file *zip.File, limit int64, target any) error {
	if file == nil {
		return errors.New("file missing")
	}
	if file.UncompressedSize64 > uint64(limit) {
		return errors.New("metadata limit exceeded")
	}
	reader, err := file.Open()
	if err != nil {
		return err
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > limit {
		return errors.New("metadata limit exceeded")
	}
	return json.Unmarshal(data, target)
}

// Count actual headers before archive/zip allocates entries; the advertised count can lie.
func checkDirectorySize(ctx context.Context, reader io.ReaderAt, size int64, limit int) error {
	length := min(size, 65_557)
	tail := make([]byte, length)
	if _, err := reader.ReadAt(tail, size-length); err != nil {
		return err
	}
	for end := len(tail) - 22; end >= 0; end-- {
		if !bytes.Equal(tail[end:end+4], []byte{'P', 'K', 5, 6}) || end+22+int(binary.LittleEndian.Uint16(tail[end+20:])) != len(tail) {
			continue
		}
		count := uint64(binary.LittleEndian.Uint16(tail[end+10:]))
		directorySize := uint64(binary.LittleEndian.Uint32(tail[end+12:]))
		directoryOffset := uint64(binary.LittleEndian.Uint32(tail[end+16:]))
		if count == 0xffff || directorySize == 0xffffffff || directoryOffset == 0xffffffff {
			locator := make([]byte, 20)
			if _, err := reader.ReadAt(locator, size-length+int64(end)-20); err != nil || !bytes.Equal(locator[:4], []byte{'P', 'K', 6, 7}) {
				return errors.New("invalid ZIP64 directory")
			}
			offset := binary.LittleEndian.Uint64(locator[8:])
			if size < 56 || offset > uint64(size-56) {
				return errors.New("invalid ZIP64 offset")
			}
			header := make([]byte, 56)
			if _, err := reader.ReadAt(header, int64(offset)); err != nil || !bytes.Equal(header[:4], []byte{'P', 'K', 6, 6}) {
				return errors.New("invalid ZIP64 header")
			}
			count = binary.LittleEndian.Uint64(header[32:])
			directorySize = binary.LittleEndian.Uint64(header[40:])
			directoryOffset = binary.LittleEndian.Uint64(header[48:])
		}
		if count > uint64(limit) {
			return errors.New("archive contains too many files")
		}
		if directoryOffset > uint64(size) || directorySize > uint64(size)-directoryOffset {
			return errors.New("invalid ZIP directory bounds")
		}
		directoryEnd := directoryOffset + directorySize
		var actual uint64
		var header [46]byte
		for directoryOffset < directoryEnd {
			if err := ctx.Err(); err != nil {
				return err
			}
			if actual >= uint64(limit) {
				return errors.New("archive contains too many files")
			}
			if directoryEnd-directoryOffset < 46 {
				return errors.New("truncated ZIP directory")
			}
			if _, err := reader.ReadAt(header[:], int64(directoryOffset)); err != nil {
				return err
			}
			if !bytes.Equal(header[:4], []byte{'P', 'K', 1, 2}) {
				return errors.New("invalid ZIP directory entry")
			}
			directoryOffset += 46 + uint64(binary.LittleEndian.Uint16(header[28:])) + uint64(binary.LittleEndian.Uint16(header[30:])) + uint64(binary.LittleEndian.Uint16(header[32:]))
			actual++
		}
		if actual != count || directoryOffset != directoryEnd {
			return errors.New("ZIP directory count or size mismatch")
		}
		return nil
	}
	return errors.New("ZIP end directory missing")
}
