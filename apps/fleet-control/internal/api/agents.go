package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/navikt/copilot/fleet-control/internal/auth"
)

// Agent profiles (#2582): a caller app may use listed tools on listed routes
// on behalf of a signed-in user. This slice stores and serves them; the
// gateway does not enforce them yet.

// AgentProfile is the API and snapshot shape.
type AgentProfile struct {
	Namespace string        `json:"namespace"` // owner team
	Name      string        `json:"name"`
	Identity  AgentIdentity `json:"identity"`
	Mode      string        `json:"mode"`
	Grants    []AgentGrant  `json:"grants"`
	Quota     AgentQuota    `json:"quota"`
	DPIA      AgentDPIA     `json:"dpia"`
	Status    string        `json:"status,omitempty"`
	ExpiresAt *time.Time    `json:"expires_at,omitempty"`
	CreatedBy string        `json:"-"`
	// NeedsResubmit marks a seeded draft: no one can approve it until a person
	// revokes and re-submits it. Set on read only (TeamAgents).
	NeedsResubmit bool `json:"needsResubmit,omitempty"`
}

// AgentIdentity is the caller app. StsSubject is reserved for the Nais STS
// workload identity and must stay empty until that issuer is chosen.
type AgentIdentity struct {
	Cluster       string `json:"cluster"`
	Namespace     string `json:"namespace"`
	App           string `json:"app"`
	EntraClientID string `json:"entra_client_id"`
	StsSubject    string `json:"sts_subject,omitempty"`
}

// AgentGrant lists the tools allowed on one route, "<namespace>/<name>".
type AgentGrant struct {
	Route   string   `json:"route"`
	Tools   []string `json:"tools"`
	MaxRisk string   `json:"max_risk"`
}

// AgentQuota is a profile's call and concurrency limits.
type AgentQuota struct {
	MaxInFlight       int `json:"max_in_flight"`
	RPM               int `json:"rpm"`
	DailyCallsPerUser int `json:"daily_calls_per_user"`
	DailyCalls        int `json:"daily_calls"`
}

// AgentDPIA records a profile's DPIA owner and reference.
type AgentDPIA struct {
	Owner     string `json:"owner"`
	Reference string `json:"reference"`
}

// Person is a signed-in human and the Nais teams they belong to.
type Person struct {
	Ident string
	Teams []string
}

// identityVerifier checks that an Entra client id belongs to the declared
// cluster/namespace/app, for example from the Nais AzureApp resource. Without
// one every profile stays unverified and out of the gateway snapshot.
type identityVerifier interface {
	Verify(ctx context.Context, id AgentIdentity) (bool, error)
}

var (
	errBadRequest = errors.New("bad request")
)

// approvalValidity is fixed by styringsmodell v0.3: approvals expire after six months.
const approvalValidity = "6 months"

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func badf(format string, a ...any) error {
	return fmt.Errorf("%w: %s", errBadRequest, fmt.Sprintf(format, a...))
}

// validate checks the profile on its own; checkRoutes checks it against the
// published routes.
func (p AgentProfile) validate() error {
	for field, v := range map[string]string{"namespace": p.Namespace, "name": p.Name, "identity.cluster": p.Identity.Cluster,
		"identity.namespace": p.Identity.Namespace, "identity.app": p.Identity.App, "dpia.owner": p.DPIA.Owner} {
		if !dnsLabelRegex.MatchString(v) {
			return badf("%s must be a DNS label", field)
		}
	}
	if p.Identity.Namespace != p.Namespace {
		return badf("identity.namespace must be the owner team's namespace")
	}
	if !uuidRe.MatchString(p.Identity.EntraClientID) {
		return badf("identity.entra_client_id must be a lowercase UUID")
	}
	if p.Identity.StsSubject != "" {
		return badf("identity.sts_subject is not supported yet")
	}
	if p.Mode != "delegated" {
		return badf("mode must be delegated; agent principals are not allowed in phase 1")
	}
	if len(p.Grants) == 0 || len(p.Grants) > 20 {
		return badf("grants must list 1 to 20 routes")
	}
	seen := map[string]bool{}
	for _, g := range p.Grants {
		ns, name, ok := strings.Cut(g.Route, "/")
		if !ok || !dnsLabelRegex.MatchString(ns) || !dnsLabelRegex.MatchString(name) {
			return badf("grant route must be <namespace>/<name>")
		}
		if seen[g.Route] {
			return badf("route %s granted twice", g.Route)
		}
		seen[g.Route] = true
		if len(g.Tools) == 0 || len(g.Tools) > 50 {
			return badf("grant %s must list 1 to 50 tools", g.Route)
		}
		for i, t := range g.Tools {
			if !toolNameRegex.MatchString(t) { // same names as validateTools; no "*" can pass
				return badf("grant %s: tool %q is not a tool name", g.Route, t)
			}
			if slices.Contains(g.Tools[:i], t) {
				return badf("grant %s: tool %s listed twice", g.Route, t)
			}
		}
		if g.MaxRisk != "read" {
			return badf("grant %s: max_risk must be read", g.Route)
		}
	}
	q := p.Quota
	if q.MaxInFlight < 1 || q.MaxInFlight > 64 || q.RPM < 1 || q.RPM > 6000 ||
		q.DailyCallsPerUser < 1 || q.DailyCallsPerUser > 100000 || q.DailyCalls < 1 || q.DailyCalls > 1000000 {
		return badf("quota out of range")
	}
	if n := len(p.DPIA.Reference); n == 0 || n > 300 || strings.ContainsFunc(p.DPIA.Reference, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return badf("dpia.reference must be 1 to 300 printable characters")
	}
	return nil
}

// seedAgentsFromEnv imports SEED_AGENTS_FILE, a JSON list of profiles, as
// drafts by "seed". Set only in dev; refused outside dev-* clusters. A profile is skipped when any profile
// with its name exists. Routes are checked at approval, not here, so a draft
// may name a route the cluster does not have yet.
func seedAgentsFromEnv(ctx context.Context, db *DB) error {
	path := os.Getenv("SEED_AGENTS_FILE")
	if path == "" {
		return nil
	}
	if !auth.DevCluster(os.Getenv("NAIS_CLUSTER_NAME")) {
		return errors.New("SEED_AGENTS_FILE is for dev-* clusters only")
	}
	raw, err := os.ReadFile(path) //nolint:gosec // operator-set path baked into the image
	if err != nil {
		return err
	}
	var profiles []AgentProfile
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&profiles); err != nil {
		return fmt.Errorf("seed agents: %w", err)
	}
	for _, p := range profiles {
		if err := p.validate(); err != nil {
			return fmt.Errorf("seed agent %s/%s: %w", p.Namespace, p.Name, err)
		}
		// The 003 table is frozen since #2748; seeds go to the v1alpha1
		// tables, skipped when the name exists there.
		if err := db.seedProfileV1(ctx, p); err != nil {
			return fmt.Errorf("seed agent %s/%s: %w", p.Namespace, p.Name, err)
		}
	}
	return nil
}

// Audit actions the pre-v1alpha1 code wrote; history still shows them.
var (
	serverActions = []string{"draft_submitted", "approved_published", "revoked", "rolled_back"}
)

func nonNil(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}
