package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"time"
)

const shutdownTimeout = 30 * time.Second

type healthState struct {
	ready atomic.Bool
	check func(context.Context) error
}

func (h *healthState) handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	status, body := http.StatusOK, "{\"status\":\"ok\"}\n"
	if r.URL.Path == "/health/ready" {
		if !h.ready.Load() {
			status, body = http.StatusServiceUnavailable, "{\"status\":\"unavailable\"}\n"
		} else {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			err := h.check(ctx)
			cancel()
			if err != nil {
				status, body = http.StatusServiceUnavailable, "{\"status\":\"unavailable\"}\n"
			}
		}
	}
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write([]byte(body))
	}
}

// serveUntilStopped drains HTTP requests and joins workers before its caller
// releases database resources. Request contexts remain alive during the drain.
func serveUntilStopped(ctx context.Context, server *http.Server, listener net.Listener, health *healthState, stopWorkers context.CancelFunc, workersDone <-chan struct{}, timeout time.Duration) error {
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	var serveErr error
	select {
	case <-ctx.Done():
	case serveErr = <-served:
	}
	health.ready.Store(false)
	stopWorkers()
	drain, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	shutdownErr := server.Shutdown(drain)
	if shutdownErr != nil {
		_ = server.Close()
	}
	if serveErr == nil {
		select {
		case serveErr = <-served:
		case <-drain.Done():
			serveErr = drain.Err()
		}
	}
	select {
	case <-workersDone:
	case <-drain.Done():
		return fmt.Errorf("workers did not stop: %w", drain.Err())
	}
	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		return serveErr
	}
	return shutdownErr
}
