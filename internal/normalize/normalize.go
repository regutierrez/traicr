package normalize

import (
	"context"
	"io/fs"

	"github.com/regutierrez/traicr/internal/domain"
)

func Version(harness string) int {
	switch harness {
	case "amp":
		return 4
	case "pi":
		return 2
	case "cursor":
		return 3
	case "claude-code", "cursor-agent", "opencode", "codex", "grok-build":
		return 1
	default:
		return 0
	}
}

func Run(ctx context.Context, descriptor domain.Descriptor, source fs.FS) (domain.Normalization, error) {
	if err := ctx.Err(); err != nil {
		return domain.Normalization{}, err
	}

	limit := int64(maxSourceBytes)
	if descriptor.Harness == "amp" {
		limit = domain.MaxAmpExportBytes
	}
	source = newLimitedFS(ctx, source, limit)
	var result domain.Normalization
	var err error
	switch descriptor.Harness {
	case "pi":
		result, err = normalizePi(ctx, source)
	case "claude-code":
		result, err = normalizeClaude(ctx, source)
	case "cursor-agent":
		result, err = normalizeCursorAgent(ctx, source)
	case "cursor":
		result, err = normalizeCursor(ctx, source)
	case "amp":
		result, err = normalizeAmp(ctx, source, descriptor.Files)
	case "opencode":
		result, err = normalizeOpenCode(ctx, source)
	case "codex":
		result, err = normalizeCodex(ctx, descriptor.Adapter, source)
	case "grok-build":
		result, err = normalizeGrok(ctx, source)
	default:
		return domain.Normalization{Status: "unsupported", Warnings: []domain.Warning{warning("unsupported_harness", "unsupported harness "+descriptor.Harness)}}, nil
	}
	result.Version = Version(descriptor.Harness)
	return result, err
}
