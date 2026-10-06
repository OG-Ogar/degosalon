package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"
)

type authBucket struct {
	attempts int
	expires  time.Time
}

var authBuckets = map[string]authBucket{}
var ErrThrottleCapacity = errors.New("authentication throttle capacity reached")

func throttleDigest(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// AllowAuthAttempt applies a fixed-window budget atomically across instances.
// A positive retry duration means the attempt is denied. Only digests of peer
// addresses/account identifiers are stored; errors never fall back to memory.
func AllowAuthAttempt(ctx context.Context, key string, limit int, window time.Duration) (time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if key == "" || limit < 1 || limit > 10000 || window < time.Second || window > 24*time.Hour {
		return 0, errors.New("invalid authentication throttle policy")
	}
	digest := throttleDigest(key)
	now := time.Now()
	if databaseEnabled() {
		// Recheck expiry in the outer DELETE so a concurrent reset of an expired
		// counter cannot be erased after its UPSERT commits.
		if _, err := db.ExecContext(ctx, "delete from degosalon_internal.auth_throttles where expires_at<=now() and key_digest in (select key_digest from degosalon_internal.auth_throttles where expires_at<=now() order by expires_at limit 100)"); err != nil {
			return 0, err
		}
		var count int
		var expires time.Time
		err := db.QueryRowContext(ctx, `insert into degosalon_internal.auth_throttles(key_digest,attempts,expires_at) values($1,1,now()+($2*interval '1 second'))
   on conflict(key_digest) do update set attempts=case when auth_throttles.expires_at<=now() then 1 else least(auth_throttles.attempts+1,$3) end,
   expires_at=case when auth_throttles.expires_at<=now() then excluded.expires_at else auth_throttles.expires_at end returning attempts,expires_at`, digest, int64(window/time.Second), limit+1).Scan(&count, &expires)
		if err != nil {
			return 0, err
		}
		if count > limit {
			retry := time.Until(expires)
			if retry < time.Second {
				retry = time.Second
			}
			return retry, nil
		}
		return 0, nil
	}
	mu.Lock()
	defer mu.Unlock()
	removed := 0
	for k, v := range authBuckets {
		if !now.Before(v.expires) {
			delete(authBuckets, k)
			removed++
			if removed == 100 {
				break
			}
		}
	}
	bucket, found := authBuckets[digest]
	if !found || !now.Before(bucket.expires) {
		if !found && len(authBuckets) >= 10000 {
			return 0, ErrThrottleCapacity
		}
		bucket = authBucket{expires: now.Add(window)}
	}
	if bucket.attempts < limit+1 {
		bucket.attempts++
	}
	authBuckets[digest] = bucket
	if bucket.attempts > limit {
		return bucket.expires.Sub(now), nil
	}
	return 0, nil
}
