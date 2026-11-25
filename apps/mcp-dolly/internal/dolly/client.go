package dolly

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/navikt/copilot/lib/mcpkit"
)

// Per-request worst-case memory: body 1 MiB + ReadAll growth + json.Compact copy => ~4 MiB.
// With MaxConcurrentUpstream = 2 this stays far below GOMEMLIMIT 200 MiB.
const (
	MaxUpstreamBodyBytes  = 1 << 20 // 1 MiB
	MaxConcurrentUpstream = 2
	consumerID            = "mcp-dolly"
	DefaultPageSize       = 20
)

type upstreamStatusError struct {
	status int
}

func (e *upstreamStatusError) Error() string {
	switch e.status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Sprintf("dolly-backend denied access (status %d)", e.status)
	case http.StatusNotFound:
		return "not found in Dolly (status 404)"
	}
	return fmt.Sprintf("dolly-backend returned status %d", e.status)
}

type Client struct {
	baseURL    string
	HTTPClient *http.Client
}

func NewClient(baseURL string, httpClient *http.Client) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), HTTPClient: httpClient}
}

type BestillingRequest struct {
	Antall       int            `json:"antall"`
	Environments []string       `json:"environments"`
	Pdldata      PdlPerson `json:"pdldata"`
}

type PdlPerson struct {
	OpprettNyPerson OpprettNyPerson `json:"opprettNyPerson"`
	Person          *PersonDTO      `json:"person,omitempty"`
}

type OpprettNyPerson struct {
	Identtype  string `json:"identtype"`
	Syntetisk  bool   `json:"syntetisk"`
	FoedtEtter string `json:"foedtEtter,omitempty"`
	FoedtFoer  string `json:"foedtFoer,omitempty"`
}

type PersonDTO struct {
	Kjoenn []KjoennDTO `json:"kjoenn,omitempty"`
}

type KjoennDTO struct {
	Kjoenn string `json:"kjoenn"`
}

func (c *Client) Get(ctx context.Context, token, path string, query url.Values) ([]byte, error) {
	target := c.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	return c.doJSON(ctx, http.MethodGet, token, target, nil)
}

// CreateOrder is the only write call to Dolly: POST /api/v1/gruppe/{gruppeId}/bestilling.
// /api/v1/gruppe/{id}/bestilling/fraidenter must never be used.
// refund reports whether a failed call may safely return the reserved quota:
// only when the request provably did not reach Dolly or Dolly answered a 4xx.
func (c *Client) CreateOrder(ctx context.Context, token string, gruppeID int64, order BestillingRequest) ([]byte, bool, error) {
	payload, err := json.Marshal(order)
	if err != nil {
		return nil, true, fmt.Errorf("encode dolly order: %w", err)
	}
	target := fmt.Sprintf("%s/api/v1/gruppe/%d/bestilling", c.baseURL, gruppeID)
	var reqSent atomic.Bool
	traceCtx := httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		WroteHeaders: func() { reqSent.Store(true) },
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			if info.Err == nil {
				reqSent.Store(true)
			}
		},
	})
	body, err := c.doJSON(traceCtx, http.MethodPost, token, target, payload)
	if err == nil {
		return body, false, nil
	}
	if !reqSent.Load() {
		return nil, true, err
	}
	var statusErr *upstreamStatusError
	if errors.As(err, &statusErr) && statusErr.status >= http.StatusBadRequest && statusErr.status < http.StatusInternalServerError {
		return nil, true, err
	}
	return nil, false, err
}

func (c *Client) doJSON(ctx context.Context, method, token, target string, reqBody []byte) ([]byte, error) {
	var bodyReader io.Reader
	if reqBody != nil {
		bodyReader = bytes.NewReader(reqBody)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("build dolly request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Nav-Call-Id", randomCallID())
	req.Header.Set("Nav-Consumer-Id", consumerID)

	resp, err := c.HTTPClient.Do(req) //nolint:gosec // baseURL validated at startup, path built from validated integers
	if err != nil {
		return nil, fmt.Errorf("dolly-backend request failed")
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &upstreamStatusError{status: resp.StatusCode}
	}
	body, err := mcpkit.ReadLimited(resp.Body, MaxUpstreamBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("dolly-backend response exceeded %d bytes; use a smaller pageSize", MaxUpstreamBodyBytes)
	}
	var out bytes.Buffer
	if err := json.Compact(&out, body); err != nil {
		return nil, fmt.Errorf("invalid JSON from dolly-backend")
	}
	return out.Bytes(), nil
}

// CurrentBrukerID returns the Dolly user id for the token's owner.
func (c *Client) CurrentBrukerID(ctx context.Context, token string) (string, error) {
	body, err := c.Get(ctx, token, "/api/v1/bruker/current", nil)
	if err != nil {
		return "", err
	}
	var b struct {
		BrukerID string `json:"brukerId"`
	}
	if err := json.Unmarshal(body, &b); err != nil || strings.TrimSpace(b.BrukerID) == "" {
		return "", fmt.Errorf("dolly-backend returned no brukerId")
	}
	return b.BrukerID, nil
}

// GroupOwnerBrukerID fetches a group via the existing read path and returns opprettetAv.brukerId.
func (c *Client) GroupOwnerBrukerID(ctx context.Context, token string, gruppeID int64) (string, error) {
	path := fmt.Sprintf("/api/v1/gruppe/%d/page/0", gruppeID)
	body, err := c.Get(ctx, token, path, url.Values{"pageSize": {strconv.Itoa(DefaultPageSize)}})
	if err != nil {
		return "", err
	}
	var g struct {
		OpprettetAv struct {
			BrukerID string `json:"brukerId"`
		} `json:"opprettetAv"`
	}
	if err := json.Unmarshal(body, &g); err != nil || strings.TrimSpace(g.OpprettetAv.BrukerID) == "" {
		return "", fmt.Errorf("dolly-backend returned no group owner")
	}
	return g.OpprettetAv.BrukerID, nil
}

func randomCallID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
