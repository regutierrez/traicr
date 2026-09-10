package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/regutierrez/traicr/internal/collector"
	"github.com/regutierrez/traicr/internal/config"
	"github.com/regutierrez/traicr/internal/version"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "traicr:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: traicr <sources|collect|login|upload|version>")
	}
	switch args[0] {
	case "version":
		if len(args) != 1 {
			return errors.New("usage: traicr version")
		}
		fmt.Fprintf(stdout, "traicr %s\n", version.CurrentBuildInfo())
		return nil
	case "sources":
		return runSources(ctx, args[1:], stdout)
	case "collect":
		return runCollect(ctx, args[1:], stdout, stderr)
	case "login":
		return runLogin(args[1:], stdin, stderr)
	case "upload":
		return runUpload(ctx, args[1:], stdout, stderr)
	default:
		return errors.New("usage: traicr <sources|collect|login|upload|version>")
	}
}

func runSources(ctx context.Context, args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("sources", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var sourceFlags values
	flags.Var(&sourceFlags, "source", "configured source as harness=path")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return errors.New("usage: traicr sources [--source harness=path]")
	}
	cfg, _, err := config.LoadCollector()
	if err != nil {
		return err
	}
	overrides, err := parseSources(sourceFlags)
	if err != nil {
		return err
	}
	configured := cloneSources(cfg.Sources)
	for harness, roots := range overrides {
		configured[harness] = roots
	}
	for _, source := range collector.Sources(ctx, configured) {
		fmt.Fprintf(stdout, "%s\t%d traces\t%s\n", source.Harness, source.Traces, source.Location)
		for _, warning := range source.Warnings {
			fmt.Fprintf(stdout, "  warning [%s]: %s\n", warning.Code, warning.Message)
		}
	}
	return nil
}

func runCollect(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("collect", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var harnesses values
	var sourceFlags values
	var output string
	var all bool
	flags.Var(&harnesses, "harness", "harness to collect")
	flags.Var(&sourceFlags, "source", "source override as harness=path")
	flags.StringVar(&output, "output", "", "archive output directory")
	flags.BoolVar(&all, "all", false, "include already acknowledged revisions")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || output == "" {
		return errors.New("usage: traicr collect --output DIR [--harness NAME] [--source harness=path] [--all]")
	}
	cfg, _, err := config.LoadCollector()
	if err != nil {
		return err
	}
	sources, err := parseSources(sourceFlags)
	if err != nil {
		return err
	}
	if len(harnesses) == 0 {
		harnesses = append(harnesses, collector.Harnesses()...)
	}
	result, err := collector.Collect(ctx, cfg, collector.CollectOptions{
		Harnesses: harnesses,
		Sources:   sources,
		OutputDir: output,
		All:       all,
		Version:   version.CurrentBuildInfo().Version,
		Progress:  collectProgressPrinter(stderr),
	})
	fmt.Fprintln(stderr)
	if err != nil {
		return err
	}
	for _, warning := range result.Warnings {
		fmt.Fprintf(stderr, "warning [%s]: %s\n", warning.Code, warning.Message)
	}
	if len(result.Archives) == 0 {
		fmt.Fprintln(stdout, "No new or changed traces found.")
		return nil
	}
	for _, archivePath := range result.Archives {
		fmt.Fprintln(stdout, archivePath)
	}
	return nil
}

// Rewrite one stderr line per progress event so long collections do not look stuck.
func collectProgressPrinter(stderr io.Writer) collector.CollectProgress {
	width := 0
	return func(phase, harness string, completed, total int) {
		var line string
		switch phase {
		case "collecting":
			line = fmt.Sprintf("collecting %s...", harness)
		case "describing":
			line = fmt.Sprintf("%s: %d/%d traces", harness, completed, total)
		case "archiving":
			line = fmt.Sprintf("writing %d traces to archive...", total)
		default:
			return
		}
		// Pad to the previous width so a shorter line fully overwrites a longer one.
		fmt.Fprintf(stderr, "\r%-*s", width, line)
		width = len(line)
	}
}

func runLogin(args []string, stdin io.Reader, stderr io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: traicr login URL (token is read from TRAICR_ADMIN_TOKEN or standard input)")
	}
	serverURL := strings.TrimRight(args[0], "/")
	parsed, err := url.Parse(serverURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("login URL must be an absolute http or https URL without credentials, query, or fragment")
	}
	token := os.Getenv("TRAICR_ADMIN_TOKEN")
	if token == "" {
		fmt.Fprint(stderr, "Admin token: ")
		line, readErr := bufio.NewReader(stdin).ReadString('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return fmt.Errorf("read token: %w", readErr)
		}
		token = strings.TrimSpace(line)
		fmt.Fprintln(stderr)
	}
	if token == "" {
		return errors.New("admin token is empty")
	}
	cfg, path, err := config.LoadCollector()
	if err != nil {
		return err
	}
	cfg.ServerURL = serverURL
	cfg.Token = token
	if err := config.SaveCollector(path, cfg); err != nil {
		return err
	}
	fmt.Fprintln(stderr, "Server login saved.")
	return nil
}

func runUpload(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: traicr upload ARCHIVE.zip [ARCHIVE.zip ...]")
	}
	cfg, configPath, err := config.LoadCollector()
	if err != nil {
		return err
	}
	progress := func(file string, sent, total int64) {
		percent := float64(sent) * 100 / float64(total)
		fmt.Fprintf(stderr, "\ruploading %s: %d/%d bytes (%.1f%%)", filepath.Base(file), sent, total, percent)
	}
	reports, err := collector.Upload(ctx, nil, &cfg, configPath, args, progress)
	fmt.Fprintln(stderr)
	if err != nil {
		return err
	}
	for _, report := range reports {
		fmt.Fprintf(stdout, "import complete: %d imported, %d updated, %d unchanged, %d partial, %d unsupported, %d failed\n", report.Imported, report.Updated, report.Unchanged, report.Partial, report.Unsupported, report.Failed)
	}
	return nil
}

type values []string

func (values *values) String() string { return strings.Join(*values, ",") }

func (values *values) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func parseSources(flags []string) (map[string][]string, error) {
	result := map[string][]string{}
	for _, value := range flags {
		harness, source, ok := strings.Cut(value, "=")
		if !ok || harness == "" || source == "" {
			return nil, fmt.Errorf("invalid --source %q; expected harness=path", value)
		}
		result[harness] = append(result[harness], source)
	}
	return result, nil
}

func cloneSources(source map[string][]string) map[string][]string {
	result := make(map[string][]string, len(source))
	for harness, roots := range source {
		result[harness] = append([]string(nil), roots...)
	}
	return result
}
