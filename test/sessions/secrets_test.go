package sessions_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var forbidden = []*regexp.Regexp{
	regexp.MustCompile(`eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`),
	regexp.MustCompile(`(?i)\bsk-[A-Za-z0-9_-]{16,}`),
	regexp.MustCompile(`(?i)\b(?:ghp|gho|ghu|ghs|ghr|github_pat)_[A-Za-z0-9_]{20,}`),
	regexp.MustCompile(`/home/pael\b`),
	regexp.MustCompile(`/Users/pael\b`),
	regexp.MustCompile(`(?i)\bmaelle\b`),
	regexp.MustCompile(`(?i)\besquie\b`),
	regexp.MustCompile(`(?i)\bakkio\b`),
	regexp.MustCompile(`\bpakkio\b`),
	regexp.MustCompile(`(?i)@[a-z0-9.-]*(gmail|akkio)\.com\b`),
}

func TestScrubbedSessionsHaveNoSecrets(t *testing.T) {
	root := sessionRoot(t)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) == ".md" || filepath.Ext(path) == ".py" {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(body)
		for _, pattern := range forbidden {
			if pattern.FindStringIndex(text) != nil {
				t.Errorf("%s still matches %s", path, pattern)
			}
		}
		switch filepath.Ext(path) {
		case ".jsonl":
			for lineNo, line := range strings.Split(text, "\n") {
				if strings.TrimSpace(line) == "" {
					continue
				}
				if !json.Valid([]byte(line)) {
					t.Errorf("%s:%d is not JSON", path, lineNo+1)
				}
			}
		case ".json":
			if !json.Valid(body) {
				t.Errorf("%s is not JSON", path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func sessionRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join("..", "..", "testdata", "sessions")
	if _, err := os.Stat(root); err != nil {
		t.Fatal(err)
	}
	return root
}
