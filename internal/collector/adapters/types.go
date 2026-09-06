package adapters

import (
	"context"

	"github.com/regutierrez/traicr/internal/archive"
	"github.com/regutierrez/traicr/internal/domain"
)

type Source struct {
	Harness  string
	Location string
	Traces   int
	Warnings []domain.Warning
}

type Result struct {
	Inputs   []archive.Input
	Warnings []domain.Warning
	Cleanup  func()
}

type Adapter interface {
	Name() string
	Discover(context.Context, []string) Source
	Collect(context.Context, []string) (Result, error)
}

func All() []Adapter {
	return []Adapter{
		commandAdapter{name: "amp", format: "amp-thread-export", executable: "amp"},
		commandAdapter{name: "opencode", format: "opencode-export", executable: "opencode"},
		codexAdapter{},
		jsonlAdapter{name: "pi", format: "pi-jsonl", defaultRoots: piRoots},
		jsonlAdapter{name: "claude-code", format: "claude-code-jsonl", defaultRoots: claudeRoots},
		jsonlAdapter{
			name: "cursor-agent", format: "cursor-agent-jsonl", companionJSON: true,
			defaultRoots: func() []string { return []string{homePath(".cursor", "projects")} },
		},
		cursorEditorAdapter{},
		grokAdapter{},
	}
}
