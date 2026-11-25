// Package httperr writes the JSON error body every copilot-cli route answers with.
package httperr

import (
	"fmt"
	"net/http"
)

// Write sends status with {"error": message}.
func Write(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, `{"error":%q}`, message)
}
