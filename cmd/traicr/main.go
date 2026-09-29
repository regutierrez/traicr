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
	"strconv"
	"strings"
	"syscall"
	"time"

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

const usage = "usage: traicr <sources|collect|login|upload|traces|version>"

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New(usage)
	}
	switch args[0] {
	case "-h", "--help", "help":
		printRootHelp(stderr)
		return nil
	case "version":
		return runVersion(args[1:], stdout, stderr)
	case "sources":
		return runSources(ctx, args[1:], stdout, stderr)
	case "collect":
		return runCollect(ctx, args[1:], stdout, stderr)
	case "login":
		return runLogin(args[1:], stdin, stderr)
	case "upload":
		return runUpload(ctx, args[1:], stdout, stderr)
	case "traces":
		return runTraces(ctx, args[1:], stdout, stderr)
	default:
		return errors.New(usage)
	}
}

func runVersion(args []string, stdout, stderr io.Writer) error {
	if len(args) == 1 && isHelp(args[0]) {
		printVersionHelp(stderr)
		return nil
	}
	if len(args) != 0 {
		return errors.New("usage: traicr version")
	}
	fmt.Fprintf(stdout, "traicr %s\n", version.CurrentBuildInfo())
	return nil
}

func runSources(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	const sourcesUsage = "usage: traicr sources [--source harness=path]"
	flags := flag.NewFlagSet("sources", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { printSourcesHelp(stderr) }
	var sourceFlags values
	flags.Var(&sourceFlags, "source", "configured source as harness=path")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return errors.New(sourcesUsage)
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
	const collectUsage = "usage: traicr collect --output DIR [--harness NAME] [--source harness=path] [--all]"
	flags := flag.NewFlagSet("collect", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { printCollectHelp(stderr) }
	var harnesses values
	var sourceFlags values
	var output string
	var all bool
	flags.Var(&harnesses, "harness", "harness to collect")
	flags.Var(&sourceFlags, "source", "source override as harness=path")
	flags.StringVar(&output, "output", "", "archive output directory")
	flags.BoolVar(&all, "all", false, "include already acknowledged revisions")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 || output == "" {
		return errors.New(collectUsage)
	}
	cfg, configPath, err := config.LoadCollector()
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
	printer := &collectStatus{statusLine: statusLine{out: stderr}}
	result, err := collector.Collect(ctx, cfg, collector.CollectOptions{
		Harnesses: harnesses,
		Sources:   sources,
		OutputDir: output,
		All:       all,
		Version:   version.CurrentBuildInfo().Version,
		Progress:  printer.report,
		ConfigDir: filepath.Dir(configPath),
	})
	printer.finish()
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

type statusLine struct {
	out   io.Writer
	width int
}

func (s *statusLine) rewrite(line string) {
	fmt.Fprintf(s.out, "\r%-*s", s.width, line)
	s.width = len(line)
}

func (s *statusLine) persist(line string) {
	s.rewrite(line)
	fmt.Fprintln(s.out)
	s.width = 0
}

func (s *statusLine) finish() {
	if s.width > 0 {
		fmt.Fprintln(s.out)
		s.width = 0
	}
}

type collectStatus struct {
	statusLine
	started time.Time
	now     func() time.Time
}

func (s *collectStatus) report(p collector.Progress) {
	if s.now == nil {
		s.now = time.Now
	}
	switch p.Phase {
	case "collecting":
		if p.Completed == 0 && p.Total == 0 {
			s.started = s.now()
			s.rewrite(p.Harness + ": collecting...")
		} else {
			s.rewrite(p.Harness + ": collecting " + count(p.Completed, p.Total))
		}
	case "describing":
		s.rewrite(p.Harness + ": describing " + count(p.Completed, p.Total))
	case "collected":
		elapsed := s.now().Sub(s.started).Round(time.Second)
		if p.Total == 0 {
			s.persist(fmt.Sprintf("%s: no traces found (%s)", p.Harness, elapsed))
		} else {
			s.persist(fmt.Sprintf("%s: %d traces found, %d new or changed (%s)", p.Harness, p.Total, p.Completed, elapsed))
		}
	case "archiving":
		s.rewrite(fmt.Sprintf("writing %d traces to archive...", p.Total))
	case "archived":
		s.persist(fmt.Sprintf("wrote %d traces to %d archive(s)", p.Completed, p.Total))
	}
}

func count(completed, total int) string {
	if total == 0 {
		return strconv.Itoa(completed)
	}
	return fmt.Sprintf("%d/%d", completed, total)
}

type uploadStatus struct {
	statusLine
}

func (s *uploadStatus) report(u collector.UploadStatus) {
	name := filepath.Base(u.File)
	switch u.Phase {
	case "sending":
		s.rewrite(fmt.Sprintf("%s: uploading %d/%d bytes (%.1f%%)", name, u.Sent, u.Size, float64(u.Sent)*100/float64(u.Size)))
	case "validating":
		s.rewrite(name + ": server validating archive...")
	case "normalizing":
		s.rewrite(fmt.Sprintf("%s: server importing %s traces", name, count(u.Completed, u.Total)))
	case "complete":
		s.finish()
	}
}

func runLogin(args []string, stdin io.Reader, stderr io.Writer) error {
	if len(args) == 1 && isHelp(args[0]) {
		printLoginHelp(stderr)
		return nil
	}
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
	if len(args) == 1 && isHelp(args[0]) {
		printUploadHelp(stderr)
		return nil
	}
	if len(args) == 0 {
		return errors.New("usage: traicr upload ARCHIVE.zip [ARCHIVE.zip ...]")
	}
	cfg, configPath, err := config.LoadCollector()
	if err != nil {
		return err
	}
	status := &uploadStatus{statusLine: statusLine{out: stderr}}
	reports, err := collector.Upload(ctx, nil, &cfg, configPath, args, status.report)
	status.finish()
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

func isHelp(arg string) bool {
	return arg == "-h" || arg == "--help" || arg == "help"
}

func printRootHelp(w io.Writer) {
	fmt.Fprintln(w, "Traicr CLI")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage: traicr [command] [options]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Collect AI coding traces from this machine, package them as Trace ZIPs, and upload them to a Traicr server.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w)
	writeCommand(w, "sources", "List detected harnesses, source locations, and available traces without writing an archive")
	writeCommand(w, "collect", "Collect new or changed traces into Trace ZIP archives")
	writeCommand(w, "login", "Save the Traicr server URL and admin token for uploads")
	writeCommand(w, "upload", "Upload Trace ZIP archives to the configured server")
	writeCommand(w, "traces", "Manage traces stored on the configured server, such as renaming them")
	writeCommand(w, "version", "Print the version number and exit")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Environment variables:")
	fmt.Fprintln(w)
	writeEnv(w, "TRAICR_ADMIN_TOKEN", "Admin token used by login when not read from standard input")
	writeEnv(w, "TRAICR_CONFIG_DIR", fmt.Sprintf("Collector configuration directory (default: %s)", defaultConfigDirDisplay()))
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Discover available traces on this machine:")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  $ traicr sources")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Log in, collect new or changed traces, and upload them:")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  $ traicr login http://127.0.0.1:8080")
	fmt.Fprintln(w, "  $ traicr collect --output ./traces")
	fmt.Fprintln(w, "  $ traicr upload ./traces/*.zip")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Collect every trace, including already acknowledged revisions:")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  $ traicr collect --all --output ./traces")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Collect one harness from a custom path:")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  $ traicr collect --harness pi --source pi=~/.pi/agent/sessions --output ./traces")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Run 'traicr <command> --help' for details on a command.")
}

func printSourcesHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage: traicr sources [options]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "List detected harnesses, source locations, available trace counts, and collection warnings without writing an archive.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w)
	writeOption(w, "--source <harness=path>", "Override the source path for a harness; repeat for multiple roots")
	writeOption(w, "-h, --help", "Show this help")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  $ traicr sources")
	fmt.Fprintln(w, "  $ traicr sources --source pi=~/.pi/agent/sessions")
}

func printCollectHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage: traicr collect --output <dir> [options]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Collect new or changed traces into Trace ZIP archives. Collection does not upload; use 'traicr upload' afterward. Without --all, already acknowledged revisions are skipped.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w)
	writeOption(w, "--output <dir>", "Directory where Trace ZIP archives are written (required)")
	writeOption(w, "--harness <name>", "Collect only this harness; repeat for multiple. Defaults to all known harnesses")
	writeOption(w, "--source <harness=path>", "Override the source path for a harness; repeat for multiple roots")
	writeOption(w, "--all", "Include traces already acknowledged by a previous successful upload")
	writeOption(w, "-h, --help", "Show this help")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  $ traicr collect --output ./traces")
	fmt.Fprintln(w, "  $ traicr collect --harness amp --output ./traces")
	fmt.Fprintln(w, "  $ traicr collect --all --output ./traces")
	fmt.Fprintln(w, "  $ traicr collect --harness pi --source pi=~/.pi/agent/sessions --output ./traces")
}

func printLoginHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage: traicr login <url>")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Save the Traicr server URL and admin token in the local collector configuration. The token is read from TRAICR_ADMIN_TOKEN when set; otherwise it is prompted on standard input. Login does not create a server-side user.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Arguments:")
	fmt.Fprintln(w)
	writeCommand(w, "url", "Absolute http or https server URL without credentials, query, or fragment")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w)
	writeOption(w, "-h, --help", "Show this help")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Environment variables:")
	fmt.Fprintln(w)
	writeEnv(w, "TRAICR_ADMIN_TOKEN", "Admin token used instead of the interactive prompt")
	writeEnv(w, "TRAICR_CONFIG_DIR", fmt.Sprintf("Collector configuration directory (default: %s)", defaultConfigDirDisplay()))
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  $ export TRAICR_ADMIN_TOKEN")
	fmt.Fprintln(w, "  $ traicr login http://127.0.0.1:8080")
}

func printUploadHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage: traicr upload <archive.zip> [archive.zip ...]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Upload one or more Trace ZIP archives to the server saved by 'traicr login'. A successful import advances local collection state for imported, updated, or unchanged revisions so the next collect can skip them.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Arguments:")
	fmt.Fprintln(w)
	writeCommand(w, "archive.zip", "Path to a Trace ZIP produced by 'traicr collect'; repeat for multiple archives")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w)
	writeOption(w, "-h, --help", "Show this help")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  $ traicr upload ./traces/*.zip")
	fmt.Fprintln(w, "  $ traicr upload ./traces/0001.zip ./traces/0002.zip")
}

func printVersionHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage: traicr version")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Print the traicr version, commit, and build date, then exit.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w)
	writeOption(w, "-h, --help", "Show this help")
}

const (
	helpNameWidth = 30
	helpEnvWidth  = 30
	helpWrapWidth = 100
)

func writeCommand(w io.Writer, name, description string) {
	writeLabeled(w, "  ", name, helpNameWidth, description, 2+helpNameWidth)
}

func writeEnv(w io.Writer, name, description string) {
	writeLabeled(w, "  ", name, helpEnvWidth, description, 2+helpEnvWidth)
}

func writeOption(w io.Writer, name, description string) {
	fmt.Fprintf(w, "  %s\n", name)
	for _, line := range wrapWords(description, helpWrapWidth-6) {
		fmt.Fprintf(w, "      %s\n", line)
	}
}

func writeLabeled(w io.Writer, indent, name string, nameWidth int, description string, descCol int) {
	prefix := indent + name
	if len(name) < nameWidth {
		prefix = indent + name + strings.Repeat(" ", nameWidth-len(name))
	} else {
		prefix += " "
	}
	lines := wrapWords(description, helpWrapWidth-descCol)
	if len(lines) == 0 {
		fmt.Fprintln(w, strings.TrimRight(prefix, " "))
		return
	}
	fmt.Fprintf(w, "%s%s\n", prefix, lines[0])
	pad := strings.Repeat(" ", descCol)
	for _, line := range lines[1:] {
		fmt.Fprintf(w, "%s%s\n", pad, line)
	}
}

func wrapWords(text string, width int) []string {
	if width < 20 {
		width = 20
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	var lines []string
	var current strings.Builder
	for _, word := range words {
		if current.Len() == 0 {
			current.WriteString(word)
			continue
		}
		if current.Len()+1+len(word) > width {
			lines = append(lines, current.String())
			current.Reset()
			current.WriteString(word)
			continue
		}
		current.WriteByte(' ')
		current.WriteString(word)
	}
	if current.Len() > 0 {
		lines = append(lines, current.String())
	}
	return lines
}

func defaultConfigDirDisplay() string {
	if dir := os.Getenv("TRAICR_CONFIG_DIR"); dir != "" {
		return dir
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "<user-config-dir>/traicr"
	}
	return filepath.Join(dir, "traicr")
}
