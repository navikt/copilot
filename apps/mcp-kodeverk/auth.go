package main

import "github.com/navikt/copilot/lib/mcpkit"

// Kept under their old names for the tests.
const maxBearerLen = mcpkit.MaxBearerLen

var extractGatewayClientID = mcpkit.GatewayClientID

// texas reads s.cfg on each call, so a config change is seen at once.
func (s *server) texas() *mcpkit.Texas {
	return &mcpkit.Texas{
		IntrospectionEndpoint: s.cfg.IntrospectionEndpoint,
		ExchangeEndpoint:      s.cfg.ExchangeEndpoint,
		GatewayClientID:       s.cfg.GatewayClientID,
		HTTPClient:            s.cfg.HTTPClient,
	}
}
