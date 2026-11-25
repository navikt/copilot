package main

import (
	"fmt"
	"time"

	"github.com/navikt/copilot/fleet-gateway/internal/oauth/server"
	"github.com/navikt/copilot/fleet-gateway/internal/proxy"
	"github.com/navikt/copilot/fleet-gateway/internal/teamtoken"
)

const (
	strategyOBO     = "obo"
	strategyRefresh = server.StrategyRefresh
)

func newTeamTokenSource(cfg *Config, store SessionStore, now func() time.Time) (proxy.TeamTokenSource, error) {
	if now == nil {
		now = time.Now
	}

	switch cfg.TeamTokenStrategy {
	case "", strategyOBO:
		return &teamtoken.OBO{
			Store:         store,
			SessionMaxAge: cfg.SessionAbsoluteMaxAge,
			Endpoint:      cfg.TexasExchangeEndpoint,
			Client:        cfg.HTTPClient,
			Now:           now,
		}, nil
	case strategyRefresh:
		return &teamtoken.Refresh{
			Store:         store,
			SessionMaxAge: cfg.SessionAbsoluteMaxAge,
			TokenEndpoint: cfg.AzureTokenEndpoint,
			ClientID:      cfg.AzureClientID,
			ClientSecret:  cfg.AzureClientSecret,
			Client:        cfg.HTTPClient,
			Now:           now,
			Timeout:       defaultOutboundTimeout,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported TEAM_TOKEN_STRATEGY %q", cfg.TeamTokenStrategy)
	}
}
