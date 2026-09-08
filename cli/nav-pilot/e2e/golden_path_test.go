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
// hjemmekatalog uten å sammenlikne med den.
//
// Den sammenlikner heller ikke nå, og det er et bevisst valg: utviklerens
// ~/.copilot er 177 000 filer og 3,6 GB på maskinen dette ble skrevet på, så
// en gjennomgang før og etter koster sekunder per test og blir ustabil hvis en
// annen økt skriver samtidig. En test som feiler av seg selv blir slått av.
//
// I stedet måles mekanismen isolasjonen hviler på: at binæren faktisk leser og
// skriver der harnessen peker den. Får sandkassen skrivingene, gikk de ikke
// noe annet sted.
func TestInstallWritesInsideTheSandbox(t *testing.T) {
	e := newEnv(t)
	src := e.pakke("plattform", "grillmester")
	cons := e.consumer("forbruker")

	real, err := os.UserHomeDir()
	if err == nil && real == e.home {
		t.Fatal("sandkassens HOME er utviklerens egen, da isolerer harnessen ingenting")
	}

	if out, code := e.run(cons, "install", "plattform", "--source", src, "--repo"); code != 0 {
		t.Fatalf("install feilet: %d\n%s", code, out)
	}

	// Kilden install lagrer havner i sandkassens config, altså ble
	// NAV_PILOT_CONFIG lest.
	body, err := os.ReadFile(filepath.Join(e.home, "config.toml"))
	if err != nil {
		t.Fatalf("kilden ble ikke lagret i sandkassens config: %v", err)
	}
	if !strings.Contains(string(body), src) {
		t.Errorf("sandkassens config nevner ikke kilden:\n%s", body)
	}

	// Og harnessen peker faktisk de variablene nav-pilot og opencode leser.
	env := e.commandEnv()
	for _, key := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "NAV_PILOT_CONFIG"} {
		val, ok := env[key]
		if !ok {
			t.Errorf("harnessen setter ikke %s, så en kjøring kan skrive utenfor sandkassen", key)
			continue
		}
		if !strings.HasPrefix(val, e.root) {
			t.Errorf("%s=%q peker utenfor sandkassen %s", key, val, e.root)
		}
	}
}

// Et arvet artefakt kom ikke fra toppkildens revisjon, så det skal ikke
// stemples med den. Sync sin «installed from <sha>»-hint ville ellers navngi
// en revisjon fila aldri var i.
func TestInheritedFilesAreNotStampedWithTheTopRevision(t *testing.T) {
	e := newEnv(t)
	base := e.pakke("basepakke", "felles")
	own := e.pakke("egenpakke", "eget")
	e.declareReuse(own, base)
	cons := e.consumer("forbruker")

	if out, code := e.run(cons, "install", "egenpakke", "--source", own, "--repo"); code != 0 {
		t.Fatalf("install feilet: %d\n%s", code, out)
	}
	body, err := os.ReadFile(filepath.Join(cons, ".github", ".nav-pilot-state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state struct {
		Files []struct {
			Path     string `json:"path"`
			Revision string `json:"revision"`
		} `json:"files"`
	}
	if err := json.Unmarshal(body, &state); err != nil {
		t.Fatal(err)
	}
	var sawOwn bool
	for _, f := range state.Files {
		switch {
		case strings.Contains(f.Path, "felles"):
			if f.Revision != "" {
				t.Errorf("det arvede artefaktet ble stemplet med %q", f.Revision)
			}
		case strings.Contains(f.Path, "eget"):
			sawOwn = true
			if f.Revision == "" {
				t.Error("pakkens eget artefakt mistet revisjonsstempelet sitt")
			}
		}
	}
	if !sawOwn {
		t.Fatal("fant ikke pakkens eget artefakt i staten, da tester dette ingenting")
	}
}

// Sync skal lese kilden erklæringa navngir, ikke den som står i config.
// Testen som fantes dekket adoptSyncSource, ikke syncScope, så kilden sync
// faktisk ba om var usett.
func TestSyncReadsTheDeclaredSource(t *testing.T) {
	e := newEnv(t)
	declared := e.pakke("erklaert", "grillmester")
	other := e.pakke("annen", "noeannet")
	cons := e.consumer("forbruker")

	if out, code := e.run(cons, "install", "erklaert", "--source", declared, "--repo"); code != 0 {
		t.Fatalf("install feilet: %d\n%s", code, out)
	}
	// Config peker et annet sted enn erklæringa.
	if out, code := e.run(cons, "config", "set", "source", other); code != 0 {
		t.Fatalf("config set feilet: %d\n%s", code, out)
	}

	out, _ := e.run(cons, "sync", "--repo")
	if strings.Contains(out, "noeannet") {
		t.Errorf("sync leste kilden fra config framfor fra erklæringa:\n%s", out)
	}
	if strings.Contains(out, "deleted in source") {
		t.Errorf("sync mente filene var slettet, altså leste den feil kilde:\n%s", out)
	}
}
