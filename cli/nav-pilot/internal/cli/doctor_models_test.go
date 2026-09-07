package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFrontmatterModel(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want string
	}{
		{"plain pin", "---\nname: a\nmodel: Claude Sonnet 5\n---\n\nbody\n", "Claude Sonnet 5"},
		{"quoted pin", "---\nmodel: \"GPT-5.6 Sol\"\n---\n", "GPT-5.6 Sol"},
		{"no pin means inherit", "---\nname: a\ntools:\n  - read\n---\n\nbody\n", ""},
		{"not frontmatter at all", "# Overskrift\n\nmodel: noe\n", ""},
		// The reason this reads the delimiter rather than grepping the file: a
		// persona that discusses model choice in its prose must not be read as
		// pinning one.
		{"model: in the body is not a pin", "---\nname: a\n---\n\nmodel: Claude Sonnet 5\n", ""},
		{"unterminated frontmatter", "---\nmodel: Claude Sonnet 5\n", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := frontmatterModel([]byte(tt.doc)); got != tt.want {
				t.Errorf("frontmatterModel = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUnavailablePins(t *testing.T) {
	catalogue := []string{"claude-sonnet-5", "gpt-5.6-sol"}
	pins := []pinnedModel{
		{Agent: "ok-label", Label: "Claude Sonnet 5", ID: "claude-sonnet-5"},
		{Agent: "dead", Label: "Claude Sonnet 4.6", ID: "claude-sonnet-4.6"},
		// A label the picker does not know resolves to no id, so the client is
		// asked for the label verbatim and fails the same way. Reported, not
		// skipped: skipping would hide exactly the pin most likely to be wrong.
		{Agent: "unknown-label", Label: "Fantasimodell 9", ID: ""},
		{Agent: "case", Label: "GPT-5.6 Sol", ID: "GPT-5.6-SOL"},
	}
	got := unavailablePins(pins, catalogue)
	var names []string
	for _, p := range got {
		names = append(names, p.Agent)
	}
	if want := []string{"dead", "unknown-label"}; !reflect.DeepEqual(names, want) {
		t.Errorf("unavailablePins = %v, want %v", names, want)
	}
}

func TestInstalledModelPins(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	scope, err := ScopeUser()
	if err != nil {
		t.Fatal(err)
	}
	dir := scope.DstPath("agents")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("pinned.agent.md", "---\nname: pinned\nmodel: Claude Sonnet 5\n---\n")
	write("inherits.agent.md", "---\nname: inherits\n---\n")
	write("notanagent.md", "---\nmodel: Claude Sonnet 5\n---\n")

	pins := installedModelPins(scope)
	if len(pins) != 1 {
		t.Fatalf("installedModelPins = %+v, want exactly the one pinned agent", pins)
	}
	if pins[0].Agent != "pinned" || pins[0].Label != "Claude Sonnet 5" {
		t.Errorf("pin = %+v, want agent \"pinned\" on \"Claude Sonnet 5\"", pins[0])
	}
	if pins[0].ID != "claude-sonnet-5" {
		t.Errorf("pin id = %q, want the resolved Copilot id", pins[0].ID)
	}
}

// The parser is measured against a real client debug line, not against the
// shape the format is assumed to have. Anchoring on "type":"chat" is what keeps
// embedding models out; without it they are reported as launchable chat models.
func TestChatModelIDSkipsEmbeddings(t *testing.T) {
	line := `x \"type\":\"chat\",\"id\":\"claude-sonnet-5\",y ` +
		`\"type\":\"embeddings\",\"id\":\"text-embedding-3-small\",z ` +
		`\"type\":\"chat\",\"id\":\"gpt-5.6-luna\",w`
	var got []string
	for _, m := range chatModelID.FindAllStringSubmatch(line, -1) {
		got = append(got, m[1])
	}
	want := []string{"claude-sonnet-5", "gpt-5.6-luna"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("chat model ids = %v, want %v", got, want)
	}
}
