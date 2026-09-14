package archive

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/regutierrez/traicr/internal/domain"
)

type Input struct {
	Descriptor domain.Descriptor
	Directory  string
}

// Only source names and bytes define a revision, never collection time or machine.
func RevisionDigest(files []domain.File) string {
	ordered := slices.Clone(files)
	slices.SortFunc(ordered, func(a, b domain.File) int { return strings.Compare(a.Path, b.Path) })
	hash := sha256.New()
	for _, file := range ordered {
		fmt.Fprintf(hash, "%s\x00%s\x00", file.Path, file.SHA256)
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}

func Describe(ctx context.Context, input Input) (domain.Descriptor, error) {
	descriptor := input.Descriptor
	descriptor.Files = nil
	err := filepath.WalkDir(input.Directory, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("source contains a non-regular file: %s", entry.Name())
		}
		relative, err := filepath.Rel(input.Directory, name)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if !safePath(relative, 32) || !strings.HasPrefix(relative, "source/") {
			return fmt.Errorf("source path must be under source/: %s", relative)
		}
		file, err := os.Open(name)
		if err != nil {
			return err
		}
		hash := sha256.New()
		size, copyErr := io.Copy(hash, contextReader{ctx, io.LimitReader(file, info.Size()+1)})
		closeErr := file.Close()
		if err := errors.Join(copyErr, closeErr); err != nil {
			return err
		}
		if size != info.Size() {
			return fmt.Errorf("source changed while hashing: %s", relative)
		}
		descriptor.Files = append(descriptor.Files, domain.File{Path: relative, Size: size, SHA256: hex.EncodeToString(hash.Sum(nil))})
		return nil
	})
	if err != nil {
		return descriptor, err
	}
	if len(descriptor.Files) == 0 {
		return descriptor, errors.New("trace has no source files")
	}
	descriptor.RevisionDigest = RevisionDigest(descriptor.Files)
	return descriptor, nil
}

func Write(ctx context.Context, outputDir string, manifest domain.Manifest, inputs []Input, splitBytes int64) ([]string, error) {
	if splitBytes <= 0 {
		splitBytes = 2_000_000_000
	}
	if err := os.MkdirAll(outputDir, 0o700); err != nil {
		return nil, err
	}
	manifest.FormatVersion = domain.FormatVersion
	if manifest.CreatedAt == "" {
		manifest.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	inputs = slices.Clone(inputs)
	for i := range inputs {
		descriptor, err := Describe(ctx, inputs[i])
		if err != nil {
			return nil, err
		}
		descriptor.Path = fmt.Sprintf("traces/%06d", i+1)
		inputs[i].Descriptor = descriptor
	}
	var outputs []string
	for first := 0; first < len(inputs); {
		last := first
		var size int64
		manifest.Traces = nil
		for last < len(inputs) {
			var traceSize int64
			for _, file := range inputs[last].Descriptor.Files {
				traceSize += file.Size
			}
			if last > first && traceSize > splitBytes-size {
				break
			}
			size += traceSize
			manifest.Traces = append(manifest.Traces, inputs[last].Descriptor)
			last++
		}
		name, err := writeZIP(ctx, outputDir, manifest, inputs[first:last], len(outputs)+1)
		if err != nil {
			return outputs, err
		}
		outputs = append(outputs, name)
		first = last
	}
	return outputs, nil
}

func writeZIP(ctx context.Context, outputDir string, manifest domain.Manifest, inputs []Input, number int) (string, error) {
	file, err := os.CreateTemp(outputDir, ".traicr-*.zip")
	if err != nil {
		return "", err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	writer := zip.NewWriter(file)
	metadata := map[string]any{"manifest.json": manifest}
	for _, input := range inputs {
		metadata[input.Descriptor.Path+"/descriptor.json"] = input.Descriptor
	}
	for name, value := range metadata {
		entry, err := writer.Create(name)
		if err != nil {
			return "", err
		}
		if err := json.NewEncoder(entry).Encode(value); err != nil {
			return "", err
		}
	}
	for _, input := range inputs {
		root, err := os.OpenRoot(input.Directory)
		if err != nil {
			return "", err
		}
		err = writeSource(ctx, writer, root, input.Descriptor)
		root.Close()
		if err != nil {
			return "", err
		}
	}
	if err := writer.Close(); err != nil {
		return "", err
	}
	if err := file.Sync(); err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	// A random suffix prevents two collections in the same second overwriting exports.
	name := filepath.Join(outputDir, fmt.Sprintf("traicr-%s-%s-%03d.zip", time.Now().UTC().Format("20060102T150405Z"), strings.TrimSuffix(strings.TrimPrefix(filepath.Base(file.Name()), ".traicr-"), ".zip"), number))
	if err := os.Rename(file.Name(), name); err != nil {
		return "", err
	}
	return name, nil
}

func writeSource(ctx context.Context, writer *zip.Writer, root *os.Root, descriptor domain.Descriptor) error {
	for _, source := range descriptor.Files {
		if err := ctx.Err(); err != nil {
			return err
		}
		file, err := root.Open(source.Path)
		if err != nil {
			return err
		}
		entry, err := writer.Create(descriptor.Path + "/" + source.Path)
		if err != nil {
			file.Close()
			return err
		}
		hash := sha256.New()
		size, copyErr := io.Copy(io.MultiWriter(entry, hash), contextReader{ctx, io.LimitReader(file, source.Size+1)})
		closeErr := file.Close()
		if err := errors.Join(copyErr, closeErr); err != nil {
			return err
		}
		if size != source.Size || hex.EncodeToString(hash.Sum(nil)) != source.SHA256 {
			return fmt.Errorf("source changed while writing archive: %s", source.Path)
		}
	}
	return nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}
