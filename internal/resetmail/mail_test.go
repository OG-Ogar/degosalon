package resetmail

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"mime/quotedprintable"
	"net"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"
)

func testEnvironment() map[string]string {
	return map[string]string{"SMTP_HOST": "smtp.example.com", "SMTP_USERNAME": "relay-user", "SMTP_PASSWORD": "private-password", "SMTP_FROM": "recovery@example.com", "SITE_URL": "https://salon.example.com/"}
}
func TestConfigurationAndCanonicalLinks(t *testing.T) {
	settings := testEnvironment()
	c, err := fromEnvironment(func(key string) string { return settings[key] })
	if err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("a", 64)
	for _, role := range []string{"customer", "barber"} {
		link, err := c.ResetLink(role, token)
		if err != nil || link != "https://salon.example.com/"+role+"/reset-password?token="+token {
			t.Fatalf("link %q: %v", link, err)
		}
	}
	for _, test := range []struct{ key, value string }{
		{"SITE_URL", "http://salon.example.com"}, {"SITE_URL", "https://user:secret@salon.example.com"}, {"SITE_URL", "https://salon.example.com/path"}, {"SITE_URL", "https://salon.example.com?token=private"}, {"SITE_URL", "https://salon.example.com/#fragment"},
		{"SMTP_FROM", "Sender <recovery@example.com>"}, {"SMTP_FROM", "recovery@example.com\r\nBcc: attacker@example.com"}, {"SMTP_HOST", ""}, {"SMTP_USERNAME", ""}, {"SMTP_PASSWORD", ""}, {"SMTP_PORT", "0"}, {"SMTP_PORT", "65536"}, {"SMTP_TLS_MODE", "none"},
	} {
		t.Run(test.key+test.value, func(t *testing.T) {
			values := testEnvironment()
			values[test.key] = test.value
			_, err := fromEnvironment(func(key string) string { return values[key] })
			if !errors.Is(err, ErrConfiguration) || strings.Contains(err.Error(), "private") {
				t.Fatalf("unsafe configuration result: %v", err)
			}
		})
	}
	settings["SITE_URL"] = ""
	settings["RENDER_EXTERNAL_URL"] = "https://salon.onrender.com"
	c, err = fromEnvironment(func(key string) string { return settings[key] })
	if err != nil || c.origin != settings["RENDER_EXTERNAL_URL"] {
		t.Fatal("Render canonical fallback failed")
	}
	for _, test := range []struct{ role, token string }{{"admin", token}, {"customer", "short"}, {"barber", strings.Repeat("z", 64)}} {
		if _, err := c.ResetLink(test.role, test.token); !errors.Is(err, ErrConfiguration) {
			t.Fatal("invalid recovery link accepted")
		}
	}
}

// This relay exists only on loopback; no test contacts a mail provider.
func relay(t *testing.T, reject bool) (Config, <-chan string) {
	t.Helper()
	certificateServer := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	certificates := certificateServer.TLS.Certificates
	pool := x509.NewCertPool()
	pool.AddCert(certificateServer.Certificate())
	certificateServer.Close()
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: certificates, MinVersion: tls.VersionTLS12})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	c := Config{host: host, port: port, username: "relay-user", password: "private-password", from: "recovery@example.com", origin: "https://salon.example.com", mode: "implicit", tlsConfig: &tls.Config{RootCAs: pool, ServerName: "example.com", MinVersion: tls.VersionTLS12}}
	message := make(chan string, 1)
	go func() {
		defer close(message)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		reader := textproto.NewReader(bufio.NewReader(conn))
		_, _ = io.WriteString(conn, "220 local relay\r\n")
		for {
			line, err := reader.ReadLine()
			if err != nil {
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO"):
				_, _ = io.WriteString(conn, "250-local relay\r\n250 AUTH PLAIN\r\n")
			case strings.HasPrefix(line, "AUTH PLAIN"):
				_, _ = io.WriteString(conn, "235 authenticated\r\n")
			case strings.HasPrefix(line, "MAIL FROM"):
				_, _ = io.WriteString(conn, "250 sender accepted\r\n")
			case strings.HasPrefix(line, "RCPT TO"):
				if reject {
					_, _ = io.WriteString(conn, "550 private recipient secret-token rejected\r\n")
				} else {
					_, _ = io.WriteString(conn, "250 recipient accepted\r\n")
				}
			case line == "DATA":
				_, _ = io.WriteString(conn, "354 send data\r\n")
				body, err := reader.ReadDotBytes()
				if err != nil {
					return
				}
				message <- string(body)
				_, _ = io.WriteString(conn, "250 queued\r\n")
			case line == "QUIT":
				_, _ = io.WriteString(conn, "221 goodbye\r\n")
				return
			default:
				_, _ = io.WriteString(conn, "500 unsupported\r\n")
			}
		}
	}()
	return c, message
}
func TestTLSRelayAcceptsRecoveryAndSanitizesFailures(t *testing.T) {
	for _, reject := range []bool{false, true} {
		c, message := relay(t, reject)
		token := strings.Repeat("b", 64)
		err := c.Send(context.Background(), "person@example.com", "barber", token)
		if reject {
			if !errors.Is(err, ErrDelivery) || strings.Contains(err.Error(), "secret-token") || strings.Contains(err.Error(), "recipient") {
				t.Fatalf("unsafe delivery error: %v", err)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		wire := <-message
		parts := strings.SplitN(wire, "\n\n", 2)
		if len(parts) != 2 {
			t.Fatalf("missing MIME body: %q", wire)
		}
		body, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(parts[1])))
		if err != nil || !strings.Contains(string(body), c.origin+"/barber/reset-password?token="+token) || !strings.Contains(string(body), "30 minutes") {
			t.Fatalf("incorrect email: %q %v", body, err)
		}
		if !strings.Contains(wire, "To: person@example.com") || strings.Contains(wire, c.password) {
			t.Fatal("incorrect or unsafe headers")
		}
	}
}
func TestCancellationInterruptsStalledRelay(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	c := Config{host: host, port: port, from: "recovery@example.com", origin: "https://salon.example.com", mode: "starttls"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- c.Send(ctx, "person@example.com", "customer", strings.Repeat("a", 64)) }()
	select {
	case conn := <-accepted:
		defer conn.Close()
	case <-time.After(time.Second):
		t.Fatal("no relay connection")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation lost: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation did not interrupt SMTP greeting")
	}
}

func TestRelayCannotDowngradeToPlaintext(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	commands := make(chan string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(time.Second))
		reader := textproto.NewReader(bufio.NewReader(conn))
		_, _ = io.WriteString(conn, "220 local relay\r\n")
		_, _ = reader.ReadLine()
		_, _ = io.WriteString(conn, "250 local relay without TLS\r\n")
		line, _ := reader.ReadLine()
		commands <- line
	}()
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	c := Config{host: host, port: port, username: "user", password: "private-password", from: "recovery@example.com", origin: "https://salon.example.com", mode: "starttls"}
	if err := c.Send(context.Background(), "person@example.com", "customer", strings.Repeat("a", 64)); !errors.Is(err, ErrDelivery) {
		t.Fatalf("plaintext relay accepted: %v", err)
	}
	if command := <-commands; command != "" {
		t.Fatalf("command sent after TLS refusal: %q", command)
	}
}

func TestRelayCertificateMustBeTrusted(t *testing.T) {
	c, _ := relay(t, false)
	c.tlsConfig = nil
	if err := c.Send(context.Background(), "person@example.com", "customer", strings.Repeat("a", 64)); !errors.Is(err, ErrDelivery) {
		t.Fatalf("untrusted relay certificate accepted: %v", err)
	}
}
