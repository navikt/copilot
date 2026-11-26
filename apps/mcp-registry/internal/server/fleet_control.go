package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/navikt/copilot/mcp-registry/internal/registry"
)

// Static entries come from allowlist.json unless REGISTRY_SOURCE is
// fleet-control (#2641). Then the file is still loaded at startup and serves
// until the first good snapshot, and whenever fleet-control never answers.
// REGISTRY_SOURCE=shadow fetches the snapshot the same way but keeps serving
// allowlist.json; it only logs and counts the differences (#2765).
const (
	registrySourceGit          = "git"
	registrySourceFleetControl = "fleet-control"
	registrySourceShadow       = "shadow"

	FleetControlSyncInterval = 30 * time.Second
	maxSnapshotBytes         = 4 << 20
	maxTexasBytes            = 64 << 10
)

var errNotModified = errors.New("snapshot not modified")

// FleetControlRegistry fetches the registry snapshot from fleet-control.
type FleetControlRegistry struct {
	baseURL, tokenEndpoint, audience string
	client                           *http.Client
	shadow                           bool // serve allowlist.json, compare only

	lastSuccess time.Time // guarded by Store.reloadMu; process start until the first good fetch

	etag       string // guarded by Store.reloadMu, with the applied generation
	generation int64
}

// FleetControlFromEnv returns nil for REGISTRY_SOURCE=git (the default) and
// an error for anything it cannot use, so a bad config stops startup.
func FleetControlFromEnv() (*FleetControlRegistry, error) {
	src := strings.TrimSpace(os.Getenv("REGISTRY_SOURCE"))
	switch src {
	case "", registrySourceGit:
		return nil, nil
	case registrySourceFleetControl, registrySourceShadow:
	default:
		return nil, fmt.Errorf("unsupported REGISTRY_SOURCE %q (git, shadow or fleet-control)", src)
	}
	f := &FleetControlRegistry{
		shadow:        src == registrySourceShadow,
		lastSuccess:   time.Now(),
		baseURL:       strings.TrimRight(strings.TrimSpace(os.Getenv("FLEET_CONTROL_URL")), "/"),
		tokenEndpoint: strings.TrimSpace(os.Getenv("NAIS_TOKEN_ENDPOINT")),
		audience:      strings.TrimSpace(os.Getenv("FLEET_CONTROL_AUDIENCE")),
		client: &http.Client{
			Timeout:       10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
	for name, v := range map[string]string{"FLEET_CONTROL_URL": f.baseURL, "NAIS_TOKEN_ENDPOINT": f.tokenEndpoint} {
		if u, err := url.Parse(v); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, fmt.Errorf("REGISTRY_SOURCE=%s needs a valid %s", src, name)
		}
	}
	if f.audience == "" {
		return nil, fmt.Errorf("REGISTRY_SOURCE=%s needs FLEET_CONTROL_AUDIENCE", src)
	}
	return f, nil
}

// m2mToken gets an app token for fleet-control from Texas.
func (f *FleetControlRegistry) m2mToken(ctx context.Context) (string, error) {
	payload, err := json.Marshal(map[string]string{"identity_provider": "entra_id", "target": f.audience})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.tokenEndpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := f.client.Do(req) //nolint:gosec // endpoint validated at startup
	if err != nil {
		return "", fmt.Errorf("texas token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("texas token request: status %d", resp.StatusCode)
	}
	var out map[string]any
	if err = json.NewDecoder(io.LimitReader(resp.Body, maxTexasBytes)).Decode(&out); err != nil {
		return "", errors.New("invalid texas token response")
	}
	token, _ := out["access_token"].(string)
	if token == "" {
		return "", errors.New("invalid texas token response")
	}
	return token, nil
}

// fetch wraps load with the fetch metrics: failures counted, age since the last
// good fetch (200 or 304) exported.
func (f *FleetControlRegistry) fetch(ctx context.Context, parse func([]byte, time.Time) ([]registry.ServerResponse, map[string]bool, error)) ([]registry.ServerResponse, map[string]bool, error) {
	servers, names, err := f.load(ctx, parse)
	if err == nil || errors.Is(err, errNotModified) {
		f.lastSuccess = time.Now()
	} else {
		fleetControlFetchFailures.Inc()
	}
	fleetControlSnapshotAge.Set(time.Since(f.lastSuccess).Seconds())
	return servers, names, err
}

// load fetches the snapshot and validates it with parse, the same path as
// allowlist.json. errNotModified means 304; any other error: keep what you have.
func (f *FleetControlRegistry) load(ctx context.Context, parse func([]byte, time.Time) ([]registry.ServerResponse, map[string]bool, error)) ([]registry.ServerResponse, map[string]bool, error) {
	token, err := f.m2mToken(ctx)
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.baseURL+"/internal/v1/snapshots/registry", nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if f.etag != "" {
		req.Header.Set("If-None-Match", f.etag)
	}
	resp, err := f.client.Do(req) //nolint:gosec // URL validated at startup
	if err != nil {
		return nil, nil, fmt.Errorf("fleet-control request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode == http.StatusNotModified && f.etag != "":
		return nil, nil, errNotModified
	case resp.StatusCode != http.StatusOK:
		return nil, nil, fmt.Errorf("fleet-control: status %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxSnapshotBytes+1))
	if err != nil {
		return nil, nil, fmt.Errorf("read snapshot: %w", err)
	}
	if len(raw) > maxSnapshotBytes {
		return nil, nil, errors.New("snapshot too large")
	}
	var snap struct {
		SchemaVersion int             `json:"schema_version"`
		Generation    int64           `json:"generation"`
		Servers       json.RawMessage `json:"servers"`
	}
	if err = json.Unmarshal(raw, &snap); err != nil {
		return nil, nil, fmt.Errorf("parse snapshot: %w", err)
	}
	if snap.SchemaVersion != 1 {
		return nil, nil, fmt.Errorf("unsupported snapshot schema_version %d", snap.SchemaVersion)
	}
	if snap.Generation < f.generation {
		return nil, nil, fmt.Errorf("snapshot generation %d is older than %d", snap.Generation, f.generation)
	}
	// Items are allowlist.json entries; parse rejects an empty list.
	doc, err := json.Marshal(map[string]json.RawMessage{"servers": snap.Servers})
	if err != nil {
		return nil, nil, err
	}
	servers, names, err := parse(doc, time.Now().UTC())
	if err != nil {
		return nil, nil, fmt.Errorf("fleet-control snapshot: %w", err)
	}
	f.etag, f.generation = resp.Header.Get("ETag"), snap.Generation
	return servers, names, nil
}
