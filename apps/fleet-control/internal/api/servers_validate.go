package api

import (
	"context"
	"slices"
	"strings"

	"github.com/navikt/copilot/fleet-control/internal/auth"
	"github.com/navikt/copilot/fleet-control/internal/model"
)

var dataCategories = []string{"Aapen", "Intern", "Fortrolig"}

// validateServerSpec checks spec with the same rules as /admin/v1 drafts
// and returns it normalised (allowedTools sorted) with the Server the
// snapshot columns are written from.
func validateServerSpec(cluster, platform, ns, name string, spec MCPServerSpec) (MCPServerSpec, model.Server, error) {
	if len(spec.Authorization) > 0 {
		return spec, model.Server{}, invalid("spec.authorization", "spec.authorization is reserved and not supported in v1alpha1")
	}
	switch {
	case spec.DataCategory == "StrengtFortrolig":
		return spec, model.Server{}, invalid("spec.dataCategory", "StrengtFortrolig needs its own risk assessment and approval first")
	case !slices.Contains(dataCategories, spec.DataCategory):
		return spec, model.Server{}, invalid("spec.dataCategory", "spec.dataCategory must be one of %s", strings.Join(dataCategories, ", "))
	}
	if len(spec.AllowedTools) > maxServerTools {
		return spec, model.Server{}, invalid("spec.allowedTools", "spec.allowedTools lists at most %d tools", maxServerTools)
	}
	if len(spec.Readme) > maxReadmeBytes {
		return spec, model.Server{}, invalid("spec.readme", "spec.readme is at most %d bytes", maxReadmeBytes)
	}
	if len(spec.Tools) > len(spec.AllowedTools) {
		return spec, model.Server{}, invalid("spec.tools", "spec.tools has more entries than allowedTools")
	}
	allowed := make(map[string]bool, len(spec.AllowedTools))
	for _, t := range spec.AllowedTools {
		allowed[t] = true
	}
	seen := make(map[string]bool, len(spec.Tools))
	for _, t := range spec.Tools {
		switch {
		case !allowed[t.Name] || seen[t.Name]:
			return spec, model.Server{}, invalid("spec.tools", "%q is not in allowedTools, or is listed twice", t.Name)
		case len(t.Description) > maxToolDescriptionBytes:
			return spec, model.Server{}, invalid("spec.tools.description", "tool %s: description is at most %d bytes", t.Name, maxToolDescriptionBytes)
		}
		seen[t.Name] = true
	}
	spec.Tools = slices.SortedFunc(slices.Values(spec.Tools), func(a, b ToolDoc) int { return strings.Compare(a.Name, b.Name) })
	if len(spec.Tools) == 0 {
		spec.Tools = nil
	}
	for t, risk := range spec.ToolRisk {
		if !allowed[t] || riskRank[risk] == 0 {
			return spec, model.Server{}, invalid("spec.toolRisk", "%q: tool must be in allowedTools and risk read, propose or write", t)
		}
	}
	if spec.DefaultRisk != "" && riskRank[spec.DefaultRisk] == 0 {
		return spec, model.Server{}, invalid("spec.defaultRisk", "must be read, propose or write")
	}
	resolved := model.ResolveToolRisk(spec.AllowedTools, spec.ToolRisk, spec.DefaultRisk)
	if len(spec.ToolRisk) == 0 {
		spec.ToolRisk = nil
	}
	if spec.RosRef != strings.TrimSpace(spec.RosRef) || len(spec.RosRef) > maxRosRefBytes {
		return spec, model.Server{}, invalid("spec.rosRef", "spec.rosRef has no surrounding spaces and is at most %d bytes", maxRosRefBytes)
	}
	// Outside dev-* (an unset cluster included) write tools need a ROS.
	if !auth.DevCluster(cluster) && spec.RosRef == "" && slices.ContainsFunc(spec.AllowedTools, func(t string) bool { return toolRiskOf(resolved, t) == "write" }) {
		return spec, model.Server{}, invalid("spec.rosRef", "the MCP server has write tools, and prod requires a risk assessment (ROS): set spec.rosRef to its link, or set spec.defaultRisk or spec.toolRisk to read or propose for tools that do not write").because(ReasonRosRequired)
	}
	var draft draftSpec
	draft.Registry = spec.Registry
	if u := spec.Upstream; u != nil {
		if !dnsLabelRegex.MatchString(u.App) {
			return spec, model.Server{}, invalid("spec.upstream.app", "must be a lowercase DNS label")
		}
		if len(spec.AllowedTools) == 0 {
			return spec, model.Server{}, invalid("spec.allowedTools", "upstream needs at least one allowed tool")
		}
		spec.AllowedTools = slices.Sorted(slices.Values(spec.AllowedTools))
		draft.Route = &struct {
			Target      string   `json:"target"`
			UpstreamURL string   `json:"upstream_url"`
			Enabled     *bool    `json:"enabled"`
			Tools       []string `json:"tools"`
		}{Target: cluster + ":" + ns + ":" + u.App, UpstreamURL: u.URL, Tools: spec.AllowedTools}
	} else if len(spec.AllowedTools) > 0 {
		return spec, model.Server{}, invalid("spec.allowedTools", "allowedTools needs upstream")
	}
	sv, err := validateDraft(cluster, ns, name, draft)
	if err != nil {
		return spec, model.Server{}, invalid("spec", "%v", err)
	}
	if sv.RegistryName != "" && !registryNameAllowed(platform, ns, sv.RegistryName) {
		return spec, model.Server{}, invalid("spec.registry.name", "%q is not yours: team %s may use no.nav.%s/<name> or io.github.navikt/%s; other names are registered by the platform team", sv.RegistryName, ns, ns, ns).because(ReasonRegistryNameNotYours)
	}
	if sv.Route != nil {
		sv.Route.ToolRisk = resolved
	}
	return spec, sv, nil
}

// riskRank orders tool risk levels: read < propose < write. Unknown is 0.
var riskRank = map[string]int{"read": 1, "propose": 2, "write": 3}

// toolRiskOf is tool's level in a resolved map; missing or unknown is write.
func toolRiskOf(resolved map[string]string, tool string) string {
	if riskRank[resolved[tool]] == 0 {
		return "write"
	}
	return resolved[tool]
}

// ForeignRegistryNames lists "<namespace>/<name>=<registry name>" for
// servers whose stored registry name the prefix rule would now refuse.
func (d *DB) ForeignRegistryNames(ctx context.Context, platform string) ([]string, error) {
	rows, err := d.pool.Query(ctx, `SELECT namespace, name, registry_name FROM servers WHERE registry_name IS NOT NULL ORDER BY 1, 2`)
	if err != nil {
		return nil, err
	}
	var out []string
	for rows.Next() {
		var ns, name, reg string
		if err := rows.Scan(&ns, &name, &reg); err != nil {
			return nil, err
		}
		if !registryNameAllowed(platform, ns, reg) {
			out = append(out, ns+"/"+name+"="+reg)
		}
	}
	return out, rows.Err()
}

// registryNameAllowed binds a registry name prefix to the team (#2853). The
// platform team may register any name, including vendor names such as
// com.microsoft/*. An unset platform team grants no one that.
func registryNameAllowed(platform, ns, regName string) bool {
	if platform != "" && ns == platform {
		return true
	}
	n := strings.ToLower(regName)
	return strings.HasPrefix(n, "no.nav."+ns+"/") || n == "io.github.navikt/"+ns
}
