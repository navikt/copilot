package main

import "github.com/navikt/copilot/fleet-gateway/internal/session"

// The store lives in internal/session. These aliases keep the old names
// working in package main until the remaining callers move (#2499).

const (
	kindPendingConsent     = session.KindPendingConsent
	kindPendingEntra       = session.KindPendingEntra
	kindPendingEntraLegacy = session.KindPendingEntraLegacy
	kindCode               = session.KindCode
	kindSession            = session.KindSession
	kindConsent            = session.KindConsent
	kindMCPSession         = session.KindMCPSession
	kindMCPSessionUser     = session.KindMCPSessionUser
	kindDCRClient          = session.KindDCRClient
	kindRefreshGrace       = session.KindRefreshGrace
	kindUserRevoked        = session.KindUserRevoked
	kindUserRevokedMarker  = session.KindUserRevokedMarker
	kindAuditOutbox        = session.KindAuditOutbox

	sealKeyBytes     = session.SealKeyBytes
	sealMaxPlaintext = session.SealMaxPlaintext
)

type (
	SessionStore = session.Store
	keyRing      = session.KeyRing
	sealedStore  = session.SealedStore
)

var (
	errStoreFull      = session.ErrStoreFull
	errSealUnknownKID = session.ErrSealUnknownKID

	newMemoryStore       = session.NewMemoryStore
	newSealedStore       = session.NewSealedStore
	newPostgresStore     = session.NewPostgresStore
	parseKeyRing         = session.ParseKeyRing
	sealAAD              = session.SealAAD
	pgEnvDSN             = session.PGEnvDSN
	checkPGRuntimeTables = session.CheckPGRuntimeTables
)
