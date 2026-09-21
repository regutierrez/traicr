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
	// Skipped is the number of traces left on disk because their source stamp
	// matched an already acknowledged revision.
	Skipped int
}

// Progress reports how many traces an adapter has gathered so far and the
// total when it is known; total is 0 while the adapter is still discovering.
type Progress func(completed, total int)

func (p Progress) report(completed, total int) {
	if p != nil {
		p(completed, total)
	}
}

// SkipUnchanged reports whether a local source with this stamp can be left
// uncopied because Collection State already has its revision digest.
type SkipUnchanged func(source, stamp string) bool

func (skip SkipUnchanged) skip(source, stamp string) bool {
	return skip != nil && source != "" && stamp != "" && skip(source, stamp)
}

type Adapter interface {
	Name() string
	Discover(context.Context, []string) Source
	Collect(context.Context, []string, Progress, SkipUnchanged) (Result, error)
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
