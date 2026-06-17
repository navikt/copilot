package main

import "github.com/navikt/copilot/cli/nav-pilot/internal/domain"

// Type aliases (zero-cost compile-time redirections)
type (
	Config         = domain.Config
	ResolvedConfig = domain.ResolvedConfig
	CLIOverrides   = domain.CLIOverrides
	modelChoice    = struct {
		ID    string
		Label string
	}
	InstallScope  = domain.InstallScope
	StateFile     = domain.StateFile
	InstalledFile = domain.InstalledFile
)

// Constant aliases
const (
	fileStatusIgnored  = domain.FileStatusIgnored
	fileStatusConflict = domain.FileStatusConflict
)

// Function and slice aliases — var means they can be called/indexed identically
var (
	bold   = domain.Bold
	dim    = domain.Dim
	green  = domain.Green
	red    = domain.Red
	yellow = domain.Yellow

	validateModelValue    = domain.ValidateModelValue
	validateOptionalModel = domain.ValidateOptionalModel
	containsStr           = domain.ContainsStr

	validModes           = domain.ValidModes
	validReasoningEffort = domain.ValidReasoningEffort
	validContextTiers    = domain.ValidContextTiers
	validLogLevels       = domain.ValidLogLevels
	validOtelLogLevels   = domain.ValidOtelLogLevels

	ScopeRepo = domain.ScopeRepo
	ScopeUser = domain.ScopeUser
)
