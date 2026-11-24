package main

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Shadow mode (#2760): with ROUTES_SOURCE=git and ROUTES_SHADOW_SOURCE=fleet-control
// the gateway polls fleet-control, never serves those routes, and reports how
// many routes differ from the Git table. A shadow failure never touches serving.

var (
	shadowDiffGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "fleet_gateway_routes_shadow_diff",
		Help: "Routes that differ between the Git route file and the fleet-control shadow snapshot. Absent until the first shadow snapshot.",
	})
	shadowDiffOnce sync.Once
	shadowFailures = promauto.NewCounter(prometheus.CounterOpts{
		Name: "fleet_gateway_routes_shadow_failures_total",
		Help: "Failed or rejected fleet-control shadow snapshot fetches.",
	})
)

// diffRoutes compares the Git route file with a fleet-control snapshot after
// both are validated, so upstreams compare in canonical .svc.cluster.local form.
func diffRoutes(file, snap []PilotRoute) []string {
	byRoute := func(rs []PilotRoute) map[string]PilotRoute {
		m := map[string]PilotRoute{}
		for _, r := range rs {
			m[r.Route] = r
		}
		return m
	}
	f, s := byRoute(file), byRoute(snap)
	var out []string
	for route, a := range f {
		b, ok := s[route]
		if !ok {
			out = append(out, route+": only in file")
			continue
		}
		at, bt := slices.Sorted(slices.Values(a.Tools)), slices.Sorted(slices.Values(b.Tools))
		for _, d := range []struct{ field, file, snap string }{
			{"team", a.Team, b.Team},
			{"target", a.Target, b.Target},
			{"upstream", a.ParsedURL.String(), b.ParsedURL.String()},
			{"enabled", fmt.Sprint(!a.isDisabled()), fmt.Sprint(!b.isDisabled())},
			{"tools", fmt.Sprint(at), fmt.Sprint(bt)},
			{"passAllTools", fmt.Sprint(a.PassAllTools), fmt.Sprint(b.PassAllTools)},
		} {
			if d.file != d.snap {
				out = append(out, fmt.Sprintf("%s: %s file=%s snapshot=%s", route, d.field, d.file, d.snap))
			}
		}
	}
	for route := range s {
		if _, ok := f[route]; !ok {
			out = append(out, route+": only in snapshot")
		}
	}
	slices.Sort(out)
	return out
}

// shadowDiffPaths returns the sorted route paths named in diff lines.
func shadowDiffPaths(diff []string) []string {
	var out []string
	for _, d := range diff {
		path, _, _ := strings.Cut(d, ": ")
		if !slices.Contains(out, path) {
			out = append(out, path)
		}
	}
	return out
}

// shadowRoutes polls fleet-control and compares; it only writes the gauge,
// the failure counter and logs. last is the previous diff, nil before the first.
type shadowRoutes struct {
	f    *fleetControlRoutes
	last []string
	seen bool
}

func (g *Gateway) shadowSync(ctx context.Context, s *shadowRoutes) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	current := g.table()
	routes, err := s.f.fetch(ctx, current)
	if err != nil {
		shadowFailures.Inc()
		slog.Warn("fleet-control shadow snapshot not compared", "error", err)
		return
	}
	if routes == nil {
		return // unchanged snapshot; the Git table is fixed for the pod's life
	}
	paths := shadowDiffPaths(diffRoutes(current.Routes, routes))
	shadowDiffOnce.Do(func() { prometheus.MustRegister(shadowDiffGauge) })
	shadowDiffGauge.Set(float64(len(paths)))
	if !s.seen || !slices.Equal(paths, s.last) {
		slog.Info("fleet-control shadow diff", "differing_routes", len(paths), "routes", paths, "generation", s.f.generation)
	}
	s.last, s.seen = paths, true
}

func (g *Gateway) startShadowSync(ctx context.Context, interval time.Duration) {
	if g.cfg.RoutesShadowSource != routesSourceFleetControl {
		return
	}
	s := &shadowRoutes{f: &fleetControlRoutes{cfg: g.cfg, client: g.cfg.HTTPClient}}
	go func() {
		g.shadowSync(ctx, s)
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				g.shadowSync(ctx, s)
			}
		}
	}()
}
