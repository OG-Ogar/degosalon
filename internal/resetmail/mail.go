// Package resetmail delivers password recovery messages over authenticated TLS.
// Wire errors are deliberately replaced: SMTP replies can contain private data.
package resetmail

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	ErrConfiguration = errors.New("reset email configuration is incomplete or invalid")
	ErrDelivery      = errors.New("reset email delivery failed")
)

type Config struct {
	host, port, username, password, from, origin, mode string
	tlsConfig                                          *tls.Config
}

func FromEnvironment() (Config, error) { return fromEnvironment(os.Getenv) }

// DeliveryKeyFromEnvironment derives an AES-256 key from a persistent server
// secret. Keep it stable across instances/restarts until outstanding jobs expire.
func DeliveryKeyFromEnvironment() ([]byte, error) {
	secret := os.Getenv("RESET_MAIL_KEY")
	if len(secret) < 32 || len(secret) > 1024 || strings.TrimSpace(secret) != secret {
		return nil, ErrConfiguration
	}
	sum := sha256.Sum256([]byte(secret))
	return sum[:], nil
}

func fromEnvironment(getenv func(string) string) (Config, error) {
	c := Config{host: getenv("SMTP_HOST"), port: getenv("SMTP_PORT"), username: getenv("SMTP_USERNAME"), password: getenv("SMTP_PASSWORD"), from: getenv("SMTP_FROM"), origin: getenv("SITE_URL"), mode: getenv("SMTP_TLS_MODE")}
	if c.origin == "" {
		c.origin = getenv("RENDER_EXTERNAL_URL")
	}
	if c.port == "" {
		c.port = "587"
	}
	if c.mode == "" {
		c.mode = "starttls"
	}
	port, err := strconv.Atoi(c.port)
	if err != nil || port < 1 || port > 65535 || c.host == "" || strings.ContainsAny(c.host, " /\r\n\t") || c.username == "" || c.password == "" || !validAddress(c.from) || (c.mode != "starttls" && c.mode != "implicit") {
		return Config{}, ErrConfiguration
	}
	u, err := url.Parse(c.origin)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || (u.Path != "" && u.Path != "/") || u.Opaque != "" {
		return Config{}, ErrConfiguration
	}
	u.Path = ""
	c.origin = u.String()
	return c, nil
}

func validAddress(value string) bool {
	if strings.ContainsAny(value, "\r\n") || len(value) > 254 {
		return false
	}
	a, err := mail.ParseAddress(value)
	return err == nil && a.Address == value && a.Name == ""
}

// ResetLink always uses configured canonical origin, never request Host headers.
func (c Config) ResetLink(role, token string) (string, error) {
	if role != "customer" && role != "barber" {
		return "", ErrConfiguration
	}
	b, err := hex.DecodeString(token)
	if err != nil || len(b) != 32 || len(token) != 64 || c.origin == "" {
		return "", ErrConfiguration
	}
	return c.origin + "/" + role + "/reset-password?token=" + token, nil
}

// Send returns only after the relay has accepted DATA. Delivery to an inbox is
// the provider's responsibility. Cancellation closes the socket immediately.
func (c Config) Send(ctx context.Context, recipient, role, token string) error {
	link, err := c.ResetLink(role, token)
	if err != nil || !validAddress(recipient) {
		return ErrConfiguration
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(c.host, c.port))
	if err != nil {
		return deliveryError(ctx)
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		return deliveryError(ctx)
	}
	tlsSettings := &tls.Config{ServerName: c.host, MinVersion: tls.VersionTLS12}
	if c.tlsConfig != nil {
		tlsSettings = c.tlsConfig.Clone()
	}
	if c.mode == "implicit" {
		tlsConn := tls.Client(conn, tlsSettings)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return deliveryError(ctx)
		}
		conn = tlsConn
	}
	client, err := smtp.NewClient(conn, c.host)
	if err != nil {
		return deliveryError(ctx)
	}
	defer client.Close()
	if c.mode == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return ErrDelivery
		}
		if err := client.StartTLS(tlsSettings); err != nil {
			return deliveryError(ctx)
		}
	}
	if err := client.Auth(smtp.PlainAuth("", c.username, c.password, c.host)); err != nil {
		return deliveryError(ctx)
	}
	if err := client.Mail(c.from); err != nil {
		return deliveryError(ctx)
	}
	if err := client.Rcpt(recipient); err != nil {
		return deliveryError(ctx)
	}
	w, err := client.Data()
	if err != nil {
		return deliveryError(ctx)
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return ErrDelivery
	}
	_, err = fmt.Fprintf(w, "From: %s\r\nTo: %s\r\nSubject: Reset your password\r\nDate: %s\r\nMessage-ID: <%s@%s>\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n", c.from, recipient, time.Now().UTC().Format(time.RFC1123Z), hex.EncodeToString(id), strings.Split(c.from, "@")[1])
	if err != nil {
		return deliveryError(ctx)
	}
	body := quotedprintable.NewWriter(w)
	if _, err := io.WriteString(body, "A password reset was requested for your account.\r\n\r\n"+link+"\r\n\r\nThis link expires after 30 minutes and works once. If you did not request this, you can ignore this message.\r\n"); err != nil {
		return deliveryError(ctx)
	}
	if err := body.Close(); err != nil {
		return deliveryError(ctx)
	}
	if err := w.Close(); err != nil {
		return deliveryError(ctx)
	}
	// Acceptance has already succeeded. A failed QUIT must not cause a retry.
	_ = client.Quit()
	return nil
}

func deliveryError(ctx context.Context) error {
	if ctx.Err() != nil {
		return errors.Join(ErrDelivery, ctx.Err())
	}
	return ErrDelivery
}
