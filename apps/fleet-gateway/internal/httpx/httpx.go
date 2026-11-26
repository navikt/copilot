// Package httpx holds HTTP response helpers shared by the gateway's handlers.
package httpx

import (
	"encoding/json"
	"net/http"
)

// WriteJSON writes payload as indented JSON with no-store caching headers.
func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(payload)
}

// StatusWriter records the status code and shed reason of a response.
type StatusWriter struct {
	http.ResponseWriter
	Status int
	Reason string // set by MarkShed; labels deliberate 5xx in metrics
}

// WriteHeader records code and passes it on.
func (w *StatusWriter) WriteHeader(code int) {
	w.Status = code
	w.ResponseWriter.WriteHeader(code)
}

// Unwrap returns the wrapped writer, for http.ResponseController and MarkShed.
func (w *StatusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// MarkShed labels a deliberate 503 so the 5xx alert can tell it apart from
// real failures: "disabled" (kill switch), "route_disabled" (per-route kill switch), "cap" (in-flight/SSE limits),
// "user_cap" (per-user in-flight limit), "user_sse_cap" (per-user SSE limit),
// "store_full" (store quota reached), "refresh_in_flight" (concurrent refresh).
// Store errors and upstream failures stay unlabelled and count.
func MarkShed(w http.ResponseWriter, reason string) {
	for {
		if sw, ok := w.(*StatusWriter); ok {
			sw.Reason = reason
			return
		}
		u, ok := w.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			return
		}
		w = u.Unwrap()
	}
}

// MaxUpstreamBodyBytes bounds a JSON body read from an upstream service.
const MaxUpstreamBodyBytes = 1 << 20
