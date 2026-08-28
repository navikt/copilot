package cli

import (
	"context"
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

	copilotEnv     = providerpkg.CopilotEnv
	findCopilotCLI = providerpkg.FindCopilotCLI
	launchPi       = providerpkg.LaunchPi

	// parseCpltVersion moved to internal/provider when the staged launch path
	// needed the same parse for its reviewed-cplt floor; internal/cli imports
	// provider, not the reverse, so this is the only direction it could go.
	parseCpltVersion = providerpkg.ParseCpltVersion

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

// Const aliases
const (
	CollectionAll = source.CollectionAll

	// defaultSourceRepo is the content source used when neither --source nor
	// the config file's source key names one.
	defaultSourceRepo = source.DefaultRepo
)

// Function aliases — closures capture the package-level `Version` var at call time.
//
// resolveSource and resolveSourceForSync are the CLI's source funnel: they
// apply the source precedence (--source > config `source` > navikt/copilot) and
// attach the agentpakke manifest, so every command that reads content gets the
// selected agentpakke and its fail-closed validation without repeating either.
var (
	resolveSource = func(ref, sourceRepo string) (*source.Source, error) {
		effective, err := sourceRepoFor(sourceRepo)
		if err != nil {
			return nil, err
		}
		src, err := source.ResolveSource(ref, effective, Version)
		if err != nil {
			return nil, err
		}
		return src, attachPakkeOrCleanup(src)
	}
	resolveSourceForSync = func(ref, sourceRepo string) (*source.Source, error) {
		effective, err := sourceRepoFor(sourceRepo)
		if err != nil {
			return nil, err
		}
		src, err := source.ResolveSourceForSync(ref, effective, Version)
		if err != nil {
			return nil, err
		}
		return src, attachPakkeOrCleanup(src)
	}
	// resolveSourceRaw skips the manifest attach, so `nav-pilot validate` can
	// report a non-conforming manifest as findings instead of failing to
	// resolve the source at all.
	resolveSourceRaw = func(ref, sourceRepo string) (*source.Source, error) {
		effective, err := sourceRepoFor(sourceRepo)
		if err != nil {
			return nil, err
		}
		return source.ResolveSource(ref, effective, Version)
	}
	findGitRoot                = source.FindGitRoot
	NewSourceResolver          = source.NewSourceResolver
	NewSourceResolverForLayout = source.NewSourceResolverForLayout
	validateSourceValue        = source.ValidateSourceValue

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
	validateName        = source.ValidateName
	validateManifest    = source.ValidateManifest
	loadManifest        = source.LoadManifest
	listCollectionDirs  = source.ListCollectionDirs
	collectAllItems     = source.CollectAllItems
	collectAllItemsWith = source.CollectAllItemsWith
)

// attachPakkeOrCleanup attaches the agentpakke manifest and removes a cloned
// checkout when the manifest is unusable, so a fail-closed resolve does not
// leak a temp dir the caller never got to defer Cleanup on.
func attachPakkeOrCleanup(src *source.Source) error {
	if err := attachPakke(src); err != nil {
		src.Cleanup()
		return err
	}
	return nil
}

// ─── artifacts aliases ───────────────────────────────────────────────────────

type (
	SyncConfig = artifacts.SyncConfig
)

var (
	versionNewer     = artifacts.VersionNewer
	versionParseable = artifacts.VersionParseable
	versionTimestamp = artifacts.VersionTimestamp
)

const (
	stateFilePath      = artifacts.StateFilePath
	syncConfigPath     = artifacts.SyncConfigPath
	openCodeCollection = artifacts.OpenCodeCollection
	openCodeScopeName  = artifacts.OpenCodeScopeName
)

var (
	readState       = artifacts.ReadState
	readScopedState = artifacts.ReadScopedState
	writeState      = artifacts.WriteState
	// Every scoped state write goes through [releasePin] first. Replacing a
	// Tier 2 pin with installed content is the moment the revision trees
	// behind it become unreachable, and it happens on four separate install
	// paths — a shared write is one place to get it right instead of four
	// places to forget it.
	writeScopedState = func(scope *InstallScope, state *StateFile) error {
		releasePin(scope, state)
		return artifacts.WriteScopedState(scope, state)
	}
)

var (
	assessStaleness = func(installedVersion string) artifacts.StalenessAssessment {
		fetchFn := func() (string, string, error) {
			client := &http.Client{
				Timeout: 5 * time.Second,
				Transport: &http.Transport{
					Proxy: http.ProxyFromEnvironment,
				},
			}
			origClient := httpClient
			httpClient = client
			defer func() { httpClient = origClient }()
			return fetchLatestVersion(context.Background())
		}
		return artifacts.AssessStaleness(installedVersion, fetchFn)
	}
)

var (
	readSyncConfig = artifacts.ReadSyncConfig
	overrideSet    = artifacts.OverrideSet
)

// cmdExport funnels the source precedence into the artifacts package, which
// resolves its own source. The agentpakke manifest is not threaded here: export
// materializes another tool's format from canonical content and is migrated
// with the rest of the provider layer in M2. Until then, artifacts refuses a
// source whose manifest declares a non-canonical layout rather than exporting
// an empty tree from paths that source does not use.
var cmdExport = func(format string, scope *InstallScope, ref, sourceRepo string, dryRun, force bool, jsonOutput bool) error {
	effective, err := sourceRepoFor(sourceRepo)
	if err != nil {
		return err
	}
	return artifacts.CmdExport(format, scope, ref, effective, Version, dryRun, force, jsonOutput)
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
