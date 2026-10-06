// Package mediastore provides bounded, server-only object uploads.
package mediastore

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const Bucket = "degosalon-media"
const Timeout = 45 * time.Second

var ErrConfiguration = errors.New("media storage is not configured correctly")
var ErrUpload = errors.New("media storage upload failed; please retry")

// Store never follows redirects: an apikey header must not reach another host.
type Store struct {
	base, key string
	client    *http.Client
}

// New requires HTTPS, an origin-only URL and a server credential. Secret API
// keys use apikey only; legacy JWT credentials also use Authorization.
func New(base, key string) (*Store, error) {
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawPath != "" || strings.TrimSpace(key) != key || key == "" || len(key) > 4096 || strings.ContainsAny(key, "\r\n") {
		return nil, ErrConfiguration
	}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		MaxIdleConns:          1,
		IdleConnTimeout:       30 * time.Second,
	}
	return &Store{base: strings.TrimRight(base, "/"), key: key, client: &http.Client{Transport: transport, Timeout: Timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

// Close releases pooled connections when this upload client is no longer used.
func (s *Store) Close() { s.client.CloseIdleConnections() }

func validPath(path string) bool {
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		return false
	}
	switch parts[0] {
	case "hairstyles", "hairstyle-videos", "products", "barbers", "customers", "site":
	default:
		return false
	}
	if len(parts[1]) == 0 || len(parts[1]) > 128 || parts[1] == "." || parts[1] == ".." {
		return false
	}
	for _, c := range parts[1] {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '.' || c == '-') {
			return false
		}
	}
	return true
}

func (s *Store) PublicURL(path string) (string, error) {
	if !validPath(path) {
		return "", ErrConfiguration
	}
	return s.base + "/storage/v1/object/public/" + Bucket + "/" + path, nil
}

// Upload streams an already validated file without loading a video into memory.
// Random caller-generated object names and disabled upsert prevent overwrite.
func (s *Store) Upload(ctx context.Context, path, kind string, size int64, body io.Reader) error {
	if !validPath(path) || size <= 0 || size > 50<<20 || !(strings.HasPrefix(kind, "image/") || strings.HasPrefix(kind, "video/")) {
		return ErrConfiguration
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.base+"/storage/v1/object/"+Bucket+"/"+path, io.LimitReader(body, size))
	if err != nil {
		return ErrUpload
	}
	req.ContentLength = size
	req.Header.Set("apikey", s.key)
	if !strings.HasPrefix(s.key, "sb_secret_") {
		req.Header.Set("Authorization", "Bearer "+s.key)
	}
	req.Header.Set("Content-Type", kind)
	req.Header.Set("x-upsert", "false")
	resp, err := s.client.Do(req)
	if err != nil {
		return ErrUpload
	}
	defer resp.Body.Close()
	// Do not return provider bodies, credentials, URLs, or low-level errors.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ErrUpload
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	return nil
}
