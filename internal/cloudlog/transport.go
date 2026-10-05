// Package cloudlog logs cloud API actions without recording credentials or payloads.
package cloudlog

import (
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

var debugEnabled atomic.Bool

// SetDebug enables detailed API logs. They are disabled by default.
func SetDebug(enabled bool) { debugEnabled.Store(enabled) }

// Transport logs each request before sending it and its result afterwards.
// Headers, query strings and bodies are deliberately excluded: they can contain
// tokens, SSH keys, cloud-init scripts and application secrets.
type Transport struct {
	Provider string
	Base     http.RoundTripper
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	if !debugEnabled.Load() {
		return base.RoundTrip(req)
	}
	started := time.Now()
	log.Printf("[%s API] %s %s: sending", t.Provider, req.Method, req.URL.EscapedPath())
	resp, err := base.RoundTrip(req)
	elapsed := time.Since(started).Round(time.Millisecond)
	if err != nil {
		// Transport errors may include a URL with sensitive query parameters.
		log.Printf("[%s API] %s %s: failed (%s)", t.Provider, req.Method, req.URL.EscapedPath(), elapsed)
	} else {
		log.Printf("[%s API] %s %s: HTTP %d (%s)", t.Provider, req.Method, req.URL.EscapedPath(), resp.StatusCode, elapsed)
	}
	return resp, err
}
