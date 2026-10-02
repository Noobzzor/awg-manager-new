package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
	"strings"
	"time"

	"github.com/hoaxisr/awg-manager/internal/logging"
)

var slowReqLog = slog.New(slog.NewTextHandler(os.Stderr, nil)).
	With(slog.String("component", "slow-http"))

func registerPprofRoutes(mux *http.ServeMux) {
	const prefix = "/debug/pprof/"
	mux.HandleFunc(prefix, pprof.Index)
	mux.HandleFunc(prefix+"cmdline", pprof.Cmdline)
	mux.HandleFunc(prefix+"profile", pprof.Profile)
	mux.HandleFunc(prefix+"symbol", pprof.Symbol)
	mux.HandleFunc(prefix+"trace", pprof.Trace)
	mux.Handle(prefix+"goroutine", pprof.Handler("goroutine"))
	mux.Handle(prefix+"heap", pprof.Handler("heap"))
	mux.Handle(prefix+"allocs", pprof.Handler("allocs"))
	mux.Handle(prefix+"block", pprof.Handler("block"))
	mux.Handle(prefix+"mutex", pprof.Handler("mutex"))
	mux.Handle(prefix+"threadcreate", pprof.Handler("threadcreate"))
}

func skipSlowRequestLog(path string) bool {
	if strings.HasPrefix(path, "/debug/pprof") {
		return true
	}
	// "/mcp" AND everything under it: the endpoint is mounted at both "/mcp"
	// and "/mcp/", and a client that normalises the trailing slash would
	// otherwise have every long-lived Streamable-HTTP stream logged as slow.
	if path == "/mcp" || strings.HasPrefix(path, "/mcp/") {
		return true
	}
	switch path {
	case "/api/events",
		"/api/diagnostics/stream",
		"/api/singbox/subscriptions/get-stream",
		"/api/terminal/ws",
		"/api/test/speed/stream",
		"/api/system-tunnels/test-speed",
		"/api/singbox/tunnels/test/speed/stream":
		return true
	default:
		if strings.HasPrefix(path, "/api/singbox/clash") {
			return true
		}
		return false
	}
}

func redactSensitiveRequestPath(path string) string {
	const prefix = "/s/"
	if !strings.HasPrefix(path, prefix) {
		return path
	}
	remainder := strings.TrimPrefix(path, prefix)
	if separator := strings.IndexByte(remainder, '/'); separator >= 0 {
		return prefix + "[REDACTED]" + remainder[separator:]
	}
	return prefix + "[REDACTED]"
}

func (s *Server) slowRequestMiddleware(threshold time.Duration, next http.Handler) http.Handler {
	if threshold <= 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if skipSlowRequestLog(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		next.ServeHTTP(w, r)
		d := time.Since(start)
		if d < threshold {
			return
		}
		ms := float64(d.Microseconds()) / 1000
		requestPath := redactSensitiveRequestPath(r.URL.Path)
		slowReqLog.Warn("slow HTTP request",
			slog.String("method", r.Method),
			slog.String("path", requestPath),
			slog.Float64("duration_ms", ms),
		)
		if s.loggingService != nil {
			s.loggingService.AppLog(
				logging.LevelWarn,
				logging.GroupSystem,
				logging.SubProfiling,
				"slow-http",
				fmt.Sprintf("%s %s", r.Method, requestPath),
				fmt.Sprintf("%.1f ms (threshold exceeded)", ms),
			)
		}
	})
}
