package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Per-request worst-case memory: body 4 MiB + ReadAll growth up to 2x transient (8 MiB) +
// decoded nested maps ~8x body (32 MiB) => ~44 MiB.
// With maxConcurrentUpstream = 2: 2 * ~44 MiB = ~88 MiB + ~20 MiB baseline < 200 MiB GOMEMLIMIT < 256 Mi pod limit.
const (
	maxUpstreamBodyBytes  = 4 << 20 // 4 MiB
	maxConcurrentUpstream = 2
	consumerID            = "mcp-kodeverk"
)

type upstreamStatusError struct {
	status int
}

func (e *upstreamStatusError) Error() string {
	return fmt.Sprintf("kodeverk-api returned status %d", e.status)
}

type kodeverkListResponse struct {
	Kodeverksnavn []string `json:"kodeverksnavn"`
}

type kodeverkKoderResponse struct {
	Koder []string `json:"koder"`
}

type kodeverkBetydningerResponse struct {
	Betydninger map[string][]betydning `json:"betydninger"`
}

type betydning struct {
	GyldigFra    string                 `json:"gyldigFra"`
	GyldigTil    string                 `json:"gyldigTil"`
	Beskrivelser map[string]beskrivelse `json:"beskrivelser,omitempty"`
}

type beskrivelse struct {
	Term  string `json:"term"`
	Tekst string `json:"tekst"`
}

type hierarkiListResponse struct {
	Hierarkinavn []string `json:"hierarkinavn"`
}

type hierarkiNoderResponse struct {
	Hierarkinivaaer []string                `json:"hierarkinivaaer"`
	Noder           map[string]hierarkinode `json:"noder"`
}

type hierarkinode struct {
	Kode       string                  `json:"kode"`
	Termer     map[string]string       `json:"termer,omitempty"`
	Tekster    map[string]string       `json:"tekster,omitempty"`
	Undernoder map[string]hierarkinode `json:"undernoder,omitempty"`
}

type kodeverkClient struct {
	baseURL    string
	httpClient *http.Client
}

func newKodeverkClient(baseURL string, httpClient *http.Client) *kodeverkClient {
	return &kodeverkClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
	}
}

func (c *kodeverkClient) getJSON(ctx context.Context, token, path string, query url.Values, dst any) error {
	target := c.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return fmt.Errorf("build kodeverk request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Nav-Call-Id", randomCallID())
	req.Header.Set("Nav-Consumer-Id", consumerID)

	resp, err := c.httpClient.Do(req) //nolint:gosec // baseURL validated at startup, path segments escaped
	if err != nil {
		return fmt.Errorf("kodeverk-api request failed")
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &upstreamStatusError{status: resp.StatusCode}
	}

	body, err := readLimitedBody(resp.Body, maxUpstreamBodyBytes)
	if err != nil {
		return fmt.Errorf("kodeverk-api response exceeded %d bytes", maxUpstreamBodyBytes)
	}

	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("invalid JSON from kodeverk-api")
	}
	return nil
}

func (c *kodeverkClient) listKodeverk(ctx context.Context, token string) ([]string, error) {
	var resp kodeverkListResponse
	if err := c.getJSON(ctx, token, "/api/v1/kodeverk", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Kodeverksnavn, nil
}

func (c *kodeverkClient) getKodeverkKoder(ctx context.Context, token, navn string) ([]string, error) {
	path := "/api/v1/kodeverk/" + url.PathEscape(navn) + "/koder"
	var resp kodeverkKoderResponse
	if err := c.getJSON(ctx, token, path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Koder, nil
}

func (c *kodeverkClient) getKodeverkBetydninger(ctx context.Context, token, navn string, ekskluderUgyldige bool, oppslagsdato string, spraak []string) (map[string][]betydning, error) {
	path := "/api/v1/kodeverk/" + url.PathEscape(navn) + "/koder/betydninger"
	q := url.Values{}
	q.Set("ekskluderUgyldige", strconv.FormatBool(ekskluderUgyldige))
	if oppslagsdato != "" {
		q.Set("oppslagsdato", oppslagsdato)
	}
	for _, s := range spraak {
		q.Add("spraak", s)
	}
	var resp kodeverkBetydningerResponse
	if err := c.getJSON(ctx, token, path, q, &resp); err != nil {
		return nil, err
	}
	return resp.Betydninger, nil
}

func (c *kodeverkClient) listHierarkier(ctx context.Context, token string) ([]string, error) {
	var resp hierarkiListResponse
	if err := c.getJSON(ctx, token, "/api/v1/hierarki", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Hierarkinavn, nil
}

func (c *kodeverkClient) getHierarkiNoder(ctx context.Context, token, navn string, spraak []string) (hierarkiNoderResponse, error) {
	path := "/api/v1/hierarki/" + url.PathEscape(navn) + "/noder"
	q := url.Values{}
	for _, s := range spraak {
		q.Add("spraak", s)
	}
	var resp hierarkiNoderResponse
	if err := c.getJSON(ctx, token, path, q, &resp); err != nil {
		return hierarkiNoderResponse{}, err
	}
	return resp, nil
}

func randomCallID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
