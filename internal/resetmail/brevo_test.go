package resetmail

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type brevoRoundTrip func(*http.Request) (*http.Response, error)

func (f brevoRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func brevoConfig(t *testing.T) Config {
	t.Helper()
	env := map[string]string{
		"BREVO_API_KEY": "private-test-key",
		"BREVO_FROM": "recovery@example.com",
		"RENDER_EXTERNAL_URL": "https://salon.example.com/",
	}
	c, err := fromEnvironment(func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestBrevoConfiguration(t *testing.T) {
	c := brevoConfig(t)
	if c.origin != "https://salon.example.com" || c.from != "recovery@example.com" {
		t.Fatal("incorrect Brevo configuration")
	}
	for _, tc := range []struct{ key, value string }{
		{"BREVO_FROM", ""},
		{"BREVO_FROM", "Name <recovery@example.com>"},
		{"BREVO_API_KEY", "key\r\ninjection"},
		{"BREVO_API_KEY", strings.Repeat("x", 4097)},
		{"SITE_URL", "http://salon.example.com"},
	} {
		t.Run(tc.key+tc.value[:min(len(tc.value), 12)], func(t *testing.T) {
			env := testEnvironment()
			env["BREVO_API_KEY"] = "private-test-key"
			env["BREVO_FROM"] = "recovery@example.com"
			env[tc.key] = tc.value
			if _, err := fromEnvironment(func(k string) string { return env[k] }); !errors.Is(err, ErrConfiguration) {
				t.Fatal("invalid Brevo configuration must not fall back to SMTP")
			}
		})
	}
}

func TestBrevoAcceptedRequest(t *testing.T) {
	for _, role := range []string{"barber", "customer"} {
		t.Run(role, func(t *testing.T) {
			c := brevoConfig(t)
			token := strings.Repeat("a", 64)
			c.httpClient = &http.Client{Transport: brevoRoundTrip(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodPost || r.URL.String() != brevoEndpoint ||
					r.Header.Get("api-key") != c.apiKey || r.Header.Get("Content-Type") != "application/json" {
					t.Fatal("incorrect authenticated API request")
				}
				deadline, ok := r.Context().Deadline()
				if !ok || time.Until(deadline) > 10*time.Second {
					t.Fatal("delivery must have a bounded deadline")
				}
				var msg brevoMessage
				if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
					t.Fatal(err)
				}
				if msg.Sender.Email != c.from || len(msg.To) != 1 || msg.To[0].Email != "customer@example.com" ||
					msg.Subject != "Reset your password" ||
					!strings.Contains(msg.TextContent, "https://salon.example.com/"+role+"/reset-password?token="+token) ||
					!strings.Contains(msg.TextContent, "30 minutes") {
					t.Fatal("incorrect password recovery payload")
				}
				return &http.Response{StatusCode: 201, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"messageId":"accepted"}`))}, nil
			})}
			if err := c.Send(context.Background(), "customer@example.com", role, token); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBrevoRejectsFailuresAndRedirects(t *testing.T) {
	for _, tc := range []struct {
		name string
		status int
		body string
	}{
		{"unauthorized", 401, "private-provider-details"},
		{"rate limited", 429, "private-provider-details"},
		{"server error", 500, "private-provider-details"},
		{"redirect", 302, ""},
		{"unexpected success", 200, `{"messageId":"accepted"}`},
		{"invalid receipt", 201, "private-provider-details"},
		{"missing id", 201, `{}`},
		{"empty id", 201, `{"messageId":" "}`},
		{"oversized receipt", 201, strings.Repeat("x", (64<<10)+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := brevoConfig(t)
			calls := 0
			c.httpClient = &http.Client{Transport: brevoRoundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: tc.status, Header: http.Header{"Location": []string{"https://other.example.com/"}},
					Body: io.NopCloser(strings.NewReader(tc.body)), Request: r}, nil
			})}
			err := c.Send(context.Background(), "customer@example.com", "customer", strings.Repeat("a", 64))
			if err != ErrDelivery || calls != 1 {
				t.Fatalf("failure must be sanitized and redirects refused: %v; calls=%d", err, calls)
			}
		})
	}
}

type brevoBrokenBody struct{}

func (brevoBrokenBody) Read([]byte) (int, error) { return 0, errors.New("private-provider-details") }
func (brevoBrokenBody) Close() error { return nil }

func TestBrevoReadFailureAndCancellation(t *testing.T) {
	c := brevoConfig(t)
	c.httpClient = &http.Client{Transport: brevoRoundTrip(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 201, Body: brevoBrokenBody{}, Header: make(http.Header)}, nil
	})}
	if err := c.Send(context.Background(), "customer@example.com", "customer", strings.Repeat("a", 64)); err != ErrDelivery {
		t.Fatalf("incomplete acceptance receipt must fail safely: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.httpClient = &http.Client{Transport: brevoRoundTrip(func(r *http.Request) (*http.Response, error) {
		cancel()
		<-r.Context().Done()
		return nil, errors.New("private-provider-details")
	})}
	defer cancel()
	err := c.Send(ctx, "customer@example.com", "customer", strings.Repeat("a", 64))
	if !errors.Is(err, context.Canceled) || !errors.Is(err, ErrDelivery) || strings.Contains(err.Error(), "private-provider-details") {
		t.Fatalf("cancellation must be retained without provider details: %v", err)
	}
}
