package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/RemkoMolier/renovate-alpine-datasource/internal/api"
)

func TestNewServerUsesDefaultAddr(t *testing.T) {
	t.Parallel()

	server := newServer(api.New())
	t.Cleanup(func() {
		_ = server.Close()
	})

	if server.Addr != defaultAddr {
		t.Fatalf("server.Addr = %q, want %q", server.Addr, defaultAddr)
	}
}

func TestRunShutsDownOnContextCancel(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &http.Server{Addr: "127.0.0.1:0", Handler: api.New().Handler()}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx, server, logger)
	}()

	time.Sleep(25 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("run error = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("run did not return after cancel")
	}
}

func TestRunReturnsListenError(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen error = %v", err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &http.Server{Addr: listener.Addr().String(), Handler: api.New().Handler()}

	err = run(context.Background(), server, logger)
	if err == nil {
		t.Fatal("run error = nil, want non-nil")
	}

	if errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("run error = %v, should not be http.ErrServerClosed", err)
	}
}
