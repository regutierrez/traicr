package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/regutierrez/traicr/internal/archive"
	"github.com/regutierrez/traicr/internal/domain"
)

type grokAdapter struct{}

func (grokAdapter) Name() string { return "grok-build" }

func grokRoots() []string {
	home := os.Getenv("GROK_HOME")
	if home == "" {
		home = homePath(".grok")
	}
	return []string{filepath.Join(home, "sessions")}
}

func (grokAdapter) Discover(_ context.Context, configured []string) Source {
	roots := chooseRoots(configured, grokRoots())
	dirs, warnings := findGrokSessions(roots)
	return Source{Harness: "grok-build", Location: strings.Join(roots, string(os.PathListSeparator)), Traces: len(dirs), Warnings: warnings}
}

func (grokAdapter) Collect(ctx context.Context, configured []string) (Result, error) {
	roots := chooseRoots(configured, grokRoots())
	dirs, warnings := findGrokSessions(roots)
	base, err := os.MkdirTemp("", "traicr-grok-*")
	if err != nil {
		return Result{}, err
	}
	result := Result{Warnings: warnings, Cleanup: func() { os.RemoveAll(base) }}
	for index, source := range dirs {
		if err := ctx.Err(); err != nil {
			result.Cleanup()
			return Result{}, err
		}
		destination := filepath.Join(base, fmt.Sprintf("%06d", index+1), "source")
		if err := os.MkdirAll(destination, 0o700); err != nil {
			result.Cleanup()
			return Result{}, err
		}
		descriptor := domain.Descriptor{Harness: "grok-build", Adapter: "grok-native-session", NativeTraceID: filepath.Base(source)}
		if err := copySessionDirectory(ctx, source, destination, &descriptor); err != nil {
			result.Warnings = append(result.Warnings, warning("unreadable_source", source+": "+err.Error()))
			continue
		}
		if info, statErr := os.Stat(source); statErr == nil {
			descriptor.NativeUpdatedAt = info.ModTime().UTC().Format(time.RFC3339Nano)
		}
		summary, readErr := os.ReadFile(filepath.Join(destination, "summary.json"))
		if readErr == nil {
			if !json.Valid(summary) {
				descriptor.Warnings = append(descriptor.Warnings, warning("unknown_schema", "summary.json is not valid JSON; bytes were preserved"))
			} else {
				var metadata struct {
					ID        string `json:"id"`
					SessionID string `json:"sessionId"`
				}
				if json.Unmarshal(summary, &metadata) == nil {
					if metadata.SessionID != "" {
						descriptor.NativeTraceID = metadata.SessionID
					} else if metadata.ID != "" {
						descriptor.NativeTraceID = metadata.ID
					}
				}
				populateRepository(&descriptor, summary)
			}
		}
		// Grok's Markdown export drops native structure, so the complete session is the durable source record.
		descriptor.Warnings = append(descriptor.Warnings, warning("native_snapshot", "Grok Markdown export is lossy; preserved summary.json, updates.jsonl, and the complete native session directory"))
		result.Inputs = append(result.Inputs, archive.Input{Descriptor: descriptor, Directory: filepath.Dir(destination)})
	}
	return result, nil
}

func findGrokSessions(roots []string) ([]string, []domain.Warning) {
	var dirs []string
	var warnings []domain.Warning
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			warnings = append(warnings, warning("unreadable_source", root+": "+err.Error()))
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			dir := filepath.Join(root, entry.Name())
			_, summaryErr := os.Stat(filepath.Join(dir, "summary.json"))
			_, updatesErr := os.Stat(filepath.Join(dir, "updates.jsonl"))
			if summaryErr == nil || updatesErr == nil {
				dirs = append(dirs, dir)
			}
		}
	}
	sort.Strings(dirs)
	return dirs, warnings
}

func copySessionDirectory(ctx context.Context, source, destination string, descriptor *domain.Descriptor) error {
	before, err := directoryStamp(source)
	if err != nil {
		return err
	}
	err = filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil || relative == "." {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if entry.Type().IsRegular() {
			if strings.HasSuffix(strings.ToLower(entry.Name()), ".jsonl") {
				complete, changed, invalid, _, err := snapshotCompleteLines(ctx, path, target)
				if !complete {
					descriptor.Warnings = append(descriptor.Warnings, warning("partial_tail", "ignored an incomplete final JSONL record in "+path))
				}
				if changed {
					descriptor.Warnings = append(descriptor.Warnings, warning("live_source_changed", "native session file changed while it was being copied: "+path))
				}
				if invalid {
					descriptor.Warnings = append(descriptor.Warnings, warning("unknown_schema", "native JSONL contains a record that is not valid JSON; bytes were preserved"))
				}
				return err
			}
			return copyFile(ctx, path, target)
		}
		descriptor.Warnings = append(descriptor.Warnings, warning("unsupported_file", "skipped non-regular native session entry "+path))
		return nil
	})
	if err != nil {
		return err
	}
	after, err := directoryStamp(source)
	if err != nil || before != after {
		descriptor.Warnings = append(descriptor.Warnings, warning("live_source_changed", "native session changed while it was being copied: "+source))
	}
	return nil
}

func directoryStamp(root string) (string, error) {
	var stamp strings.Builder
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		fmt.Fprintf(&stamp, "%s:%d:%d\n", path, info.Size(), info.ModTime().UnixNano())
		return nil
	})
	return stamp.String(), err
}
