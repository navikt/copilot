package main

import (
	"context"
	"net/http"

	"github.com/navikt/copilot/fleet-gateway/internal/oauth"
	"github.com/navikt/copilot/fleet-gateway/internal/oauth/server"
)

// oauth builds the OAuth server from the gateway's current config and fields,
// which tests swap after construction.
func (g *Gateway) oauth() *server.Server {
	c := g.cfg
	return server.New(server.Deps{
		Cfg: &server.Config{
			BaseURL:                 c.BaseURL,
			AzureClientID:           c.AzureClientID,
			AzureIssuer:             c.AzureIssuer,
			EntraScope:              c.EntraScope,
			CookieSecret:            c.CookieSecret,
			CookieVerifyKeys:        c.CookieVerifyKeys,
			HTTPClient:              c.HTTPClient,
			AllowedRedirectURIs:     c.AllowedRedirectURIs,
			AllowedClientIDs:        c.AllowedClientIDs,
			SessionMaxAge:           c.SessionMaxAge,
			SessionAbsoluteMaxAge:   c.SessionAbsoluteMaxAge,
			SessionIdleTimeout:      c.SessionIdleTimeout,
			ConsentTTL:              c.ConsentTTL,
			RefreshGraceWindow:      c.RefreshGraceWindow,
			RTStorage:               c.RTStorage,
			IdentitySource:          c.IdentitySource,
			TeamTokenStrategy:       c.TeamTokenStrategy,
			TexasIntrospectEndpoint: c.TexasIntrospectEndpoint,
			DCREnabled:              c.DCREnabled,
			DCRClientTTL:            c.DCRClientTTL,
			DCRClientMaxAge:         c.DCRClientMaxAge,
			DCRRateLimitRequests:    c.DCRRateLimitRequests,
			DCRRateLimitGlobal:      c.DCRRateLimitGlobal,
		},
		Now:          g.now,
		Store:        g.store,
		RTKeys:       g.rtKeys,
		CIMDCache:    g.cimdCache,
		RateLimitLog: g.rateLimitLog,
		RefreshWait:  g.refreshWait,
		Entra:        g.entra,
		Table:        g.table,
		UserRevoked:  g.userRevoked,
	})
}

func (g *Gateway) prmHandler(w http.ResponseWriter, r *http.Request) { g.oauth().PRM(w, r) }
func (g *Gateway) asMetadataHandler(w http.ResponseWriter, r *http.Request) {
	g.oauth().ASMetadata(w, r)
}
func (g *Gateway) authorizeHandler(w http.ResponseWriter, r *http.Request) { g.oauth().Authorize(w, r) }
func (g *Gateway) callbackHandler(w http.ResponseWriter, r *http.Request)  { g.oauth().Callback(w, r) }
func (g *Gateway) consentPostHandler(w http.ResponseWriter, r *http.Request) {
	g.oauth().ConsentPost(w, r)
}
func (g *Gateway) tokenHandler(w http.ResponseWriter, r *http.Request)    { g.oauth().Token(w, r) }
func (g *Gateway) revokeHandler(w http.ResponseWriter, r *http.Request)   { g.oauth().Revoke(w, r) }
func (g *Gateway) registerHandler(w http.ResponseWriter, r *http.Request) { g.oauth().Register(w, r) }

func (g *Gateway) getSession(ctx context.Context, hash string) (*oauth.Session, bool, error) {
	return g.oauth().GetSession(ctx, hash)
}
