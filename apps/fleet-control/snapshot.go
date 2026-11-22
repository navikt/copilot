package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

// Snapshot is one published generation of a kind ("routes" or "registry").
// Items is the JSON array served to consumers.
type Snapshot struct {
	Kind       string
	Generation int64
	ETag       string
	Items      json.RawMessage
}

// snapshotItem is a routes or registry snapshot element; Key identifies it for
// the shrink guard and is not served.
type snapshotItem struct {
	key  string
	body any
}

type routeItem struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Route
}

var errEmptySnapshot = errors.New("snapshot would be empty")

// snapshotItems returns the published items of kind and the keys of revoked
// servers. Registry items expose only the public server.json entry.
func snapshotItems(kind string, servers []Server) (items []snapshotItem, revoked map[string]bool, err error) {
	revoked = map[string]bool{}
	for _, s := range servers {
		var it snapshotItem
		switch {
		case kind == "routes" && s.Route != nil:
			it = snapshotItem{key: s.Key(), body: routeItem{s.Namespace, s.Name, *s.Route}}
		case kind == "registry" && s.Registry != nil:
			it = snapshotItem{key: s.RegistryName, body: s.Registry}
		case kind != "routes" && kind != "registry":
			return nil, nil, fmt.Errorf("unknown snapshot kind %q", kind)
		default:
			continue
		}
		if s.Status == "revoked" {
			revoked[it.key] = true
			continue
		}
		items = append(items, it)
	}
	return items, revoked, nil
}

// buildSnapshot returns the snapshot to serve for servers given the previous
// one (nil if none). It fails closed: an empty result is rejected, and so is
// one that drops more than max(3, 20 %) of the previous keys for any reason
// other than a revoke.
func buildSnapshot(kind string, servers []Server, prev *Snapshot) (*Snapshot, error) {
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
		return &Snapshot{Kind: kind, Generation: 1, ETag: etag, Items: raw}, nil
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
	return &Snapshot{Kind: kind, Generation: prev.Generation + 1, ETag: etag, Items: raw}, nil
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
