package handlers

import (
	"barber-booking/storage"
	"errors"
	"math"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"
)

type authPolicy struct {
	peer, account int
	window        time.Duration
}

var authPolicies = map[string]authPolicy{
	"login": {100, 10, 15 * time.Minute}, "forgot": {30, 3, 30 * time.Minute}, "reset": {60, 10, 30 * time.Minute}, "register": {20, 5, time.Hour},
}

func authClientAddress(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return "unknown"
	}
	peer = peer.Unmap()
	var prefixes []netip.Prefix
	for _, value := range strings.Split(os.Getenv("TRUSTED_PROXY_CIDRS"), ",") {
		if prefix, err := netip.ParsePrefix(strings.TrimSpace(value)); err == nil && prefix.Bits() > 0 {
			prefixes = append(prefixes, prefix)
		}
	}
	trusted := func(ip netip.Addr) bool {
		for _, prefix := range prefixes {
			if prefix.Contains(ip) {
				return true
			}
		}
		return false
	}
	header := r.Header.Get("X-Forwarded-For")
	if !trusted(peer) || len(header) > 4096 {
		return peer.String()
	}
	hops := strings.Split(header, ",")
	if len(hops) > 20 {
		return peer.String()
	}
	current := peer
	for i := len(hops) - 1; i >= 0 && trusted(current); i-- {
		ip, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			return peer.String()
		}
		current = ip.Unmap()
	}
	return current.String()
}
func authBudget(w http.ResponseWriter, r *http.Request, key string, limit int, window time.Duration) bool {
	retry, err := storage.AllowAuthAttempt(r.Context(), key, limit, window)
	if err != nil {
		if errors.Is(err, storage.ErrThrottleCapacity) {
			journeyError(w, "Sign-in requests are busy. Please try again shortly.", http.StatusServiceUnavailable)
		} else {
			serverError(w, "check authentication request budget", err)
		}
		return false
	}
	if retry > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retry.Seconds()))))
		w.Header().Set("Cache-Control", "no-store")
		journeyError(w, "Too many attempts. Please wait before trying again.", http.StatusTooManyRequests)
		return false
	}
	return true
}
func beginAuthAttempt(w http.ResponseWriter, r *http.Request, action string) bool {
	policy := authPolicies[action]
	if !authBudget(w, r, "auth:"+action+":peer:"+authClientAddress(r), policy.peer, policy.window) {
		return false
	}
	if action != "register" {
		limitFormBody(w, r, maxMutationFormBytes)
	}
	return parseBoundedForm(w, r)
}

func authAccountAttempt(w http.ResponseWriter, r *http.Request, role, action, identity string) bool {
	policy := authPolicies[action]
	return authBudget(w, r, "auth:"+action+":account:"+role+":"+strings.ToLower(strings.TrimSpace(identity)), policy.account, policy.window)
}
