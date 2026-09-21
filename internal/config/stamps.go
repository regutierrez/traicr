package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const stampsFile = "source_stamps.json"

// SourceStamp is the last observed local file identity for a collected trace.
// It is not Collection State: skip only when Stamp matches and Digest is acknowledged.
type SourceStamp struct {
	Stamp    string `json:"stamp"`
	Digest   string `json:"digest"`
	Harness  string `json:"harness"`
	NativeID string `json:"native_trace_id"`
}

func LoadSourceStamps(dir string) (map[string]SourceStamp, error) {
	if dir == "" {
		return map[string]SourceStamp{}, nil
	}
	data, err := os.ReadFile(filepath.Join(dir, stampsFile))
	if errors.Is(err, os.ErrNotExist) {
		return map[string]SourceStamp{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read source stamps: %w", err)
	}
	var stamps map[string]SourceStamp
	if err := json.Unmarshal(data, &stamps); err != nil {
		return nil, fmt.Errorf("parse source stamps: %w", err)
	}
	if stamps == nil {
		stamps = map[string]SourceStamp{}
	}
	return stamps, nil
}

func SaveSourceStamps(dir string, stamps map[string]SourceStamp) error {
	if dir == "" {
		return nil
	}
	if stamps == nil {
		stamps = map[string]SourceStamp{}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create source stamp directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("protect source stamp directory: %w", err)
	}
	data, err := json.MarshalIndent(stamps, "", "  ")
	if err != nil {
		return fmt.Errorf("encode source stamps: %w", err)
	}
	data = append(data, '\n')
	path := filepath.Join(dir, stampsFile)
	tmp, err := os.CreateTemp(dir, ".source-stamps-*")
	if err != nil {
		return fmt.Errorf("create source stamps: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("protect source stamps: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write source stamps: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync source stamps: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close source stamps: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace source stamps: %w", err)
	}
	return os.Chmod(path, 0o600)
}
