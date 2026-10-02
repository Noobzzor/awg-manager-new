package transport

import (
	"net/http"
	"time"
)

// sharedTransport is the HTTP transport reused across all RCI Client
// instances. Pool size synchronised with cap=30 default of the NDMS
// concurrency semaphore (cmd/awg-manager/main.go): MaxIdleConnsPerHost
// must exceed cap so that the 30 simultaneous in-flight requests never
// pay TCP-handshake cost on a hot keep-alive pool.
var sharedTransport = &http.Transport{
	MaxIdleConns:        64,
	MaxIdleConnsPerHost: 50,
	IdleConnTimeout:     90 * time.Second,
	DisableKeepAlives:   false,
}
