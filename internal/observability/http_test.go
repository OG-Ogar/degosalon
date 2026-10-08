package observability

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestsCorrelateLogsWithoutRequestSecrets(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	mux := http.NewServeMux()
	var seen string
	mux.HandleFunc("/customer/", func(w http.ResponseWriter, r *http.Request) {
		seen = RequestID(r.Context())
		w.WriteHeader(409)
		_, _ = w.Write([]byte("safe response"))
	})
	handler := Requests(logger, mux)
	request := httptest.NewRequest("POST", "/customer/reset-secret?token=secret-query", strings.NewReader("password=secret-body"))
	request.Header.Set("X-Request-ID", "secret-forged-id")
	request.Header.Set("Cookie", "session=secret-cookie")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if len(seen) != 32 || seen != response.Header().Get("X-Request-ID") || seen == "secret-forged-id" {
		t.Fatal("request ID was not generated/correlated")
	}
	var event map[string]any
	if err := json.Unmarshal(output.Bytes(), &event); err != nil {
		t.Fatal(err)
	}
	if event["request_id"] != seen || event["route"] != "/customer/" || event["status"] != float64(409) {
		t.Fatalf("wrong request metadata: %v", event)
	}
	if strings.Contains(output.String(), "secret-") {
		t.Fatal("logged request secrets or raw path")
	}
	firstID := seen
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest("GET", "/customer/", nil))
	if second.Header().Get("X-Request-ID") == firstID {
		t.Fatal("request IDs reused")
	}
}
func TestResponsePreservesFirstStatusAndUnwrap(t *testing.T) {
	recorder := httptest.NewRecorder()
	response := &Response{ResponseWriter: recorder}
	response.WriteHeader(202)
	response.WriteHeader(500)
	_, _ = response.Write([]byte("ok"))
	if response.StatusCode() != 202 || recorder.Code != 202 || response.Unwrap() != recorder {
		t.Fatal("response semantics changed")
	}
}
func TestPanicsAreLoggedAndPropagate(t *testing.T) {
	var output bytes.Buffer
	handler := Requests(slog.New(slog.NewJSONHandler(&output, nil)), http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("private panic") }))
	func() {
		defer func() {
			if recover() == nil {
				t.Error("panic swallowed")
			}
		}()
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	}()
	if !strings.Contains(output.String(), `"status":500`) || strings.Contains(output.String(), "private panic") {
		t.Fatal("panic result not logged safely")
	}
}
