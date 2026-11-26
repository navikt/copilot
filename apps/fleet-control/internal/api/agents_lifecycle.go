package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/navikt/copilot/fleet-control/internal/canon"
)

// releaseExpiredClientIDs frees client ids of profiles whose approval has
// expired, in the caller's transaction. shortcut: done lazily when a client
// id is claimed, not by a job; a job is needed once expiry must stop
// gateway traffic on time (it does not today: /internal/v1/agents filters).
func releaseExpiredClientIDs(ctx context.Context, tx pgx.Tx, clientID string) error {
	_, err := tx.Exec(ctx, `UPDATE agent_profile_resources r SET published_client_id = NULL
		FROM agent_profile_generations g
		WHERE g.namespace = r.namespace AND g.name = r.name AND g.generation = r.published_generation
		  AND r.published_client_id = $1::uuid AND g.expires_at <= now()`, clientID)
	return err
}

func (d *DB) submitProfile(ctx context.Context, c caller, m ObjectMeta, spec AgentProfileSpec, verified, create bool) error {
	return d.insertProfileGeneration(ctx, c, m, spec, verified, create)
}

func (d *DB) insertProfileGeneration(ctx context.Context, c caller, m ObjectMeta, spec AgentProfileSpec, verified, create bool) error {
	hashes, err := canon.Hashes(spec)
	if err != nil {
		return err
	}
	hash := hashes[0]
	grantHashes := map[string]string{}
	for _, g := range spec.Grants {
		if grantHashes[g.ServerRef.key()], err = grantHash(spec, g); err != nil {
			return err
		}
	}
	ns, name := m.Namespace, m.Name
	return pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		if create {
			tag, err := tx.Exec(ctx, `INSERT INTO agent_profile_resources (namespace, name, labels, annotations) VALUES ($1, $2, $3::jsonb, $4::jsonb) ON CONFLICT DO NOTHING`,
				ns, name, jsonText(m.Labels), jsonText(m.Annotations))
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return stErr(http.StatusConflict, ReasonAlreadyExists, "agentprofiles %q already exists", name)
			}
		}
		row, err := scanProfileRow(tx.QueryRow(ctx, `SELECT `+profileRowCols+` FROM agent_profile_resources r WHERE r.namespace = $1 AND r.name = $2 FOR UPDATE`, ns, name))
		if err != nil {
			return err
		}
		if !create {
			if err := requireResourceVersion(m.ResourceVersion, row.RV); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE agent_profile_resources SET labels = $3::jsonb, annotations = $4::jsonb WHERE namespace = $1 AND name = $2`,
				ns, name, jsonText(m.Labels), jsonText(m.Annotations)); err != nil {
				return err
			}
			var same bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM agent_profile_generations WHERE namespace = $1 AND name = $2
				AND spec_hash = $5 AND (generation = $4 OR (generation = $3 AND created_at > now() - $6::interval)))`,
				ns, name, row.Pending, row.Published, hash, pendingValidity).Scan(&same); err != nil {
				return err
			}
			if same && !row.Revoked {
				return auditV1(ctx, tx, c, "metadata_updated", "AgentProfile", ns, name, nil, map[string]any{"labels": m.Labels, "annotations": m.Annotations})
			}
		}
		if err := blockedContent(ctx, tx, "agent_profile_generations", ns, name, hashes); err != nil {
			return err
		}
		if err := releaseExpiredClientIDs(ctx, tx, spec.Identity.EntraClientID); err != nil {
			return err
		}
		var taken bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM agent_profile_resources WHERE published_client_id = $1::uuid AND NOT (namespace = $2 AND name = $3))`,
			spec.Identity.EntraClientID, ns, name).Scan(&taken); err != nil {
			return err
		}
		if taken {
			return conflict("spec.identity.entraClientID", ReasonClientIDInUse, "the client id is in another live profile")
		}
		gen := row.Latest + 1
		specJSON, _ := json.Marshal(spec)
		hashJSON, _ := json.Marshal(grantHashes)
		var oid *string
		if c.Oid != "" {
			oid = &c.Oid
		}
		// Written as both entra_client_id and {issuer, subject} (#2896).
		if entraIssuer == "" {
			return stErr(http.StatusServiceUnavailable, ReasonIssuerNotConfigured, "the identity issuer is not configured")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO agent_profile_generations (namespace, name, generation, spec, spec_hash, entra_client_id, submitted_by, submitted_by_kind,
			submitted_by_oid, identity_verified_at, grant_hashes, identity_issuer, identity_subject)
			VALUES ($1, $2, $3, $4::jsonb, $5, $6::uuid, $7, $8, $9::uuid, CASE WHEN $10 THEN now() END, $11::jsonb, $12, $13)`,
			ns, name, gen, string(specJSON), hash, spec.Identity.EntraClientID, c.Ident, c.actor().Kind, oid, verified, string(hashJSON), entraIssuer, spec.Identity.EntraClientID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE agent_profile_resources SET pending_generation = $3 WHERE namespace = $1 AND name = $2`, ns, name, gen); err != nil {
			return err
		}
		return auditV1(ctx, tx, c, "agent_profile_submitted", "AgentProfile", ns, name, &gen, map[string]any{"contentHash": hash, "grantHashes": grantHashes, "identityVerified": verified})
	})
}

// grantVisible is errV1NotFound unless the caller is in the profile's team
// or ref is granted in its pending or published generation. It runs right
// after the row lock, before any check that reveals state, so a stranger
// gets the same 404 for a missing profile and one that is not theirs.
func grantVisible(ctx context.Context, tx pgx.Tx, c caller, row profileRow, ref TypedRef) error {
	if c.in(row.NS) {
		return nil
	}
	var granted bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM agent_profile_generations WHERE namespace = $1 AND name = $2
		AND generation IN ($3, $4) AND grant_hashes ? $5)`, row.NS, row.Name, row.Pending, row.Published, ref.key()).Scan(&granted); err != nil {
		return err
	}
	if !granted {
		return errV1NotFound
	}
	return nil
}

// grantProfile gives one grant on the pending generation. The profile is
// published once every grant in that generation is given.
func (d *DB) grantProfile(ctx context.Context, c caller, ns, name string, gen int, ref TypedRef, hash string) error {
	expired := false
	err := pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		row, err := scanProfileRow(tx.QueryRow(ctx, `SELECT `+profileRowCols+` FROM agent_profile_resources r WHERE r.namespace = $1 AND r.name = $2 FOR UPDATE`, ns, name))
		if err != nil {
			return err
		}
		if err = grantVisible(ctx, tx, c, row, ref); err != nil {
			return err
		}
		if row.Pending == nil || *row.Pending != gen {
			return conflict("generation", ReasonGenerationNotPending, "generation %d is not the one awaiting approval", gen)
		}
		g, err := loadProfileGen(ctx, tx, ns, name, gen)
		if err != nil {
			return err
		}
		want, ok := g.GrantHashes[ref.key()]
		i := slices.IndexFunc(g.Spec.Grants, func(gr AgentGrantV1) bool { return gr.ServerRef.key() == ref.key() })
		if !ok || i < 0 {
			// Before the expiry check: only a grant in this generation may write an approval_expired row.
			return conflict("serverRef", ReasonGrantNotInGeneration, "generation %d grants nothing on %s", gen, ref.key())
		}
		now, err := txNow(ctx, tx)
		if err != nil {
			return err
		}
		if !now.Before(g.pendingExpiresAt()) {
			// Commit only the audit row; the caller gets 410. Earlier grants lapse with the generation.
			// Record the stored hash, never the caller's.
			expired = true
			return auditV1(ctx, tx, c, "approval_expired", "AgentProfile", ns, name, &gen, map[string]any{"outcome": "expired", "server": ref.key(), "contentHash": want, "expiresAt": g.pendingExpiresAt()})
		}
		switch {
		case want != hash || !strings.HasPrefix(hash, canon.Prefix):
			return conflict("contentHash", ReasonContentHashMismatch, "contentHash does not match the grant on %s", ref.key())
		case g.SubKind != "User" || g.SubOid == nil:
			return stErr(http.StatusForbidden, ReasonSubmitterNotPerson, "the pending generation has no person as submitter; a person must submit it again")
		case *g.SubOid == c.Oid || g.SubName == c.Ident:
			return stErr(http.StatusForbidden, ReasonSelfApproval, "the submitter cannot grant access to their own profile")
		}
		// FOR SHARE waits for, and blocks, a concurrent server write.
		if _, err = tx.Exec(ctx, `SELECT 1 FROM servers WHERE namespace = $1 AND name = $2 FOR SHARE`, ref.Namespace, ref.Name); err != nil {
			return err
		}
		if err = checkGrantServers(ctx, tx, g.Spec.Grants[i:i+1]); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `INSERT INTO agent_profile_grants (namespace, name, generation, server, content_hash, granted_by, granted_by_kind, granted_by_oid, granted_at)
			VALUES ($1, $2, $3, $4, $5, $6, 'User', $7::uuid, clock_timestamp()) ON CONFLICT DO NOTHING`, ns, name, gen, ref.key(), hash, c.Ident, c.Oid)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return conflict("serverRef", ReasonAlreadyGranted, "access to %s is already granted", ref.key())
		}
		if err := auditV1(ctx, tx, c, "agent_profile_access_granted", "AgentProfile", ns, name, &gen, map[string]any{"server": ref.key(), "contentHash": hash}); err != nil {
			return err
		}
		var granted int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM agent_profile_grants WHERE namespace = $1 AND name = $2 AND generation = $3`, ns, name, gen).Scan(&granted); err != nil {
			return err
		}
		if granted < len(g.Spec.Grants) {
			// Touch the resource so resourceVersion and the ETag move.
			_, err := tx.Exec(ctx, `UPDATE agent_profile_resources SET revoked = revoked WHERE namespace = $1 AND name = $2`, ns, name)
			return err
		}
		// Every server may have changed since its grant: lock them all in key
		// order and check each grant is still valid before publishing.
		for _, gr := range g.Spec.Grants { // sorted by serverRef at validation
			if _, err := tx.Exec(ctx, `SELECT 1 FROM servers WHERE namespace = $1 AND name = $2 FOR SHARE`, gr.ServerRef.Namespace, gr.ServerRef.Name); err != nil {
				return err
			}
		}
		if err := checkGrantServers(ctx, tx, g.Spec.Grants); err != nil {
			return err
		}
		var stale bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM agent_profile_grants a JOIN server_versions v
			ON v.namespace = split_part(a.server, '/', 1) AND v.name = split_part(a.server, '/', 2)
			WHERE a.namespace = $1 AND a.name = $2 AND a.generation = $3 AND (a.revoked_at IS NOT NULL OR v.revoked_at > a.granted_at))`, ns, name, gen).Scan(&stale); err != nil {
			return err
		}
		if stale {
			return conflict("serverRef", ReasonGrantRevokedSinceApproval, "a grant was revoked, or its server revoked, after it was given; submit the profile again")
		}
		if err := releaseExpiredClientIDs(ctx, tx, g.ClientID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE agent_profile_resources SET published_generation = $3, pending_generation = NULL, revoked = false,
			published_client_id = CASE WHEN $5 THEN $4::uuid END WHERE namespace = $1 AND name = $2`, ns, name, gen, g.ClientID, g.VerifiedAt != nil); err != nil {
			if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23505" {
				return conflict("spec.identity.entraClientID", ReasonClientIDInUse, "the client id is in another live profile")
			}
			return err
		}
		// The six months run from the oldest grant, not the last one (#2731).
		if _, err := tx.Exec(ctx, `UPDATE agent_profile_generations SET approved_at = now(), expires_at = (SELECT min(granted_at) FROM agent_profile_grants
			WHERE namespace = $1 AND name = $2 AND generation = $3) + $4::interval
			WHERE namespace = $1 AND name = $2 AND generation = $3`, ns, name, gen, approvalValidity); err != nil {
			return err
		}
		return auditV1(ctx, tx, c, "agent_profile_published", "AgentProfile", ns, name, &gen, map[string]any{"contentHash": g.Hash})
	})
	if err == nil && expired {
		return errPendingExpired(gen)
	}
	return err
}

// revokeProfileV1 revokes the whole published profile, or with ref only
// that grant. Final for the generation: a restart is a new submission.
func (d *DB) revokeProfileV1(ctx context.Context, c caller, by, ns, name, reason string, ref *TypedRef) error {
	return pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		row, err := scanProfileRow(tx.QueryRow(ctx, `SELECT `+profileRowCols+` FROM agent_profile_resources r WHERE r.namespace = $1 AND r.name = $2 FOR UPDATE`, ns, name))
		if err != nil {
			return err
		}
		if ref != nil {
			if err := grantVisible(ctx, tx, c, row, *ref); err != nil {
				return err
			}
		}
		if row.Published == nil || row.Revoked {
			return conflict("", ReasonNotPublished, "profile is not published")
		}
		if ref != nil {
			tag, err := tx.Exec(ctx, `UPDATE agent_profile_grants SET revoked_at = clock_timestamp() WHERE namespace = $1 AND name = $2 AND generation = $3 AND server = $4 AND revoked_at IS NULL`,
				ns, name, *row.Published, ref.key())
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return conflict("serverRef", ReasonNoActiveGrant, "no active grant on %s", ref.key())
			}
			// Touch the resource so resourceVersion moves.
			if _, err := tx.Exec(ctx, `UPDATE agent_profile_resources SET revoked = revoked WHERE namespace = $1 AND name = $2`, ns, name); err != nil {
				return err
			}
			return auditV1(ctx, tx, c, "agent_profile_access_revoked", "AgentProfile", ns, name, row.Published, map[string]any{"reason": reason, "server": ref.key()})
		}
		if _, err := tx.Exec(ctx, `UPDATE agent_profile_resources SET revoked = true, published_client_id = NULL WHERE namespace = $1 AND name = $2`, ns, name); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE agent_profile_generations SET revoked_at = now() WHERE namespace = $1 AND name = $2 AND generation = $3`, ns, name, *row.Published); err != nil {
			return err
		}
		return auditV1(ctx, tx, c, "agent_profile_revoked", "AgentProfile", ns, name, row.Published, map[string]any{"reason": reason, "by": by})
	})
}

// seedProfileV1 stores a seeded profile as generation 1 by kind System. A
// seeded author is not a person, so nobody can grant it until a person
// submits it again.
func (d *DB) seedProfileV1(ctx context.Context, p AgentProfile) error {
	spec := AgentProfileSpec{
		Identity: AgentIdentityV1{App: p.Identity.App, Cluster: p.Identity.Cluster, EntraClientID: p.Identity.EntraClientID},
		Mode:     "Delegated",
		Quota:    AgentQuotaV1{MaxInFlight: p.Quota.MaxInFlight, RequestsPerMinute: p.Quota.RPM, DailyCallsPerUser: p.Quota.DailyCallsPerUser, DailyCalls: p.Quota.DailyCalls},
		DPIA:     &AgentDPIAV1{Owner: p.DPIA.Owner, Reference: p.DPIA.Reference},
	}
	for _, g := range p.Grants {
		ns, name, _ := strings.Cut(g.Route, "/")
		spec.Grants = append(spec.Grants, AgentGrantV1{ServerRef: TypedRef{Group: apiGroup, Kind: "MCPServer", Namespace: ns, Name: name}, AllowedTools: g.Tools, MaxRisk: "Read"})
	}
	spec, err := validateProfileSpec(spec)
	if err != nil {
		return err
	}
	err = d.insertProfileGeneration(ctx, caller{Ident: "seed", Kind: "System"}, ObjectMeta{Namespace: p.Namespace, Name: p.Name}, spec, false, true)
	if se, ok := errors.AsType[*statusError](err); ok && se.reason == "AlreadyExists" {
		return nil
	}
	return err
}
