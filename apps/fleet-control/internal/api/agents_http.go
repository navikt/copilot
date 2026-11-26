package api

import (
	"context"
	"errors"
	"net/http"
)

// --- handlers ---

func listProfilesNS(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	ns := r.PathValue("namespace")
	if err := memberOf(c, ns); err != nil {
		return err
	}
	items, err := db.profileObjects(r.Context(), ns)
	if err != nil {
		return err
	}
	writeObject(w, http.StatusOK, map[string]any{"apiVersion": apiGroupVersion, "kind": "AgentProfileList", "items": emptyIfNil(items)})
	return nil
}

// listAwaitingProfiles answers ?awaitingApprovalBy=me with excerpts of the
// profiles whose pending generation waits for a grant from the caller's teams.
func listAwaitingProfiles(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	if r.URL.Query().Get("awaitingApprovalBy") != "me" {
		return stErr(http.StatusBadRequest, ReasonBadRequest, "only ?awaitingApprovalBy=me is supported")
	}
	items, err := db.awaitingProfiles(r.Context(), c)
	if err != nil {
		return err
	}
	writeObject(w, http.StatusOK, map[string]any{"apiVersion": apiGroupVersion, "kind": "AgentProfileList", "items": emptyIfNil(items)})
	return nil
}

func getProfile(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	ns, name := r.PathValue("namespace"), r.PathValue("name")
	// A non-member learns nothing from timing: a hidden profile and a missing
	// one both cost this one query (#2696). Like excerpt, it reads the pending
	// generation when there is one.
	if !c.in(ns) {
		var shared bool
		if err := db.pool.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM agent_profile_resources r
			JOIN agent_profile_generations g ON g.namespace = r.namespace AND g.name = r.name AND g.generation = COALESCE(r.pending_generation, r.published_generation)
			CROSS JOIN jsonb_object_keys(g.grant_hashes) k
			WHERE r.namespace = $1 AND r.name = $2 AND split_part(k, '/', 1) = ANY($3))`, ns, name, c.Teams).Scan(&shared); err != nil {
			return err
		}
		if !shared {
			return errV1NotFound
		}
	}
	obj, err := db.profileObject(r.Context(), ns, name)
	switch {
	case errors.Is(err, errV1NotFound):
		return errV1NotFound
	case err != nil:
		return err
	case c.in(ns):
		w.Header().Set("ETag", `"`+obj.Metadata.ResourceVersion+`"`)
		writeObject(w, http.StatusOK, obj)
		return nil
	}
	ex, ok := excerpt(obj, c)
	if !ok {
		return errV1NotFound
	}
	writeObject(w, http.StatusOK, ex)
	return nil
}

// excerpt is what a server's owner team sees of another team's profile:
// identity, mode, quota and only their own grants, with the pending
// generation and the hash for those grants. No actors, history or other
// teams' grants.
func excerpt(obj AgentProfileV1, c caller) (AgentProfileV1, bool) {
	spec := obj.Spec
	if p := obj.Status.Pending; p != nil && p.Spec != nil {
		spec = *p.Spec
	}
	out := AgentProfileV1{APIVersion: obj.APIVersion, Kind: obj.Kind, Metadata: ObjectMeta{Name: obj.Metadata.Name, Namespace: obj.Metadata.Namespace}}
	out.Spec = AgentProfileSpec{Identity: spec.Identity, Mode: spec.Mode, Quota: spec.Quota}
	for _, g := range spec.Grants {
		if c.in(g.ServerRef.Namespace) {
			out.Spec.Grants = append(out.Spec.Grants, g)
		}
	}
	if len(out.Spec.Grants) == 0 {
		return out, false
	}
	for _, g := range obj.Status.Grants {
		if c.in(g.ServerRef.Namespace) {
			g.GrantedBy = nil
			out.Status.Grants = append(out.Status.Grants, g)
		}
	}
	if p := obj.Status.Pending; p != nil {
		out.Status.Pending = &Pending[AgentProfileSpec]{Generation: p.Generation}
	}
	// An expired profile stays visible with its expiry, as the owner team sees it (#2696).
	out.Status.ExpiresAt = obj.Status.ExpiresAt
	out.Status.NeedsResubmit = obj.Status.NeedsResubmit
	return out, true
}

func getProfileGeneration(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	ns, name := r.PathValue("namespace"), r.PathValue("name")
	if err := memberOf(c, ns); err != nil {
		return err
	}
	n, err := pathGeneration(r)
	if err != nil {
		return err
	}
	obj, err := db.profileObject(r.Context(), ns, name)
	if err != nil {
		return err
	}
	g, err := loadProfileGen(r.Context(), db.pool, ns, name, n)
	if err != nil {
		return err
	}
	obj.Spec = g.Spec
	obj.Metadata.Generation = int64(n)
	obj.Status = AgentProfileStatus{ObservedGeneration: obj.Status.ObservedGeneration, ContentHash: g.Hash, SubmittedBy: g.submitter(), ApprovedAt: g.ApprovedAt,
		IdentityVerifiedAt: g.VerifiedAt, ExpiresAt: g.ExpiresAt, Grants: grantHashStatus(g)}
	writeObject(w, http.StatusOK, obj)
	return nil
}

func grantHashStatus(g profileGen) []GrantStatus {
	out := make([]GrantStatus, 0, len(g.Spec.Grants))
	for _, gr := range g.Spec.Grants {
		out = append(out, GrantStatus{ServerRef: gr.ServerRef, ContentHash: g.GrantHashes[gr.ServerRef.key()]})
	}
	return out
}

func (s *server) createProfile(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	return s.submitProfileHTTP(w, r, c, db, true)
}

func (s *server) putProfile(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	return s.submitProfileHTTP(w, r, c, db, false)
}

func (s *server) submitProfileHTTP(w http.ResponseWriter, r *http.Request, c caller, db *DB, create bool) error {
	ns, name := r.PathValue("namespace"), r.PathValue("name")
	if err := memberOf(c, ns); err != nil {
		return err
	}
	in, err := decodeResource[AgentProfileSpec](w, r, "AgentProfile", ns, name)
	if err != nil {
		return err
	}
	spec, err := validateProfileSpec(in.Spec)
	if err != nil {
		return err
	}
	if err = checkGrantServers(r.Context(), db.pool, spec.Grants); err != nil {
		return err
	}
	verified := false
	if s.verifier != nil {
		id := AgentIdentity{Cluster: spec.Identity.Cluster, Namespace: ns, App: spec.Identity.App, EntraClientID: spec.Identity.EntraClientID}
		if verified, err = s.verifier.Verify(r.Context(), id); err != nil {
			return err
		}
		if !verified {
			return invalid("spec.identity.entraClientID", "the client id does not belong to %s:%s:%s", id.Cluster, ns, id.App)
		}
	}
	if err = db.submitProfile(r.Context(), c, in.Metadata, spec, verified, create); err != nil {
		return err
	}
	obj, err := db.profileObject(r.Context(), ns, in.Metadata.Name)
	if err != nil {
		return err
	}
	code := http.StatusOK
	if create {
		code = http.StatusCreated
	}
	w.Header().Set("ETag", `"`+obj.Metadata.ResourceVersion+`"`)
	writeObject(w, code, obj)
	return nil
}

// grantTeamGuard lets only the owner team of ref act on its grant. A member
// of the profile's team gets 403, anyone else 404.
func grantTeamGuard(c caller, ns string, ref TypedRef) error {
	switch {
	case !dnsLabelRegex.MatchString(ref.Namespace) || !dnsLabelRegex.MatchString(ref.Name):
		return stErr(http.StatusBadRequest, ReasonBadRequest, "serverRef must have DNS-label namespace and name")
	case c.in(ref.Namespace):
		return nil
	case c.in(ns):
		return stErr(http.StatusForbidden, ReasonNotServerOwnerTeam, "only the owner team of MCP server %s can act on its grant", ref.key())
	}
	return errV1NotFound
}

func approveProfile(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	ns, name := r.PathValue("namespace"), r.PathValue("name")
	var body struct {
		Generation  int      `json:"generation"`
		ServerRef   TypedRef `json:"serverRef"`
		ContentHash string   `json:"contentHash"`
	}
	if err := decodeBody(w, r, &body); err != nil {
		return stErr(http.StatusBadRequest, ReasonBadRequest, "%v", err)
	}
	if err := grantTeamGuard(c, ns, body.ServerRef); err != nil {
		return err
	}
	if err := db.grantProfile(r.Context(), c, ns, name, body.Generation, body.ServerRef, body.ContentHash); err != nil {
		return err
	}
	return writeProfileFor(r.Context(), w, db, c, ns, name)
}

func (s *server) revokeProfile(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	ns, name := r.PathValue("namespace"), r.PathValue("name")
	var body struct {
		Reason    string    `json:"reason"`
		ServerRef *TypedRef `json:"serverRef"`
	}
	if err := decodeBody(w, r, &body); err != nil {
		return stErr(http.StatusBadRequest, ReasonBadRequest, "%v", err)
	}
	by := ""
	var err error
	if body.ServerRef != nil {
		by, err = "server-owner", grantTeamGuard(c, ns, *body.ServerRef)
	} else {
		by, err = s.operatorOr(c, ns)
	}
	if err != nil {
		return err
	}
	reason, err := reasonText(body.Reason)
	if err != nil {
		return err
	}
	if err := db.revokeProfileV1(r.Context(), c, by, ns, name, reason, body.ServerRef); err != nil {
		return err
	}
	return writeProfileFor(r.Context(), w, db, c, ns, name)
}

func writeProfileFor(ctx context.Context, w http.ResponseWriter, db *DB, c caller, ns, name string) error {
	obj, err := db.profileObject(ctx, ns, name)
	if err != nil {
		return err
	}
	if !c.in(ns) {
		ex, ok := excerpt(obj, c)
		if !ok {
			// The platform team stopping a profile sees only its identity and state.
			ex = AgentProfileV1{APIVersion: obj.APIVersion, Kind: obj.Kind, Metadata: ObjectMeta{Name: name, Namespace: ns}}
			ex.Status.Conditions = obj.Status.Conditions
		}
		obj = ex
	}
	writeObject(w, http.StatusOK, obj)
	return nil
}
