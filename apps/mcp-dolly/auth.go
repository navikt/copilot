package main

import (
	"context"
	"errors"
	"log/slog"

	"github.com/navikt/copilot/lib/mcpkit"
)

// Texas introspection and OBO exchange live in lib/mcpkit. These names are
// kept so main.go, tools.go and the tests read as before.
const maxBearerLen = mcpkit.MaxBearerLen

var (
	errUnauthorized  = mcpkit.ErrUnauthorized
	errForbidden     = mcpkit.ErrForbidden
	errNotConfigured = mcpkit.ErrNotConfigured

	validateHTTPURL        = mcpkit.ValidateHTTPURL
	extractGatewayClientID = mcpkit.GatewayClientID
	extractBearerToken     = mcpkit.BearerToken
)

// texas reads s.cfg on each call, so a config change is seen at once.
func (s *server) texas() *mcpkit.Texas {
	return &mcpkit.Texas{
		IntrospectionEndpoint: s.cfg.IntrospectionEndpoint,
		ExchangeEndpoint:      s.cfg.ExchangeEndpoint,
		GatewayClientID:       s.cfg.GatewayClientID,
		HTTPClient:            s.cfg.HTTPClient,
	}
}

func (s *server) validateToken(ctx context.Context, token string) (string, error) {
	return s.texas().Introspect(ctx, token)
}

// exchangeToken trades the user token for a Dolly OBO token. A Texas error
// response is logged and shown with only its OAuth code and AADSTS number.
func (s *server) exchangeToken(ctx context.Context, userToken string) (string, error) {
	tok, err := s.texas().Exchange(ctx, userToken, s.cfg.DollyScope)
	var ee *mcpkit.ExchangeError
	if errors.As(err, &ee) && ee.StatusCode != 0 {
		slog.Warn("texas token exchange failed", "status", ee.StatusCode, "error", ee.OAuthError, "aadsts", ee.AADSTS)
		msg := ee.Msg
		if ee.OAuthError != "" {
			msg += ": " + ee.OAuthError
		}
		if ee.AADSTS != "" {
			msg += " " + ee.AADSTS
		}
		return "", &mcpkit.ExchangeError{StatusCode: ee.StatusCode, Msg: msg, OAuthError: ee.OAuthError, AADSTS: ee.AADSTS}
	}
	return tok, err
}
