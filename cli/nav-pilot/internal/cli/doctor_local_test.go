package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestReportLocalModel(t *testing.T) {
	tests := []struct {
		name    string
		r       ResolvedConfig
		managed bool
		want    []string
		dont    []string
	}{
		{name: "not configured",
			want: []string{"Not configured", "nav-pilot alpha local setup", "about 22k tokens", "80%", "at least 30k", "GPU with more memory", "unified memory", "cloud models"}},
		{name: "own server", r: ResolvedConfig{LocalEndpoint: "http://127.0.0.1:11434/v1", LocalEndpointModel: "gemma4:12b", LocalEnabled: true},
			want: []string{"Own server: http://127.0.0.1:11434/v1, model gemma4:12b (local dispatch on)", "nav-pilot alpha local doctor"},
			dont: []string{"Not configured"}},
		{name: "managed", managed: true,
			want: []string{"Managed MLX server (local dispatch off)", "nav-pilot alpha local status"},
			dont: []string{"Not configured"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b bytes.Buffer
			reportLocalModel(&b, tt.r, tt.managed)
			out := stripANSI(b.String())
			for _, w := range tt.want {
				if !strings.Contains(out, w) {
					t.Errorf("missing %q in:\n%s", w, out)
				}
			}
			for _, d := range tt.dont {
				if strings.Contains(out, d) {
					t.Errorf("unexpected %q in:\n%s", d, out)
				}
			}
		})
	}
}
