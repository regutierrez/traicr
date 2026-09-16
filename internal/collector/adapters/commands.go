package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/regutierrez/traicr/internal/archive"
	"github.com/regutierrez/traicr/internal/domain"
)

const (
	commandTimeout = 30 * time.Second
	listLimit      = 500
	maxListBytes   = 32 << 20
	maxExportBytes = 512 << 20
)

type commandAdapter struct {
	name       string
	format     string
	executable string
}

type commandTrace struct {
	ID         string
	Title      string
	UpdatedAt  string
	ParentID   string
	CWD        string
	Repository domain.Repository
}

func (a commandAdapter) Name() string { return a.name }

func (a commandAdapter) Discover(ctx context.Context, _ []string) Source {
	traces, warnings := a.list(ctx)
	return Source{Harness: a.name, Location: a.executable + " CLI", Traces: len(traces), Warnings: warnings}
}

func (a commandAdapter) Collect(ctx context.Context, _ []string, progress Progress) (Result, error) {
	traces, warnings := a.list(ctx)
	base, err := os.MkdirTemp("", "traicr-"+a.name+"-*")
	if err != nil {
		return Result{}, err
	}
	result := Result{Warnings: warnings, Cleanup: func() { os.RemoveAll(base) }}
	for index, trace := range traces {
		progress.report(index+1, len(traces))
		dir := filepath.Join(base, fmt.Sprintf("%06d", index+1))
		if err := os.MkdirAll(filepath.Join(dir, "source"), 0o700); err != nil {
			result.Cleanup()
			return Result{}, err
		}
		path := filepath.Join(dir, "source", "export.json")
		args := []string{"export", trace.ID}
		if a.name == "amp" {
			args = []string{"threads", "export", trace.ID}
		}
		if err := runToFile(ctx, a.executable, args, path, maxExportBytes); err != nil {
			result.Warnings = append(result.Warnings, warning("export_failed", fmt.Sprintf("%s %s: %v", a.name, trace.ID, err)))
			os.RemoveAll(dir)
			continue
		}
		exported, inspectErr := inspectCommandExport(a.name, path, trace.ID)
		if inspectErr != nil {
			result.Warnings = append(result.Warnings, warning("unknown_schema", fmt.Sprintf("%s %s export has an unsupported schema; bytes were preserved: %v", a.name, trace.ID, inspectErr)))
		} else {
			if exported.Title != "" {
				trace.Title = exported.Title
			}
			if exported.UpdatedAt != "" {
				// Amp's list index lags the thread's own updatedAt by hours, so a mismatch
				// there reflects server indexing, not an edit during collection.
				if a.name != "amp" && trace.UpdatedAt != "" && trace.UpdatedAt != exported.UpdatedAt {
					result.Warnings = append(result.Warnings, warning("live_source_changed", fmt.Sprintf("%s %s changed between listing and export", a.name, trace.ID)))
				}
				trace.UpdatedAt = exported.UpdatedAt
			}
			trace.ParentID = exported.ParentID
			trace.CWD = exported.CWD
			trace.Repository = exported.Repository
		}
		descriptor := domain.Descriptor{
			Harness:             a.name,
			Adapter:             a.format,
			NativeTraceID:       trace.ID,
			NativeUpdatedAt:     trace.UpdatedAt,
			ParentNativeTraceID: trace.ParentID,
			Title:               trace.Title,
			WorkingDirectory:    trace.CWD,
		}
		if trace.Repository != (domain.Repository{}) {
			descriptor.Repository = trace.Repository
		} else if trace.CWD != "" {
			root, remote := gitRepository(trace.CWD)
			descriptor.Repository = domain.Repository{Root: root, Remote: remote}
		}
		if a.name == "amp" {
			descriptor.Warnings = collectAmpImages(ctx, a.executable, dir)
			result.Warnings = append(result.Warnings, descriptor.Warnings...)
		}
		if err := ctx.Err(); err != nil {
			result.Cleanup()
			return Result{}, err
		}
		result.Inputs = append(result.Inputs, archive.Input{Descriptor: descriptor, Directory: dir})
	}
	return result, nil
}

func (a commandAdapter) list(ctx context.Context) ([]commandTrace, []domain.Warning) {
	if _, err := exec.LookPath(a.executable); err != nil {
		return nil, []domain.Warning{warning("missing_executable", a.executable+" is not installed or not on PATH")}
	}
	if a.name == "amp" {
		return a.listAmp(ctx)
	}
	data, err := runOutput(ctx, a.executable, []string{"session", "list", "--format", "json"}, maxListBytes)
	if err != nil {
		return nil, []domain.Warning{warning("command_failed", "opencode session list failed: "+err.Error())}
	}
	traces, err := parseOpenCodeList(data)
	if err != nil {
		return nil, []domain.Warning{warning("unknown_schema", "opencode session list returned an unsupported schema: "+err.Error())}
	}
	return traces, nil
}

// Amp orders offset pages inconsistently, so later pages can repeat earlier
// threads and skip others. Repeats are dropped here; skipped threads are picked
// up by a later collection once the listing settles.
func (a commandAdapter) listAmp(ctx context.Context) ([]commandTrace, []domain.Warning) {
	var traces []commandTrace
	seen := map[string]bool{}
	repeated := 0
	for offset := 0; ; offset += listLimit {
		args := []string{"threads", "list", "--json", "--include-archived", "--limit", strconv.Itoa(listLimit), "--offset", strconv.Itoa(offset)}
		data, err := runOutput(ctx, a.executable, args, maxListBytes)
		if err != nil {
			return traces, []domain.Warning{warning("command_failed", "amp threads list failed; check authentication and network access: "+err.Error())}
		}
		page, err := parseAmpList(data)
		if err != nil {
			return traces, []domain.Warning{warning("unknown_schema", "amp threads list returned an unsupported schema: "+err.Error())}
		}
		for _, trace := range page {
			if seen[trace.ID] {
				repeated++
				continue
			}
			seen[trace.ID] = true
			traces = append(traces, trace)
		}
		if len(page) < listLimit {
			if repeated > 0 {
				return traces, []domain.Warning{warning("unstable_listing", fmt.Sprintf("amp threads list pages overlapped; %d repeated entries were dropped and some threads may be missing until the next collection", repeated))}
			}
			return traces, nil
		}
	}
}

func parseAmpList(data []byte) ([]commandTrace, error) {
	var entries []struct {
		ID           string `json:"id"`
		Title        string `json:"title"`
		Updated      string `json:"updated"`
		Tree         string `json:"tree"`
		MessageCount int    `json:"messageCount"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	traces := make([]commandTrace, len(entries))
	for index, entry := range entries {
		if entry.ID == "" || entry.Updated == "" {
			return nil, errors.New("Amp list entry has no id or updated timestamp")
		}
		traces[index] = commandTrace{ID: entry.ID, Title: entry.Title, UpdatedAt: entry.Updated}
	}
	return traces, nil
}

func parseOpenCodeList(data []byte) ([]commandTrace, error) {
	var entries []struct {
		ID        string `json:"id"`
		Title     string `json:"title"`
		Updated   int64  `json:"updated"`
		Created   int64  `json:"created"`
		ProjectID string `json:"projectId"`
		Directory string `json:"directory"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	traces := make([]commandTrace, len(entries))
	for index, entry := range entries {
		if entry.ID == "" || entry.Updated <= 0 {
			return nil, errors.New("OpenCode list entry has no id or updated timestamp")
		}
		traces[index] = commandTrace{ID: entry.ID, Title: entry.Title, UpdatedAt: time.UnixMilli(entry.Updated).UTC().Format(time.RFC3339Nano), CWD: entry.Directory}
	}
	return traces, nil
}

func runOutput(parent context.Context, executable string, args []string, limit int64) ([]byte, error) {
	ctx, cancel := context.WithTimeout(parent, commandTimeout)
	defer cancel()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := exec.CommandContext(ctx, executable, args...)
	command.Stdout = &limitedWriter{writer: &stdout, remaining: limit}
	command.Stderr = &limitedWriter{writer: &stderr, remaining: 1 << 20}
	err := command.Run()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("timed out after %s", commandTimeout)
	}
	if err != nil {
		return nil, commandError(err, stderr.String())
	}
	return stdout.Bytes(), nil
}

func runToFile(parent context.Context, executable string, args []string, path string, limit int64) error {
	ctx, cancel := context.WithTimeout(parent, commandTimeout)
	defer cancel()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	command := exec.CommandContext(ctx, executable, args...)
	command.Stdout = &limitedWriter{writer: file, remaining: limit}
	command.Stderr = &limitedWriter{writer: &stderr, remaining: 1 << 20}
	runErr := command.Run()
	closeErr := file.Close()
	if ctx.Err() != nil {
		return fmt.Errorf("timed out after %s", commandTimeout)
	}
	if runErr != nil {
		return commandError(runErr, stderr.String())
	}
	return closeErr
}

type limitedWriter struct {
	writer    io.Writer
	remaining int64
}

func (writer *limitedWriter) Write(data []byte) (int, error) {
	if int64(len(data)) > writer.remaining {
		return 0, errors.New("command output exceeded safety limit")
	}
	written, err := writer.writer.Write(data)
	writer.remaining -= int64(written)
	return written, err
}

func commandError(err error, stderr string) error {
	stderr = strings.TrimSpace(stderr)
	if stderr == "" {
		return err
	}
	if len(stderr) > 500 {
		stderr = stderr[:500] + "…"
	}
	return fmt.Errorf("%w: %s", err, stderr)
}

func inspectCommandExport(harness, path, expectedID string) (commandTrace, error) {
	file, err := os.Open(path)
	if err != nil {
		return commandTrace{}, err
	}
	defer file.Close()
	if harness == "amp" {
		var export struct {
			Version   int        `json:"v"`
			ID        string     `json:"id"`
			Title     string     `json:"title"`
			UpdatedAt string     `json:"updatedAt"`
			Messages  []struct{} `json:"messages"`
			Env       struct {
				Initial struct {
					WorkingDirectory string `json:"workingDirectory"`
					Trees            []struct {
						URI        string `json:"uri"`
						Repository struct {
							URL string `json:"url"`
						} `json:"repository"`
					} `json:"trees"`
				} `json:"initial"`
			} `json:"env"`
		}
		decoder := json.NewDecoder(file)
		if err := decoder.Decode(&export); err != nil {
			return commandTrace{}, err
		}
		if decoder.Decode(&struct{}{}) != io.EOF {
			return commandTrace{}, errors.New("Amp export contains trailing data")
		}
		if export.Version <= 0 || export.ID != expectedID || export.UpdatedAt == "" || export.Messages == nil {
			return commandTrace{}, errors.New("Amp export is missing v, matching id, updatedAt, or messages")
		}
		repository := domain.Repository{}
		for _, tree := range export.Env.Initial.Trees {
			if tree.Repository.URL == "" {
				continue
			}
			repository.Remote = tree.Repository.URL
			if treeURI, err := url.Parse(tree.URI); err == nil && treeURI.Scheme == "file" {
				repository.Root = treeURI.Path
			}
			break
		}
		return commandTrace{ID: export.ID, Title: export.Title, UpdatedAt: export.UpdatedAt, CWD: export.Env.Initial.WorkingDirectory, Repository: repository}, nil
	}
	var export struct {
		Info struct {
			ID        string `json:"id"`
			Title     string `json:"title"`
			ParentID  string `json:"parentID"`
			Directory string `json:"directory"`
			Time      struct {
				Updated int64 `json:"updated"`
			} `json:"time"`
		} `json:"info"`
		Messages []struct{} `json:"messages"`
	}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&export); err != nil {
		return commandTrace{}, err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return commandTrace{}, errors.New("OpenCode export contains trailing data")
	}
	if export.Info.ID != expectedID || export.Info.Time.Updated <= 0 || export.Messages == nil {
		return commandTrace{}, errors.New("OpenCode export is missing matching info.id, info.time.updated, or messages")
	}
	return commandTrace{ID: export.Info.ID, Title: export.Info.Title, ParentID: export.Info.ParentID, CWD: export.Info.Directory, UpdatedAt: time.UnixMilli(export.Info.Time.Updated).UTC().Format(time.RFC3339Nano)}, nil
}
