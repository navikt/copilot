package main

// The MCP 2026-07-28 wire rules live in lib/mcpkit. These names are kept so
// mcp.go and the tests read as before.

import "github.com/navikt/copilot/lib/mcpkit"

const (
	mcpModernVersion           = mcpkit.ModernVersion
	mcpCodeHeaderMismatch      = mcpkit.CodeHeaderMismatch
	mcpCodeUnsupportedVersion  = mcpkit.CodeUnsupportedProtocolVersion
	mcpCodeMethodNotFound      = mcpkit.CodeMethodNotFound
	mcpCodeInvalidParams       = mcpkit.CodeInvalidParams
	mcpCodeParseError          = mcpkit.CodeParseError
	mcpCodeInvalidRequest      = mcpkit.CodeInvalidRequest
	mcpMetaProtocolVersion     = mcpkit.MetaProtocolVersion
	mcpMetaClientCapabilities  = mcpkit.MetaClientCapabilities
	mcpMetaServerInfo          = mcpkit.MetaServerInfo
	mcpLegacyUntilEnv          = mcpkit.LegacyUntilEnv
	mcpDiscoverTTLMilliseconds = mcpkit.DiscoverTTLMilliseconds
)

// Initialize-era revisions; mcpkit keeps its list unexported. Must match it.
var mcpLegacyVersions = []string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"}

type (
	mcpRPCError = mcpkit.WireError
	mcpEnvelope = mcpkit.Envelope
)

var (
	mcpNegotiateLegacyVersion = mcpkit.NegotiateLegacyVersion
	mcpLegacyAllowed          = mcpkit.LegacyAllowed
	isLegacyMCPRequest        = mcpkit.IsLegacyRequest
	mcpUnsupportedVersion     = mcpkit.UnsupportedVersion
	validateModernMCPRequest  = mcpkit.ValidateModernRequest
	mcpModernResult           = mcpkit.ModernResult
	mcpDiscoverResult         = mcpkit.DiscoverResult
	writeMCPError             = mcpkit.WriteError
)
