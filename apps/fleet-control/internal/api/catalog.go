package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

// --- catalog ---

// CatalogEntry has its own type and serialisation, so a new MCPServer field
// never leaks into the catalog by itself. No actors, upstream, labels,
// annotations, uid or resourceVersion.
type CatalogEntry struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		DataCategory string          `json:"dataCategory,omitempty"`
		AllowedTools []string        `json:"allowedTools,omitempty"`
		Registry     json.RawMessage `json:"registry,omitempty"`
		// Tools carries the owner team's tool descriptions for other teams.
		Tools []ToolDoc `json:"tools,omitempty"`
	} `json:"spec"`
	Status struct {
		Route               *RouteStatus `json:"route,omitempty"`
		PublishedGeneration int          `json:"publishedGeneration"`
		ApprovedAt          *time.Time   `json:"approvedAt,omitempty"`
		// Consumers are the live agent profiles with access, by name only
		// (owner decision 2026-11-25, #2981). Set on the single entry.
		Consumers []ConsumerRef `json:"consumers,omitempty"`
	} `json:"status"`
}

// ConsumerRef names an agent profile and its team. No identity, grants or actors.
type ConsumerRef struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

func listCatalog(w http.ResponseWriter, r *http.Request, _ caller, db *DB) error {
	items, err := db.catalog(r.Context(), r.PathValue("namespace"), "")
	if err != nil {
		return err
	}
	writeObject(w, http.StatusOK, map[string]any{"apiVersion": apiGroupVersion, "kind": "MCPServerCatalogEntryList", "items": items})
	return nil
}

func getCatalogEntry(w http.ResponseWriter, r *http.Request, _ caller, db *DB) error {
	items, err := db.catalog(r.Context(), r.PathValue("namespace"), r.PathValue("name"))
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return errV1NotFound
	}
	if items[0].Status.Consumers, err = db.consumers(r.Context(), items[0].Metadata.Namespace+"/"+items[0].Metadata.Name); err != nil {
		return err
	}
	writeObject(w, http.StatusOK, items[0])
	return nil
}

// consumers lists live agent profiles whose published generation holds an unrevoked grant on server.
func (d *DB) consumers(ctx context.Context, server string) ([]ConsumerRef, error) {
	rows, err := d.pool.Query(ctx, `SELECT r.namespace, r.name FROM agent_profile_resources r
		JOIN agent_profile_grants g ON g.namespace = r.namespace AND g.name = r.name AND g.generation = r.published_generation
		WHERE g.server = $1 AND g.revoked_at IS NULL AND NOT r.revoked AND r.published_client_id IS NOT NULL
		ORDER BY r.namespace, r.name`, server)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[ConsumerRef])
}

// catalog lists published, unrevoked servers, optionally for one namespace or name.
func (d *DB) catalog(ctx context.Context, ns, name string) ([]CatalogEntry, error) {
	rows, err := d.pool.Query(ctx, `SELECT s.namespace, s.name, v.version, COALESCE(v.spec, `+legacySpecSQL+`)::text, v.approved_at
		FROM servers s JOIN server_versions v ON v.namespace = s.namespace AND v.name = s.name AND v.version = s.published_version
		WHERE s.status = 'published' AND ($1 = '' OR s.namespace = $1) AND ($2 = '' OR s.name = $2)
		ORDER BY s.namespace, s.name`, ns, name)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (CatalogEntry, error) {
		var e CatalogEntry
		var spec string
		if err = row.Scan(&e.Metadata.Namespace, &e.Metadata.Name, &e.Status.PublishedGeneration, &spec, &e.Status.ApprovedAt); err != nil {
			return e, err
		}
		var s MCPServerSpec
		if err = json.Unmarshal([]byte(spec), &s); err != nil {
			return e, err
		}
		e.APIVersion, e.Kind = apiGroupVersion, "MCPServerCatalogEntry"
		e.Spec.DataCategory, e.Spec.AllowedTools, e.Spec.Registry, e.Spec.Tools = s.DataCategory, s.AllowedTools, s.Registry, s.Tools
		if s.Upstream != nil {
			e.Status.Route = &RouteStatus{Path: "/mcp/" + e.Metadata.Namespace + "/" + e.Metadata.Name}
		}
		return e, nil
	})
	return emptyIfNil(out), err
}
