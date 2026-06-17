package source

import (
	"testing"
)

func TestSplitFrontmatter(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		wantFM string
		wantBd string
		wantOK bool
	}{
		{
			name:   "standard frontmatter",
			input:  "---\nname: test\ndescription: hello\n---\n\nBody content here.\n",
			wantFM: "name: test\ndescription: hello\n",
			wantBd: "\nBody content here.\n",
			wantOK: true,
		},
		{
			name:   "no frontmatter",
			input:  "Just a regular file.\nNo frontmatter here.\n",
			wantFM: "",
			wantBd: "Just a regular file.\nNo frontmatter here.\n",
			wantOK: false,
		},
		{
			name:   "empty frontmatter",
			input:  "---\n---\n\nBody only.\n",
			wantFM: "",
			wantBd: "\nBody only.\n",
			wantOK: true,
		},
		{
			name:   "frontmatter with nested YAML",
			input:  "---\nname: skill\nmetadata:\n  domain: auth\n  tags:\n    - security\n---\n\n# Content\n",
			wantFM: "name: skill\nmetadata:\n  domain: auth\n  tags:\n    - security\n",
			wantBd: "\n# Content\n",
			wantOK: true,
		},
		{
			name:   "frontmatter only no body",
			input:  "---\nname: test\n---\n",
			wantFM: "name: test\n",
			wantBd: "",
			wantOK: true,
		},
		{
			name:   "no closing delimiter",
			input:  "---\nname: test\nno closing\n",
			wantFM: "",
			wantBd: "---\nname: test\nno closing\n",
			wantOK: false,
		},
		{
			name:   "delimiter not at start",
			input:  "some text\n---\nname: test\n---\n",
			wantFM: "",
			wantBd: "some text\n---\nname: test\n---\n",
			wantOK: false,
		},
		{
			name:   "CRLF line endings",
			input:  "---\r\nname: test\r\ndescription: hello\r\n---\r\n\r\nBody content.\r\n",
			wantFM: "name: test\ndescription: hello\n",
			wantBd: "\nBody content.\n",
			wantOK: true,
		},
		{
			name:   "trailing whitespace on delimiter",
			input:  "---  \nname: test\n---   \n\nBody.\n",
			wantFM: "name: test\n",
			wantBd: "\nBody.\n",
			wantOK: true,
		},
		{
			name:   "only opening delimiter no newline",
			input:  "---",
			wantFM: "",
			wantBd: "---",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, body, ok := SplitFrontmatter([]byte(tt.input))
			if ok != tt.wantOK {
				t.Fatalf("hasFrontmatter = %v, want %v", ok, tt.wantOK)
			}
			if string(fm) != tt.wantFM {
				t.Errorf("frontmatter:\ngot:  %q\nwant: %q", string(fm), tt.wantFM)
			}
			if string(body) != tt.wantBd {
				t.Errorf("body:\ngot:  %q\nwant: %q", string(body), tt.wantBd)
			}
		})
	}
}

func TestStripFrontmatterKeys(t *testing.T) {
	tests := []struct {
		name string
		fm   string
		keys []string
		want string
	}{
		{
			name: "strip simple key",
			fm:   "name: test\ndescription: hello\nlicense: MIT\n",
			keys: []string{"name"},
			want: "description: hello\nlicense: MIT\n",
		},
		{
			name: "strip multiple keys",
			fm:   "name: test\ndescription: hello\nlicense: MIT\n",
			keys: []string{"name", "license"},
			want: "description: hello\n",
		},
		{
			name: "strip nested key",
			fm:   "name: test\nmetadata:\n  domain: auth\n  tags:\n    - security\ndescription: hello\n",
			keys: []string{"metadata"},
			want: "name: test\ndescription: hello\n",
		},
		{
			name: "strip key not present",
			fm:   "name: test\ndescription: hello\n",
			keys: []string{"nonexistent"},
			want: "name: test\ndescription: hello\n",
		},
		{
			name: "strip all keys",
			fm:   "name: test\ndescription: hello\n",
			keys: []string{"name", "description"},
			want: "",
		},
		{
			name: "empty frontmatter",
			fm:   "",
			keys: []string{"name"},
			want: "",
		},
		{
			name: "strip tools from agent",
			fm:   "name: nav-pilot\ndescription: Plan and build\ntools:\n  - github\n  - terminal\n",
			keys: []string{"name", "tools"},
			want: "description: Plan and build\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripFrontmatterKeys([]byte(tt.fm), tt.keys)
			if string(got) != tt.want {
				t.Errorf("stripFrontmatterKeys:\ngot:  %q\nwant: %q", string(got), tt.want)
			}
		})
	}
}

func TestExtractFrontmatterValue(t *testing.T) {
	tests := []struct {
		name   string
		fm     string
		key    string
		want   string
		wantOK bool
	}{
		{
			name:   "simple value",
			fm:     "name: test\ndescription: hello world\n",
			key:    "description",
			want:   "hello world",
			wantOK: true,
		},
		{
			name:   "quoted value",
			fm:     "name: \"my agent\"\n",
			key:    "name",
			want:   "my agent",
			wantOK: true,
		},
		{
			name:   "single quoted value",
			fm:     "name: 'my agent'\n",
			key:    "name",
			want:   "my agent",
			wantOK: true,
		},
		{
			name:   "key not found",
			fm:     "name: test\n",
			key:    "description",
			want:   "",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ExtractFrontmatterValue([]byte(tt.fm), tt.key)
			if ok != tt.wantOK {
				t.Fatalf("found = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("value = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildAgentFrontmatter(t *testing.T) {
	tests := []struct {
		name        string
		description string
		mode        string
		want        string
	}{
		{
			name:        "simple description subagent",
			description: "Plan and build Nav apps",
			mode:        "subagent",
			want:        "description: Plan and build Nav apps\nmode: subagent\n",
		},
		{
			name:        "primary mode",
			description: "Plan and build Nav apps",
			mode:        "primary",
			want:        "description: Plan and build Nav apps\nmode: primary\n",
		},
		{
			name:        "empty mode defaults to subagent",
			description: "Plan and build Nav apps",
			mode:        "",
			want:        "description: Plan and build Nav apps\nmode: subagent\n",
		},
		{
			name:        "description with colon",
			description: "Norsk teknisk redaktør: klarspråk og fagtermer",
			mode:        "subagent",
			want:        "description: \"Norsk teknisk redaktør: klarspråk og fagtermer\"\nmode: subagent\n",
		},
		{
			name:        "description with hash",
			description: "Review code # quality",
			mode:        "subagent",
			want:        "description: \"Review code # quality\"\nmode: subagent\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildAgentFrontmatter(tt.description, tt.mode)
			if string(got) != tt.want {
				t.Errorf("buildAgentFrontmatter:\ngot:  %q\nwant: %q", string(got), tt.want)
			}
		})
	}
}

func TestOpenCodeAgentMode(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"nav-pilot", "primary"},
		{"nav-pilot-opus", "primary"},
		{"auth", "subagent"},
		{"research", "subagent"},
		{"", "subagent"},
	}
	for _, tt := range tests {
		if got := OpenCodeAgentMode(tt.name); got != tt.want {
			t.Errorf("OpenCodeAgentMode(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestTransformPromptFrontmatter(t *testing.T) {
	fm := []byte("name: aksel-component\ndescription: Generate Aksel components\n")
	got := TransformPromptFrontmatter(fm)
	want := "description: Generate Aksel components\n"
	if string(got) != want {
		t.Errorf("transformPromptFrontmatter:\ngot:  %q\nwant: %q", string(got), want)
	}
}

func TestYamlQuoteIfNeededEmpty(t *testing.T) {
	got := yamlQuoteIfNeeded("")
	if got != `""` {
		t.Errorf("yamlQuoteIfNeeded empty = %q, want %q", got, `""`)
	}
}

func TestReassemble(t *testing.T) {
	tests := []struct {
		name string
		fm   string
		body string
		want string
	}{
		{
			name: "with frontmatter and body",
			fm:   "name: test\n",
			body: "# Content\n",
			want: "---\nname: test\n---\n\n# Content\n",
		},
		{
			name: "empty frontmatter",
			fm:   "",
			body: "# Content\n",
			want: "# Content\n",
		},
		{
			name: "frontmatter without trailing newline",
			fm:   "name: test",
			body: "# Content\n",
			want: "---\nname: test\n---\n\n# Content\n",
		},
		{
			name: "frontmatter only",
			fm:   "name: test\n",
			body: "",
			want: "---\nname: test\n---\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Reassemble([]byte(tt.fm), []byte(tt.body))
			if string(got) != tt.want {
				t.Errorf("reassemble:\ngot:  %q\nwant: %q", string(got), tt.want)
			}
		})
	}
}
