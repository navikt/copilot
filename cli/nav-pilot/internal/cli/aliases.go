package cli

import (
	"net/http"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
	telemetrypkg "github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

// ─── domain aliases ──────────────────────────────────────────────────────────

// Type aliases (zero-cost compile-time redirections)
type (
	Config         = domain.Config
	ResolvedConfig = domain.ResolvedConfig
	CLIOverrides   = domain.CLIOverrides
	InstallScope   = domain.InstallScope
	StateFile      = domain.StateFile
	InstalledFile  = domain.InstalledFile
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

// ─── provider aliases ────────────────────────────────────────────────────────

type (
	Provider              = providerpkg.Provider
	ProviderSyncResult    = providerpkg.ProviderSyncResult
	ProviderContextStatus = providerpkg.ProviderContextStatus
)

var (
	providerFor      = providerpkg.ProviderFor
	allProviders     = providerpkg.AllProviders
	validProviderIDs = providerpkg.ValidProviderIDs

	recordFreshness = providerpkg.RecordFreshness

	copilotEnv = providerpkg.CopilotEnv
	launchPi   = providerpkg.LaunchPi

	openCodeDefaultModel  = providerpkg.OpenCodeDefaultModel
	isKnownCopilotModel   = providerpkg.IsKnownCopilotModel
	knownCopilotModelIDs  = providerpkg.KnownCopilotModelIDs
	isKnownOpenCodeModel  = providerpkg.IsKnownOpenCodeModel
	knownOpenCodeModelIDs = providerpkg.KnownOpenCodeModelIDs
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

// Function aliases — closures capture the package-level `Version` var at call time
var (
	resolveSource = func(ref, sourceRepo string) (*source.Source, error) {
		return source.ResolveSource(ref, sourceRepo, Version)
	}
	resolveSourceForSync = func(ref, sourceRepo string) (*source.Source, error) {
		return source.ResolveSourceForSync(ref, sourceRepo, Version)
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

	// manifest.go
	validateName       = source.ValidateName
	validateManifest   = source.ValidateManifest
	loadManifest       = source.LoadManifest
	listCollectionDirs = source.ListCollectionDirs
	collectAllItems    = source.CollectAllItems
)

// ─── artifacts aliases ───────────────────────────────────────────────────────

type (
	SyncConfig = artifacts.SyncConfig
)

var (
	versionNewer     = artifacts.VersionNewer
	versionTimestamp = artifacts.VersionTimestamp
)

const (
	stateFilePath      = artifacts.StateFilePath
	syncConfigPath     = artifacts.SyncConfigPath
	openCodeCollection = artifacts.OpenCodeCollection
	openCodeScopeName  = artifacts.OpenCodeScopeName
)

var (
	readState        = artifacts.ReadState
	readScopedState  = artifacts.ReadScopedState
	writeState       = artifacts.WriteState
	writeScopedState = artifacts.WriteScopedState
)

var (
	assessStaleness = func(installedVersion string) artifacts.StalenessAssessment {
		fetchFn := func() (string, string, error) {
			client := &http.Client{Timeout: 2 * time.Second}
			origClient := httpClient
			httpClient = client
			defer func() { httpClient = origClient }()
			return fetchLatestVersion()
		}
		return artifacts.AssessStaleness(installedVersion, fetchFn)
	}
)

var (
	readSyncConfig = artifacts.ReadSyncConfig
	overrideSet    = artifacts.OverrideSet
)

var cmdExport = func(format string, scope *InstallScope, ref, sourceRepo string, dryRun, force bool, jsonOutput bool) error {
	return artifacts.CmdExport(format, scope, ref, sourceRepo, Version, dryRun, force, jsonOutput)
}

var writeOpenCodeState = artifacts.WriteOpenCodeState

// ─── telemetry aliases ───────────────────────────────────────────────────────

// Type aliases
type (
	telemetryRecorder = telemetrypkg.Recorder
	noopTelemetry     = telemetrypkg.NoopRecorder
)

// Function aliases
var (
	initTelemetry       = telemetrypkg.InitTelemetry
	telemetryEnabled    = telemetrypkg.TelemetryEnabled
	lookupEnvValue      = telemetrypkg.LookupEnvValue
	copilotDeviceID     = telemetrypkg.CopilotDeviceID
	getOrCreateDeviceID = telemetrypkg.GetOrCreateDeviceID
	debugLog            = telemetrypkg.DebugLog
	getConfigDir        = telemetrypkg.GetConfigDir

	_ = telemetryEnabled
	_ = copilotDeviceID
	_ = getOrCreateDeviceID
	_ = getConfigDir
)
