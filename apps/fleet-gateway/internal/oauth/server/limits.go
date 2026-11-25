package server

import (
	"crypto/rand"
	"encoding/base64"
	"time"
)

// Values main's config also uses; main aliases them.
const (
	IdentitySourceIntrospection = "introspection"
	RTStorageClientEncrypted    = "client-encrypted"
	StrategyRefresh             = "refresh"

	DefaultSessionAbsoluteMaxAge = 7 * 24 * time.Hour
	DefaultCIMDTTL               = 5 * time.Minute
	CIMDStaleMax                 = 24 * time.Hour
	MaxClaimsBytes               = 4096
	MaxLogErrorLen               = 64
)

const (
	defaultSessionTTL         = time.Hour
	defaultDCRRateLimitWindow = time.Hour
	maxDCRBodyBytes           = 16 * 1024
	maxFormBodyBytes          = 20 * 1024
	maxCIMDBodyBytes          = 64 * 1024
	maxUpstreamBodyBytes      = 1 << 20

	// maxLogClientIDLen bounds a client-supplied client_id (CIMD URL or DCR id) in audit lines.
	maxLogClientIDLen = 256
)

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// TruncateLogValue sanitizes s and cuts it to maxChars runes.
func TruncateLogValue(s string, maxChars int) string {
	s = sanitizeLogValue(s)
	if maxChars <= 0 {
		return ""
	}
	count := 0
	for i := range s {
		if count == maxChars {
			return s[:i]
		}
		count++
	}
	return s
}
