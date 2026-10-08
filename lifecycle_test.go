package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHealthSeparatesLivenessReadinessAndHidesDependencyErrors(t *testing.T) {
	calls := 0
	h := &healthState{check: func(ctx context.Context) error {
		calls++
		if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 2*time.Second {
			t.Error("unbounded dependency check")
		}
		return errors.New("secret database credential")
	}}
	for _, test := range []struct {
		method, path string
		ready        bool
		status       int
	}{
		{"GET", "/health/live", false, 200}, {"GET", "/health/ready", false, 503},
		{"GET", "/health/ready", true, 503}, {"HEAD", "/health/live", true, 200},
		{"POST", "/health/ready", true, 405},
	} {
		h.ready.Store(test.ready)
		response := httptest.NewRecorder()
		h.handle(response, httptest.NewRequest(test.method, test.path, nil))
		if response.Code != test.status || strings.Contains(response.Body.String(), "secret") {
			t.Fatalf("health response: %d %s", response.Code, response.Body.String())
		}
		if test.method == "HEAD" && response.Body.Len() != 0 {
			t.Fatal("HEAD included body")
		}
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("health can be cached")
		}
	}
	if calls != 1 {
		t.Fatalf("liveness or drain queried dependency: %d", calls)
	}
	h.check = func(context.Context) error { return nil }
	h.ready.Store(true)
	response := httptest.NewRecorder()
	h.handle(response, httptest.NewRequest("GET", "/health/ready", nil))
	if response.Code != 200 {
		t.Fatal("healthy dependency failed readiness")
	}
}

func TestShutdownDrainsRequestsAndJoinsWorkers(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	workerCtx, stopWorkers := context.WithCancel(context.Background())
	defer stopWorkers()
	workerStopped, workersDone := make(chan struct{}), make(chan struct{})
	releaseWorker := make(chan struct{})
	defer close(releaseWorker)
	go func() { <-workerCtx.Done(); close(workerStopped); <-releaseWorker; close(workersDone) }()
	entered, releaseRequest := make(chan struct{}), make(chan struct{})
	defer close(releaseRequest)
	requestDone := make(chan error, 1)
	health := &healthState{check: func(context.Context) error { return nil }}
	health.ready.Store(true)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-releaseRequest
		if r.Context().Err() != nil {
			requestDone <- r.Context().Err()
			return
		}
		_, err := io.WriteString(w, "completed")
		requestDone <- err
	})}
	finished := make(chan error, 1)
	go func() {
		finished <- serveUntilStopped(ctx, server, listener, health, stopWorkers, workersDone, 5*time.Second)
	}()
	responseDone := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 5 * time.Second}
		response, err := client.Get("http://" + listener.Addr().String())
		if err != nil {
			responseDone <- err
			return
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err == nil && string(body) != "completed" {
			err = errors.New("request was not drained")
		}
		responseDone <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("request never entered")
	}
	stop()
	select {
	case <-workerStopped:
	case <-time.After(5 * time.Second):
		t.Fatal("worker was not canceled")
	}
	if health.ready.Load() {
		t.Error("draining instance remains ready")
	}
	select {
	case err := <-finished:
		t.Fatalf("returned before request/worker completion: %v", err)
	default:
	}
	releaseRequest <- struct{}{}
	if err := <-requestDone; err != nil {
		t.Fatal(err)
	}
	if err := <-responseDone; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-finished:
		t.Fatalf("returned before worker joined: %v", err)
	default:
	}
	releaseWorker <- struct{}{}
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown did not complete")
	}
}

func TestShutdownBoundsUnresponsiveWorker(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, stop := context.WithCancel(context.Background())
	stop()
	h := &healthState{}
	h.ready.Store(true)
	err = serveUntilStopped(ctx, &http.Server{}, listener, h, func() {}, make(chan struct{}), 20*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) || h.ready.Load() {
		t.Fatalf("unbounded or ready shutdown: %v", err)
	}
}
