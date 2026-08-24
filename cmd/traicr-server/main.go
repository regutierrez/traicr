// Command traicr-server runs the private Traicr archive.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/regutierrez/traicr/internal/config"
	"github.com/regutierrez/traicr/internal/server"
	"github.com/regutierrez/traicr/internal/version"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "version" {
		fmt.Printf("traicr-server %s\n", version.CurrentBuildInfo())
		return
	}
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: traicr-server [version]")
		os.Exit(2)
	}

	serverConfig, err := config.LoadServerConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := server.RunHTTPServer(ctx, serverConfig, logger); err != nil {
		logger.Error("Traicr server stopped", "error", err)
		os.Exit(1)
	}
}
