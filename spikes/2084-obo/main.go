// Spike for navikt/copilot#2084: chained Entra OBO via Texas. Do not merge.
// One binary, three roles: gateway and team set NEXT_TARGET/NEXT_URL, down does not.
package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var client = &http.Client{
	Timeout:       10 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirects refused") },
}

// preAuthorized reports whether azp is the client id of an app in AZURE_APP_PRE_AUTHORIZED_APPS.
func preAuthorized(azp any) bool {
	var apps []struct {
		ClientID string `json:"clientId"`
	}
	_ = json.Unmarshal([]byte(os.Getenv("AZURE_APP_PRE_AUTHORIZED_APPS")), &apps)
	for _, a := range apps {
		if a.ClientID != "" && a.ClientID == azp {
			return true
		}
	}
	return false
}

func texas(endpoint string, form url.Values) (map[string]any, int, error) {
	resp, err := client.PostForm(endpoint, form)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	var out map[string]any
	err = json.NewDecoder(resp.Body).Decode(&out)
	return out, resp.StatusCode, err
}

func handle(w http.ResponseWriter, r *http.Request) {
	res := map[string]any{"app": os.Getenv("NAIS_APP_NAME")}
	defer func() {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(res)
	}()
	tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		res["error"] = "no bearer token"
		return
	}
	in, _, err := texas(os.Getenv("NAIS_TOKEN_INTROSPECTION_ENDPOINT"), url.Values{"identity_provider": {"entra_id"}, "token": {tok}})
	if err != nil || in["active"] != true {
		w.WriteHeader(http.StatusUnauthorized)
		res["error"] = "introspection failed"
		res["introspection_error"] = in["error"]
		return
	}
	claims := map[string]any{}
	for _, k := range []string{"aud", "azp", "azp_name", "NAVident", "scp", "idtyp"} {
		if v, ok := in[k]; ok {
			claims[k] = v
		}
	}
	res["claims"] = claims
	if in["idtyp"] == "app" {
		w.WriteHeader(http.StatusUnauthorized)
		res["error"] = "app token rejected"
		return
	}
	if !preAuthorized(in["azp"]) {
		w.WriteHeader(http.StatusUnauthorized)
		res["error"] = "azp not pre-authorized"
		return
	}
	target := os.Getenv("NEXT_TARGET")
	if target == "" {
		return
	}
	// ?mode=passthrough forwards the inbound token unchanged; ?mode=app sends a client-credentials token.
	mode := r.URL.Query().Get("mode")
	res["mode"] = mode
	next := tok
	if mode != "passthrough" {
		form := url.Values{"identity_provider": {"entra_id"}, "target": {target}}
		endpoint := os.Getenv("NAIS_TOKEN_EXCHANGE_ENDPOINT")
		if mode == "app" {
			endpoint = os.Getenv("NAIS_TOKEN_ENDPOINT")
		} else {
			form.Set("user_token", tok)
		}
		out, code, err := texas(endpoint, form)
		next, _ = out["access_token"].(string)
		if err != nil || code != http.StatusOK || next == "" {
			w.WriteHeader(http.StatusBadGateway)
			// Texas error bodies carry error/error_description only, never tokens.
			res["texas_error"] = map[string]any{"status": code, "error": out["error"], "error_description": out["error_description"], "err": errString(err)}
			return
		}
	}
	req, err := http.NewRequest(http.MethodGet, os.Getenv("NEXT_URL"), nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		res["error"] = err.Error()
		return
	}
	req.Header.Set("Authorization", "Bearer "+next)
	resp, err := client.Do(req)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		res["downstream_error"] = err.Error()
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	res["downstream_status"] = resp.StatusCode
	if json.Valid(body) {
		res["downstream"] = json.RawMessage(body)
	} else {
		res["downstream"] = string(body)
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func main() {
	http.HandleFunc("/", handle)
	http.HandleFunc("/isalive", func(w http.ResponseWriter, _ *http.Request) {})
	log.Fatal(http.ListenAndServe(":8080", nil))
}
