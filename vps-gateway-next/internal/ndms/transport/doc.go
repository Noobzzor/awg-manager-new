// Package transport is the low-level NDMS HTTP client for awg-manager.
//
// Client exposes Get / GetRaw / GetStream / Post / PostBatch — the only
// HTTP-level entry points used by every query/ and command/ consumer.
// All calls go through a bounded concurrency semaphore that keeps NDMS
// from being overloaded by our own bursts.
//
// Prefer GetStream over GetRaw when the caller decodes the response body
// immediately (e.g. json.NewDecoder) and does not need to retain the raw
// bytes — GetStream avoids buffering the full response into memory.
package transport
