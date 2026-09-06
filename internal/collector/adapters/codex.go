package adapters

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/regutierrez/traicr/internal/archive"
	"github.com/regutierrez/traicr/internal/domain"
)

type codexAdapter struct{}

func (codexAdapter) Name() string { return "codex" }

func (codexAdapter) Discover(ctx context.Context, configured []string) Source {
	if len(configured) == 0 {
		if _, err := exec.LookPath("codex"); err == nil {
			client, err := startCodex(ctx)
			if err == nil {
				defer client.close()
				threads, listErr := client.list()
				if listErr == nil {
					return Source{Harness: "codex", Location: "codex app-server", Traces: len(threads)}
				}
			}
		}
	}
	files, warnings := findFiles(chooseRoots(configured, codexRoots()), ".jsonl")
	warnings = append(warnings, warning("compatibility_fallback", "codex app-server was unavailable; raw rollout files will be preserved without resuming a thread"))
	return Source{Harness: "codex", Location: strings.Join(chooseRoots(configured, codexRoots()), string(os.PathListSeparator)), Traces: len(files), Warnings: warnings}
}

func (codexAdapter) Collect(ctx context.Context, configured []string) (Result, error) {
	if len(configured) == 0 {
		client, err := startCodex(ctx)
		if err == nil {
			result, collectErr := client.collect()
			client.close()
			if collectErr == nil {
				return result, nil
			}
		}
	}
	fallback := jsonlAdapter{name: "codex", format: "codex-rollout-jsonl", defaultRoots: codexRoots}
	result, err := fallback.Collect(ctx, configured)
	result.Warnings = append(result.Warnings, warning("compatibility_fallback", "preserved raw Codex rollout files because the read-only app-server protocol was unavailable"))
	return result, err
}

func codexRoots() []string {
	home := os.Getenv("CODEX_HOME")
	if home == "" {
		home = homePath(".codex")
	}
	return []string{filepath.Join(home, "sessions"), filepath.Join(home, "archived_sessions")}
}

type codexClient struct {
	command   *exec.Cmd
	stdin     io.WriteCloser
	responses chan []byte
	scanError chan error
	done      chan struct{}
	nextID    int
	cancel    context.CancelFunc
}

type codexThread struct {
	ID        string
	Title     string
	UpdatedAt string
	CWD       string
	ParentID  string
}

func startCodex(parent context.Context) (*codexClient, error) {
	if _, err := exec.LookPath("codex"); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	command := exec.CommandContext(ctx, "codex", "app-server")
	stdin, err := command.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	command.Stderr = io.Discard
	if err := command.Start(); err != nil {
		cancel()
		return nil, err
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64<<10), maxListBytes)
	client := &codexClient{command: command, stdin: stdin, responses: make(chan []byte), scanError: make(chan error, 1), done: make(chan struct{}), cancel: cancel}
	go func() {
		for scanner.Scan() {
			select {
			case client.responses <- append([]byte(nil), scanner.Bytes()...):
			case <-client.done:
				return
			}
		}
		select {
		case client.scanError <- scanner.Err():
		case <-client.done:
			return
		}
		close(client.responses)
	}()
	if _, err := client.request("initialize", map[string]any{"clientInfo": map[string]string{"name": "traicr", "version": "1"}, "capabilities": map[string]any{}}); err != nil {
		client.close()
		return nil, err
	}
	if err := client.notify("initialized", map[string]any{}); err != nil {
		client.close()
		return nil, err
	}
	return client, nil
}

func (client *codexClient) list() ([]codexThread, error) {
	var threads []codexThread
	var cursor string
	for {
		params := map[string]any{"limit": 100}
		if cursor != "" {
			params["cursor"] = cursor
		}
		result, err := client.request("thread/list", params)
		if err != nil {
			return nil, err
		}
		var page struct {
			Data []struct {
				ID             string  `json:"id"`
				Preview        string  `json:"preview"`
				Name           *string `json:"name"`
				CWD            string  `json:"cwd"`
				UpdatedAt      int64   `json:"updatedAt"`
				SessionID      string  `json:"sessionId"`
				ForkedFromID   *string `json:"forkedFromId"`
				ParentThreadID *string `json:"parentThreadId"`
			} `json:"data"`
			NextCursor *string `json:"nextCursor"`
		}
		if err := json.Unmarshal(result, &page); err != nil {
			return nil, err
		}
		if page.Data == nil {
			return nil, errors.New("thread/list result has no data array")
		}
		for _, item := range page.Data {
			if item.ID == "" || item.UpdatedAt <= 0 {
				return nil, errors.New("thread/list item has no id")
			}
			title := item.Preview
			if item.Name != nil && *item.Name != "" {
				title = *item.Name
			}
			parentID := ""
			if item.ParentThreadID != nil {
				parentID = *item.ParentThreadID
			} else if item.ForkedFromID != nil {
				parentID = *item.ForkedFromID
			}
			threads = append(threads, codexThread{ID: item.ID, Title: title, UpdatedAt: time.Unix(item.UpdatedAt, 0).UTC().Format(time.RFC3339Nano), CWD: item.CWD, ParentID: parentID})
		}
		if page.NextCursor == nil || *page.NextCursor == "" {
			return threads, nil
		}
		cursor = *page.NextCursor
	}
}

func (client *codexClient) collect() (Result, error) {
	threads, err := client.list()
	if err != nil {
		return Result{}, err
	}
	base, err := os.MkdirTemp("", "traicr-codex-*")
	if err != nil {
		return Result{}, err
	}
	result := Result{Cleanup: func() { os.RemoveAll(base) }}
	for index, thread := range threads {
		raw, err := client.request("thread/read", map[string]any{"threadId": thread.ID, "includeTurns": true})
		if err != nil {
			result.Warnings = append(result.Warnings, warning("export_failed", "codex thread/read "+thread.ID+": "+err.Error()))
			continue
		}
		dir := filepath.Join(base, fmt.Sprintf("%06d", index+1))
		if err := os.MkdirAll(filepath.Join(dir, "source"), 0o700); err != nil {
			result.Cleanup()
			return Result{}, err
		}
		if err := os.WriteFile(filepath.Join(dir, "source", "thread.json"), append(raw, '\n'), 0o600); err != nil {
			result.Cleanup()
			return Result{}, err
		}
		descriptor := domain.Descriptor{Harness: "codex", Adapter: "codex-app-server", NativeTraceID: thread.ID, NativeUpdatedAt: thread.UpdatedAt, ParentNativeTraceID: thread.ParentID, Title: thread.Title, WorkingDirectory: thread.CWD}
		if thread.CWD != "" {
			root, remote := gitRepository(thread.CWD)
			descriptor.Repository = domain.Repository{Root: root, Remote: remote}
		}
		result.Inputs = append(result.Inputs, archive.Input{Descriptor: descriptor, Directory: dir})
	}
	return result, nil
}

func (client *codexClient) request(method string, params any) (json.RawMessage, error) {
	client.nextID++
	id := client.nextID
	request := map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}
	if err := json.NewEncoder(client.stdin).Encode(request); err != nil {
		return nil, err
	}
	timer := time.NewTimer(commandTimeout)
	defer timer.Stop()
	for {
		var line []byte
		select {
		case line = <-client.responses:
			if line == nil {
				select {
				case err := <-client.scanError:
					if err != nil {
						return nil, err
					}
				default:
				}
				return nil, io.ErrUnexpectedEOF
			}
		case <-timer.C:
			return nil, fmt.Errorf("%s timed out after %s", method, commandTimeout)
		}
		var response struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  json.RawMessage `json:"error"`
		}
		if json.Unmarshal(line, &response) != nil || response.ID != id {
			continue
		}
		if len(response.Error) > 0 && string(response.Error) != "null" {
			return nil, fmt.Errorf("%s: %s", method, response.Error)
		}
		if len(response.Result) == 0 {
			return nil, fmt.Errorf("%s returned no result", method)
		}
		return append(json.RawMessage(nil), response.Result...), nil
	}
}

func (client *codexClient) notify(method string, params any) error {
	return json.NewEncoder(client.stdin).Encode(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

func (client *codexClient) close() {
	close(client.done)
	client.cancel()
	client.stdin.Close()
	if client.command.Process != nil {
		client.command.Process.Kill()
	}
	client.command.Wait()
}
