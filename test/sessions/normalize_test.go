package sessions_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/regutierrez/traicr/internal/domain"
	"github.com/regutierrez/traicr/internal/normalize"
)

func TestNormalizeScrubbedSessions(t *testing.T) {
	root := sessionRoot(t)
	cases := []struct {
		dir     string
		harness string
		adapter string
		events  int
	}{
		{"pi-herdr", "pi", "pi-jsonl", 1772},
		{"amp-transcript-support", "amp", "amp-thread-export", 1747},
		{"claude-cleanup", "claude-code", "claude-code-jsonl", 436},
	}
	for _, test := range cases {
		t.Run(test.dir, func(t *testing.T) {
			result, err := normalize.Run(context.Background(), domain.Descriptor{Harness: test.harness, Adapter: test.adapter}, os.DirFS(filepath.Join(root, test.dir)))
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != "normalized" && result.Status != "partially_parsed" {
				t.Fatalf("status = %q warnings=%#v", result.Status, result.Warnings)
			}
			if len(result.Events) != test.events {
				t.Fatalf("events = %d, want %d", len(result.Events), test.events)
			}
		})
	}
}
