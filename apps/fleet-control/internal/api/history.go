package api

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
)

// history returns the newest events for one resource, and whether it exists.
func (d *DB) history(ctx context.Context, kind, ns, name string, legacy []string, limit int) ([]AuditEvent, bool, error) {
	table := "servers"
	if kind == "AgentProfile" {
		table = "agent_profile_resources"
	}
	var found bool
	if err := d.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM `+table+` WHERE namespace = $1 AND name = $2)`, ns, name).Scan(&found); err != nil || !found {
		return nil, found, err
	}
	rows, err := d.pool.Query(ctx, `SELECT at, actor, COALESCE(actor_kind, ''), COALESCE(actor_client_id, ''), action, COALESCE(generation, version), COALESCE(detail, 'null'::jsonb)::text
		FROM audit_events WHERE namespace = $1 AND name = $2 AND (resource_kind = $3 OR (resource_kind IS NULL AND action = ANY($4)))
		ORDER BY id DESC LIMIT $5`, ns, name, kind, legacy, limit)
	if err != nil {
		return nil, true, err
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (AuditEvent, error) {
		var e AuditEvent
		var detail string
		scanErr := row.Scan(&e.At, &e.Actor.Name, &e.Actor.Kind, &e.Actor.ClientID, &e.Action, &e.Generation, &detail)
		if detail != "null" {
			e.Detail = json.RawMessage(detail)
		}
		return e, scanErr
	})
	return emptyIfNil(out), true, err
}
