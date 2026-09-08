package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Den gylne stien, ende til ende: lag pakke, valider, installer, synk.
func TestGoldenPath(t *testing.T) {
	e := newEnv(t)
	src := e.pakke("plattform", "grillmester", "barista")
	cons := e.consumer("forbruker")

	if out, code := e.run(src, "validate", "--source", src); code != 0 {
		t.Fatalf("validate feilet: %d\n%s", code, out)
	}
	out, code := e.run(cons, "install", "plattform", "--source", src, "--repo")
	if code != 0 {
		t.Fatalf("install feilet: %d\n%s", code, out)
	}
	for _, a := range []string{"grillmester", "barista"} {
		if !e.exists(filepath.Join(cons, ".github", "agents", a+".agent.md")) {
			t.Errorf("%s ble ikke installert", a)
		}
	}
	out, code = e.run(cons, "sync", "--repo", "--source", src)
	if code != 0 {
		t.Errorf("sync på et ferskt install ga kode %d, ventet 0\n%s", code, out)
	}
	if !strings.Contains(out, "up to date") {
		t.Errorf("sync sa ikke at alt var oppdatert:\n%s", out)
	}
}

// Komposisjon: install henter begge pakkene, og sync lar det arvede være.
// Dette er defekten fra 8. september, der sync meldte hvert arvet artefakt
// som slettet oppstrøms og --apply fjernet dem.
func TestReuseSurvivesSync(t *testing.T) {
	e := newEnv(t)
	base := e.pakke("basepakke", "felles", "grillmester")
	own := e.pakke("egenpakke", "eget", "grillmester")
	e.declareReuse(own, base)
	cons := e.consumer("forbruker")

	out, code := e.run(cons, "install", "egenpakke", "--source", own, "--repo")
	if code != 0 {
		t.Fatalf("install feilet: %d\n%s", code, out)
	}
	if !strings.Contains(out, "Reuses:") {
		t.Errorf("install sa ikke hva den gjenbrukte:\n%s", out)
	}
	inherited := filepath.Join(cons, ".github", "agents", "felles.agent.md")
	if !e.exists(inherited) {
		t.Fatal("det arvede artefaktet ble ikke installert")
	}

	out, code = e.run(cons, "sync", "--repo", "--source", own)
	if strings.Contains(out, "deleted in source") {
		t.Errorf("sync mente arvede filer var slettet oppstrøms:\n%s", out)
	}
	if code != 0 {
		t.Errorf("sync ga kode %d, ventet 0\n%s", code, out)
	}

	if out, code := e.run(cons, "sync", "--repo", "--apply", "--source", own); code != 0 {
		t.Fatalf("sync --apply feilet: %d\n%s", code, out)
	}
	if !e.exists(inherited) {
		t.Error("sync --apply slettet det arvede artefaktet")
	}
}

// Kollisjonsregelen: pakkens eget vinner over det gjenbrukte.
func TestOwnArtifactShadowsInherited(t *testing.T) {
	e := newEnv(t)
	base := e.pakke("basepakke", "grillmester")
	own := e.pakke("egenpakke", "grillmester")
	e.declareReuse(own, base)
	cons := e.consumer("forbruker")

	if out, code := e.run(cons, "install", "egenpakke", "--source", own, "--repo"); code != 0 {
		t.Fatalf("install feilet: %d\n%s", code, out)
	}
	body, err := os.ReadFile(filepath.Join(cons, ".github", "agents", "grillmester.agent.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "egenpakke") {
		t.Errorf("den gjenbrukte versjonen vant kollisjonen:\n%s", body)
	}
}

// En arvet fil som har driftet lokalt skal hentes fra pakka den kom fra.
// Uten det feiler --apply med «no such file or directory».
func TestApplyRestoresDriftedInheritedFile(t *testing.T) {
	e := newEnv(t)
	base := e.pakke("basepakke", "felles")
	own := e.pakke("egenpakke", "eget")
	e.declareReuse(own, base)
	cons := e.consumer("forbruker")

	if out, code := e.run(cons, "install", "egenpakke", "--source", own, "--repo"); code != 0 {
		t.Fatalf("install feilet: %d\n%s", code, out)
	}
	inherited := filepath.Join(cons, ".github", "agents", "felles.agent.md")
	e.write(inherited, "LOKAL DRIFT\n")

	out, code := e.run(cons, "sync", "--repo", "--apply", "--source", own)
	if code != 0 {
		t.Fatalf("sync --apply ga kode %d\n%s", code, out)
	}
	if strings.Contains(out, "Could not update") {
		t.Errorf("--apply klarte ikke lese den arvede fila:\n%s", out)
	}
	body, _ := os.ReadFile(inherited)
	if strings.Contains(string(body), "LOKAL DRIFT") {
		t.Error("den driftede arvede fila ble ikke oppdatert")
	}
}

// Install i pakkens eget repo skal ikke skrive om gjenbrukserklæringa.
func TestSelfInstallKeepsReuseDeclaration(t *testing.T) {
	e := newEnv(t)
	base := e.pakke("basepakke", "felles")
	own := e.pakke("egenpakke", "eget")
	e.declareReuse(own, base)

	if out, code := e.run(own, "install", "egenpakke", "--source", own, "--repo"); code != 0 {
		t.Fatalf("install feilet: %d\n%s", code, out)
	}
	body, err := os.ReadFile(filepath.Join(own, ".nav-pilot", "agentpakke.lock.json"))
	if err != nil {
		t.Fatal(err)
	}
	var decl struct {
		Source string `json:"source"`
	}
	if err := json.Unmarshal(body, &decl); err != nil {
		t.Fatal(err)
	}
	if decl.Source != base {
		t.Errorf("erklæringa peker på %q, ventet basen %q", decl.Source, base)
	}
}

// --json skal gi et dokument som parser, også når sync rydder.
func TestSyncJSONStaysParseable(t *testing.T) {
	e := newEnv(t)
	src := e.pakke("plattform", "grillmester")
	cons := e.consumer("forbruker")
	if out, code := e.run(cons, "install", "plattform", "--source", src, "--repo"); code != 0 {
		t.Fatalf("install feilet: %d\n%s", code, out)
	}
	out, _ := e.run(cons, "sync", "--repo", "--json", "--source", src)
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("sync --json ga noe som ikke er JSON: %v\n%s", err, out)
	}
}

// Isolasjonen er hele forutsetningen for suiten, så den måles framfor å antas.
//
// Testen som sto her het «ingenting skrives utenfor sandkassen» og sjekket
// bare at sandkassens config fantes; den hentet til og med utviklerens ekte
// hjemmekatalog uten å sammenlikne med den. Navnet lovet mer enn kroppen
// holdt, som er den samme feilen resten av denne økta handlet om.
//
// Det som faktisk er etterprøvbart: kilden install lagrer havner i
// sandkassens config og ikke i utviklerens, og alt install skrev ligger under
// forbrukerrepoet.
func TestInstallWritesOnlyInsideTheSandbox(t *testing.T) {
	e := newEnv(t)
	src := e.pakke("plattform", "grillmester")
	cons := e.consumer("forbruker")

	before := homeSnapshot(t)
	if out, code := e.run(cons, "install", "plattform", "--source", src, "--repo"); code != 0 {
		t.Fatalf("install feilet: %d\n%s", code, out)
	}

	cfg := filepath.Join(e.home, "config.toml")
	body, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatalf("kilden ble ikke lagret i sandkassens config: %v", err)
	}
	if !strings.Contains(string(body), src) {
		t.Errorf("sandkassens config nevner ikke kilden:\n%s", body)
	}
	if after := homeSnapshot(t); after != before {
		t.Errorf("utviklerens hjemmekatalog endret seg under kjøringa:\n før: %s\n etter: %s", before, after)
	}
}

// homeSnapshot beskriver de to katalogene nav-pilot ellers ville skrevet i,
// i utviklerens ekte hjemmekatalog. Endrer den seg over en kjøring, lekker
// sandkassen.
func homeSnapshot(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	var b strings.Builder
	for _, p := range []string{filepath.Join(home, ".copilot"), filepath.Join(home, ".nav-pilot")} {
		info, err := os.Stat(p)
		switch {
		case err != nil:
			b.WriteString(p + "=fraværende;")
		default:
			b.WriteString(p + "=" + info.ModTime().String() + ";")
		}
	}
	return b.String()
}
