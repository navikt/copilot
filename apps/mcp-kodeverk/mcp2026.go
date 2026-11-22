package main

// The MCP 2026-07-28 wire rules live in lib/mcpkit. These names are kept so
// tools.go and the tests read as before.

import "github.com/navikt/copilot/lib/mcpkit"

const (
	mcpModernVersion          = mcpkit.ModernVersion
	mcpCodeHeaderMismatch     = mcpkit.CodeHeaderMismatch
	mcpCodeUnsupportedVersion = mcpkit.CodeUnsupportedVersion
	mcpCodeMethodNotFound     = mcpkit.CodeMethodNotFound
	mcpCodeInvalidParams      = mcpkit.CodeInvalidParams
	mcpMetaProtocolVersion    = mcpkit.MetaProtocolVersion
	mcpMetaClientCapabilities = mcpkit.MetaClientCapabilities
	mcpMetaServerInfo         = mcpkit.MetaServerInfo
	mcpLegacyUntilEnv         = mcpkit.LegacyUntilEnv
)

type mcpRPCError = mcpkit.RPCError
