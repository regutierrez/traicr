package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/regutierrez/traicr/internal/client"
	"github.com/regutierrez/traicr/internal/config"
)

const tracesUsage = "usage: traicr traces <rename>"

func runTraces(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New(tracesUsage)
	}
	switch args[0] {
	case "-h", "--help", "help":
		printTracesHelp(stderr)
		return nil
	case "rename":
		return runTracesRename(ctx, args[1:], stdout, stderr)
	default:
		return errors.New(tracesUsage)
	}
}

func runTracesRename(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	const renameUsage = "usage: traicr traces rename <id> <title> | traicr traces rename --clear <id>"
	flags := flag.NewFlagSet("traces rename", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { printTracesRenameHelp(stderr) }
	var clear bool
	flags.BoolVar(&clear, "clear", false, "remove the manual title")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	positional := flags.Args()
	if (clear && len(positional) != 1) || (!clear && len(positional) != 2) {
		return errors.New(renameUsage)
	}
	id, err := strconv.ParseInt(positional[0], 10, 64)
	if err != nil || id <= 0 {
		return fmt.Errorf("trace id must be a positive integer, got %q", positional[0])
	}
	var title string
	if !clear {
		title = positional[1]
		if strings.TrimSpace(title) == "" {
			return errors.New("title is empty; use --clear to remove a manual title")
		}
	}
	cfg, _, err := config.LoadCollector()
	if err != nil {
		return err
	}
	api, err := client.New(cfg, nil)
	if err != nil {
		return err
	}
	trace, err := api.SetTitleOverride(ctx, id, title)
	if err != nil {
		return err
	}
	shown := trace.Title
	if shown == "" {
		shown = trace.NativeTraceID
	}
	fmt.Fprintf(stdout, "%d\t%s\n", trace.ID, shown)
	return nil
}

func printTracesHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage: traicr traces <command> [options]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Manage traces stored on the server saved by 'traicr login'.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w)
	writeCommand(w, "rename", "Set or clear a trace's manual title")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Run 'traicr traces <command> --help' for details on a command.")
}

func printTracesRenameHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage: traicr traces rename <id> <title>")
	fmt.Fprintln(w, "       traicr traces rename --clear <id>")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Set a manual title for a trace on the server. The manual title replaces the collected title everywhere it is shown, stays searchable, and is never changed by later uploads. The collected title stays searchable too. Prints the trace ID and the title now shown.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Arguments:")
	fmt.Fprintln(w)
	writeCommand(w, "id", "Numeric trace ID, as shown in the Web UI address /traces/<id>")
	writeCommand(w, "title", "New title: one line, at most 200 characters")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w)
	writeOption(w, "--clear", "Remove the manual title and show the collected title again. Put options before arguments")
	writeOption(w, "-h, --help", "Show this help")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  $ traicr traces rename 3362 \"Speed up incremental collect\"")
	fmt.Fprintln(w, "  $ traicr traces rename --clear 3362")
}
