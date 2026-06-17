package main

import (
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
	telemetrypkg "github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

// ─── domain aliases ──────────────────────────────────────────────────────────

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

// ─── source aliases ──────────────────────────────────────────────────────────

// Type aliases
type (
	Source         = source.Source
	ArtifactKind   = source.ArtifactKind
	Resolved       = source.Resolved
	Manifest       = source.Manifest
	SourceResolver = source.SourceResolver
)

// Var aliases for kind constants and maps
var (
	KindAgent       = source.KindAgent
	KindSkill       = source.KindSkill
	KindInstruction = source.KindInstruction
	KindPrompt      = source.KindPrompt
	AllKinds        = source.AllKinds
	kindByName      = source.KindByName
)

// Const alias
const CollectionAll = source.CollectionAll

// Function aliases — closures capture the package-level `version` var at call time
var (
	resolveSource = func(ref, sourceRepo string) (*source.Source, error) {
		return source.ResolveSource(ref, sourceRepo, version)
	}
	resolveSourceForSync = func(ref, sourceRepo string) (*source.Source, error) {
		return source.ResolveSourceForSync(ref, sourceRepo, version)
	}
	findGitRoot       = source.FindGitRoot
	NewSourceResolver = source.NewSourceResolver

	// files.go
	fileHash               = source.FileHash
	dirHash                = source.DirHash
	copyFile               = source.CopyFile
	checkSymlink           = source.CheckSymlink
	copyDir                = source.CopyDir
	countDirFiles          = source.CountDirFiles
	copyArtifact           = source.CopyArtifact
	rawArtifactHash        = source.RawArtifactHash
	comparableArtifactHash = source.ComparableArtifactHash
	checkConflict          = source.CheckConflict

	// frontmatter.go
	splitFrontmatter           = source.SplitFrontmatter
	buildAgentFrontmatter      = source.BuildAgentFrontmatter
	transformPromptFrontmatter = source.TransformPromptFrontmatter
	reassemble                 = source.Reassemble
	extractFrontmatterValue    = source.ExtractFrontmatterValue

	// manifest.go
	validateName       = source.ValidateName
	validateManifest   = source.ValidateManifest
	loadManifest       = source.LoadManifest
	listCollectionDirs = source.ListCollectionDirs
	collectAllItems    = source.CollectAllItems
)

// ─── telemetry aliases ───────────────────────────────────────────────────────

// Type aliases
type (
	telemetryRecorder = telemetrypkg.Recorder
	noopTelemetry     = telemetrypkg.NoopRecorder
)

// Function aliases
var (
	initTelemetry             = telemetrypkg.InitTelemetry
	telemetryEnabled          = telemetrypkg.TelemetryEnabled
	lookupEnvValue            = telemetrypkg.LookupEnvValue
	setEnvValue               = telemetrypkg.SetEnvValue
	setEnvIfAbsent            = telemetrypkg.SetEnvIfAbsent
	applyCopilotOTelEnv       = func(env []string) ([]string, bool) { return telemetrypkg.ApplyCopilotOTelEnv(env, version) }
	applyOpenCodeOTelEnv      = func(env []string) ([]string, bool) { return telemetrypkg.ApplyOpenCodeOTelEnv(env, version) }
	copilotOTelEndpointActive = telemetrypkg.CopilotOTelEndpointConfigured
	copilotDeviceID           = telemetrypkg.CopilotDeviceID
	getOrCreateDeviceID       = telemetrypkg.GetOrCreateDeviceID
	debugLog                  = telemetrypkg.DebugLog
	getConfigDir              = telemetrypkg.GetConfigDir

	_ = telemetryEnabled
	_ = copilotDeviceID
	_ = getOrCreateDeviceID
	_ = getConfigDir
)
