package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/navikt/copilot/fleet-control/internal/canon"
	"github.com/navikt/copilot/fleet-control/internal/model"
	"github.com/navikt/copilot/fleet-control/internal/store"
)

// auditV1 writes an audit row with the actor's form and oid in their own
// columns. detail never holds an oid. version mirrors generation, so the
// /admin/v1 audit view keeps working.
func auditV1(ctx context.Context, tx pgx.Tx, c caller, action, kind, ns, name string, gen *int, detail map[string]any) error {
	return store.InsertAudit(ctx, tx, store.AuditRow{Actor: c.Ident, ActorKind: c.actor().Kind, ActorOid: c.Oid, ActorClientID: c.Azp,
		Action: action, ResourceKind: kind, Namespace: ns, Name: name, Version: gen, Generation: gen, Detail: detail})
}

// blockedContent is 422 when any of hashes equals a blocked generation's
// hash on this resource. Hashes compare as whole strings (see canon.Hashes).
func blockedContent(ctx context.Context, tx pgx.Tx, table, ns, name string, hashes []string) error {
	var blocked bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM `+table+` WHERE namespace = $1 AND name = $2 AND blocked AND spec_hash = ANY($3))`, ns, name, hashes).Scan(&blocked); err != nil {
		return err
	}
	if blocked {
		return invalid("spec", "this content is blocked and can never be published again").because(ReasonContentBlocked)
	}
	return nil
}

// submitServer is POST (create) and PUT. A new generation is made only when
// the spec differs from both the pending and the published one, or when the
// server is revoked (restart). Otherwise only labels and annotations change.
// It returns the changed fields (non-nil) when the generation auto-published.
func (d *DB) submitServer(ctx context.Context, c caller, m ObjectMeta, spec MCPServerSpec, sv model.Server, create bool) ([]string, error) {
	hashes, herr := canon.Hashes(spec)
	if herr != nil {
		return nil, herr
	}
	hash := hashes[0]
	var auto []string
	err := pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		if create {
			tag, err := tx.Exec(ctx, `INSERT INTO servers (namespace, name, status, labels, annotations) VALUES ($1, $2, 'draft', $3::jsonb, $4::jsonb) ON CONFLICT DO NOTHING`,
				sv.Namespace, sv.Name, jsonText(m.Labels), jsonText(m.Annotations))
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return stErr(http.StatusConflict, ReasonAlreadyExists, "mcpservers %q already exists", sv.Name)
			}
		}
		row, err := lockServerRow(ctx, tx, sv.Namespace, sv.Name)
		if err != nil {
			return err
		}
		if !create {
			if err = requireResourceVersion(m.ResourceVersion, row.RV); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `UPDATE servers SET labels = $3::jsonb, annotations = $4::jsonb WHERE namespace = $1 AND name = $2`,
				sv.Namespace, sv.Name, jsonText(m.Labels), jsonText(m.Annotations)); err != nil {
				return err
			}
			var now time.Time
			if now, err = txNow(ctx, tx); err != nil {
				return err
			}
			same := false
			for _, v := range []*int{row.Pending, row.Published} {
				if v == nil {
					continue
				}
				var h *string
				var at time.Time
				if err = tx.QueryRow(ctx, `SELECT spec_hash, created_at FROM server_versions WHERE namespace = $1 AND name = $2 AND version = $3`, sv.Namespace, sv.Name, *v).Scan(&h, &at); err != nil {
					return err
				}
				// An expired pending generation does not count: the same content may be submitted again.
				live := v == row.Published || now.Before(at.Add(pendingValidity))
				same = same || (live && deref(h) == hash)
			}
			if same && row.Status != "revoked" {
				return auditV1(ctx, tx, c, "metadata_updated", "MCPServer", sv.Namespace, sv.Name, nil, map[string]any{"labels": m.Labels, "annotations": m.Annotations})
			}
		}
		if err = blockedContent(ctx, tx, "server_versions", sv.Namespace, sv.Name, hashes); err != nil {
			return err
		}
		version := row.Latest + 1
		var registry, path, target, upstream *string
		var tools []string
		if sv.Registry != nil {
			r := string(sv.Registry)
			registry = &r
		}
		if r := sv.Route; r != nil {
			path, target, upstream, tools = &r.Path, &r.Target, &r.UpstreamURL, r.Tools
		}
		specJSON, err := json.Marshal(spec)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO server_versions (namespace, name, version, registry, route_path, target, upstream_url, enabled, tools,
			submitted_by, submitted_by_kind, submitted_by_oid, spec, spec_hash)
			VALUES ($1, $2, $3, $4::jsonb, $5, $6, $7, true, $8, $9, 'User', $10::uuid, $11::jsonb, $12)`,
			sv.Namespace, sv.Name, version, registry, path, target, upstream, tools, c.Ident, c.Oid, string(specJSON), hash); err != nil {
			return err
		}
		change, err := classifySubmit(ctx, tx, row, sv.Namespace, sv.Name, specJSON)
		if err != nil {
			return err
		}
		if !change.Escalates {
			auto = append([]string{}, change.Changed...)
			return autoPublish(ctx, tx, c, sv, version, hash, row.Pending, change.Changed)
		}
		if _, err := tx.Exec(ctx, `UPDATE servers SET pending_version = $3, updated_at = now() WHERE namespace = $1 AND name = $2`, sv.Namespace, sv.Name, version); err != nil {
			return err
		}
		return auditV1(ctx, tx, c, "draft_submitted", "MCPServer", sv.Namespace, sv.Name, &version, map[string]any{"contentHash": hash, "replaced": row.Pending, "escalations": change.Reasons})
	})
	if err != nil {
		return nil, err
	}
	return auto, nil
}

// classifySubmit compares a new generation with the published one. No
// published generation, a revoked server or a published generation the
// platform team never approved (approved_via NULL) always escalates.
func classifySubmit(ctx context.Context, tx pgx.Tx, row serverRow, ns, name string, next []byte) (model.Change, error) {
	if row.Status != "published" || row.Published == nil {
		return model.Classify(nil, next), nil
	}
	var prev string
	var via *string
	if err := tx.QueryRow(ctx, `SELECT COALESCE(v.spec, `+legacySpecSQL+`)::text, v.approved_via FROM server_versions v
		WHERE v.namespace = $1 AND v.name = $2 AND v.version = $3`, ns, name, *row.Published).Scan(&prev, &via); err != nil {
		return model.Change{}, err
	}
	if via == nil {
		return model.Change{Escalates: true, Reasons: []string{"the published version was not approved by the platform team"}}, nil
	}
	return model.Classify(json.RawMessage(prev), next), nil
}

// autoPublish publishes an owner-team change that adds no access, in the
// submit transaction, with the same route-lock and snapshot-budget checks
// as approveServer. A pending generation it replaces is dropped.
func autoPublish(ctx context.Context, tx pgx.Tx, c caller, sv model.Server, version int, hash string, replaced *int, changed []string) error {
	before, err := lockRoutes(ctx, tx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE server_versions SET approved_by = 'auto', approved_at = now(), published_at = now(), approved_via = 'auto'
		WHERE namespace = $1 AND name = $2 AND version = $3`, sv.Namespace, sv.Name, version); err != nil {
		return err
	}
	var reg *string
	if sv.RegistryName != "" {
		reg = &sv.RegistryName
	}
	if _, err := tx.Exec(ctx, `UPDATE servers SET published_version = $3, pending_version = NULL, status = 'published', registry_name = $4, updated_at = now()
		WHERE namespace = $1 AND name = $2`, sv.Namespace, sv.Name, version, reg); err != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23505" {
			return conflict("spec.registry.name", ReasonRegistryNameTaken, "registry name is used by another server")
		}
		return err
	}
	if err := checkRoutesFit(ctx, tx, sv.Namespace, before); err != nil {
		return err
	}
	return auditV1(ctx, tx, c, "auto_published", "MCPServer", sv.Namespace, sv.Name, &version,
		map[string]any{"contentHash": hash, "approvedBy": "auto", "changed": changed, "replaced": replaced})
}

// approveServer publishes the pending generation. The approver is a member
// of the platform team and may be the submitter (self_approved). The
// submitter is a User with an oid.
func (d *DB) approveServer(ctx context.Context, c caller, platform, ns, name string, gen int, hash string) error {
	expired := false
	err := pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		row, err := lockServerRow(ctx, tx, ns, name)
		if err != nil {
			return err
		}
		if row.Pending == nil || *row.Pending != gen {
			return conflict("generation", ReasonGenerationNotPending, "generation %d is not the one awaiting approval", gen)
		}
		v, err := loadVersion(ctx, tx, ns, name, gen)
		if err != nil {
			return err
		}
		now, err := txNow(ctx, tx)
		if err != nil {
			return err
		}
		if !now.Before(v.expiresAt()) {
			// Commit only the audit row; the caller gets 410. Record the stored hash, never the caller's.
			expired = true
			return auditV1(ctx, tx, c, "approval_expired", "MCPServer", ns, name, &gen, map[string]any{"outcome": "expired", "contentHash": deref(v.Hash), "expiresAt": v.expiresAt()})
		}
		switch {
		case v.Hash == nil || *v.Hash != hash || !strings.HasPrefix(hash, canon.Prefix):
			return conflict("contentHash", ReasonContentHashMismatch, "contentHash does not match the pending generation")
		case deref(v.SubKind) != "User" || v.SubOid == nil:
			return stErr(http.StatusForbidden, ReasonSubmitterNotPerson, "the pending generation has no person as submitter; a person must submit it again")
		case c.Oid == "":
			// self_approved compares oids; an empty one would also fail the uuid cast below.
			return stErr(http.StatusForbidden, ReasonApproverNoOid, "the approver has no object id")
		case platform == "" || !c.in(platform):
			return stErr(http.StatusForbidden, ReasonNotPlatformTeam, "only the platform team approves MCP server versions")
		case v.RegName != nil && !registryNameAllowed(platform, ns, *v.RegName):
			return invalid("spec.registry.name", "generation %d has registry name %q, which team %s may not use", gen, *v.RegName, ns).because(ReasonRegistryNameNotYours)
		}
		hashes := []string{hash}
		if err = blockedContent(ctx, tx, "server_versions", ns, name, hashes); err != nil {
			return err
		}
		// Routes are locked after the expiry and approver checks, so neither waits on the route lock.
		before, err := lockRoutes(ctx, tx)
		if err != nil {
			return err
		}
		self := *v.SubOid == c.Oid || v.SubName == c.Ident
		if _, err := tx.Exec(ctx, `UPDATE server_versions SET approved_by = $4, approved_by_oid = $5::uuid, approved_at = now(), published_at = now(), self_approved = $6, approved_via = 'platform'
			WHERE namespace = $1 AND name = $2 AND version = $3`, ns, name, gen, c.Ident, c.Oid, self); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE servers SET published_version = $3, pending_version = NULL, status = 'published', registry_name = $4, updated_at = now()
			WHERE namespace = $1 AND name = $2`, ns, name, gen, v.RegName); err != nil {
			if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23505" {
				return conflict("spec.registry.name", ReasonRegistryNameTaken, "registry name is used by another server")
			}
			return err
		}
		if err := checkRoutesFit(ctx, tx, ns, before); err != nil {
			return err
		}
		detail := map[string]any{"contentHash": hash, "submittedBy": v.SubName, "outcome": "approved"}
		if self {
			detail["outcome"] = "self_approved"
		}
		return auditV1(ctx, tx, c, "approved_published", "MCPServer", ns, name, &gen, detail)
	})
	if err == nil && expired {
		return errPendingExpired(gen)
	}
	return err
}

// revokeServer stops the published generation. It is final for that
// generation: rollback never undoes it.
func (d *DB) revokeServer(ctx context.Context, c caller, by, ns, name, reason string, block bool) error {
	return pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		row, err := lockServerRow(ctx, tx, ns, name)
		if err != nil {
			return err
		}
		if row.Status != "published" {
			return conflict("", ReasonNotPublished, "server is %s, not published", row.Status)
		}
		if _, err := tx.Exec(ctx, `UPDATE servers SET status = 'revoked', updated_at = now() WHERE namespace = $1 AND name = $2`, ns, name); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE server_versions SET revoked_at = clock_timestamp(), blocked = blocked OR $4 WHERE namespace = $1 AND name = $2 AND version = $3`, ns, name, *row.Published, block); err != nil {
			return err
		}
		return auditV1(ctx, tx, c, "revoked", "MCPServer", ns, name, row.Published, map[string]any{"reason": reason, "blocked": block, "by": by})
	})
}

// rollbackServer republishes an earlier published generation. fromGeneration
// must be the published one. A refused target is 409, never a skip to an
// older generation the person did not choose.
func (d *DB) rollbackServer(ctx context.Context, c caller, platform, by, ns, name string, from int, to *int, reason string) error {
	return pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		row, err := lockServerRow(ctx, tx, ns, name)
		if err != nil {
			return err
		}
		before, err := lockRoutes(ctx, tx)
		if err != nil {
			return err
		}
		if row.Status != "published" {
			return conflict("", ReasonRollbackRevoked, "server is %s; rollback never lifts a revocation", row.Status)
		}
		if *row.Published != from {
			return conflict("fromGeneration", ReasonFromGenerationMismatch, "the published generation is %d", *row.Published)
		}
		target := 0
		if to != nil {
			target = *to
		} else if err = tx.QueryRow(ctx, `SELECT COALESCE(max(version), 0) FROM server_versions WHERE namespace = $1 AND name = $2 AND published_at IS NOT NULL AND version < $3`, ns, name, from).Scan(&target); err != nil {
			return err
		}
		if target < 1 || target >= from {
			return conflict("toGeneration", ReasonRollbackTargetNotEligible, "rollback goes only to an earlier generation that was published before")
		}
		v, err := loadVersion(ctx, tx, ns, name, target)
		if errors.Is(err, errV1NotFound) || (err == nil && v.PublishedAt == nil) {
			return conflict("toGeneration", ReasonRollbackTargetNotEligible, "generation %d was never published", target)
		}
		if err != nil {
			return err
		}
		// Only to a generation the platform team approved, directly or as an
		// owner change that added no access (auto) or a git seed (#2634).
		var via *string
		if err := tx.QueryRow(ctx, `SELECT approved_via FROM server_versions WHERE namespace = $1 AND name = $2 AND version = $3`, ns, name, target).Scan(&via); err != nil {
			return err
		}
		if via == nil {
			return conflict("toGeneration", ReasonRollbackTargetNotApproved, "generation %d was not approved by the platform team", target)
		}
		var refused bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM server_versions WHERE namespace = $1 AND name = $2 AND (revoked_at IS NOT NULL OR blocked) AND spec_hash = $3)`, ns, name, v.Hash).Scan(&refused); err != nil {
			return err
		}
		switch {
		case v.Blocked:
			return conflict("toGeneration", ReasonGenerationBlocked, "generation %d is blocked", target)
		case v.RevokedAt != nil:
			return conflict("toGeneration", ReasonGenerationRevoked, "generation %d was revoked", target)
		case refused:
			return conflict("toGeneration", ReasonContentRefused, "generation %d has the same content as a revoked or blocked generation", target)
		}
		if v.RegName != nil && !registryNameAllowed(platform, ns, *v.RegName) {
			return invalid("spec.registry.name", "generation %d has registry name %q, which team %s may not use", target, *v.RegName, ns).because(ReasonRegistryNameNotYours)
		}
		if _, err := tx.Exec(ctx, `UPDATE servers SET published_version = $3, registry_name = $4, updated_at = now() WHERE namespace = $1 AND name = $2`, ns, name, target, v.RegName); err != nil {
			if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23505" {
				return conflict("spec.registry.name", ReasonRegistryNameTaken, "registry name is used by another server")
			}
			return err
		}
		if err := checkRoutesFit(ctx, tx, ns, before); err != nil {
			return err
		}
		return auditV1(ctx, tx, c, "rolled_back", "MCPServer", ns, name, &target, map[string]any{"reason": reason, "from": from, "by": by})
	})
}

// BackfillSpecs fills spec and spec_hash for every server version that was
// ever published and has none. It runs on every start and touches only rows
// without spec, so it catches rows that old pods wrote during a rolling
// deploy or after a rollback. Pending versions never get a hash: they must
// be submitted again by a person.
func (d *DB) BackfillSpecs(ctx context.Context) (int, error) {
	n := 0
	err := pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", store.LockKey); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT v.namespace, v.name, v.version, `+legacySpecSQL+`::text FROM server_versions v WHERE v.spec IS NULL AND v.published_at IS NOT NULL`)
		if err != nil {
			return err
		}
		type todo struct {
			NS, Name string
			V        int
			Spec     string
		}
		list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[todo])
		if err != nil {
			return err
		}
		for _, t := range list {
			var spec MCPServerSpec
			if err := json.Unmarshal([]byte(t.Spec), &spec); err != nil {
				return err
			}
			spec.AllowedTools = slices.Sorted(slices.Values(spec.AllowedTools))
			if len(spec.AllowedTools) == 0 {
				spec.AllowedTools = nil
			}
			hash, err := canon.Hash(spec)
			if err != nil {
				return err
			}
			raw, _ := json.Marshal(spec)
			if _, err := tx.Exec(ctx, `UPDATE server_versions SET spec = $4::jsonb, spec_hash = $5 WHERE namespace = $1 AND name = $2 AND version = $3 AND spec IS NULL`,
				t.NS, t.Name, t.V, string(raw), hash); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	return n, err
}
