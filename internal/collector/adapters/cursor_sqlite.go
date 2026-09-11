package adapters

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/regutierrez/traicr/internal/archive"
	"github.com/regutierrez/traicr/internal/domain"
	_ "modernc.org/sqlite"
)

type cursorEditorAdapter struct{}

type cursorRow struct {
	Table string `json:"table"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

type cursorTrace struct {
	ID   string
	Rows []cursorRow
}

func (cursorEditorAdapter) Name() string { return "cursor" }

func cursorDatabaseRoots() []string {
	root := cursorConfigRoot()
	return []string{filepath.Join(root, "globalStorage"), filepath.Join(root, "workspaceStorage")}
}

func (cursorEditorAdapter) Discover(_ context.Context, configured []string) Source {
	roots := chooseRoots(configured, cursorDatabaseRoots())
	files, warnings := findFiles(roots, ".vscdb", ".sqlite", ".db")
	return Source{Harness: "cursor", Location: strings.Join(roots, string(os.PathListSeparator)), Traces: len(files), Warnings: warnings}
}

func (cursorEditorAdapter) Collect(ctx context.Context, configured []string, progress Progress) (Result, error) {
	roots := chooseRoots(configured, cursorDatabaseRoots())
	files, warnings := findFiles(roots, ".vscdb", ".sqlite", ".db")
	base, err := os.MkdirTemp("", "traicr-cursor-*")
	if err != nil {
		return Result{}, err
	}
	result := Result{Warnings: warnings, Cleanup: func() { os.RemoveAll(base) }}
	inputIndex := 0
	for _, path := range files {
		rows, rowWarnings, err := readCursorRows(ctx, path)
		result.Warnings = append(result.Warnings, rowWarnings...)
		if err != nil {
			result.Warnings = append(result.Warnings, warning("sqlite_read_failed", path+": "+err.Error()))
			continue
		}
		info, _ := os.Stat(path)
		for _, trace := range groupCursorRows(rows) {
			inputIndex++
			progress.report(inputIndex, 0)
			dir := filepath.Join(base, fmt.Sprintf("%06d", inputIndex))
			if err := os.MkdirAll(filepath.Join(dir, "source"), 0o700); err != nil {
				result.Cleanup()
				return Result{}, err
			}
			output, err := os.OpenFile(filepath.Join(dir, "source", "rows.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
			if err != nil {
				result.Cleanup()
				return Result{}, err
			}
			encoder := json.NewEncoder(output)
			for _, row := range trace.Rows {
				if err := encoder.Encode(row); err != nil {
					output.Close()
					result.Cleanup()
					return Result{}, err
				}
			}
			if err := output.Close(); err != nil {
				result.Cleanup()
				return Result{}, err
			}
			descriptor := domain.Descriptor{Harness: "cursor", Adapter: "cursor-sqlite-rows", NativeTraceID: trace.ID}
			if info != nil {
				descriptor.NativeUpdatedAt = info.ModTime().UTC().Format(time.RFC3339Nano)
			}
			for _, row := range trace.Rows {
				if !json.Valid([]byte(row.Value)) {
					descriptor.Warnings = append(descriptor.Warnings, warning("unknown_schema", "Cursor row value is not JSON; original table, key, and value were preserved"))
					break
				}
			}
			result.Inputs = append(result.Inputs, archive.Input{Descriptor: descriptor, Directory: dir})
		}
	}
	return result, nil
}

func readCursorRows(ctx context.Context, path string) ([]cursorRow, []domain.Warning, error) {
	uri := &url.URL{Scheme: "file", Path: filepath.ToSlash(path), RawQuery: "mode=ro"}
	database, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, nil, err
	}
	defer database.Close()
	database.SetMaxOpenConns(1)
	// The read-only transaction pins one WAL-aware snapshot without copying or mutating Cursor's database.
	transaction, err := database.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, nil, err
	}
	defer transaction.Rollback()
	if _, err := transaction.ExecContext(ctx, "PRAGMA query_only=ON"); err != nil {
		return nil, nil, err
	}
	tables, err := cursorTables(ctx, transaction)
	if err != nil {
		return nil, nil, err
	}
	var result []cursorRow
	for _, table := range tables {
		query := `SELECT key, value FROM "` + table + `" WHERE lower(key) LIKE '%composer%' OR lower(key) LIKE '%chat%' OR lower(key) LIKE '%bubble%'`
		rows, err := transaction.QueryContext(ctx, query)
		if err != nil {
			return nil, nil, err
		}
		for rows.Next() {
			var key, value string
			if err := rows.Scan(&key, &value); err != nil {
				rows.Close()
				return nil, nil, err
			}
			result = append(result, cursorRow{Table: table, Key: key, Value: value})
		}
		if err := rows.Close(); err != nil {
			return nil, nil, err
		}
	}
	if err := transaction.Commit(); err != nil {
		return nil, nil, err
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Table == result[j].Table {
			return result[i].Key < result[j].Key
		}
		return result[i].Table < result[j].Table
	})
	if len(tables) == 0 {
		return result, []domain.Warning{warning("unknown_schema", path+" has no supported Cursor key/value table; no rows were treated as parsed")}, nil
	}
	return result, nil, nil
}

func cursorTables(ctx context.Context, transaction *sql.Tx) ([]string, error) {
	rows, err := transaction.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name IN ('ItemTable', 'cursorDiskKV') ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	return tables, rows.Err()
}

func groupCursorRows(rows []cursorRow) []cursorTrace {
	var traces []cursorTrace
	for _, row := range rows {
		lowerKey := strings.ToLower(row.Key)
		if strings.Contains(lowerKey, "composerdata") {
			trace := cursorTrace{ID: cursorRowID(row), Rows: []cursorRow{row}}
			ids := referencedBubbleIDs(row.Value)
			for _, candidate := range rows {
				if !strings.Contains(strings.ToLower(candidate.Key), "bubble") {
					continue
				}
				for id := range ids {
					if cursorBubbleKeyMatches(candidate.Key, id) {
						trace.Rows = append(trace.Rows, candidate)
						break
					}
				}
			}
			traces = append(traces, trace)
			continue
		}
		if strings.Contains(lowerKey, "chatdata") {
			traces = append(traces, cursorTrace{ID: cursorRowID(row), Rows: []cursorRow{row}})
		}
	}
	return traces
}

func cursorRowID(row cursorRow) string {
	var composer struct {
		ID         string `json:"id"`
		ComposerID string `json:"composerId"`
	}
	if json.Unmarshal([]byte(row.Value), &composer) == nil {
		if composer.ComposerID != "" {
			return composer.ComposerID
		}
		if composer.ID != "" {
			return composer.ID
		}
	}
	if _, suffix, ok := strings.Cut(row.Key, ":"); ok && suffix != "" {
		return suffix
	}
	return row.Table + ":" + row.Key
}

func cursorBubbleKeyMatches(key, id string) bool {
	return key == id || strings.HasSuffix(key, ":"+id) || strings.HasSuffix(key, "/"+id)
}

func referencedBubbleIDs(value string) map[string]bool {
	var decoded any
	if json.Unmarshal([]byte(value), &decoded) != nil {
		return nil
	}
	ids := map[string]bool{}
	var visit func(any)
	visit = func(current any) {
		switch value := current.(type) {
		case map[string]any:
			for key, child := range value {
				if strings.EqualFold(key, "bubbleId") || strings.EqualFold(key, "bubbleID") {
					if id, ok := child.(string); ok && id != "" {
						ids[id] = true
					}
				}
				visit(child)
			}
		case []any:
			for _, child := range value {
				visit(child)
			}
		}
	}
	visit(decoded)
	return ids
}
