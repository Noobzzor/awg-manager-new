// Package env provides typed env-var helpers with safe defaults.
//
// Both helpers fall back to the provided default on any parse error or
// out-of-range value, logging a WARN line via the stdlib log package so
// the daemon's process log surfaces operator typos in init.d scripts.
// They are called from main.go BEFORE the project's logging service is
// initialised, hence the use of stdlib log instead of internal/logging.
package env

import (
	"log"
	"os"
	"strconv"
	"time"
)

// IntDefault returns the integer value of env var key. Falls back to def
// when the variable is unset, empty, non-numeric, zero, or negative.
// Logs a WARN for explicit-invalid values (non-numeric, negative); the
// unset/empty/zero paths fall back silently to avoid log noise on
// fresh installations.
func IntDefault(key string, def int) int {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		log.Printf("env: %s=%q is not a valid integer, using default %d", key, raw, def)
		return def
	}
	if n <= 0 {
		if n < 0 {
			log.Printf("env: %s=%d is non-positive, using default %d", key, n, def)
		}
		return def
	}
	return n
}

// DurationDefault returns the time.Duration value of env var key.
// Accepts standard Go duration syntax ("2s", "500ms", "0s", "1m30s").
// Falls back to def on parse error. Zero is allowed — it is a meaningful
// "feature off" value for callers like SaveCoordinator settle delay.
func DurationDefault(key string, def time.Duration) time.Duration {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		log.Printf("env: %s=%q is not a valid duration, using default %s", key, raw, def)
		return def
	}
	if d < 0 {
		log.Printf("env: %s=%s is negative, using default %s", key, d, def)
		return def
	}
	return d
}
