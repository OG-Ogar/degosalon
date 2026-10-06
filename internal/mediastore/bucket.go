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

const MaxFileBytes int64 = 50 << 20

var ErrBucket = errors.New("media bucket is unavailable or incompatible")
var supportedMIME = []string{"image/jpeg", "image/png", "image/gif", "image/webp", "video/mp4", "video/webm", "video/quicktime"}

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
	if bucket.Limit != nil && *bucket.Limit < MaxFileBytes {
		return false
	}
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

func (s *Store) checkBucket(ctx context.Context) (missing bool, err error) {
	response, err := s.bucketRequest(ctx, http.MethodGet, "/"+Bucket, nil)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return true, nil
	}

	data, err := io.ReadAll(io.LimitReader(response.Body, (16<<10)+1))
	if err != nil || len(data) > 16<<10 {
		return false, ErrBucket
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
			return true, nil
		}
	}
	if response.StatusCode != http.StatusOK {
		return false, ErrBucket
	}
	var bucket bucketConfig
	if json.Unmarshal(data, &bucket) != nil || !compatibleBucket(bucket) {
		return false, ErrBucket
	}
	return false, nil
}

// EnsureBucket verifies an existing bucket without changing it. When create is
// true, it creates only a missing bucket and then reads back its configuration.
// A concurrent creator or a lost create acknowledgment is resolved by readback;
// private/incompatible existing buckets are never made public or overwritten.
func (s *Store) EnsureBucket(ctx context.Context, create bool) error {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	missing, err := s.checkBucket(ctx)
	if err != nil {
		return err
	}
	if !missing {
		return nil
	}
	if !create {
		return ErrBucket
	}
	limit := MaxFileBytes
	body, _ := json.Marshal(bucketConfig{ID: Bucket, Name: Bucket, Public: true, Type: "STANDARD", Limit: &limit, MIME: supportedMIME})
	response, createErr := s.bucketRequest(ctx, http.MethodPost, "", body)
	if response != nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		response.Body.Close()
	}
	// Do not trust POST success alone, nor an error body that may contain secrets.
	missing, err = s.checkBucket(ctx)
	if err != nil || missing || createErr != nil && ctx.Err() != nil {
		return ErrBucket
	}
	return nil
}
