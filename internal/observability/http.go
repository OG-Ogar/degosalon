// Package observability records request metadata without bodies, query strings,
// cookies or user-controlled request IDs.
package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"
)

type requestKey struct{}

func NewRequestID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}
func RequestID(ctx context.Context) string { id, _ := ctx.Value(requestKey{}).(string); return id }

type Response struct {
	http.ResponseWriter
	Status int
}

func (w *Response) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *Response) WriteHeader(status int) {
	if status < 200 {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	if w.Status != 0 {
		return
	}
	w.Status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *Response) Write(body []byte) (int, error) {
	if w.Status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}
func (w *Response) StatusCode() int {
	if w.Status == 0 {
		return http.StatusOK
	}
	return w.Status
}

func Requests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := NewRequestID()
		if err != nil {
			http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("X-Request-ID", id)
		r = r.WithContext(context.WithValue(r.Context(), requestKey{}, id))
		response := &Response{ResponseWriter: w}
		started := time.Now()
		completed := false
		defer func() {
			status := response.StatusCode()
			if !completed {
				status = http.StatusInternalServerError
			}
			// ServeMux assigns a registered pattern rather than a raw URL containing
			// account IDs, reset tokens or user-entered search/contact values.
			logger.InfoContext(r.Context(), "http_request", "request_id", id, "method", r.Method, "route", r.Pattern, "status", status, "duration_ms", time.Since(started).Milliseconds())
		}()
		next.ServeHTTP(response, r)
		completed = true
	})
}
