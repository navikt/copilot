// Package server is the OAuth 2.1 authorization server: authorize, callback,
// consent, token, revoke, register and the metadata endpoints.
package server

import (
	"context"
	"github.com/navikt/copilot/fleet-gateway/internal/audit"
	"net/http"
	"time"

	"github.com/navikt/copilot/fleet-gateway/internal/oauth/entra"
	"github.com/navikt/copilot/fleet-gateway/internal/ratelimit"
	"github.com/navikt/copilot/fleet-gateway/internal/routing"
	"github.com/navikt/copilot/fleet-gateway/internal/session"
	"github.com/navikt/copilot/fleet-gateway/internal/ttlmap"
)

// Config holds the settings the OAuth handlers read, copied from main's Config.
type Config struct {
	BaseURL                 string
	AzureClientID           string
	AzureIssuer             string
	EntraScope              string
	CookieSecret            []byte
	CookieVerifyKeys        [][]byte
	HTTPClient              *http.Client
	AllowedRedirectURIs     map[string]bool
	AllowedClientIDs        map[string]bool
	SessionMaxAge           time.Duration
	SessionAbsoluteMaxAge   time.Duration
	SessionIdleTimeout      time.Duration
	ConsentTTL              time.Duration
	RefreshGraceWindow      time.Duration
	RTStorage               string
	IdentitySource          string
	TeamTokenStrategy       string
	TexasIntrospectEndpoint string
	DCREnabled              bool
	DCRClientTTL            time.Duration
	DCRClientMaxAge         time.Duration
	DCRRateLimitRequests    int
	DCRRateLimitGlobal      int
}

// Deps is what main wires into a Server.
type Deps struct {
	Cfg          *Config
	Now          func() time.Time
	Store        session.Store
	RTKeys       *session.KeyRing
	CIMDCache    *ttlmap.Map[CachedCIMD]
	RateLimitLog *ratelimit.LogSampler
	RefreshWait  time.Duration
	Entra        *entra.Client
	Table        func() *routing.Table
	UserRevoked  func(ctx context.Context, navIdent string, issuedAt time.Time) (bool, error)
}

// Server holds the OAuth handlers. main builds one per request from its current
// config and fields, which tests swap after construction.
type Server struct {
	cfg          *Config
	now          func() time.Time
	store        session.Store
	rtKeys       *session.KeyRing
	cimdCache    *ttlmap.Map[CachedCIMD]
	rateLimitLog *ratelimit.LogSampler
	refreshWait  time.Duration
	entra        *entra.Client
	table        func() *routing.Table
	userRevoked  func(ctx context.Context, navIdent string, issuedAt time.Time) (bool, error)
}

// New returns a Server over d.
func New(d Deps) *Server {
	return &Server{
		cfg: d.Cfg, now: d.Now, store: d.Store, rtKeys: d.RTKeys, cimdCache: d.CIMDCache,
		rateLimitLog: d.RateLimitLog, refreshWait: d.RefreshWait, entra: d.Entra,
		table: d.Table, userRevoked: d.UserRevoked,
	}
}

func sanitizeLogValue(s string) string { return audit.Sanitize(s) }
