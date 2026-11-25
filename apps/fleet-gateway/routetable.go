package main

import (
	"github.com/navikt/copilot/fleet-gateway/internal/routing"
)

// table returns the current routes. A Gateway built without newGateway has
// none, so every route lookup fails closed.
func (g *Gateway) table() *routing.Table {
	if rt := g.routeTable.Load(); rt != nil {
		return rt
	}
	return &routing.Table{}
}

// afterRouteLookup is a test hook between dispatch and the handler; a no-op in production.
var afterRouteLookup = func() {}
