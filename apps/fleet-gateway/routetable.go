package main

// RouteTable is the set of MCP routes the gateway serves. Handlers read it
// through Gateway.table(). It is loaded from the route file, and with
// ROUTES_SOURCE=fleet-control swapped whole by syncRoutes (#2641).
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

// routeByResource finds the route whose resource URL is res.
func (rt *RouteTable) routeByResource(res string) (PilotRoute, bool) {
	for _, r := range rt.Routes {
		if r.FullRouteID == res {
			return r, true
		}
	}
	return PilotRoute{}, false
}

// table returns the current routes. A Gateway built without newGateway has
// none, so every route lookup fails closed.
func (g *Gateway) table() *RouteTable {
	if rt := g.routeTable.Load(); rt != nil {
		return rt
	}
	return &RouteTable{}
}

// afterRouteLookup is a test hook between dispatch and the handler; a no-op in production.
var afterRouteLookup = func() {}
