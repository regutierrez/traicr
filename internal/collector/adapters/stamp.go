package adapters

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

func sourceStamp(paths []string) (string, error) {
	ordered := append([]string(nil), paths...)
	sort.Strings(ordered)
	var stamp strings.Builder
	for _, path := range ordered {
		info, err := os.Lstat(path)
		if err != nil {
			return "", err
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("source is not a regular file: %s", path)
		}
		dev, ino := fileIdentity(info)
		fmt.Fprintf(&stamp, "%s\x00%d\x00%d\x00%d\x00%d\n", path, dev, ino, info.Size(), info.ModTime().UnixNano())
	}
	return stamp.String(), nil
}

func fileIdentity(info os.FileInfo) (uint64, uint64) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0
	}
	return uint64(stat.Dev), uint64(stat.Ino)
}

func jsonlSourceFiles(a jsonlAdapter, path string) []string {
	files := []string{path}
	if a.companionJSON {
		entries, err := os.ReadDir(filepath.Dir(path))
		if err == nil {
			for _, entry := range entries {
				if !entry.Type().IsRegular() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
					continue
				}
				files = append(files, filepath.Join(filepath.Dir(path), entry.Name()))
			}
		}
	}
	if a.name == "claude-code" && strings.Contains(filepath.ToSlash(path), "/subagents/agent-") {
		metadataPath := strings.TrimSuffix(path, ".jsonl") + ".meta.json"
		if _, err := os.Stat(metadataPath); err == nil {
			files = append(files, metadataPath)
		}
	}
	sort.Strings(files)
	return files
}

func directorySourceFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type().IsRegular() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}
