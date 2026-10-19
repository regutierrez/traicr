package store

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Status is the archive health summary behind GET /api/v1/status.
type Status struct {
	RunningImportID int64           `json:"running_import_id,omitempty"`
	Machines        []MachineStatus `json:"machines"`
	Imports         ImportCounts    `json:"imports"`
	Traces          int64           `json:"traces"`
	Events          int64           `json:"events"`
	Titles          TitleStatus     `json:"titles"`
	Storage         StorageStatus   `json:"storage"`
}

// MachineStatus describes a Source Machine's newest import.
type MachineStatus struct {
	ID                     string `json:"id"`
	Hostname               string `json:"hostname"`
	LastImportAt           string `json:"last_import_at"`
	LastImportID           int64  `json:"last_import_id"`
	LastImportComplete     bool   `json:"last_import_complete"`
	LastImportFailedTraces int    `json:"last_import_failed_traces"`
}

type ImportCounts struct {
	Total            int64 `json:"total"`
	WithFailedTraces int64 `json:"with_failed_traces"`
	// Incomplete counts imports that started but never stored a report, except the running one.
	Incomplete int64 `json:"incomplete"`
}

type TitleStatus struct {
	Enabled bool `json:"enabled"`
	// Pending counts traces with no manual or collected title whose latest revision is not yet named.
	Pending int64 `json:"pending"`
}

type StorageStatus struct {
	DatabaseBytes int64 `json:"database_bytes"`
	ObjectsBytes  int64 `json:"objects_bytes"`
}

// failedTraces reads the failed count from a stored report. The JSON text is
// stored as a BLOB, so the cast keeps SQLite from treating it as JSONB.
const failedTraces = "COALESCE(json_extract(CAST(i.report_json AS TEXT),'$.failed'),0)"

func (s *Store) Status(ctx context.Context) (Status, error) {
	status := Status{Machines: []MachineStatus{}}
	rows, err := s.db.QueryContext(ctx, `SELECT m.id,m.hostname,i.created_at,i.id,i.report_json IS NOT NULL,`+failedTraces+`
		FROM (SELECT machine_id,MAX(id) AS id FROM imports GROUP BY machine_id) latest
		JOIN imports i ON i.id=latest.id JOIN source_machines m ON m.id=latest.machine_id
		ORDER BY i.id DESC`)
	if err != nil {
		return Status{}, err
	}
	for rows.Next() {
		var machine MachineStatus
		if err := rows.Scan(&machine.ID, &machine.Hostname, &machine.LastImportAt, &machine.LastImportID, &machine.LastImportComplete, &machine.LastImportFailedTraces); err != nil {
			rows.Close()
			return Status{}, err
		}
		status.Machines = append(status.Machines, machine)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return Status{}, err
	}
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(`+failedTraces+`>0),0) FROM imports i`).Scan(&status.Imports.Total, &status.Imports.WithFailedTraces)
	if err != nil {
		return Status{}, err
	}
	// Read the running import on both sides of the query, so an import that
	// starts or finishes meanwhile is never counted as interrupted.
	before := s.activeImport.Load()
	unfinished, err := s.unfinishedImports(ctx)
	if err != nil {
		return Status{}, err
	}
	after := s.activeImport.Load()
	for _, id := range unfinished {
		if id != before && id != after {
			status.Imports.Incomplete++
		}
	}
	status.RunningImportID = after
	err = s.db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM traces),(SELECT COUNT(*) FROM events),
		(SELECT COUNT(*) FROM traces t WHERE t.title_override='' AND t.title=''
			AND (SELECT MAX(r.id) FROM trace_revisions r WHERE r.trace_id=t.id)>t.generated_title_revision_id),
		(SELECT COALESCE(SUM(size),0) FROM source_objects)`).Scan(&status.Traces, &status.Events, &status.Titles.Pending, &status.Storage.ObjectsBytes)
	if err != nil {
		return Status{}, err
	}
	for _, name := range []string{"traicr.db", "traicr.db-wal"} {
		info, err := os.Stat(filepath.Join(s.dataDir, name))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return Status{}, err
		}
		status.Storage.DatabaseBytes += info.Size()
	}
	return status, nil
}

// unfinishedImports lists imports with no stored report: running or interrupted.
func (s *Store) unfinishedImports(ctx context.Context) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id FROM imports WHERE report_json IS NULL")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
