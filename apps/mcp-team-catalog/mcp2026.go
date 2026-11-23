package main

// The MCP 2026-07-28 wire rules live in lib/mcpkit. These names are kept so
// main.go, tools.go and the tests read as before.

import "github.com/navikt/copilot/lib/mcpkit"

const (
	mcpModernVersion           = mcpkit.ModernVersion
	mcpCodeHeaderMismatch      = mcpkit.CodeHeaderMismatch
	mcpCodeUnsupportedVersion  = mcpkit.CodeUnsupportedProtocolVersion
	mcpCodeMethodNotFound      = mcpkit.CodeMethodNotFound
	mcpCodeInvalidParams       = mcpkit.CodeInvalidParams
	mcpCodeParseError          = mcpkit.CodeParseError
	mcpMetaProtocolVersion     = mcpkit.MetaProtocolVersion
	mcpMetaClientCapabilities  = mcpkit.MetaClientCapabilities
	mcpMetaServerInfo          = mcpkit.MetaServerInfo
	mcpLegacyUntilEnv          = mcpkit.LegacyUntilEnv
	mcpDiscoverTTLMilliseconds = mcpkit.DiscoverTTLMilliseconds
)

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
