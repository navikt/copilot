package survey

import (
	"strings"
	"testing"
)

func TestExportSkipsHidden(t *testing.T) {
	in := `{"answers":{"c":"skjul meg","d":"behold"},"question_versions":{},"context":{},"hidden":["c ` + TextHash("skjul meg") + `","d ` + TextHash("annet") + `"]}`
	var out strings.Builder
	if err := RunExport(strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "skjul meg") || !strings.Contains(out.String(), "behold") {
		t.Fatal(out.String())
	}
}
