package cli

import (
	"os"
	"strings"
	"testing"
)

// #623 klaget på ordlyden «in conflict state and were skipped», fulgt av «Run
// sync --apply to apply updates». Den ble skrevet om ett sted, men fantes to
// steder, og grenen der det ikke er noe annet å synke beholdt den gamle
// (#651). Testen leser kilden, fordi det er formuleringene i seg selv saken
// handler om, og de finnes ikke igjen i noen returverdi.
func TestNoStaleConflictWordingRemains(t *testing.T) {
	src := codeWithoutComments(readSourceFile(t, "sync.go"))
	// Bare den konfliktspesifikke ordlyden. "Run sync --apply to apply
	// updates" er riktig der det faktisk finnes oppdateringer å ta; det #623
	// klaget på var å si det om en konflikt.
	for _, phrase := range []string{
		"in conflict state and were skipped",
	} {
		if strings.Contains(src, phrase) {
			t.Errorf("sync.go inneholder fortsatt %q, som #623 ba om å bli kvitt", phrase)
		}
	}
}

// Meldinga skal finnes ett sted. To kopier er det som lot dem gli fra
// hverandre i utgangspunktet.
func TestConflictSummaryHasOneCallSiteEach(t *testing.T) {
	src := codeWithoutComments(readSourceFile(t, "sync.go"))
	if got := strings.Count(src, "differ from what nav-pilot installed"); got != 1 {
		t.Errorf("formuleringa står %d steder i sync.go, ventet 1", got)
	}
	if got := strings.Count(src, "printConflictSummary(scope, conflictPaths, src.SHA)"); got != 2 {
		t.Errorf("printConflictSummary kalles %d steder, ventet 2", got)
	}
}

// doctor nevnte ikke konflikter i det hele tatt (#651).
func TestDoctorReportsConflicts(t *testing.T) {
	src := readSourceFile(t, "doctor.go")
	if !strings.Contains(src, "reportScopeConflicts") {
		t.Fatal("doctor.go nevner ikke konflikter")
	}
	if got := strings.Count(src, "reportScopeConflicts("); got != 3 {
		t.Errorf("reportScopeConflicts nevnt %d ganger (definisjon pluss to scope), ventet 3", got)
	}
}

func readSourceFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// codeWithoutComments drops // lines, so a doc comment that quotes the old
// wording in order to explain why it is gone does not read as the wording
// still being there.
func codeWithoutComments(src string) string {
	var kept []string
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}
