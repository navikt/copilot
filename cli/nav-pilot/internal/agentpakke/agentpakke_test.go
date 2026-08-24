package agentpakke

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTier(t *testing.T) {
	m := &Manifest{Clients: map[string]ClientEntry{
		"copilot": {
			PrimaryAgents: []string{"grillmester"},
			Payloads:      map[string]Payload{"full": {Path: "plugin"}},
		},
		"opencode": {PrimaryAgents: []string{"nav-pilot"}},
		"pi":       {PrimaryAgents: []string{"nav-pilot"}, Payloads: map[string]Payload{}},
	}}

	tests := []struct {
		client string
		want   int
	}{
		{"copilot", TierPayload},
		{"opencode", TierLayout},
		{"pi", TierLayout}, // an empty payloads map declares no payload, so it is not Tier 2
		{"claude-code", TierUnknown},
		{"", TierUnknown},
	}
	for _, tt := range tests {
		if got := m.Tier(tt.client); got != tt.want {
			t.Errorf("Tier(%q) = %d, want %d", tt.client, got, tt.want)
		}
	}

	if !m.HasTier(TierPayload) || !m.HasTier(TierLayout) {
		t.Error("HasTier should see both tiers in a mixed manifest")
	}
	if m.HasTier(TierUnknown) {
		t.Error("HasTier(TierUnknown) = true, want false")
	}
}

func TestPayloadManifestPath(t *testing.T) {
	tests := []struct {
		name    string
		payload Payload
		want    string
	}{
		{"by convention", Payload{Path: "targets/opencode-v1"}, "targets/opencode-v1/manifest.json"},
		{"override wins", Payload{Path: "plugin", Manifest: "policy/plugin-manifest.json"}, "policy/plugin-manifest.json"},
		{"blank override falls back", Payload{Path: "plugin", Manifest: "  "}, "plugin/manifest.json"},
	}
	for _, tt := range tests {
		if got := tt.payload.ManifestPath(); got != tt.want {
			t.Errorf("%s: ManifestPath() = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestDefaultContextFallback(t *testing.T) {
	m := &Manifest{Clients: map[string]ClientEntry{
		"copilot":  {DefaultContext: "focused"},
		"opencode": {},
	}}
	tests := []struct{ client, want string }{
		{"copilot", "focused"},
		{"opencode", DefaultContext},
		{"claude-code", DefaultContext},
	}
	for _, tt := range tests {
		if got := m.DefaultContext(tt.client); got != tt.want {
			t.Errorf("DefaultContext(%q) = %q, want %q", tt.client, got, tt.want)
		}
	}
}

func TestAvailableClients(t *testing.T) {
	m := &Manifest{Clients: map[string]ClientEntry{
		"opencode":    {},
		"claude-code": {},
		"copilot":     {},
	}}
	want := []string{"copilot", "opencode"}
	if got := m.AvailableClients(); !reflect.DeepEqual(got, want) {
		t.Errorf("AvailableClients() = %v, want %v (unknown keys are ignored, not offered)", got, want)
	}
	if got := len(m.ClientIDs()); got != 3 {
		t.Errorf("ClientIDs() dropped an unknown client key: got %d entries, want 3", got)
	}
}

func TestNilManifestHelpers(t *testing.T) {
	var m *Manifest
	if _, ok := m.Client("copilot"); ok {
		t.Error("Client on a nil manifest reported a hit")
	}
	if m.Tier("copilot") != TierUnknown {
		t.Error("Tier on a nil manifest is not TierUnknown")
	}
	if m.ClientIDs() != nil || m.PrimaryAgents("copilot") != nil {
		t.Error("nil manifest should yield nil slices")
	}
	if m.DefaultContext("copilot") != DefaultContext {
		t.Error("nil manifest should still yield the default context")
	}
}

// --- legacy synthesis ---

func TestDefaultIsAValidManifest(t *testing.T) {
	// The legacy adapter is not a special case: it must survive the same
	// validation a manifest read off disk does, or consumers would need to know
	// where their Manifest came from.
	data, err := json.Marshal(Default())
	if err != nil {
		t.Fatalf("marshalling Default(): %v", err)
	}
	if _, err := parse(data, devVersion); err != nil {
		t.Fatalf("Default() does not satisfy the published schema: %v\n%s", err, data)
	}
}

func TestDefaultMirrorsCurrentBehavior(t *testing.T) {
	m := Default()

	// Values duplicated from their current call sites. When those move to the
	// manifest in stage 2, this test is what catches a silent drift in between:
	//   internal/provider/copilot_launch.go  CopilotAgentPersona
	//   internal/provider/provider.go        OpenCodeAgentPersona, OpenCodeDefaultModel
	//   internal/source/frontmatter.go       openCodePrimaryAgents
	if got := m.PrimaryAgents("copilot"); !reflect.DeepEqual(got, []string{"nav-pilot"}) {
		t.Errorf("copilot primaryAgents = %v, want [nav-pilot] (CopilotAgentPersona)", got)
	}
	if got := m.PrimaryAgents("opencode"); !reflect.DeepEqual(got, []string{"nav-pilot", "nav-pilot-opus"}) {
		t.Errorf("opencode primaryAgents = %v, want [nav-pilot nav-pilot-opus] (openCodePrimaryAgents)", got)
	}
	if got := m.PrimaryAgents("opencode")[0]; got != "nav-pilot" {
		t.Errorf("opencode launch persona = %q, want nav-pilot (OpenCodeAgentPersona)", got)
	}
	if got := m.DefaultModel("opencode"); got != "github-copilot/auto" {
		t.Errorf("opencode defaultModel = %q, want github-copilot/auto (OpenCodeDefaultModel)", got)
	}
	if got := m.DefaultModel("copilot"); got != "" {
		t.Errorf("copilot defaultModel = %q, want empty — copilot pins no default today", got)
	}
	if m.IsPrimaryAgent("opencode", "kafka") {
		t.Error("kafka is a subagent in opencode today, not a primary agent")
	}
	if m.Layout == nil || m.Layout.Agents != "agents" || m.Layout.Skills != "skills" {
		t.Errorf("layout = %+v, want the agents//skills/ content layout", m.Layout)
	}
	for _, client := range KnownClients {
		if m.Tier(client) != TierLayout {
			t.Errorf("Tier(%q) = %d, want TierLayout — Nav's default content is markdown, not payloads", client, m.Tier(client))
		}
	}
}

func TestSynthesizeLegacyCarriesTheCollection(t *testing.T) {
	tests := []struct {
		name       string
		collection string
		wantName   string
	}{
		{"named collection", "utvikler", "utvikler"},
		{"hyphenated collection", "nav-pilot", "nav-pilot"},
		{"no collection", "", DefaultName},
		{"synthetic all-collection is not an identifier", "(all)", DefaultName},
		{"uppercase is not an identifier", "Utvikler", DefaultName},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := SynthesizeLegacy(tt.collection)
			if m.Name != tt.wantName {
				t.Errorf("SynthesizeLegacy(%q).Name = %q, want %q", tt.collection, m.Name, tt.wantName)
			}
			data, err := json.Marshal(m)
			if err != nil {
				t.Fatalf("marshalling: %v", err)
			}
			if _, err := parse(data, devVersion); err != nil {
				t.Fatalf("synthesized manifest fails validation: %v", err)
			}
		})
	}
}

// --- published schema ---

func TestEmbeddedSchemaMatchesPublishedFile(t *testing.T) {
	// The file in cli/nav-pilot/schemas is what agentpakke repos lint against;
	// the binary must validate with those exact bytes.
	published := filepath.Join("..", "..", "schemas", "agentpakke-v1.json")
	onDisk, err := os.ReadFile(published)
	if err != nil {
		t.Fatalf("reading %s: %v", published, err)
	}
	if !bytes.Equal(onDisk, SchemaJSON()) {
		t.Fatalf("the embedded schema differs from %s", published)
	}
}

func TestPublishedSchemaCompiles(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal(SchemaJSON(), &doc); err != nil {
		t.Fatalf("the published schema is not valid JSON: %v", err)
	}
	if got := doc["$id"]; got != SchemaID {
		t.Errorf("schema $id = %v, want %s", got, SchemaID)
	}
	sch, err := schema()
	if err != nil {
		t.Fatalf("compiling the published schema: %v", err)
	}
	if sch == nil {
		t.Fatal("compiled schema is nil")
	}
}

func TestSchemaJSONIsACopy(t *testing.T) {
	first := SchemaJSON()
	if len(first) == 0 {
		t.Fatal("SchemaJSON() is empty")
	}
	first[0] = 'X'
	if SchemaJSON()[0] == 'X' {
		t.Error("SchemaJSON() handed out the embedded bytes; callers could corrupt the schema")
	}
}
