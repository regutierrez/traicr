package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/regutierrez/traicr/internal/config"
	"github.com/regutierrez/traicr/internal/normalize"
	"github.com/regutierrez/traicr/internal/store"
)

const shutdownTimeout = 10 * time.Second

// RunHTTPServer serves requests until cancellation and then shuts down gracefully.
func RunHTTPServer(ctx context.Context, serverConfig config.ServerConfig, logger *slog.Logger) error {
	if err := os.MkdirAll(serverConfig.DataDir, 0o700); err != nil {
		return fmt.Errorf("server data directory: %w", err)
	}
	database, err := store.Open(serverConfig.DataDir)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer database.Close()
	handler, err := NewHTTPHandler(serverConfig, database, logger)
	if err != nil {
		return err
	}
	workerContext, stopWorker := context.WithCancel(ctx)
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		if err := database.GC(workerContext); err != nil && workerContext.Err() == nil {
			logger.Error("object cleanup failed", "error_type", fmt.Sprintf("%T", err))
		}
		if err := database.Renormalize(workerContext, normalize.Version, normalize.Run); err != nil && workerContext.Err() == nil {
			logger.Error("normalizer rebuild failed; previous events retained", "error_type", fmt.Sprintf("%T", err))
		}
	}()
	defer func() {
		stopWorker()
		<-workerDone
	}()

	httpServer := &http.Server{
		Addr:              serverConfig.ListenAddress,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    32 << 10,
		BaseContext:       func(net.Listener) context.Context { return ctx },
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
		if err := httpServer.Shutdown(shutdownContext); err != nil {
			httpServer.Close()
			return err
		}
		return nil
	}
}
