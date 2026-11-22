// Package identity describes who is calling fleet-gateway, independent of the
// issuer that vouched for it (#2582). Today the only issuer is the gateway's
// own session; Nais STS or Entra agent identities can implement Resolver
// later without touching the audit or policy code.
package identity

import "errors"

// IssuerSession is the gateway's own OAuth session store.
const IssuerSession = "fleet-gateway"

// AssuranceSession is a sealed session issued after an Entra login and consent.
const AssuranceSession = "session"

// ActorTypeClient means the actor is the OAuth client the user logged in through.
const ActorTypeClient = "oauth_client"

// Caller is a validated caller. Subject is the user the call is made for;
// Actor is the agent or client acting for them. Phase 1 is delegated only, so
// Subject is always a user.
type Caller struct {
	Issuer    string
	Subject   string // NAVident
	Actor     string // agent or client id
	ActorType string
	// Chain lists the actors from the one calling the gateway inward; a
	// session has one hop, a token-exchange chain would have more.
	Chain     []string
	Assurance string
}

// Resolver turns an already validated credential V into a Caller. A resolver
// never reads request headers: an actor a client claims for itself is not an
// identity.
type Resolver[V any] interface {
	Resolve(V) (Caller, error)
}

// Session is the part of a validated gateway session that names the caller.
type Session struct {
	NAVIdent, ClientID string
}

// SessionResolver is the Resolver for the gateway's own sessions.
type SessionResolver struct{}

// Resolve errors.
var (
	ErrNoSubject = errors.New("identity: session has no subject")
	ErrNoActor   = errors.New("identity: session has no client id")
)

// Resolve fails closed: a session without a user or client names nobody.
func (SessionResolver) Resolve(s Session) (Caller, error) {
	switch {
	case s.NAVIdent == "":
		return Caller{}, ErrNoSubject
	case s.ClientID == "":
		return Caller{}, ErrNoActor
	}
	return Caller{
		Issuer:    IssuerSession,
		Subject:   s.NAVIdent,
		Actor:     s.ClientID,
		ActorType: ActorTypeClient,
		Chain:     []string{s.ClientID},
		Assurance: AssuranceSession,
	}, nil
}
