// Command renovate-alpine-datasource serves Alpine Linux package metadata for
// consumption by Renovate's customDatasources mechanism.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RemkoMolier/renovate-alpine-datasource/internal/api"
)

const (
	defaultAddr     = ":8080"
	shutdownTimeout = 5 * time.Second
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(signalCtx, newServer(api.New()), logger); err != nil {
		logger.Error("http server stopped unexpectedly", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(ctx context.Context, server *http.Server, logger *slog.Logger) error {
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return fmt.Errorf("listen and serve: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("shutdown server: %w", err)
		}

		if err := <-serverErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server stopped: %w", err)
		}

		logger.Info("http server shutdown completed")

		return nil
	}
}

func newServer(apiServer *api.API) *http.Server {
	return &http.Server{
		Addr:    defaultAddr,
		Handler: apiServer.Handler(),
	}
}
