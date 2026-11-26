package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/navikt/copilot/fleet-control/internal/auth"
)

// API v1alpha1 (#2748, design: copilot-intern fleet/api-v1alpha1.md v0.6).
// Kubernetes-shaped JSON over REST on Postgres. Writing a spec proposes it;
// another person in the owner team approves the exact content by generation
// and contentHash. /admin/v1 runs beside it until PR 4.

const (
	apiGroup        = "fleet.nav.no"
	apiGroupVersion = apiGroup + "/v1alpha1"
	apisPrefix      = "/apis/" + apiGroupVersion
)

// Actor is who did something. Name is the NAVident for a User. The Entra
// oid is stored but never part of an Actor.
type Actor struct {
	// Kind is empty only for rows written before 005 or by /admin/v1: the
	// code never guesses it from the name.
	Kind     string `json:"kind,omitempty"`
	Name     string `json:"name"`
	ClientID string `json:"clientID,omitempty"`
}

// ObjectMeta is the metadata every resource carries. On POST and PUT the
// server ignores uid, generation and creationTimestamp from the client.
type ObjectMeta struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	UID               string            `json:"uid,omitempty"`
	ResourceVersion   string            `json:"resourceVersion,omitempty"`
	Generation        int64             `json:"generation,omitempty"`
	CreationTimestamp *time.Time        `json:"creationTimestamp,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
}

// Condition is metav1.Condition.
type Condition struct {
	Type               string    `json:"type"`
	Status             string    `json:"status"`
	Reason             string    `json:"reason"`
	Message            string    `json:"message,omitempty"`
	ObservedGeneration int64     `json:"observedGeneration"`
	LastTransitionTime time.Time `json:"lastTransitionTime"`
}

func cond(typ string, ok bool, reason string, gen int64, at time.Time) Condition {
	st := "False"
	if ok {
		st = "True"
	}
	return Condition{Type: typ, Status: st, Reason: reason, ObservedGeneration: gen, LastTransitionTime: at.UTC()}
}

// Cause is one entry of metav1.StatusDetails.causes.
type Cause struct {
	Field   string `json:"field,omitempty"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// statusError is a refusal answered as metav1.Status.
type statusError struct {
	code   int
	reason string
	msg    string
	causes []Cause
}

func (e *statusError) Error() string { return e.msg }

func stErr(code int, reason, format string, a ...any) *statusError {
	return &statusError{code: code, reason: reason, msg: fmt.Sprintf(format, a...)}
}

func invalid(field, format string, a ...any) *statusError {
	msg := fmt.Sprintf(format, a...)
	return &statusError{code: http.StatusUnprocessableEntity, reason: ReasonInvalid, msg: msg, causes: []Cause{{Field: field, Reason: "FieldValueInvalid", Message: msg}}}
}

// because gives a 422 from invalid a more specific reason.
func (e *statusError) because(reason string) *statusError {
	e.reason = reason
	return e
}

func conflict(field, causeReason, format string, a ...any) *statusError {
	msg := fmt.Sprintf(format, a...)
	return &statusError{code: http.StatusConflict, reason: causeReason, msg: msg, causes: []Cause{{Field: field, Reason: causeReason, Message: msg}}}
}

var (
	errV1NotFound = stErr(http.StatusNotFound, ReasonNotFound, "not found")
)

// writeStatus answers err as metav1.Status. An apiError from shared write
// code keeps its code; anything else is 503 and is logged, not shown.
func writeStatus(w http.ResponseWriter, r *http.Request, kind string, err error) {
	if err == nil {
		return
	}
	se, ok := errors.AsType[*statusError](err)
	if !ok {
		if ae, isAPI := errors.AsType[*apiError](err); isAPI {
			se = &statusError{code: ae.code, reason: ae.reason, msg: ae.msg}
		} else {
			// Strip line breaks: path and err carry request-derived text (CodeQL go/log-injection).
			clean := strings.NewReplacer("\n", " ", "\r", " ")
			slog.Error("v1alpha1 request failed", "path", clean.Replace(r.URL.Path), "error", clean.Replace(err.Error()))
			se = stErr(http.StatusServiceUnavailable, ReasonServiceUnavailable, "database or team lookup unavailable")
		}
	}
	if se.code == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
	}
	body := map[string]any{"apiVersion": "v1", "kind": "Status", "status": "Failure", "code": se.code, "reason": se.reason, "message": se.msg}
	if kind != "" || len(se.causes) > 0 {
		details := map[string]any{"group": apiGroup}
		if kind != "" {
			details["kind"] = kind
		}
		if n := r.PathValue("name"); n != "" {
			details["name"] = n
		}
		if len(se.causes) > 0 {
			details["causes"] = se.causes
		}
		body["details"] = details
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(se.code)
	_ = json.NewEncoder(w).Encode(body)
}

func writeObject(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// caller is the person behind a /apis request.
type caller struct {
	Kind  string // "" is User; System only for seeding
	Ident string
	Oid   string
	Teams []string
	// ClientID is the ordering app's azp for a delegated AgentJob request,
	// "" for a person via my-copilot. Teams is empty when it is set.
	ClientID string
	// Azp is the authenticated token's azp on every route, recorded in the
	// audit row's actor_client_id.
	Azp string
}

func (c caller) in(team string) bool { return slices.Contains(c.Teams, team) }

func (c caller) actor() Actor {
	if c.Kind != "" {
		return Actor{Kind: c.Kind, Name: c.Ident}
	}
	return Actor{Kind: "User", Name: c.Ident}
}

type apisHandler func(w http.ResponseWriter, r *http.Request, c caller, db *DB) error

// apis guards a person route. Every step fails closed: a token without a
// person, an oid or the right azp is 401; Texas, the team lookup or the
// database unavailable is 503. Membership is checked by the handler, before
// any lookup.
func (s *server) apis(a *auth.Authz, adminClientID string, teams auth.TeamDirectory, kind string, h apisHandler) http.HandlerFunc {
	return s.apisFor(a, adminClientID, teams, kind, false, h)
}

// apisFor is apis; delegated also lets a person's token from another app
// through, with caller.ClientID set and no team lookup.
func (s *server) apisFor(a *auth.Authz, adminClientID string, teams auth.TeamDirectory, kind string, delegated bool, h apisHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ns, name := r.PathValue("namespace"), r.PathValue("name"); (ns != "" && !dnsLabelRegex.MatchString(ns)) || (name != "" && !dnsLabelRegex.MatchString(name)) {
			writeStatus(w, r, kind, stErr(http.StatusBadRequest, ReasonBadRequest, "namespace and name must be lowercase DNS labels"))
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		ident, email, oid, azp, err := a.Orderer(ctx, r.Header.Get("Authorization"), adminClientID, delegated)
		if errors.Is(err, auth.ErrUnauthorized) {
			writeStatus(w, r, kind, stErr(http.StatusUnauthorized, ReasonPersonTokenRequired, "a person's token with NAVident and oid is required"))
			return
		}
		if err != nil {
			writeStatus(w, r, kind, err)
			return
		}
		c := caller{Ident: ident, Oid: oid, Azp: azp}
		if azp != adminClientID {
			c.ClientID = azp
		} else if c.Teams, err = teams.Teams(ctx, email); err != nil {
			writeStatus(w, r, kind, fmt.Errorf("team lookup: %w", err))
			return
		}
		var db *DB
		if box := s.store.Load(); box != nil {
			db, _ = box.snapshotStore.(*DB)
		}
		if db == nil {
			writeStatus(w, r, kind, errDBUnavailable)
			return
		}
		writeStatus(w, r, kind, h(w, r.WithContext(ctx), c, db))
	}
}

// apisRoutes mounts v1alpha1. Like /admin/v1 it needs the admin client.
func (s *server) apisRoutes(mux *http.ServeMux, a *auth.Authz, adminClientID string, teams auth.TeamDirectory) {
	if adminClientID == "" {
		return
	}
	ns := apisPrefix + "/namespaces/{namespace}/"
	routes := []struct {
		pattern, kind string
		h             apisHandler
	}{
		{"GET " + apisPrefix + "/{$}", "", discovery},
		{"GET " + apisPrefix + "/whoami", "", whoami},
		{"GET " + ns + "apps", "", listTeamApps(teams)},
		{"GET " + apisPrefix + "/mcpservers", "mcpservers", s.listMyServers},
		{"GET " + ns + "mcpservers", "mcpservers", s.listServersNS},
		{"POST " + ns + "mcpservers", "mcpservers", s.createServer},
		{"GET " + ns + "mcpservers/{name}", "mcpservers", s.getServerV1},
		{"PUT " + ns + "mcpservers/{name}", "mcpservers", s.putServer},
		{"DELETE " + ns + "mcpservers/{name}", "mcpservers", methodNotAllowed},
		{"GET " + ns + "mcpservers/{name}/generations/{n}", "mcpservers", s.getServerGeneration},
		{"POST " + ns + "mcpservers/{name}/approval", "mcpservers", s.approveServer},
		{"POST " + ns + "mcpservers/{name}/revocation", "mcpservers", s.revokeServer},
		{"POST " + ns + "mcpservers/{name}/rollback", "mcpservers", s.rollbackServer},
		{"GET " + ns + "mcpservers/{name}/history", "mcpservers", historyOf("MCPServer", serverActions)},
		{"GET " + apisPrefix + "/mcpservercatalogentries", "mcpservercatalogentries", listCatalog},
		{"GET " + ns + "mcpservercatalogentries", "mcpservercatalogentries", listCatalog},
		{"GET " + ns + "mcpservercatalogentries/{name}", "mcpservercatalogentries", getCatalogEntry},
		{"GET " + apisPrefix + "/agentprofiles", "agentprofiles", listAwaitingProfiles},
		{"GET " + ns + "agentprofiles", "agentprofiles", listProfilesNS},
		{"POST " + ns + "agentprofiles", "agentprofiles", s.createProfile},
		{"GET " + ns + "agentprofiles/{name}", "agentprofiles", getProfile},
		{"PUT " + ns + "agentprofiles/{name}", "agentprofiles", s.putProfile},
		{"DELETE " + ns + "agentprofiles/{name}", "agentprofiles", methodNotAllowed},
		{"GET " + ns + "agentprofiles/{name}/generations/{n}", "agentprofiles", getProfileGeneration},
		{"POST " + ns + "agentprofiles/{name}/approval", "agentprofiles", approveProfile},
		{"POST " + ns + "agentprofiles/{name}/revocation", "agentprofiles", s.revokeProfile},
		{"GET " + ns + "agentprofiles/{name}/history", "agentprofiles", historyOf("AgentProfile", agentActionsV1)},
	}
	for _, rt := range routes {
		mux.HandleFunc(rt.pattern, s.apis(a, adminClientID, teams, rt.kind, rt.h))
	}
	for pattern, h := range map[string]apisHandler{
		"GET " + ns + "agentjobs":                     listJobsV1,
		"POST " + ns + "agentjobs":                    createJobV1,
		"GET " + ns + "agentjobs/{name}":              getJobV1,
		"POST " + ns + "agentjobs/{name}/termination": terminateJobV1,
	} {
		mux.HandleFunc(pattern, s.apisFor(a, adminClientID, teams, "agentjobs", true, h))
	}
}

func methodNotAllowed(_ http.ResponseWriter, _ *http.Request, _ caller, _ *DB) error {
	return stErr(http.StatusMethodNotAllowed, ReasonMethodNotAllowed, "resources are not deleted in v1alpha1; revoke instead")
}

func discovery(w http.ResponseWriter, _ *http.Request, _ caller, _ *DB) error {
	res := func(name, kind string, verbs ...string) map[string]any {
		return map[string]any{"name": name, "namespaced": true, "kind": kind, "verbs": verbs}
	}
	writeObject(w, http.StatusOK, map[string]any{
		"kind": "APIResourceList", "apiVersion": "v1", "groupVersion": apiGroupVersion,
		"resources": []any{
			res("mcpservers", "MCPServer", "get", "list", "create", "update"),
			res("mcpservercatalogentries", "MCPServerCatalogEntry", "get", "list"),
			res("agentprofiles", "AgentProfile", "get", "list", "create", "update"),
			res("agentjobs", "AgentJob", "get", "list", "create"),
		},
	})
	return nil
}

// whoami is the only answer that holds an oid: the caller's own.
func whoami(w http.ResponseWriter, _ *http.Request, c caller, _ *DB) error {
	writeObject(w, http.StatusOK, map[string]any{"name": c.Ident, "oid": c.Oid, "teams": nonNil(c.Teams)})
	return nil
}

// memberOf refuses a non-member with 404, so nobody can probe names.
func memberOf(c caller, ns string) error {
	if !c.in(ns) {
		return errV1NotFound
	}
	return nil
}

func pathGeneration(r *http.Request) (int, error) {
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil || n < 1 {
		return 0, errV1NotFound
	}
	return n, nil
}

// resourceIn is the request shape shared by MCPServer and AgentProfile.
// status is read and dropped, so "get, change, send back" works.
type resourceIn[S any] struct {
	APIVersion string          `json:"apiVersion"`
	Kind       string          `json:"kind"`
	Metadata   ObjectMeta      `json:"metadata"`
	Spec       S               `json:"spec"`
	Status     json.RawMessage `json:"status,omitempty"`
}

// decodeResource reads a resource for ns/name (name "" on POST: taken from
// the body). A namespace or name in the body unlike the path is 400, never
// silently overridden.
func decodeResource[S any](w http.ResponseWriter, r *http.Request, kind, ns, name string) (resourceIn[S], error) {
	var in resourceIn[S]
	if err := decodeBody(w, r, &in); err != nil {
		return in, stErr(http.StatusBadRequest, ReasonBadRequest, "%v", err)
	}
	switch {
	case in.APIVersion != apiGroupVersion || in.Kind != kind:
		return in, stErr(http.StatusBadRequest, ReasonKindMismatch, "apiVersion must be %s and kind %s", apiGroupVersion, kind)
	case in.Metadata.Namespace != ns:
		return in, stErr(http.StatusBadRequest, ReasonNamespaceMismatch, "metadata.namespace must be %s, as in the path", ns)
	case name != "" && in.Metadata.Name != name:
		return in, stErr(http.StatusBadRequest, ReasonNameMismatch, "metadata.name must be %s, as in the path", name)
	case !dnsLabelRegex.MatchString(in.Metadata.Name):
		return in, invalid("metadata.name", "must be a lowercase DNS label")
	}
	return in, validateMeta(in.Metadata)
}

var (
	labelKeyRe   = regexp.MustCompile(`^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*/)?[A-Za-z0-9]([-A-Za-z0-9_.]{0,61}[A-Za-z0-9])?$`)
	labelValueRe = regexp.MustCompile(`^([A-Za-z0-9]([-A-Za-z0-9_.]{0,61}[A-Za-z0-9])?)?$`)
)

// validateMeta applies the Kubernetes rules to labels and annotations. The
// prefix fleet.nav.no/ is reserved for the server.
func validateMeta(m ObjectMeta) error {
	if len(m.Labels) > 32 {
		return invalid("metadata.labels", "at most 32 labels")
	}
	for k, v := range m.Labels {
		if !validMetaKey(k) || !labelValueRe.MatchString(v) {
			return invalid("metadata.labels", "label %q is not a valid Kubernetes label", k)
		}
	}
	size := 0
	for k, v := range m.Annotations {
		if !validMetaKey(k) {
			return invalid("metadata.annotations", "annotation key %q is not valid", k)
		}
		size += len(k) + len(v)
	}
	if size > 16<<10 {
		return invalid("metadata.annotations", "annotations exceed 16 KiB")
	}
	return nil
}

func validMetaKey(k string) bool {
	prefix, _, hasPrefix := strings.Cut(k, "/")
	return labelKeyRe.MatchString(k) && !strings.HasPrefix(k, apiGroup+"/") && (!hasPrefix || len(prefix) <= 253)
}

// requireResourceVersion is the optimistic lock on PUT: missing or stale is 409.
func requireResourceVersion(sent string, current int64) error {
	if sent == "" || sent != strconv.FormatInt(current, 10) {
		return conflict("metadata.resourceVersion", ReasonStaleResourceVersion, "the object has been modified; read it again and apply your changes to the latest version")
	}
	return nil
}

func emptyIfNil[T any](v []T) []T {
	if v == nil {
		return []T{}
	}
	return v
}

func jsonMap(raw string) map[string]string {
	m := map[string]string{}
	_ = json.Unmarshal([]byte(raw), &m)
	if len(m) == 0 {
		return nil
	}
	return m
}

func jsonText(v any) string {
	if v == nil {
		return "{}"
	}
	raw, _ := json.Marshal(v)
	if string(raw) == "null" {
		return "{}"
	}
	return string(raw)
}

// AuditEvent is one history entry. The actor's oid is never shown.
type AuditEvent struct {
	At         time.Time       `json:"at"`
	Actor      Actor           `json:"actor"`
	Action     string          `json:"action"`
	Generation *int            `json:"generation,omitempty"`
	Detail     json.RawMessage `json:"detail,omitempty"`
}

var agentActionsV1 = []string{"agent_profile_submitted", "agent_profile_access_granted", "agent_profile_published", "agent_profile_revoked", "agent_profile_access_revoked", "metadata_updated", "approval_expired"}

// historyOf serves the audit trail to members of the owner team only.
func historyOf(kind string, legacyActions []string) apisHandler {
	return func(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
		ns, name := r.PathValue("namespace"), r.PathValue("name")
		if err := memberOf(c, ns); err != nil {
			return err
		}
		limit := 500
		if l := r.URL.Query().Get("limit"); l != "" {
			n, err := strconv.Atoi(l)
			if err != nil || n < 1 || n > 500 {
				return stErr(http.StatusBadRequest, ReasonBadRequest, "limit must be 1 to 500")
			}
			limit = n
		}
		items, found, err := db.history(r.Context(), kind, ns, name, legacyActions, limit)
		if err != nil {
			return err
		}
		if !found {
			return errV1NotFound
		}
		writeObject(w, http.StatusOK, map[string]any{"apiVersion": apiGroupVersion, "kind": "AuditEventList", "items": items})
		return nil
	}
}

// listTeamApps answers the team's Nais applications to its members. A
// non-member gets 404 like every other team route; no Console client is 503.
func listTeamApps(teams auth.TeamDirectory) apisHandler {
	return func(w http.ResponseWriter, r *http.Request, c caller, _ *DB) error {
		ns := r.PathValue("namespace")
		if err := memberOf(c, ns); err != nil {
			return err
		}
		l, ok := teams.(auth.AppLister)
		if !ok {
			return stErr(http.StatusServiceUnavailable, ReasonConsoleUnavailable, "the Nais application list is not available here")
		}
		apps, err := l.Apps(r.Context(), ns)
		if err != nil {
			return fmt.Errorf("list apps: %w", err)
		}
		writeObject(w, http.StatusOK, map[string]any{"apiVersion": apiGroupVersion, "kind": "TeamAppList", "items": apps})
		return nil
	}
}
