package storage

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestAuthThrottle(t *testing.T) { Reset(); t.Cleanup(Reset); checkAuthThrottle(t) }
func checkAuthThrottle(t *testing.T) {
	ctx := context.Background()
	key := "concurrent:" + t.Name()
	var wg sync.WaitGroup
	results := make(chan time.Duration, 30)
	failures := make(chan error, 30)
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			retry, err := AllowAuthAttempt(ctx, key, 5, time.Minute)
			results <- retry
			failures <- err
		}()
	}
	wg.Wait()
	close(results)
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	accepted := 0
	for retry := range results {
		if retry == 0 {
			accepted++
		} else if retry <= 0 || retry > time.Minute {
			t.Fatalf("retry %v", retry)
		}
	}
	if accepted != 5 {
		t.Fatalf("accepted %d, want 5", accepted)
	}
	if retry, err := AllowAuthAttempt(ctx, key+":other", 5, time.Minute); err != nil || retry != 0 {
		t.Fatalf("isolated key: %v %v", retry, err)
	}
	digest := throttleDigest(key)
	if databaseEnabled() {
		if _, err := db.Exec("update degosalon_internal.auth_throttles set expires_at=now()-interval '1 second' where key_digest=$1", digest); err != nil {
			t.Fatal(err)
		}
	} else {
		mu.Lock()
		bucket := authBuckets[digest]
		bucket.expires = time.Now().Add(-time.Second)
		authBuckets[digest] = bucket
		mu.Unlock()
	}
	if retry, err := AllowAuthAttempt(ctx, key, 5, time.Minute); err != nil || retry != 0 {
		t.Fatalf("expired budget: %v %v", retry, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := AllowAuthAttempt(cancelled, key, 5, time.Minute); err == nil {
		t.Fatal("cancelled operation succeeded")
	}
}

func TestMemoryAuthThrottleCapacity(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	now := time.Now()
	for i := 0; i < 10000; i++ {
		authBuckets[fmt.Sprint(i)] = authBucket{attempts: 1, expires: now.Add(time.Minute)}
	}
	if _, err := AllowAuthAttempt(context.Background(), "new", 1, time.Minute); !errors.Is(err, ErrThrottleCapacity) {
		t.Fatalf("unbounded counters: %v", err)
	}
	for key, bucket := range authBuckets {
		bucket.expires = now.Add(-time.Second)
		authBuckets[key] = bucket
		break
	}
	if retry, err := AllowAuthAttempt(context.Background(), "new", 1, time.Minute); err != nil || retry != 0 {
		t.Fatalf("expired capacity recovery: %v %v", retry, err)
	}
}
