package api

import (
	"context"
	"slices"
	"strings"

	"github.com/navikt/copilot/fleet-control/internal/canon"
	"github.com/navikt/copilot/fleet-control/internal/model"
	"github.com/navikt/copilot/fleet-control/internal/store"
)

// validateProfileSpec checks spec on its own and returns it normalised:
// grants sorted by serverRef, tools sorted.
func validateProfileSpec(spec AgentProfileSpec) (AgentProfileSpec, error) {
	id := spec.Identity
	switch {
	case !dnsLabelRegex.MatchString(id.App):
		return spec, invalid("spec.identity.app", "must be a lowercase DNS label")
	case !dnsLabelRegex.MatchString(id.Cluster):
		return spec, invalid("spec.identity.cluster", "must be a lowercase DNS label")
	case !uuidRe.MatchString(id.EntraClientID):
		return spec, invalid("spec.identity.entraClientID", "must be a lowercase UUID")
	case id.StsSubject != "":
		return spec, invalid("spec.identity.stsSubject", "is reserved until the Nais STS issuer is chosen")
	case spec.Mode != "Delegated":
		return spec, invalid("spec.mode", "must be Delegated in phase 1")
	case len(spec.Grants) == 0 || len(spec.Grants) > 20:
		return spec, invalid("spec.grants", "must list 1 to 20 grants")
	}
	grants := make([]AgentGrantV1, 0, len(spec.Grants))
	seen := map[string]bool{}
	for _, g := range spec.Grants {
		ref := g.ServerRef
		if ref.Group != apiGroup || ref.Kind != "MCPServer" || !dnsLabelRegex.MatchString(ref.Namespace) || !dnsLabelRegex.MatchString(ref.Name) {
			return spec, invalid("spec.grants.serverRef", "serverRef must be group fleet.nav.no, kind MCPServer, and DNS-label namespace and name")
		}
		if seen[ref.key()] {
			return spec, invalid("spec.grants.serverRef", "MCP server %s is granted twice", ref.key())
		}
		seen[ref.key()] = true
		if len(g.AllowedTools) == 0 || len(g.AllowedTools) > 50 {
			return spec, invalid("spec.grants.allowedTools", "grant %s must list 1 to 50 tools", ref.key())
		}
		for i, t := range g.AllowedTools {
			if !toolNameRegex.MatchString(t) || slices.Contains(g.AllowedTools[:i], t) {
				return spec, invalid("spec.grants.allowedTools", "grant %s: %q is not a tool name, or is listed twice", ref.key(), t)
			}
		}
		if g.MaxRisk != "Read" {
			return spec, invalid("spec.grants.maxRisk", "must be Read in phase 1")
		}
		g.AllowedTools = slices.Sorted(slices.Values(g.AllowedTools))
		grants = append(grants, g)
	}
	slices.SortFunc(grants, func(a, b AgentGrantV1) int { return strings.Compare(a.ServerRef.key(), b.ServerRef.key()) })
	spec.Grants = grants
	q := spec.Quota
	if q.MaxInFlight < 1 || q.MaxInFlight > 64 || q.RequestsPerMinute < 1 || q.RequestsPerMinute > 6000 ||
		q.DailyCallsPerUser < 1 || q.DailyCallsPerUser > 100000 || q.DailyCalls < 1 || q.DailyCalls > 1000000 {
		return spec, invalid("spec.quota", "quota out of range")
	}
	if spec.DPIA == nil || !dnsLabelRegex.MatchString(spec.DPIA.Owner) {
		return spec, invalid("spec.dpia.owner", "must be a team (DNS label)")
	}
	if n := len(spec.DPIA.Reference); n == 0 || n > 300 || strings.ContainsFunc(spec.DPIA.Reference, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return spec, invalid("spec.dpia.reference", "must be 1 to 300 printable characters")
	}
	return spec, nil
}

// grantHash covers what the server's owner team sees: identity, mode,
// quota and their one grant (owner decision 13).
func grantHash(spec AgentProfileSpec, g AgentGrantV1) (string, error) {
	return canon.Hash(struct {
		Identity AgentIdentityV1 `json:"identity"`
		Mode     string          `json:"mode"`
		Quota    AgentQuotaV1    `json:"quota"`
		Grant    AgentGrantV1    `json:"grant"`
	}{spec.Identity, spec.Mode, spec.Quota, g})
}

// checkGrantServers requires every grant to name a published, unrevoked
// gateway server that lists every granted tool.
func checkGrantServers(ctx context.Context, q store.Querier, grants []AgentGrantV1) error {
	servers, err := store.QueryServers(ctx, q)
	if err != nil {
		return err
	}
	for _, g := range grants {
		i := slices.IndexFunc(servers, func(s model.Server) bool { return s.Key() == g.ServerRef.key() })
		if i < 0 || servers[i].Status != "published" || servers[i].Route == nil {
			return invalid("spec.grants.serverRef", "MCP server %s is not published behind fleet-gateway", g.ServerRef.key())
		}
		for _, t := range g.AllowedTools {
			if !slices.Contains(servers[i].Route.Tools, t) {
				return invalid("spec.grants.allowedTools", "MCP server %s has no tool %s", g.ServerRef.key(), t)
			}
		}
	}
	return nil
}
