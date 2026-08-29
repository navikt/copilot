// Package local holds the local-inference model manifest: the versioned,
// served JSON that names the models a developer's machine can run locally, and
// the rules nav-pilot enforces on it before it trusts a single entry.
//
// The manifest is generated elsewhere (navikt/mlx-workspace, `mise run
// model-manifest`) and served as JSON, so this package never authors model
// entries — it validates them, caches the last good copy, and answers the one
// question the rest of the binary asks: is this model id a local one, and what
// does it need. Nothing outside this package should string-match on a model id
// to decide that; [IsLocal] and [Lookup] are the predicate.
//
// # Ignore-unknown
//
// Unknown fields are ignored, never fatal (encoding/json's default), the same
// rule the agentpakke contract states: adding a field to the served manifest
// must never require every developer to upgrade nav-pilot first. Fail-closed
// applies to malformed declarations of *known* constructs, and to the trust
// boundary below.
//
// # Trust boundary
//
// The served file names weights that a developer's machine downloads and loads
// into its own process, and the environment that process runs under. That makes
// the publisher of each model and the reach of that environment trust
// decisions, not formatting ones, so both live in code ([allowedPublishers],
// [allowedParamKey]) rather than in the served file: widening either is a
// reviewable diff in this repo, not an edit to a JSON file in another one.
//
// # Never blocking
//
// [Resolve] prefers the network but never depends on it: a failed, slow or
// unparsable fetch falls back to the last-known-good cache on disk, and that to
// the copy embedded in the binary. A developer on a train gets a manifest.
package local

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	_ "embed"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// ManifestURL is where the generated manifest is served from. It is the raw
// file in the repo that generates it, so there is no publishing step between
// the generator and this binary that could serve something else.
const ManifestURL = "https://raw.githubusercontent.com/navikt/mlx-workspace/main/manifest/models.json"

// SupportedSchemaMajors lists the manifest schema majors this binary
// understands. A major outside this set is rejected whole: a new major means
// the fields this code reads may no longer mean what they meant, and guessing
// is worse than using the copy this binary was built against.
var SupportedSchemaMajors = []string{"1"}

// allowedPublishers is the trust boundary, not a style rule. Every model id in
// the manifest must be published by one of these Hugging Face orgs, because
// accepting the manifest means a developer's machine will fetch and execute
// those weights. Widening this set must be a code change here, reviewed like
// any other, and never an edit to the served file.
var allowedPublishers = []string{"mlx-community", "lmstudio-community"}

// allowedParamKey is the other half of that boundary. A model's Params become
// the server process's environment verbatim ([Server.Start]), and an
// environment variable decides things the publisher allow-list cannot see:
// PYTHONPATH runs manifest-chosen code at import, DYLD_INSERT_LIBRARIES loads a
// library into the process, and HF_ENDPOINT keeps the allow-listed model *name*
// while moving the host the bytes come from. Restricting keys to the
// generator's own MLX_ namespace keeps a new knob a manifest change, and keeps
// the manifest out of every variable the server itself does not read.
var allowedParamKey = regexp.MustCompile(`^MLX_[A-Z0-9_]+$`)

//go:embed models.json
var embeddedManifest []byte

// Model is one entry in the manifest: a model a machine with the stated memory
// can serve, plus the environment the server needs to run it.
type Model struct {
	// Key is the manifest-local identity of the entry (the profile it was
	// generated from).
	Key string `json:"key"`

	// Name is the human-readable label, used as-is in the model picker.
	Name string `json:"name"`

	// Model is the model id — publisher/repo on Hugging Face. This is the
	// value [Lookup] matches and the one a launch passes on as --model.
	Model string `json:"model"`

	// Backend is the server that loads it ("mlx-lm").
	Backend string `json:"backend"`

	// Default marks the single entry chosen when the developer names none.
	Default bool `json:"default"`

	// Role and Expect are the generator's prose about what the model is for
	// and what it can actually deliver, shown when a developer picks a model.
	Role   string `json:"role"`
	Expect string `json:"expect"`

	// WeightsGB, MinRAMGB and WiredLimitGB are the machine requirements: disk
	// for the weights, total RAM to run at all, and the wired-memory limit the
	// server is tuned against.
	WeightsGB    int `json:"weights_gb"`
	MinRAMGB     int `json:"min_ram_gb"`
	WiredLimitGB int `json:"wired_limit_gb"`

	// Params is the server environment for this model, passed through as-is.
	// It is deliberately an untyped map: the generator adds knobs as models
	// need them, and a typed struct here would make every new knob a nav-pilot
	// release.
	Params map[string]string `json:"params"`
}

// Manifest is a parsed, validated local-model manifest.
type Manifest struct {
	// SchemaVersion is the contract version of the served file. It is a JSON
	// number in the contract ("schema_version": 1), kept as json.Number so a
	// minor bump (1.1) stays readable as major 1 rather than being rounded
	// into a float this code then has to format back.
	SchemaVersion json.Number `json:"schema_version"`

	// Channel is the release channel the file was generated for ("alpha").
	Channel string `json:"channel"`

	Models []Model `json:"models"`
}

// Parse validates raw manifest bytes and returns the manifest. Validation is
// fail-closed and whole-file: a manifest that violates any rule is refused
// entirely rather than filtered down to its acceptable entries, because a
// manifest carrying an entry this binary refuses is not the manifest the
// generator meant to publish.
func Parse(data []byte) (*Manifest, error) {
	var m Manifest
	// No DisallowUnknownFields: unknown fields are the contract's forward
	// compatibility, see the package doc.
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("local-model manifest is not valid JSON: %w", err)
	}
	if err := m.checkSchemaVersion(); err != nil {
		return nil, err
	}
	if err := m.checkModels(); err != nil {
		return nil, err
	}
	return &m, nil
}

// checkSchemaVersion refuses a schema major this binary does not implement. The
// error names both versions and says what happens next, because the developer
// reading it cannot fix the served file and needs to know their session is
// still working — from the cached copy — and that upgrading is the fix.
func (m *Manifest) checkSchemaVersion() error {
	major, _, _ := strings.Cut(m.SchemaVersion.String(), ".")
	if slices.Contains(SupportedSchemaMajors, major) {
		return nil
	}
	return fmt.Errorf(
		"local-model manifest schema_version %q is newer than this nav-pilot understands (supported: %s); "+
			"the last cached manifest is used instead — run %s to get a binary that reads it",
		m.SchemaVersion.String(), strings.Join(SupportedSchemaMajors, ", "), domain.Bold("nav-pilot update"))
}

// checkModels enforces the four rules the served file cannot be trusted to
// keep on its own: a usable model id, an allow-listed publisher, an environment
// confined to the MLX_ namespace, and exactly one default.
func (m *Manifest) checkModels() error {
	var defaults []string
	for i, model := range m.Models {
		// Named by key where there is one: "models[3]" is not something a
		// manifest author can find quickly in a generated file.
		where := model.Key
		if where == "" {
			where = fmt.Sprintf("models[%d]", i)
		}
		// Shape first: an id nav-pilot cannot pass on as --model is broken
		// regardless of who published it.
		if err := domain.ValidateModelValue(model.Model); err != nil {
			return fmt.Errorf("local-model manifest entry %q: %w", where, err)
		}
		publisher, _, ok := strings.Cut(model.Model, "/")
		if !ok || !slices.Contains(allowedPublishers, publisher) {
			return fmt.Errorf(
				"local-model manifest entry %q names model %q, which is not published by an allowed publisher (%s); "+
					"the manifest names weights this machine downloads and runs, so allowing another publisher is a nav-pilot code change, not a manifest change",
				where, model.Model, strings.Join(allowedPublishers, ", "))
		}
		// Same boundary as the publisher above: the allow-list decides which
		// weights run, the environment decides where they come from and what
		// is loaded alongside them. Sorted so a manifest with more than one
		// offending key names the same one every run.
		for _, key := range slices.Sorted(maps.Keys(model.Params)) {
			if !allowedParamKey.MatchString(key) {
				return fmt.Errorf(
					"local-model manifest entry %q sets param %q on model %q, which is outside the MLX_ namespace (%s); "+
						"params become the environment of the server process, so a key outside it can point the download at another host or load code into the process — allowing another key is a nav-pilot code change, not a manifest change",
					where, key, model.Model, allowedParamKey)
			}
		}
		if model.Default {
			defaults = append(defaults, where)
		}
	}
	// Exactly one: zero leaves the picker with nothing to preselect, and two
	// makes "the default" depend on iteration order — a silent wrong answer
	// rather than a loud one.
	if len(defaults) != 1 {
		return fmt.Errorf(
			"local-model manifest must mark exactly one model as \"default\": true, this one marks %d (%s)",
			len(defaults), strings.Join(defaults, ", "))
	}
	return nil
}

// Source names where the manifest [Resolve] returned came from.
type Source string

const (
	SourceNetwork  Source = "network"
	SourceCache    Source = "cache"
	SourceEmbedded Source = "embedded"
)

// fetchManifest is the network half of [Resolve], a package-level variable so
// tests exercise every fallback without touching the network — the same seam
// internal/provider's version probes use.
//
// The timeout is short on purpose. This is a fetch nav-pilot can always do
// without: waiting on a hotel captive portal to answer is strictly worse than
// using yesterday's manifest, which is almost always the same file.
var fetchManifest = func(url string) ([]byte, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	// The cap is generous next to a manifest of a few kilobytes, and stops a
	// wrong URL (an HTML error page, a redirect to something large) from being
	// read into memory in full before it fails to parse.
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// navPilotPath joins a path under ~/.nav-pilot, the convention the config file
// and the pinned agentpakke trees already follow. Stated once, so the manifest
// cache and the runtime data directory cannot drift into two nav-pilot
// directories. "" when there is no home directory; every caller treats that as
// "no such file" rather than as an error.
func navPilotPath(parts ...string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(append([]string{home, ".nav-pilot"}, parts...)...)
}

// cachePath is where the last-known-good manifest is kept. A variable so tests
// can point it at a temp dir. No home means no cache: Resolve falls through to
// the embedded copy rather than failing.
var cachePath = func() string { return navPilotPath("local-models.json") }

// Resolve returns the manifest to use: freshly fetched when the network
// answers with something valid, otherwise the last-known-good cache, otherwise
// the copy embedded in the binary.
//
// The returned error is advisory, not fatal — it explains why a fallback was
// used so a caller can say so once — and the manifest is usable whenever it is
// non-nil. Only a broken embedded copy (a defect in this repo, pinned by a
// test) yields a nil manifest.
func Resolve() (*Manifest, Source, error) {
	data, err := fetchManifest(ManifestURL)
	if err == nil {
		m, perr := Parse(data)
		if perr == nil {
			// Cached only after it parses and validates: a last-known-*good*
			// cache that can hold a manifest this binary refuses is not a
			// fallback, it is the same failure saved for next time.
			return m, SourceNetwork, writeCache(data)
		}
		err = perr
	}
	m, src, cerr := Cached()
	if m == nil {
		// Unreachable unless the embedded file was edited into something
		// invalid, which TestEmbeddedManifestIsValid catches in CI.
		return nil, src, errors.Join(err, cerr)
	}
	if src == SourceCache {
		return m, src, fmt.Errorf("using the cached local-model manifest: %w", err)
	}
	return m, src, fmt.Errorf("using the local-model manifest built into nav-pilot: %w", errors.Join(err, cerr))
}

// Cached is [Resolve] without the network: the last-known-good cache,
// otherwise the copy embedded in the binary.
//
// It exists because "never blocking" (see the package doc) is a promise about
// startup, and [Resolve] only keeps it in the sense that it eventually gives
// up. Every nav-pilot invocation arms local dispatch at startup, so a Resolve
// there put a five-second connect on the front of `nav-pilot config get` for
// anyone behind a captive portal. Only init and start act on the manifest, and
// only they may pay for a fresh one.
//
// The error is advisory, and only ever explains a cache that was skipped. A nil
// manifest means the embedded copy is broken, which is a defect in this repo.
func Cached() (*Manifest, Source, error) {
	var err error
	if path := cachePath(); path != "" {
		if cached, cerr := os.ReadFile(path); cerr == nil {
			m, perr := Parse(cached)
			if perr == nil {
				return m, SourceCache, nil
			}
			err = perr
		}
	}
	m, perr := Parse(embeddedManifest)
	if perr != nil {
		return nil, SourceEmbedded, errors.Join(err, perr)
	}
	return m, SourceEmbedded, err
}

// writeCache stores a validated manifest as the last-known-good copy. A write
// failure is reported but does not spoil the fetch: the manifest in hand is
// still the freshest one, this session just will not have a better fallback
// next time.
func writeCache(data []byte) error {
	path := cachePath()
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("caching the local-model manifest: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("caching the local-model manifest: %w", err)
	}
	return nil
}

// active is the manifest [IsLocal] and [Lookup] answer from. It defaults to
// the embedded copy, so no code path reads a file or a socket to learn whether
// a model id is local; [SetActive] installs the resolved one.
// Mirrors source.ActivePakke, which holds the agentpakke declarations the same
// way and for the same reason.
var active = embeddedOrEmpty()

// embeddedOrEmpty parses the embedded manifest. A parse failure is a defect in
// this repo (the file ships with the binary and TestEmbeddedManifestIsValid
// pins it), so it yields an empty manifest rather than a panic: the predicates
// then answer "not local", which is the safe answer in a launch path.
func embeddedOrEmpty() *Manifest {
	m, err := Parse(embeddedManifest)
	if err != nil {
		return &Manifest{}
	}
	return m
}

// SetActive installs the manifest the predicates answer from. A nil manifest
// restores the embedded copy.
func SetActive(m *Manifest) {
	if m == nil {
		m = embeddedOrEmpty()
	}
	active = m
}

// Active returns the manifest the predicates answer from. Never nil.
func Active() *Manifest { return active }

// Lookup returns the manifest entry for a model id.
func Lookup(model string) (Model, bool) {
	for _, m := range Active().Models {
		if m.Model == model {
			return m, true
		}
	}
	return Model{}, false
}

// enabled is the opt-in gate, and it is off until something turns it on.
//
// Off by default is the whole design of the alpha: 650 people will never run
// `nav-pilot alpha local init`, and for every one of them every call below must
// answer exactly as it did before this package existed. A default of "on if the
// manifest lists it" would have made the manifest — a file in another repo —
// able to switch a stranger's launch onto a local model.
var enabled bool

// SetEnabled turns local dispatch on or off for this process. nav-pilot sets it
// once at startup from the persisted config, and only true when local is both
// installed ([Installed]) and enabled by the developer.
func SetEnabled(on bool) { enabled = on }

// Enabled reports whether local dispatch is on.
func Enabled() bool { return enabled }

// IsLocal reports whether a model id is served locally *and* local dispatch is
// enabled. This is the single predicate for that question: everything else in
// the binary asks it rather than matching on the id itself, so the answer
// changes with the manifest and the opt-in, and not with a prefix check
// somewhere.
//
// The opt-in is folded in here rather than checked beside every call site
// because a call site that forgets it is a silent change of where a stranger's
// prompt goes. [Lookup] is the ungated half, for the commands that must read
// the manifest before anything is enabled.
func IsLocal(model string) bool {
	if !enabled {
		return false
	}
	_, ok := Lookup(model)
	return ok
}
