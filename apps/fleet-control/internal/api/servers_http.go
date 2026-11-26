package api

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
)

// --- handlers ---

func (s *server) listServersNS(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	ns := r.PathValue("namespace")
	if err := s.serverReader(c, ns); err != nil {
		return err
	}
	return writeServerList(r.Context(), w, db, []string{ns})
}

// listMyServers lists the caller's teams' servers. The platform team also
// gets every namespace with a generation awaiting approval.
func (s *server) listMyServers(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	teams := c.Teams
	if s.isPlatform(c) {
		rows, err := db.pool.Query(r.Context(), `SELECT DISTINCT namespace FROM servers WHERE pending_version IS NOT NULL`)
		if err != nil {
			return err
		}
		pending, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return err
		}
		teams = append(slices.Clone(teams), pending...)
	}
	return writeServerList(r.Context(), w, db, teams)
}

// isPlatform: the caller is in the platform team. Unset team: nobody.
func (s *server) isPlatform(c caller) bool {
	return s.operatorTeam != "" && c.in(s.operatorTeam)
}

// serverReader lets the owner team and the platform team, which approves
// in every namespace, read an MCPServer. Anyone else gets 404.
func (s *server) serverReader(c caller, ns string) error {
	if s.isPlatform(c) {
		return nil
	}
	return memberOf(c, ns)
}

func writeServerList(ctx context.Context, w http.ResponseWriter, db *DB, teams []string) error {
	items, err := db.serverObjects(ctx, teams, "")
	if err != nil {
		return err
	}
	writeObject(w, http.StatusOK, map[string]any{"apiVersion": apiGroupVersion, "kind": "MCPServerList", "items": emptyIfNil(items)})
	return nil
}

func (s *server) getServerV1(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	ns, name := r.PathValue("namespace"), r.PathValue("name")
	if err := s.serverReader(c, ns); err != nil {
		return err
	}
	return writeServer(r.Context(), w, db, ns, name, http.StatusOK)
}

func writeServer(ctx context.Context, w http.ResponseWriter, db *DB, ns, name string, code int) error {
	obj, err := db.serverObject(ctx, ns, name)
	if err != nil {
		return err
	}
	w.Header().Set("ETag", `"`+obj.Metadata.ResourceVersion+`"`)
	writeObject(w, code, obj)
	return nil
}

func (s *server) getServerGeneration(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	ns, name := r.PathValue("namespace"), r.PathValue("name")
	if err := s.serverReader(c, ns); err != nil {
		return err
	}
	n, err := pathGeneration(r)
	if err != nil {
		return err
	}
	obj, err := db.serverObject(r.Context(), ns, name)
	if err != nil {
		return err
	}
	v, err := loadVersion(r.Context(), db.pool, ns, name, n)
	if err != nil {
		return err
	}
	obj.Spec = v.Spec
	obj.Metadata.Generation = int64(n)
	obj.Status = MCPServerStatus{ObservedGeneration: obj.Status.ObservedGeneration, ContentHash: deref(v.Hash), SubmittedBy: v.submitter(), ApprovedBy: v.approver(), ApprovedAt: v.ApprovedAt}
	writeObject(w, http.StatusOK, obj)
	return nil
}

func (s *server) createServer(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	ns := r.PathValue("namespace")
	if err := memberOf(c, ns); err != nil {
		return err
	}
	in, err := decodeResource[MCPServerSpec](w, r, "MCPServer", ns, "")
	if err != nil {
		return err
	}
	spec, sv, err := validateServerSpec(s.cluster, s.operatorTeam, ns, in.Metadata.Name, in.Spec)
	if err != nil {
		return err
	}
	if _, err := db.submitServer(r.Context(), c, in.Metadata, spec, sv, true); err != nil {
		return err
	}
	return writeServer(r.Context(), w, db, ns, in.Metadata.Name, http.StatusCreated)
}

func (s *server) putServer(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	ns, name := r.PathValue("namespace"), r.PathValue("name")
	if err := memberOf(c, ns); err != nil {
		return err
	}
	in, err := decodeResource[MCPServerSpec](w, r, "MCPServer", ns, name)
	if err != nil {
		return err
	}
	spec, sv, err := validateServerSpec(s.cluster, s.operatorTeam, ns, name, in.Spec)
	if err != nil {
		return err
	}
	auto, err := db.submitServer(r.Context(), c, in.Metadata, spec, sv, false)
	if err != nil {
		return err
	}
	if auto != nil {
		s.notifyServer(r.Context(), db, "auto", ns, name, "owner", auto...)
	}
	return writeServer(r.Context(), w, db, ns, name, http.StatusOK)
}

func (s *server) approveServer(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	ns, name := r.PathValue("namespace"), r.PathValue("name")
	if err := approverGuard(c, s.operatorTeam, ns); err != nil {
		return err
	}
	var body struct {
		Generation  int    `json:"generation"`
		ContentHash string `json:"contentHash"`
	}
	if err := decodeBody(w, r, &body); err != nil {
		return stErr(http.StatusBadRequest, ReasonBadRequest, "%v", err)
	}
	if err := db.approveServer(r.Context(), c, s.operatorTeam, ns, name, body.Generation, body.ContentHash); err != nil {
		return err
	}
	s.notifyServer(r.Context(), db, "approve", ns, name, "platform")
	return writeServer(r.Context(), w, db, ns, name, http.StatusOK)
}

// approverGuard: only the platform team approves MCPServer generations, in
// every namespace (owner decision 2026-11-26, #2634). A member of the owner
// team outside it gets 403, anyone else 404. Unset platform team: nobody.
func approverGuard(c caller, platform, ns string) error {
	switch {
	case platform != "" && c.in(platform):
		return nil
	case c.in(ns):
		return stErr(http.StatusForbidden, ReasonNotPlatformTeam, "only the platform team approves MCP server versions")
	}
	return errV1NotFound
}

// operatorOr lets members of ns, and of the platform team, stop a resource.
// Anyone else gets 404. It returns who acted: "owner" or "platform".
func (s *server) operatorOr(c caller, ns string) (string, error) {
	switch {
	case c.in(ns):
		return "owner", nil
	case s.operatorTeam != "" && c.in(s.operatorTeam):
		return "platform", nil
	}
	return "", errV1NotFound
}

func reasonText(raw string) (string, error) {
	if t := strings.TrimSpace(raw); t != "" && len(t) <= 500 {
		return t, nil
	}
	return "", stErr(http.StatusBadRequest, ReasonReasonRequired, "reason must be 1 to 500 characters")
}

func (s *server) revokeServer(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	ns, name := r.PathValue("namespace"), r.PathValue("name")
	by, err := s.operatorOr(c, ns)
	if err != nil {
		return err
	}
	var body struct {
		Reason string `json:"reason"`
		Block  bool   `json:"block"`
	}
	if err = decodeBody(w, r, &body); err != nil {
		return stErr(http.StatusBadRequest, ReasonBadRequest, "%v", err)
	}
	reason, err := reasonText(body.Reason)
	if err != nil {
		return err
	}
	if err := db.revokeServer(r.Context(), c, by, ns, name, reason, body.Block); err != nil {
		return err
	}
	s.notifyServer(r.Context(), db, "revoke", ns, name, by)
	return writeServerFor(r.Context(), w, db, c, ns, name)
}

func (s *server) rollbackServer(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	ns, name := r.PathValue("namespace"), r.PathValue("name")
	// Rollback is the owner team's: it only returns to a generation the
	// platform team already approved, so it needs no approver.
	if err := memberOf(c, ns); err != nil {
		return err
	}
	by := "owner"
	var err error
	var body struct {
		FromGeneration int    `json:"fromGeneration"`
		ToGeneration   *int   `json:"toGeneration"`
		Reason         string `json:"reason"`
	}
	if err = decodeBody(w, r, &body); err != nil {
		return stErr(http.StatusBadRequest, ReasonBadRequest, "%v", err)
	}
	reason, err := reasonText(body.Reason)
	if err != nil {
		return err
	}
	if err := db.rollbackServer(r.Context(), c, s.operatorTeam, by, ns, name, body.FromGeneration, body.ToGeneration, reason); err != nil {
		return err
	}
	s.notifyServer(r.Context(), db, "rollback", ns, name, by)
	return writeServerFor(r.Context(), w, db, c, ns, name)
}

// writeServerFor answers the platform team, which is not in the owner team,
// without the owner-only parts.
func writeServerFor(ctx context.Context, w http.ResponseWriter, db *DB, c caller, ns, name string) error {
	obj, err := db.serverObject(ctx, ns, name)
	if err != nil {
		return err
	}
	if !c.in(ns) {
		obj.Status.Pending, obj.Status.SubmittedBy, obj.Status.ApprovedBy = nil, nil, nil
		obj.Spec.Upstream = nil
	}
	writeObject(w, http.StatusOK, obj)
	return nil
}
