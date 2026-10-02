package storage

import "strings"

// DefaultSingboxLogLevel is "info": the old "trace" default self-inflicted
// hundreds of journal lines per minute on low-RAM routers. Explicitly
// configured "trace" keeps working.
const DefaultSingboxLogLevel = "info"

var validSingboxLogLevels = map[string]struct{}{
	"trace": {},
	"debug": {},
	"info":  {},
	"warn":  {},
	"error": {},
	"fatal": {},
	"panic": {},
}

func NormalizeSingboxLogLevel(v string) string {
	normalized := strings.ToLower(strings.TrimSpace(v))
	if _, ok := validSingboxLogLevels[normalized]; ok {
		return normalized
	}
	return DefaultSingboxLogLevel
}

func IsValidSingboxLogLevel(v string) bool {
	normalized := strings.ToLower(strings.TrimSpace(v))
	_, ok := validSingboxLogLevels[normalized]
	return ok
}
