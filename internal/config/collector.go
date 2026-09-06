package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const collectorFile = "collector.json"

type Collector struct {
	MachineID string                        `json:"machine_id"`
	ServerURL string                        `json:"server_url,omitempty"`
	Token     string                        `json:"token,omitempty"`
	Sources   map[string][]string           `json:"sources,omitempty"`
	State     map[string]CollectionRevision `json:"collection_state,omitempty"`
}

type CollectionRevision struct {
	Digest    string `json:"digest"`
	MachineID string `json:"machine_id"`
}

func LoadCollector() (Collector, string, error) {
	dir, err := collectorDir()
	if err != nil {
		return Collector{}, "", err
	}
	path := filepath.Join(dir, collectorFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		machineID, idErr := newMachineID()
		if idErr != nil {
			return Collector{}, "", idErr
		}
		cfg := Collector{MachineID: machineID, Sources: map[string][]string{}, State: map[string]CollectionRevision{}}
		if saveErr := SaveCollector(path, cfg); saveErr != nil {
			return Collector{}, "", saveErr
		}
		return cfg, path, nil
	}
	if err != nil {
		return Collector{}, "", fmt.Errorf("read collector configuration: %w", err)
	}
	var cfg Collector
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Collector{}, "", fmt.Errorf("parse collector configuration: %w", err)
	}
	if cfg.MachineID == "" {
		return Collector{}, "", errors.New("collector configuration has no machine_id")
	}
	if cfg.Sources == nil {
		cfg.Sources = map[string][]string{}
	}
	if cfg.State == nil {
		cfg.State = map[string]CollectionRevision{}
	}
	return cfg, path, nil
}

func SaveCollector(path string, cfg Collector) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create collector configuration directory: %w", err)
	}
	if err := os.Chmod(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("protect collector configuration directory: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode collector configuration: %w", err)
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".collector-*")
	if err != nil {
		return fmt.Errorf("create collector configuration: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("protect collector configuration: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write collector configuration: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync collector configuration: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close collector configuration: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace collector configuration: %w", err)
	}
	return os.Chmod(path, 0o600)
}

func collectorDir() (string, error) {
	if dir := os.Getenv("TRAICR_CONFIG_DIR"); dir != "" {
		return dir, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find user configuration directory: %w", err)
	}
	return filepath.Join(dir, "traicr"), nil
}

func newMachineID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("create machine ID: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}
