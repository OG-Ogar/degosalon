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

// MaxUploadDuration bounds how long a single object may take to reach storage.
// A large clip on a slow mobile connection needs far longer than the 45-second
// control-plane timeout.
const MaxUploadDuration = 30 * time.Minute

var ErrConfiguration = errors.New("media storage is not configured correctly")
var ErrUpload = errors.New("media storage upload failed; please retry")
var ErrFileTooLarge = errors.New("media file exceeds the storage limit")

// uploadTimeout allows a modest 256 KiB/s while never returning less than half
// a minute for a small note.
func uploadTimeout(size int64) time.Duration {
	window := 30*time.Second + time.Duration(size/(256<<10))*time.Second
	if window > MaxUploadDuration {
		return MaxUploadDuration
	}
	return window
}

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
		MaxIdleConns:          16,
		MaxIdleConnsPerHost:   16,
		IdleConnTimeout:       90 * time.Second,
	}
	// No client-wide Timeout: each operation supplies its own context deadline.
	// A fixed timeout would kill a large upload while it is still streaming.
	return &Store{base: strings.TrimRight(base, "/"), key: key, client: &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

// Close releases pooled connections when this upload client is no longer used.
func (s *Store) Close() { s.client.CloseIdleConnections() }

func validPath(path string) bool {
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		return false
	}
	switch parts[0] {
	case "hairstyles", "hairstyle-videos", "products", "product-videos", "barbers", "customers", "site",
		"chat", "chat-voice", "chat-video":
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
	if !validPath(path) || size <= 0 || size > MaxFileBytes || !(strings.HasPrefix(kind, "image/") || strings.HasPrefix(kind, "video/") || strings.HasPrefix(kind, "audio/")) {
		return ErrConfiguration
	}
	ctx, cancel := context.WithTimeout(ctx, uploadTimeout(size))
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
		// Recognize only a size rejection and discard the provider text.
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		message := strings.ToLower(string(detail))
		if resp.StatusCode == http.StatusRequestEntityTooLarge ||
			strings.Contains(message, "maximum allowed size") ||
			strings.Contains(message, "payload too large") {
			return ErrFileTooLarge
		}
		return ErrUpload
	}
	if _, err := io.Copy(io.Discard, io.LimitReader(resp.Body, 4096)); err != nil || ctx.Err() != nil {
		return ErrUpload
	}
	return nil
}

var ErrDelete = errors.New("media storage cleanup failed")
var ownedName = regexp.MustCompile(`^([a-f0-9]{32}|[0-9]{10,20})\.(jpg|jpeg|png|gif|webp|mp4|webm|mov|m4v|3gp|m4a|ogg|oga|wav)$`)

func mediaKind(extension string) string {
	switch extension {
	case "mp4", "webm", "mov", "m4v", "3gp":
		return "video"
	case "m4a", "ogg", "oga", "wav":
		return "audio"
	default:
		return "image"
	}
}

// Owns accepts only canonical URLs in this project's application bucket and
// known directories, with an application-generated or legacy timestamp name.
// The directory decides the media kind, so a photo cannot pose as a voice note.
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
	switch parts[0] {
	case "hairstyle-videos", "product-videos", "chat-video":
		return mediaKind(extension) == "video"
	case "chat-voice":
		return mediaKind(extension) == "audio"
	default:
		return mediaKind(extension) == "image"
	}
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
	if _, err := io.Copy(io.Discard, io.LimitReader(response.Body, 4096)); err != nil || ctx.Err() != nil {
		return ErrDelete
	}
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
	switch parts[0] {
	case "hairstyle-videos", "product-videos":
		return path, video
	case "chat-voice", "chat-video":
		// Chat attachments are not part of the legacy catalogue migration;
		// they are written straight to the media store by the new upload path.
		return "", false
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
