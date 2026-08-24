package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/regutierrez/traicr/internal/config"
)

const shutdownTimeout = 10 * time.Second

// RunHTTPServer serves requests until cancellation and then shuts down gracefully.
func RunHTTPServer(ctx context.Context, serverConfig config.ServerConfig, logger *slog.Logger) error {
	if err := os.MkdirAll(serverConfig.DataDir, 0o700); err != nil {
		return fmt.Errorf("server data directory: %w", err)
	}

	httpServer := &http.Server{
		Addr:              serverConfig.ListenAddress,
		Handler:           NewHTTPHandler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("Traicr server listening", "address", serverConfig.ListenAddress)
		serverErrors <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return httpServer.Shutdown(shutdownContext)
	}
}
