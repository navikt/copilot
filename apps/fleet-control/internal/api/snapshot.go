package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/navikt/copilot/fleet-control/internal/model"
)

// snapshotItem is a routes or registry snapshot element; Key identifies it for
// the shrink guard and is not served.
type snapshotItem struct {
	key  string
	body any
}

type routeItem struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	// Cluster is the target's cluster. DataClass is omitted when unknown, and
	// a consumer that enforces data classes must then treat it as the highest (#2896).
	Cluster   string `json:"cluster"`
	DataClass string `json:"data_class,omitempty"`
	model.Route
}

func newRouteItem(s model.Server, r model.Route) routeItem {
	cluster, _, _ := strings.Cut(r.Target, ":")
	return routeItem{s.Namespace, s.Name, cluster, s.DataClass, r}
}

var errEmptySnapshot = errors.New("snapshot would be empty")

// snapshotItems returns the published items of kind and the keys of revoked
// servers. Registry items expose only the public server.json entry.
func snapshotItems(kind string, servers []model.Server) (items []snapshotItem, revoked map[string]bool, err error) {
	revoked = map[string]bool{}
	for _, s := range servers {
		var it snapshotItem
		switch {
		case kind == "routes" && s.Route != nil:
			it = snapshotItem{key: s.Key(), body: newRouteItem(s, *s.Route)}
		case kind == "registry" && s.Registry != nil:
			it = snapshotItem{key: s.RegistryName, body: s.Registry}
		case kind != "routes" && kind != "registry":
			return nil, nil, fmt.Errorf("unknown snapshot kind %q", kind)
		default:
			continue
		}
		if s.Status == "revoked" {
			revoked[it.key] = true
			if kind != "routes" {
				continue
			}
			// A revoked route stays as disabled, so the gateway answers 503
			// instead of serving it. Dropping it could empty the snapshot or
			// remove the gateway's primary resource, and both are rejected,
			// which would keep the revoked route live (#2688).
			r := *s.Route
			r.Enabled = false
			// One tool is enough for a disabled route and bounds the row, so
			// revoked rows from before this change cannot push an existing
			// snapshot past the gateway's size limit.
			r.Tools = r.Tools[:min(len(r.Tools), 1)]
			r.ToolRisk = nil
			// The gateway never calls a disabled route's upstream, but still
			// validates it; the target's app in its namespace passes and stays short.
			r.UpstreamURL = "http://" + r.Target[strings.LastIndex(r.Target, ":")+1:] + "." + s.Namespace
			it.body = newRouteItem(s, r)
		}
		items = append(items, it)
	}
	return items, revoked, nil
}

// buildSnapshot returns the snapshot to serve for servers given the previous
// one (nil if none). It fails closed: an empty result is rejected, and so is
// one that drops more than max(3, 20 %) of the previous keys for any reason
// other than a revoke.
func buildSnapshot(kind string, servers []model.Server, prev *model.Snapshot) (*model.Snapshot, error) {
	items, revoked, err := snapshotItems(kind, servers)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, errEmptySnapshot
	}
	keys := map[string]bool{}
	bodies := make([]any, 0, len(items))
	for _, it := range items {
		if keys[it.key] {
			return nil, fmt.Errorf("duplicate %s key %q", kind, it.key)
		}
		keys[it.key] = true
		bodies = append(bodies, it.body)
	}
	if kind == "routes" {
		paths := map[string]bool{}
		for _, b := range bodies {
			p := b.(routeItem).Path
			if paths[p] {
				return nil, fmt.Errorf("duplicate route path %q", p)
			}
			paths[p] = true
		}
	}
	raw, err := json.Marshal(bodies)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	etag := hex.EncodeToString(sum[:])
	if prev == nil {
		return &model.Snapshot{Kind: kind, Generation: 1, ETag: etag, Items: raw}, nil
	}
	if prev.ETag == etag {
		return prev, nil
	}
	prevKeys, err := snapshotKeys(kind, prev.Items)
	if err != nil {
		return nil, fmt.Errorf("previous %s snapshot: %w", kind, err)
	}
	removed := 0
	for _, k := range prevKeys {
		if !keys[k] && !revoked[k] {
			removed++
		}
	}
	if limit := max(3, len(prevKeys)/5); removed > limit {
		return nil, fmt.Errorf("%s snapshot would drop %d of %d entries (limit %d)", kind, removed, len(prevKeys), limit)
	}
	return &model.Snapshot{Kind: kind, Generation: prev.Generation + 1, ETag: etag, Items: raw}, nil
}

// maxRoutesSnapshotBytes keeps the routes snapshot under fleet-gateway's
// 1 MiB read limit, with room for the envelope and for revokes, which add a
// byte each when enabled turns false (#2687).
var maxRoutesSnapshotBytes = 1<<20 - 64<<10

// maxTeamRoutesBytes is one namespace's share of the routes snapshot, so a
// single team cannot fill the shared limit and block everyone else (#2735).
var maxTeamRoutesBytes = 64 << 10

// routesFit refuses servers whose routes snapshot fleet-gateway would reject,
// or whose routes in namespace ns exceed the team budget. Only the writing
// team's budget is checked, so a team over budget cannot block other teams.
// A write that does not grow the total or the team share always passes, so a
// rollback can repair a snapshot that is already over a limit (#2735).
// Writes that can grow the snapshot call it before they commit, so the
// published snapshot never reaches a size that stops all later updates.
func routesFit(before, servers []model.Server, ns string) error {
	total, team, err := routesSize(servers, ns)
	if err != nil {
		return refuse(http.StatusConflict, ReasonInvalidSnapshot, "routes snapshot would be invalid: %v", err)
	}
	oldTotal, oldTeam, err := routesSize(before, ns)
	if err != nil {
		oldTotal, oldTeam = 0, 0 // an invalid current state gives no allowance
	}
	if total > maxRoutesSnapshotBytes && total > oldTotal {
		return refuse(http.StatusConflict, ReasonOverSnapshotBudget, "routes snapshot would be %d bytes, over the limit of %d", total, maxRoutesSnapshotBytes)
	}
	if team > maxTeamRoutesBytes && team > oldTeam {
		return refuse(http.StatusConflict, ReasonOverTeamBudget, "routes of team %s would be %d bytes, over the team limit of %d", ns, team, maxTeamRoutesBytes)
	}
	return nil
}

// routesSize gives the size of the routes snapshot and of namespace ns's share.
func routesSize(servers []model.Server, ns string) (total, team int, err error) {
	snap, err := buildSnapshot("routes", servers, nil)
	if errors.Is(err, errEmptySnapshot) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	items, _, _ := snapshotItems("routes", servers) // buildSnapshot succeeded, so this does
	for _, it := range items {
		if it.body.(routeItem).Namespace != ns {
			continue
		}
		b, err := json.Marshal(it.body)
		if err != nil {
			return 0, 0, err
		}
		team += len(b) + 1 // the comma in the array
	}
	return len(snap.Items), team, nil
}

// snapshotKeys reads the keys back from a stored snapshot.
func snapshotKeys(kind string, items json.RawMessage) ([]string, error) {
	var rows []struct {
		Namespace string `json:"namespace"`
		Name      string `json:"name"`
	}
	if err := json.Unmarshal(items, &rows); err != nil {
		return nil, err
	}
	keys := make([]string, len(rows))
	for i, r := range rows {
		if kind == "routes" {
			keys[i] = r.Namespace + "/" + r.Name
		} else {
			keys[i] = r.Name
		}
	}
	return keys, nil
}
