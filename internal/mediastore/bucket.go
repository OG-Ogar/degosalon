package mediastore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	// MaxFileBytes is the largest attachment the application accepts and the
	// bucket size requested when a new bucket is created.
	MaxFileBytes int64 = 500 << 20
	// minBucketBytes is the smallest existing bucket limit that may start the
	// application. The requested limit is MaxFileBytes, but a lower
	// project-wide cap (50 MB on Supabase's free plan, for example) must not
	// stop a deployment; an upload above the real bucket limit fails clearly.
	minBucketBytes int64 = 1 << 20
)

var ErrBucket = errors.New("media bucket is unavailable or incompatible")
var supportedMIME = []string{
	"image/jpeg", "image/png", "image/gif", "image/webp",
	"video/mp4", "video/webm", "video/quicktime",
	"audio/webm", "audio/ogg", "audio/mp4", "audio/mpeg", "audio/wav",
}

type bucketConfig struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Public bool     `json:"public"`
	Type   string   `json:"type,omitempty"`
	Limit  *int64   `json:"file_size_limit"`
	MIME   []string `json:"allowed_mime_types"`
}

// FromEnvironment uses only server credentials. An absent origin is a
// configuration error here; development local-file behavior is chosen by the
// application before calling this constructor.
func FromEnvironment() (*Store, error) {
	key := strings.TrimSpace(os.Getenv("SUPABASE_SECRET_KEY"))
	if key == "" {
		key = strings.TrimSpace(os.Getenv("SUPABASE_SERVICE_ROLE_KEY"))
	}
	return New(strings.TrimSpace(os.Getenv("SUPABASE_URL")), key)
}

func compatibleBucket(bucket bucketConfig) bool {
	if bucket.ID != Bucket || bucket.Name != Bucket || !bucket.Public || (bucket.Type != "" && bucket.Type != "STANDARD") {
		return false
	}
	// The size limit is checked by bucketShapeMatches. It is deliberately not
	// tied to MaxFileBytes so a project-wide cap cannot block startup.
	// Null/empty means unrestricted at the provider. Otherwise every application
	// type must be allowed, including a supported image/* or video/* wildcard.
	if len(bucket.MIME) > 0 {
		for _, kind := range supportedMIME {
			permitted := false
			for _, allowed := range bucket.MIME {
				if allowed == kind || allowed == strings.SplitN(kind, "/", 2)[0]+"/*" || allowed == "*/*" {
					permitted = true
					break
				}
			}
			if !permitted {
				return false
			}
		}
	}
	return true
}

func (s *Store) bucketRequest(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, s.base+"/storage/v1/bucket"+path, bytes.NewReader(body))
	if err != nil {
		return nil, ErrBucket
	}
	req.Header.Set("apikey", s.key)
	if !strings.HasPrefix(s.key, "sb_secret_") {
		req.Header.Set("Authorization", "Bearer "+s.key)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := s.client.Do(req)
	if err != nil {
		return nil, ErrBucket
	}
	return response, nil
}

func (s *Store) checkBucket(ctx context.Context) (bucketConfig, bool, error) {
	response, err := s.bucketRequest(ctx, http.MethodGet, "/"+Bucket, nil)
	if err != nil {
		return bucketConfig{}, false, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return bucketConfig{}, true, nil
	}

	data, err := io.ReadAll(io.LimitReader(response.Body, (16<<10)+1))
	if err != nil || len(data) > 16<<10 {
		return bucketConfig{}, false, ErrBucket
	}
	if response.StatusCode == http.StatusBadRequest {
		// Some Storage API versions wrap NoSuchBucket in HTTP 400. Recognize
		// only that structured error; authentication/validation failures must
		// never be mistaken for a missing bucket.
		var failure struct {
			Code   string          `json:"code"`
			Status json.RawMessage `json:"statusCode"`
			Error  string          `json:"error"`
		}
		if json.Unmarshal(data, &failure) == nil && (string(failure.Status) == `"404"` || string(failure.Status) == "404") && (failure.Code == "NoSuchBucket" || failure.Code == "" && failure.Error == "Bucket not found") {
			return bucketConfig{}, true, nil
		}
	}
	if response.StatusCode != http.StatusOK {
		return bucketConfig{}, false, ErrBucket
	}
	var bucket bucketConfig
	if json.Unmarshal(data, &bucket) != nil {
		return bucketConfig{}, false, ErrBucket
	}
	return bucket, false, nil
}

// bucketShapeMatches reports whether an existing bucket belongs to this
// application and is public enough to use. MIME is checked separately because
// an older bucket may simply predate a newly supported media type. A null
// limit is unrestricted; otherwise even a provider-capped limit is accepted as
// long as it can hold a small attachment.
func bucketShapeMatches(bucket bucketConfig) bool {
	if bucket.ID != Bucket || bucket.Name != Bucket || !bucket.Public || (bucket.Type != "" && bucket.Type != "STANDARD") {
		return false
	}
	return bucket.Limit == nil || *bucket.Limit >= minBucketBytes
}

// widenedMIME returns the bucket's allowlist with every application media type
// added. Existing entries are kept, so this only ever widens what may be
// uploaded; it never removes an operator's own additions.
func widenedMIME(current []string) []string {
	allowed := map[string]bool{}
	for _, kind := range current {
		allowed[kind] = true
	}
	widened := append([]string{}, current...)
	for _, kind := range supportedMIME {
		if allowed[kind] {
			continue
		}
		allowed[kind] = true
		widened = append(widened, kind)
	}
	return widened
}

// EnsureBucket verifies an existing bucket without changing it, except for one
// additive repair: a bucket that predates voice notes has no audio types in its
// allowlist, so those types are appended. Nothing is ever narrowed, made
// public, or overwritten. When create is true, only a missing bucket is created
// and then read back; a concurrent creator or lost acknowledgment is resolved
// by readback.
func (s *Store) EnsureBucket(ctx context.Context, create bool) error {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	bucket, missing, err := s.checkBucket(ctx)
	if err != nil {
		return err
	}
	if !missing {
		if compatibleBucket(bucket) && bucketShapeMatches(bucket) {
			return nil
		}
		if !bucketShapeMatches(bucket) {
			return ErrBucket
		}
		return s.widenBucketMIME(ctx, bucket)
	}
	if !create {
		return ErrBucket
	}

	// Do not trust POST success alone, nor an error body that may contain
	// secrets. A project-wide file size cap lower than MaxFileBytes can make
	// the first request fail, so retry without a per-bucket limit; the bucket
	// then inherits the project maximum.
	limit := MaxFileBytes
	response, createErr := s.bucketRequest(ctx, http.MethodPost, "", bucketCreatePayload(&limit))
	createFailed := createErr != nil || response == nil || response.StatusCode < 200 || response.StatusCode >= 300
	closeBucketResponse(response)

	bucket, missing, err = s.checkBucket(ctx)
	if err != nil {
		return ErrBucket
	}
	if missing && createFailed {
		response, _ = s.bucketRequest(ctx, http.MethodPost, "", bucketCreatePayload(nil))
		closeBucketResponse(response)

		bucket, missing, err = s.checkBucket(ctx)
		if err != nil || missing {
			return ErrBucket
		}
	}
	if missing {
		return ErrBucket
	}
	if !compatibleBucket(bucket) || !bucketShapeMatches(bucket) {
		return ErrBucket
	}
	return nil
}

func bucketCreatePayload(limit *int64) []byte {
	payload := map[string]any{
		"id":                 Bucket,
		"name":               Bucket,
		"public":             true,
		"type":               "STANDARD",
		"allowed_mime_types": supportedMIME,
	}
	if limit != nil {
		payload["file_size_limit"] = *limit
	}
	body, _ := json.Marshal(payload)
	return body
}

func closeBucketResponse(response *http.Response) {
	if response == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	response.Body.Close()
}

// widenBucketMIME adds the media types an older bucket is missing, then reads
// the bucket back. The verified visibility is sent alongside so the update
// cannot reset it, and a lower size limit is raised to MaxFileBytes when the
// project allows it. Nothing is ever narrowed; if the provider rejects the
// larger limit, the repair is retried at the existing limit so an older
// deployment still gets the MIME fix it needs.
func (s *Store) widenBucketMIME(ctx context.Context, bucket bucketConfig) error {
	update := map[string]any{
		"public":             bucket.Public,
		"allowed_mime_types": widenedMIME(bucket.MIME),
	}
	if bucket.Limit != nil {
		update["file_size_limit"] = *bucket.Limit
	}
	raised := false
	if bucket.Limit != nil && *bucket.Limit < MaxFileBytes {
		update["file_size_limit"] = MaxFileBytes
		raised = true
	}
	if s.updateBucket(ctx, update) != nil {
		if !raised {
			return ErrBucket
		}
		update["file_size_limit"] = *bucket.Limit
		if s.updateBucket(ctx, update) != nil {
			return ErrBucket
		}
	}

	updated, missing, err := s.checkBucket(ctx)
	if err != nil || missing || !compatibleBucket(updated) || !bucketShapeMatches(updated) {
		return ErrBucket
	}
	return nil
}

func (s *Store) updateBucket(ctx context.Context, update map[string]any) error {
	body, _ := json.Marshal(update)
	response, err := s.bucketRequest(ctx, http.MethodPut, "/"+Bucket, body)
	closeBucketResponse(response)
	if err != nil || response == nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return ErrBucket
	}
	return nil
}
