package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// writePakke lays down a minimal conforming agentpakke with the named agents.
func writePakke(t *testing.T, dir, name string, agents ...string) {
	t.Helper()
	for _, sub := range []string{".nav-pilot", "agents", "skills"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	manifest := `{"contractVersion":"1","name":"` + name + `","description":"` + name +
		`","layout":{"agents":"agents","skills":"skills"},"clients":{"copilot":{"primaryAgents":["` + agents[0] + `"]}}}`
	if err := os.WriteFile(filepath.Join(dir, ".nav-pilot", "agentpakke.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, a := range agents {
		body := "---\nname: " + a + "\ndescription: fra " + name + "\n---\n" + name + "\n"
		if err := os.WriteFile(filepath.Join(dir, "agents", a+".agent.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func declareReuse(t *testing.T, dir, base string) {
	t.Helper()
	body := `{"contractVersion":"1","source":"` + base + `"}`
	if err := os.WriteFile(filepath.Join(dir, ".nav-pilot", "agentpakke.lock.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func loadSource(t *testing.T, dir string) *Source {
	t.Helper()
	src := &Source{Dir: dir, Repo: dir}
	if err := attachPakke(src); err != nil {
		t.Fatal(err)
	}
	return src
}

// En pakke som gjenbruker en annen installerer begges innhold, og sitt eget
// vinner der navnene kolliderer (#572).
func TestComposedInstallInheritsAndShadows(t *testing.T) {
	baseDir, ownDir := t.TempDir(), t.TempDir()
	writePakke(t, baseDir, "basepakke", "felles", "grillmester")
	writePakke(t, ownDir, "egenpakke", "eget", "grillmester")
	declareReuse(t, ownDir, baseDir)

	src := loadSource(t, ownDir)
	resolver, bases, err := composeResolver(resolverFor(src.Dir, src.Pakke), src)
	if err != nil {
		t.Fatal(err)
	}
	reused := bases.nearest()
	if reused == nil || reused.Dir != baseDir {
		t.Fatalf("gjenbrukt kilde = %v, ventet %s", reused, baseDir)
	}
	manifest, err := pakkeContents(resolver, src)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(manifest.Agents, ","); got != "eget,felles,grillmester" {
		t.Errorf("agenter = %q, ventet eget,felles,grillmester", got)
	}
	got, ok := resolver.Get(KindAgent, "grillmester")
	if !ok {
		t.Fatal("grillmester ble ikke løst")
	}
	if !strings.HasPrefix(got.AbsPath, ownDir) {
		t.Errorf("grillmester løste til %s, ventet pakkens eget under %s", got.AbsPath, ownDir)
	}
}

// To pakker som gjenbruker hverandre har ingen rekkefølge å løses i. Uten
// vakta henter composeResolver dem vekselvis til stacken tar slutt.
func TestReuseCycleIsRefused(t *testing.T) {
	aDir, bDir := t.TempDir(), t.TempDir()
	writePakke(t, aDir, "a", "en")
	writePakke(t, bDir, "b", "to")
	declareReuse(t, aDir, bDir)
	declareReuse(t, bDir, aDir)

	src := loadSource(t, aDir)
	_, _, err := composeResolver(resolverFor(src.Dir, src.Pakke), src)
	if err == nil {
		t.Fatal("en gjenbrukssyklus ble godtatt")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("feilmeldinga nevner ikke syklusen: %v", err)
	}
}

// En pakke uten erklæring skal løses nøyaktig som før kjedinga fantes.
func TestPakkeWithoutDeclarationIsUnchanged(t *testing.T) {
	dir := t.TempDir()
	writePakke(t, dir, "alene", "en")
	src := loadSource(t, dir)
	own := resolverFor(src.Dir, src.Pakke)
	composed, bases, err := composeResolver(own, src)
	if err != nil {
		t.Fatal(err)
	}
	reused := bases.nearest()
	if reused != nil {
		t.Errorf("fant en gjenbrukt kilde der ingen er erklært: %v", reused)
	}
	if composed.Base() != nil {
		t.Error("resolveren fikk en base uten at noe er erklært")
	}
	if _, err := agentpakke.LoadDeclaration(dir); err == nil {
		t.Error("testpakka har en erklæring, da tester dette noe annet enn det står")
	}
}

// En gjenbruk av et repo uten sha ville klonet det main tilfeldigvis holder,
// så to installasjoner en uke fra hverandre komponerte ulikt innhold mens
// begge rapporterte samme erklæring.
func TestUnpinnedRepoReuseIsRefused(t *testing.T) {
	dir := t.TempDir()
	writePakke(t, dir, "egen", "eget")
	declareReuse(t, dir, "navikt/basepakke")

	src := loadSource(t, dir)
	_, _, err := composeResolver(resolverFor(src.Dir, src.Pakke), src)
	if err == nil {
		t.Fatal("en gjenbruk uten revisjon ble godtatt")
	}
	if !strings.Contains(err.Error(), "without a revision") {
		t.Errorf("feilmeldinga sier ikke hva som mangler: %v", err)
	}
}

// Kontroll: en lokal sti har ingen revisjon å pinne, og skal fortsatt virke.
// Uten denne ville testen over passert selv om vakta nektet alt.
func TestUnpinnedLocalPathReuseIsAllowed(t *testing.T) {
	baseDir, ownDir := t.TempDir(), t.TempDir()
	writePakke(t, baseDir, "basepakke", "felles")
	writePakke(t, ownDir, "egenpakke", "eget")
	declareReuse(t, ownDir, baseDir)

	src := loadSource(t, ownDir)
	_, bases, err := composeResolver(resolverFor(src.Dir, src.Pakke), src)
	if err != nil {
		t.Fatalf("en lokal sti uten sha ble nektet: %v", err)
	}
	reused := bases.nearest()
	if reused == nil {
		t.Fatal("den lokale basen ble ikke gjenbrukt")
	}
}

// Sync-komposisjonen er dekket i e2e/golden_path_test.go, ikke her.
//
// Testen som sto på dette stedet kalte composeResolver selv i stedet for å
// kjøre sync, altså gjentok den det sync gjør. Den passerte med kallet fjernet
// fra sync.go, som er nøyaktig defekten den ble skrevet for å fange. En test
// som ikke kan feile er verre enn ingen, fordi den ser ut som dekning.

// En kilde uten manifest komponerer ikke, selv om den skulle ha en erklæring
// liggende. Ellers ville en collection-kilde hentet et fremmed repo midt i en
// installasjon som aldri ba om det.
func TestLegacySourceDoesNotCompose(t *testing.T) {
	baseDir, ownDir := t.TempDir(), t.TempDir()
	writePakke(t, baseDir, "basepakke", "felles")
	if err := os.MkdirAll(filepath.Join(ownDir, ".nav-pilot"), 0o755); err != nil {
		t.Fatal(err)
	}
	declareReuse(t, ownDir, baseDir)

	src := &Source{Dir: ownDir, Repo: ownDir}
	if err := attachPakke(src); err != nil {
		t.Fatal(err)
	}
	if src.Pakke != nil {
		t.Fatal("kilden fikk et manifest, da tester dette noe annet enn det står")
	}
	_, bases, err := composeResolver(resolverFor(src.Dir, src.Pakke), src)
	if err != nil {
		t.Fatal(err)
	}
	reused := bases.nearest()
	if reused != nil {
		t.Errorf("en manifestløs kilde komponerte %s", reused.Dir)
	}
}

// Selvreferansen og syklusvakta sammenlikner kilder med sameSourceRepo, ikke
// med ren likhet. "Navikt/A" og "navikt/a" er ett repo, og to skrivemåter av
// samme sti er én katalog. Med ren likhet kunne en pakke navngi seg selv i en
// annen bokstavstørrelse og bli kjedet til seg selv.
func TestSelfReferenceIsCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	writePakke(t, dir, "egen", "eget")
	// Erklæringa navngir repoet med annen bokstavstørrelse enn kildelabelen.
	body := `{"contractVersion":"1","source":"NAVIKT/Grillmester","sha":"` + strings.Repeat("a", 40) + `"}`
	if err := os.WriteFile(filepath.Join(dir, ".nav-pilot", "agentpakke.lock.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	src := loadSource(t, dir)
	src.Repo = "navikt/grillmester"

	_, bases, err := composeResolver(resolverFor(src.Dir, src.Pakke), src)
	if err != nil {
		t.Fatalf("komposisjonen feilet: %v", err)
	}
	reused := bases.nearest()
	if reused != nil {
		t.Errorf("pakka ble kjedet til seg selv via %q", reused.Repo)
	}
}

// En sti skrevet med etterslept skråstrek er den samme katalogen.
func TestSelfReferenceResolvesPathSpelling(t *testing.T) {
	dir := t.TempDir()
	writePakke(t, dir, "egen", "eget")
	body := `{"contractVersion":"1","source":"` + dir + `/"}`
	if err := os.WriteFile(filepath.Join(dir, ".nav-pilot", "agentpakke.lock.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	src := loadSource(t, dir)

	_, bases, err := composeResolver(resolverFor(src.Dir, src.Pakke), src)
	if err != nil {
		t.Fatalf("komposisjonen feilet: %v", err)
	}
	reused := bases.nearest()
	if reused != nil {
		t.Errorf("pakka ble kjedet til seg selv via %q", reused.Dir)
	}
}

// En payload-pakke har ingen filer å arve fra: leveringsenheten er en
// digest-bundet revisjon, ikke filer på stier. Å komponere den meldte
// «Reuses:» og bidro så med ingenting, altså en påstand install ikke kunne
// innfri.
func TestPayloadOnlyBaseIsRefused(t *testing.T) {
	baseDir, ownDir := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(baseDir, ".nav-pilot"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(baseDir, "plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	payloadManifest := `{"contractVersion":"1","name":"t2base","description":"payload",` +
		`"clients":{"copilot":{"payloads":{"full":{"path":"plugin","primaryAgents":["x"]}}}}}`
	if err := os.WriteFile(filepath.Join(baseDir, ".nav-pilot", "agentpakke.json"), []byte(payloadManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	writePakke(t, ownDir, "egenpakke", "eget")
	declareReuse(t, ownDir, baseDir)

	src := loadSource(t, ownDir)
	_, bases, err := composeResolver(resolverFor(src.Dir, src.Pakke), src)
	if err == nil {
		t.Fatalf("en payload-pakke ble godtatt som base, gjenbrukt = %v", bases.nearest())
	}
	if !strings.Contains(err.Error(), "Tier 2") {
		t.Errorf("feilmeldinga sier ikke hvorfor: %v", err)
	}
}

// Motsatt retning: en Tier 2-pakke som selv erklærer gjenbruk. Install og sync
// pinner en revisjon av payloadene og leser aldri kildens egen erklæring, så
// komposisjonen skjer aldri. Det nektes ikke — erklæringa skader ingenting, og
// en pakke som alt sender en, skal fortsatt installere — men validate er stedet
// forfatteren spør om manifestet gjør det hun tror, så der skal det sies (#870).
func TestTier2ReuseDeclarationIsReportedInert(t *testing.T) {
	dir := tier2PinSourceTree(t)
	declareReuse(t, dir, t.TempDir())

	src := &Source{Dir: dir, SHA: "abc1234", Repo: "navikt/t2"}
	_, _, warnings, findings := validateSourceTree(src)
	if len(findings) > 0 {
		t.Fatalf("erklæringa gjorde pakka ugyldig, den skal bare varsles: %v", findings)
	}
	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, agentpakke.DeclarationPath) {
		t.Errorf("varselet navngir ikke fila:\n%s", joined)
	}
	if !strings.Contains(joined, "Tier 2") {
		t.Errorf("varselet sier ikke at det er tieren som gjør erklæringa virkningsløs:\n%s", joined)
	}
	if !strings.Contains(joined, "provenance.base") {
		t.Errorf("varselet sier ikke hva en Tier 2-forfatter skal gjøre i stedet:\n%s", joined)
	}
}

// Varselet gjelder nøyaktig den ene formen. En Tier 2-pakke uten erklæring har
// ingenting å varsle om, og en Tier 1-pakke med erklæring får gjenbruken sin
// løst ved hver install og sync: der er den virkelig, og et varsel ville løyet.
func TestReuseDeclarationWarningIsScopedToTier2(t *testing.T) {
	t.Run("tier 2 uten erklæring", func(t *testing.T) {
		src := &Source{Dir: tier2PinSourceTree(t), SHA: "abc1234", Repo: "navikt/t2"}
		if _, _, warnings, _ := validateSourceTree(src); len(warnings) > 0 {
			t.Errorf("varsler uten at noe er erklært: %v", warnings)
		}
	})
	t.Run("tier 1 med erklæring", func(t *testing.T) {
		baseDir, ownDir := t.TempDir(), t.TempDir()
		writePakke(t, baseDir, "basepakke", "felles")
		writePakke(t, ownDir, "egenpakke", "eget")
		declareReuse(t, ownDir, baseDir)

		src := &Source{Dir: ownDir, SHA: "abc1234", Repo: "navikt/t1"}
		_, _, warnings, findings := validateSourceTree(src)
		if len(findings) > 0 {
			t.Fatalf("en Tier 1-pakke med erklæring ble ugyldig: %v", findings)
		}
		if len(warnings) > 0 {
			t.Errorf("varsler om en gjenbruk som faktisk skjer: %v", warnings)
		}
	})
}

// En base med et manifest nav-pilot ikke kan bruke skal nekte, ikke bli
// komponert som om den var manifestløs. Svelges feilen, får basen nil Pakke,
// og resolveren leser da de kanoniske katalogene i stedet for layouten basen
// faktisk erklærer: innholdet blir stille feil eller borte.
func TestBaseWithUnusableManifestIsRefused(t *testing.T) {
	baseDir, ownDir := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(baseDir, ".nav-pilot"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Gyldig JSON, men bryter kontrakten: layout.agents peker ut av repoet.
	broken := `{"contractVersion":"1","name":"base","description":"b","layout":{"agents":"../utenfor"},"clients":{"copilot":{"primaryAgents":["base"]}}}`
	if err := os.WriteFile(filepath.Join(baseDir, ".nav-pilot", "agentpakke.json"), []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	writePakke(t, ownDir, "egenpakke", "eget")
	declareReuse(t, ownDir, baseDir)

	src := loadSource(t, ownDir)
	_, bases, err := composeResolver(resolverFor(src.Dir, src.Pakke), src)
	if err == nil {
		t.Fatalf("en base med ubrukelig manifest ble godtatt, gjenbrukt = %v", bases.nearest())
	}
	if !strings.Contains(err.Error(), "agentpakke") {
		t.Errorf("feilmeldinga peker ikke på manifestet: %v", err)
	}
}

// composingSource resolves a pakke that reuses another. "eget" is its own,
// "felles" is inherited: a path that does not compose delivers the first and
// silently drops the second (#844).
func composingSource(t *testing.T) *Source {
	t.Helper()
	baseDir, ownDir := t.TempDir(), t.TempDir()
	writePakke(t, baseDir, "basepakke", "felles")
	writePakke(t, ownDir, "egenpakke", "eget")
	declareReuse(t, ownDir, baseDir)
	return loadSource(t, ownDir)
}

// stateAgents names the agents a scope's state records as installed.
func stateAgents(t *testing.T, scope *InstallScope) map[string]bool {
	t.Helper()
	state, err := readScopedState(scope)
	if err != nil || state == nil {
		t.Fatalf("readScopedState = (%v, %v), want the install", state, err)
	}
	got := map[string]bool{}
	for _, f := range state.Files {
		got[strings.TrimSuffix(filepath.Base(f.Path), ".agent.md")] = true
	}
	return got
}

func wantAgents(t *testing.T, scope *InstallScope, names ...string) {
	t.Helper()
	got := stateAgents(t, scope)
	for _, n := range names {
		if !got[n] {
			t.Errorf("agent %q was not installed; state holds %v", n, got)
		}
	}
}

// Composition was wired into `install <navn>` and `sync` and nowhere else, so
// four other ways in delivered only the top pakke's content. A consumer running
// `nav-pilot install --user --all` against a pakke that reuses another got half
// of it, with no error and no warning (#844).
func TestEveryInstallPathComposes(t *testing.T) {
	forceNonInteractive = true
	t.Cleanup(func() { forceNonInteractive = false })

	// `--all` is the one that matters: a team that builds on a platform pakke
	// rather than forking it is exactly the team that runs it.
	t.Run("install --all", func(t *testing.T) {
		scope := pinEnv(t)
		src := composingSource(t)
		if err := installAllFromSource(scope, src, nil, false, false, false); err != nil {
			t.Fatalf("installAllFromSource: %v", err)
		}
		wantAgents(t, scope, "eget", "felles")
	})

	// The picker builds its own manifest and hands it to installAllFromSource,
	// so an item it never offered is an item that never lands — composing the
	// installer alone would not have been enough.
	t.Run("interactive picker", func(t *testing.T) {
		scope := pinEnv(t)
		src := composingSource(t)
		if err := interactiveUserInstallFromSource(scope, src, ""); err != nil {
			t.Fatalf("interactiveUserInstallFromSource: %v", err)
		}
		wantAgents(t, scope, "eget", "felles")
	})

	// `install <navn> --type <kind>` lands in cmdAdd without passing the
	// dispatcher.
	t.Run("single artifact by type", func(t *testing.T) {
		scope := pinEnv(t)
		src := composingSource(t)
		stubResolveSource(t, src)
		if err := cmdAdd("agent", "felles", scope, "", "", false, false, false); err != nil {
			t.Fatalf("cmdAdd of an inherited agent: %v", err)
		}
		wantAgents(t, scope, "felles")
	})

	// Without --type the dispatcher matches the name against the resolver
	// first. Uncomposed it found nothing and answered "not found" for an
	// artifact the pakke ships.
	t.Run("single artifact by name", func(t *testing.T) {
		scope := pinEnv(t)
		src := composingSource(t)
		stubResolveSource(t, src)
		if err := cmdInstallAuto("felles", "", scope, "", "", false, false, false); err != nil {
			t.Fatalf("install of an inherited agent by name: %v", err)
		}
		wantAgents(t, scope, "felles")
	})

	// `list` is what made the rest hard to see: it left out the inherited
	// content too, so what the user read matched what they got and both were
	// incomplete.
	t.Run("list", func(t *testing.T) {
		pinEnv(t)
		stubResolveSource(t, composingSource(t))
		var err error
		out := captureStdoutFor(t, func() { err = cmdList(nil, "", "", true, true) })
		if err != nil {
			t.Fatalf("cmdList: %v", err)
		}
		if !strings.Contains(out, "felles") {
			t.Errorf("list does not show the inherited agent:\n%s", out)
		}
		if !strings.Contains(out, "eget") {
			t.Errorf("list does not show the pakke's own agent:\n%s", out)
		}
	})
}

// A consumer's `items` list selects from everything the pakke installs, the
// inherited half included: applyDeclaredItems validates against the composed
// manifest, so an inherited name is a name it can narrow to rather than one it
// refuses as "not shipped". That already held on this path; it is pinned here
// because routing every path through composedResolverFor is what could quietly
// take it away, and because `nav-pilot list --items` — the output the refusal
// sends the reader to — only started showing those names in #844.
func TestDeclaredItemsCanNameAnInheritedArtifact(t *testing.T) {
	forceNonInteractive = true
	t.Cleanup(func() { forceNonInteractive = false })
	isolatedConfig(t)

	src := composingSource(t)
	target := t.TempDir()
	if err := os.MkdirAll(filepath.Join(target, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(target, agentpakke.ManifestDir), 0o755); err != nil {
		t.Fatal(err)
	}
	decl := `{"contractVersion":"1","source":"` + src.Repo + `","items":{"felles":"agent"}}`
	if err := os.WriteFile(filepath.Join(target, agentpakke.DeclarationPath), []byte(decl), 0o644); err != nil {
		t.Fatal(err)
	}

	scope := ScopeRepo(target)
	if err := cmdInstallFromSource("egenpakke", src, scope, false, false, false); err != nil {
		t.Fatalf("install narrowed to an inherited agent: %v", err)
	}
	got := stateAgents(t, scope)
	if !got["felles"] {
		t.Errorf("the declared inherited agent did not land; state holds %v", got)
	}
	if got["eget"] {
		t.Errorf("the item list was ignored: %v", got)
	}
}

// declarePinnedReuse erklærer gjenbruk av en repo-formet kilde. Repo-formen
// krever en revisjon, til forskjell fra den lokale stien declareReuse skriver.
func declarePinnedReuse(t *testing.T, dir, base string) {
	t.Helper()
	body := `{"contractVersion":"1","source":"` + base + `","sha":"` + strings.Repeat("c", 40) + `"}`
	if err := os.WriteFile(filepath.Join(dir, ".nav-pilot", "agentpakke.lock.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fakeRemotePakker gjør repo-formede kilder klonbare uten nett. Hver kilde
// klones til sin egen katalog under rota som returneres, slik testen kan telle
// checkoutene en komposisjon legger igjen. kjede sier hvilken pakke hver kilde
// gjenbruker videre; tom verdi er nederste ledd.
func fakeRemotePakker(t *testing.T, kjede map[string]string) string {
	t.Helper()
	root := t.TempDir()
	orig := source.CloneRemoteFn
	t.Cleanup(func() { source.CloneRemoteFn = orig })
	source.CloneRemoteFn = func(_, sourceRepo string) (*source.Source, error) {
		next, ok := kjede[sourceRepo]
		if !ok {
			return nil, fmt.Errorf("testen klonet %q, som den ikke har lagt opp", sourceRepo)
		}
		dir, err := os.MkdirTemp(root, "klone-*")
		if err != nil {
			return nil, err
		}
		name := strings.TrimPrefix(sourceRepo, "navikt/")
		writePakke(t, dir, name, name+"-agent")
		if next != "" {
			declarePinnedReuse(t, dir, next)
		}
		return &source.Source{Dir: dir, TempDir: dir, SHA: strings.Repeat("b", 40)}, nil
	}
	return root
}

// clonedDirs er checkoutene som fortsatt ligger under rota.
func clonedDirs(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// Hvert ledd i en gjenbrukskjede klones til sin egen temp-katalog. Kalleren kan
// bare rydde det den får igjen, og fikk før #867 bare det nærmeste leddet:
// mellomleddene nådde aldri fram, så hver install, sync og list la igjen en
// katalog per ledd bak det første.
func TestComposedChainCleansEveryTempCheckout(t *testing.T) {
	clones := fakeRemotePakker(t, map[string]string{
		"navikt/mellom": "navikt/bunn",
		"navikt/bunn":   "",
	})
	ownDir := t.TempDir()
	writePakke(t, ownDir, "egenpakke", "eget")
	declarePinnedReuse(t, ownDir, "navikt/mellom")

	src := loadSource(t, ownDir)
	resolver, bases, err := composeResolver(resolverFor(src.Dir, src.Pakke), src)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := resolver.Get(KindAgent, "bunn-agent"); !ok {
		t.Fatal("det nederste leddet ble ikke komponert inn, da måler testen noe annet enn den sier")
	}
	if len(bases) != 2 {
		t.Fatalf("kjeden kalleren fikk = %d ledd, ventet 2", len(bases))
	}
	if got := clonedDirs(t, clones); len(got) != 2 {
		t.Fatalf("checkouter under komposisjonen = %d, ventet 2", len(got))
	}

	bases.cleanup()

	if got := clonedDirs(t, clones); len(got) != 0 {
		t.Errorf("checkouter igjen etter opprydding: %v", got)
	}
}

// En komposisjon som feiler underveis har alt klonet leddene over bruddet.
// Kalleren får ingenting å rydde, så hvert ledd må ryddes der det ble laget.
func TestFailedCompositionLeavesNoTempCheckout(t *testing.T) {
	clones := fakeRemotePakker(t, map[string]string{
		"navikt/mellom": "navikt/bunn",
		"navikt/bunn":   "navikt/mellom",
	})
	ownDir := t.TempDir()
	writePakke(t, ownDir, "egenpakke", "eget")
	declarePinnedReuse(t, ownDir, "navikt/mellom")

	src := loadSource(t, ownDir)
	_, _, err := composeResolver(resolverFor(src.Dir, src.Pakke), src)
	if err == nil {
		t.Fatal("en gjenbrukssyklus ble godtatt")
	}
	if got := clonedDirs(t, clones); len(got) != 0 {
		t.Errorf("checkouter igjen etter at komposisjonen feilet: %v", got)
	}
}
