package main

// RouteTable is the set of MCP routes the gateway serves. Handlers read it
// through Gateway.table(), so a later control service can swap it whole
// (#2334). Nothing swaps it yet: it is loaded once from the route file.
type RouteTable struct {
	Routes           []PilotRoute // file order; Routes[0] is the primary resource
	RoutesByPath     map[string]PilotRoute
	AllowedResources map[string]bool
	PrimaryResource  string
}

// newRouteTable builds the table from a validated config, keeping file order.
// Disabled routes stay in it, so they answer 503 rather than 404.
func newRouteTable(cfg *Config) *RouteTable {
	rt := &RouteTable{
		Routes:           append([]PilotRoute(nil), cfg.Routes...),
		RoutesByPath:     make(map[string]PilotRoute, len(cfg.Routes)),
		AllowedResources: make(map[string]bool, len(cfg.Routes)),
	}
	for _, r := range rt.Routes {
		rt.RoutesByPath[r.Route] = r
		rt.AllowedResources[r.FullRouteID] = true
	}
	if len(rt.Routes) > 0 {
		rt.PrimaryResource = rt.Routes[0].FullRouteID
	}
	return rt
}

// table returns the current routes. A Gateway built without newGateway has
// none, so every route lookup fails closed.
func (g *Gateway) table() *RouteTable {
	if rt := g.routeTable.Load(); rt != nil {
		return rt
	}
	return &RouteTable{}
}
