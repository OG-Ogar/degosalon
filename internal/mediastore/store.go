// Package mediastore provides bounded, server-only object uploads.
package mediastore

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
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

var ErrDelete = errors.New("media storage cleanup failed")
var ownedName = regexp.MustCompile(`^([a-f0-9]{32}|[0-9]{10,20})\.(jpg|jpeg|png|gif|webp|mp4|webm|mov)$`)

// Owns accepts only canonical URLs in this project's application bucket and
// known directories, with an application-generated or legacy timestamp name.
func (s *Store) Owns(uri string) bool {
	prefix := s.base + "/storage/v1/object/public/" + Bucket + "/"
	if !strings.HasPrefix(uri, prefix) {
		return false
	}
	path := strings.TrimPrefix(uri, prefix)
	if !validPath(path) {
		return false
	}
	parts := strings.Split(path, "/")
	if !ownedName.MatchString(parts[1]) {
		return false
	}
	extension := parts[1][strings.LastIndex(parts[1], ".")+1:]
	video := extension == "mp4" || extension == "webm" || extension == "mov"
	return video == (parts[0] == "hairstyle-videos")
}

// Delete removes exactly one owned object through the Storage API. A successful
// response for an already absent object is idempotent; bucket/auth failures are
// retained as errors rather than treated as proof of deletion.
func (s *Store) Delete(ctx context.Context, uri string) error {
	if !s.Owns(uri) {
		return ErrConfiguration
	}
	path := strings.TrimPrefix(uri, s.base+"/storage/v1/object/public/"+Bucket+"/")
	body, _ := json.Marshal(struct {
		Prefixes []string `json:"prefixes"`
	}{[]string{path}})
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, s.base+"/storage/v1/object/"+Bucket, bytes.NewReader(body))
	if err != nil {
		return ErrDelete
	}
	req.Header.Set("apikey", s.key)
	if !strings.HasPrefix(s.key, "sb_secret_") {
		req.Header.Set("Authorization", "Bearer "+s.key)
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(req)
	if err != nil {
		return ErrDelete
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ErrDelete
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	return nil
}

// LegacyObjectPath recognizes canonical local upload references. It only names
// a single file in an application directory; query strings, escapes, traversal
// and unsupported filename/content types are excluded.
func LegacyObjectPath(uri string) (string, bool) {
	if !strings.HasPrefix(uri, "/uploads/") {
		return "", false
	}
	path := strings.TrimPrefix(uri, "/uploads/")
	if !validPath(path) {
		return "", false
	}
	parts := strings.Split(path, "/")
	at := strings.LastIndex(parts[1], ".")
	if at < 1 {
		return "", false
	}
	extension := parts[1][at+1:]
	video := extension == "mp4" || extension == "webm" || extension == "mov"
	image := extension == "jpg" || extension == "jpeg" || extension == "png" || extension == "gif" || extension == "webp"
	if parts[0] == "hairstyle-videos" {
		return path, video
	}
	return path, image
}

// ReadPublicImage fetches only an owned immutable image. No credentials are sent,
// redirects are refused, and both transport time and response bytes are bounded.
func (s *Store) ReadPublicImage(ctx context.Context, uri string) ([]byte, error) {
	if !s.Owns(uri) || strings.HasSuffix(uri, ".mp4") || strings.HasSuffix(uri, ".webm") || strings.HasSuffix(uri, ".mov") {
		return nil, ErrConfiguration
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, ErrConfiguration
	}
	response, err := s.client.Do(req)
	if err != nil {
		return nil, ErrUpload
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.ContentLength > 5<<20 {
		return nil, ErrUpload
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, (5<<20)+1))
	if err != nil || len(data) > 5<<20 {
		return nil, ErrUpload
	}
	return data, nil
}
