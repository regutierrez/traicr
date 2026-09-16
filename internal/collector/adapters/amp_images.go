package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/regutierrez/traicr/internal/amp"
	"github.com/regutierrez/traicr/internal/domain"
)

func collectAmpImages(ctx context.Context, executable, dir string) []domain.Warning {
	file, err := os.Open(filepath.Join(dir, "source/export.json"))
	if err != nil {
		return []domain.Warning{warning("attachment_scan_failed", err.Error())}
	}
	var export any
	err = json.NewDecoder(io.LimitReader(file, domain.MaxAmpExportBytes)).Decode(&export)
	file.Close()
	if err != nil {
		return []domain.Warning{warning("attachment_scan_failed", "Amp images could not be read from the retained export: "+err.Error())}
	}
	var warnings []domain.Warning
	seen := map[string]bool{}
	remaining := int64(domain.MaxAmpExportBytes)
	for _, attachment := range amp.Attachments(export, "") {
		url, path := amp.AttachmentLocation(attachment.URL)
		if path == "" || seen[path] || attachment.Inline {
			continue
		}
		seen[path] = true
		if ctx.Err() != nil {
			break
		}
		size, err := downloadAmpImage(ctx, executable, url, filepath.Join(dir, filepath.FromSlash(path)), remaining)
		if err != nil {
			warnings = append(warnings, warning("attachment_download_failed", fmt.Sprintf("%s: Amp image was not archived: %v", attachment.SourcePointer, err)))
			continue
		}
		remaining -= size
	}
	return warnings
}

func downloadAmpImage(ctx context.Context, executable, url, path string, limit int64) (int64, error) {
	if limit <= 0 {
		return 0, errors.New("512 MiB image budget exhausted for this trace")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return 0, err
	}
	args := []string{"-c", `ulimit -f "$1" || exit; shift; exec "$@"`, "traicr-amp-image", strconv.FormatInt((limit+511)/512, 10), executable, "files", "get", url, "-o", path}
	_, err := runOutput(ctx, "sh", args, 1<<20)
	if err == nil {
		var info os.FileInfo
		info, err = os.Stat(path)
		if err == nil {
			if info.Size() > limit {
				err = errors.New("download exceeds the remaining image budget")
			} else if info.Size() == 0 {
				err = errors.New("download is empty")
			} else {
				return info.Size(), nil
			}
		}
	}
	os.Remove(path)
	return 0, err
}
