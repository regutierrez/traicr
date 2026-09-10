package collector

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sort"
	"time"

	"github.com/regutierrez/traicr/internal/archive"
	"github.com/regutierrez/traicr/internal/collector/adapters"
	"github.com/regutierrez/traicr/internal/config"
	"github.com/regutierrez/traicr/internal/domain"
)

type CollectOptions struct {
	Harnesses  []string
	Sources    map[string][]string
	OutputDir  string
	All        bool
	SplitBytes int64
	Version    string
	Progress   CollectProgress
}

// CollectProgress reports collection phases: "collecting" while a harness adapter
// gathers traces (total unknown), "describing" as each gathered trace is hashed,
// and "archiving" once before the ZIP files are written.
type CollectProgress func(phase, harness string, completed, total int)

type Collection struct {
	Archives []string
	Warnings []domain.Warning
}

func Harnesses() []string {
	all := adapters.All()
	names := make([]string, 0, len(all))
	for _, adapter := range all {
		names = append(names, adapter.Name())
	}
	return names
}

func Sources(ctx context.Context, configured map[string][]string) []adapters.Source {
	all := adapters.All()
	result := make([]adapters.Source, 0, len(all))
	for _, adapter := range all {
		result = append(result, adapter.Discover(ctx, configured[adapter.Name()]))
	}
	return result
}

func Collect(ctx context.Context, cfg config.Collector, options CollectOptions) (Collection, error) {
	if options.OutputDir == "" {
		return Collection{}, errors.New("collection output directory is required")
	}
	selected := map[string]bool{}
	for _, harness := range options.Harnesses {
		selected[harness] = true
	}
	if len(selected) == 0 {
		return Collection{}, errors.New("select at least one harness or use --all")
	}
	configured := make(map[string][]string, len(cfg.Sources)+len(options.Sources))
	for harness, roots := range cfg.Sources {
		configured[harness] = append([]string(nil), roots...)
	}
	for harness, roots := range options.Sources {
		configured[harness] = roots
	}
	var inputs []archive.Input
	var warnings []domain.Warning
	var cleanups []func()
	defer func() {
		for _, cleanup := range cleanups {
			cleanup()
		}
	}()
	progress := options.Progress
	if progress == nil {
		progress = func(string, string, int, int) {}
	}
	found := map[string]bool{}
	for _, adapter := range adapters.All() {
		if !selected[adapter.Name()] {
			continue
		}
		found[adapter.Name()] = true
		progress("collecting", adapter.Name(), 0, 0)
		result, err := adapter.Collect(ctx, configured[adapter.Name()])
		if err != nil {
			return Collection{}, fmt.Errorf("collect %s: %w", adapter.Name(), err)
		}
		if result.Cleanup != nil {
			cleanups = append(cleanups, result.Cleanup)
		}
		warnings = append(warnings, result.Warnings...)
		for i, input := range result.Inputs {
			progress("describing", adapter.Name(), i+1, len(result.Inputs))
			descriptor, err := archive.Describe(input)
			if err != nil {
				warnings = append(warnings, domain.Warning{Code: "snapshot_failed", Message: adapter.Name() + " " + input.Descriptor.NativeTraceID + ": " + err.Error()})
				continue
			}
			key := StateKey(descriptor.Harness, descriptor.NativeTraceID)
			acknowledged := cfg.State[key]
			if !options.All && acknowledged.MachineID == cfg.MachineID && acknowledged.Digest == descriptor.RevisionDigest {
				continue
			}
			input.Descriptor = descriptor
			inputs = append(inputs, input)
		}
	}
	for harness := range selected {
		if !found[harness] {
			return Collection{}, fmt.Errorf("unknown harness %q", harness)
		}
	}
	if len(inputs) == 0 {
		return Collection{Warnings: warnings}, nil
	}
	hostname, err := os.Hostname()
	if err != nil {
		return Collection{}, fmt.Errorf("read hostname: %w", err)
	}
	sort.Slice(inputs, func(i, j int) bool {
		if inputs[i].Descriptor.Harness == inputs[j].Descriptor.Harness {
			return inputs[i].Descriptor.NativeTraceID < inputs[j].Descriptor.NativeTraceID
		}
		return inputs[i].Descriptor.Harness < inputs[j].Descriptor.Harness
	})
	manifest := domain.Manifest{
		FormatVersion:    domain.FormatVersion,
		CollectorVersion: options.Version,
		CreatedAt:        time.Now().UTC().Format(time.RFC3339Nano),
		SourceMachine:    domain.Machine{ID: cfg.MachineID, Hostname: hostname, OS: runtime.GOOS, Arch: runtime.GOARCH},
	}
	progress("archiving", "", 0, len(inputs))
	paths, err := archive.Write(ctx, options.OutputDir, manifest, inputs, options.SplitBytes)
	if err != nil {
		return Collection{}, err
	}
	return Collection{Archives: paths, Warnings: warnings}, nil
}

func StateKey(harness, nativeTraceID string) string { return harness + "\x00" + nativeTraceID }
