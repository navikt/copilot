package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The key tables on the docs page and in docs/README.nav-pilot.md are made
// from configKeyDefs: which keys exist, their flags, allowed values and
// defaults. Only the Norwegian prose is written by hand, below. When this test
// fails, run
//
//	go test ./internal/cli -run TestConfigKeyDocs -update-config-docs
//
// and commit the two files.

var updateConfigDocs = flag.Bool("update-config-docs", false, "rewrite the config key tables in the docs")

var (
	configDocsPage   = filepath.Join("..", "..", "..", "..", "apps", "my-copilot", "src", "app", "(nb)", "nav-pilot", "docs", "page.tsx")
	configDocsReadme = filepath.Join("..", "..", "..", "..", "docs", "README.nav-pilot.md")
)

// configKeyDocNB is each user-facing key's description on the Norwegian docs.
var configKeyDocNB = map[string]string{
	"version":             "Skjemaversjon. Mangler den, leses filen som versjon 1, og nav-pilot sier fra med én linje.",
	"client":              "Klient å starte: copilot, opencode eller pi (eksperimentell). Alle kjører i cplt-sandkassen.",
	"source":              "Hvor agentpakken hentes fra: et GitHub-repo eller en lokal checkout. Settes av install --source --save-source; nav-pilot config unset source går tilbake til standarden.",
	"model":               "Modell å bruke. En Copilot-id som claude-opus-4.8 virker for copilot og opencode (opencode kjører den som github-copilot/<id>); opencode tar også provider/model. nav-pilot config explain model lister id-ene.",
	"mode":                "Modus for Copilot-agenten. plan tilsvarer opencode --agent plan; autopilot er kun Copilot.",
	"reasoning_effort":    "Resonneringsinnsats. Copilot bruker --effort, opencode bruker --variant.",
	"context_tier":        "Kontekstnivå. Kun Copilot, og nav-pilot advarer om feltet er satt for opencode.",
	"allow_all_tools":     "La agenten kjøre alle verktøy uten å spørre først.",
	"ask_user":            "La agenten stoppe og spørre deg. Kun Copilot, og nav-pilot advarer om feltet er satt for opencode.",
	"auto_launch":         "Start kodeagenten etter synk eller installasjon. Med false skriver nav-pilot bare ut kommandoen.",
	"auto_update":         "Oppgrader nav-pilot automatisk når en ny versjon er ute, uten å spørre.",
	"log_level":           "Loggnivå for Copilot CLI.",
	"otel_log_level":      "Loggnivå for OpenTelemetry i Copilot CLI (OTEL_LOG_LEVEL). En OTEL_LOG_LEVEL i skallet vinner, og config show merker den env.",
	"local_enabled":       "Send avgrensede oppgaver til en lokal modell (alfa). Settes av alpha local init, nullstilles av alpha local off. Så lenge den er false finnes ingen lokale modeller i nav-pilot.",
	"local_autostart":     "La en vanlig nav-pilot starte den lokale serveren når den trengs og ingen kjører. Av som standard: å starte en 21 GB prosess uten å bli bedt om det er ikke greit.",
	"local_loop_guard":    "Hvor mange identiske verktøykall på rad som avslutter en lokal tur, uansett hva de returnerer. Gir kallene samme resultat hver gang, holder det med halvparten (minst 2).",
	"local_model":         "Hvilken lokal modell serveren laster (alfa). Tom betyr standardmodellen i manifestet. Enklest satt med nav-pilot alpha local use <key>.",
	"hook_loop_guard":     "Samme løkkeregel i alle Copilot CLI-økter, også i skyen: en postToolUse-hook i ~/.copilot/hooks/ sier fra til modellen når den står fast. false fjerner hooken ved neste oppstart.",
	"hook_redact_secrets": "Masker hemmeligheter (GitHub-tokener, AWS-nøkkel-id-er, private nøkler, JWT-er, verdien i password=/api_key=) i verktøyresultater før modellen leser dem, i alle Copilot CLI-økter.",
	"hook_redact_fnr":     "Masker fødselsnummer, D-nummer og H-nummer i verktøyresultater. Bare elleve sifre der datoen og begge kontrollsifrene stemmer blir maskert.",
	"hook_injection_note": "Sett en merknad foran verktøyresultater som ser ut som instrukser til modellen («ignore previous instructions», rollemarkører), så modellen behandler dem som data. Stopper ingenting.",
	"copilot_auth_mode":   "Hvilken innlogging som når cplt for Copilot. auto begrenser ingenting; env_only krever et token i GH_TOKEN, GITHUB_TOKEN eller COPILOT_GITHUB_TOKEN; gh_only fjerner dem.",
}

// configKeyValuesNB describes the values of the keys that take free text.
var configKeyValuesNB = map[string]string{
	"source":      "owner/name eller en absolutt sti",
	"model":       "modell-id, f.eks. claude-opus-4.8",
	"local_model": "modell-id fra manifestet",
}

type configKeyDocRow struct{ key, flag, values, desc string }

func configKeyDocRows(t *testing.T) []configKeyDocRow {
	t.Helper()
	var rows []configKeyDocRow
	for _, name := range userKeyNames() {
		kd := findKeyDef(name)
		desc, ok := configKeyDocNB[name]
		if !ok {
			t.Fatalf("config key %s has no Norwegian description in configKeyDocNB", name)
		}
		values := configKeyValuesNB[name]
		switch {
		case len(kd.allowed) > 0:
			values = strings.Join(kd.allowed, " · ")
		case kd.kind == keyKindBool:
			values = "true · false"
		case kd.kind == keyKindInt:
			values = "et heltall"
		}
		if values == "" {
			t.Fatalf("config key %s has no values text", name)
		}
		if kd.defaultVal != "" && name != "version" {
			values += " (standard: " + kd.defaultVal + ")"
		}
		flag := kd.flag
		if flag == "" {
			flag = "—"
		}
		rows = append(rows, configKeyDocRow{name, flag, values, desc})
	}
	return rows
}

// tsString is s as a double-quoted string literal, the way prettier leaves it.
func tsString(t *testing.T, s string) string {
	t.Helper()
	if strings.ContainsAny(s, "\"\\") {
		t.Fatalf("docs text must not hold a quote or backslash (prettier would change the quotes): %s", s)
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return strings.TrimSpace(b.String())
}

// renderConfigKeysTS is the CONFIG_KEYS array as prettier (printWidth 120)
// formats it: a long values string moves to its own line, while key, flag
// and desc are short enough keys that prettier never breaks after them.
func renderConfigKeysTS(t *testing.T, rows []configKeyDocRow) string {
	var b strings.Builder
	b.WriteString("const CONFIG_KEYS = [\n")
	for _, r := range rows {
		b.WriteString("  {\n")
		fmt.Fprintf(&b, "    key: %s,\n", tsString(t, r.key))
		fmt.Fprintf(&b, "    flag: %s,\n", tsString(t, r.flag))
		values := "    values: " + tsString(t, r.values) + ","
		if len([]rune(values)) > 120 {
			values = "    values:\n      " + tsString(t, r.values) + ","
		}
		b.WriteString(values + "\n")
		fmt.Fprintf(&b, "    desc: %s,\n", tsString(t, r.desc))
		b.WriteString("  },\n")
	}
	b.WriteString("];\n")
	return b.String()
}

func renderConfigKeysMarkdown(rows []configKeyDocRow) string {
	var b strings.Builder
	b.WriteString("| Nøkkel | CLI-flagg | Verdier | Beskrivelse |\n| --- | --- | --- | --- |\n")
	for _, r := range rows {
		cell := strings.NewReplacer("|", "\\|", "<", "&lt;", ">", "&gt;")
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s |\n", r.key, r.flag, cell.Replace(r.values), cell.Replace(r.desc))
	}
	return b.String()
}

// replaceBetween swaps the text from start up to and including end.
func replaceBetween(t *testing.T, content, start, end, with string) string {
	t.Helper()
	i := strings.Index(content, start)
	if i < 0 {
		t.Fatalf("marker %q not found", start)
	}
	j := strings.Index(content[i:], end)
	if j < 0 {
		t.Fatalf("marker %q not found after %q", end, start)
	}
	return content[:i] + with + content[i+j+len(end):]
}

func TestConfigKeyDocs(t *testing.T) {
	rows := configKeyDocRows(t)
	for _, doc := range []struct {
		path, start, end, body string
	}{
		{configDocsPage, "const CONFIG_KEYS = [\n", "\n];\n", renderConfigKeysTS(t, rows)},
		{configDocsReadme, "<!-- config-keys:start -->\n", "<!-- config-keys:end -->\n",
			"<!-- config-keys:start -->\n" + renderConfigKeysMarkdown(rows) + "<!-- config-keys:end -->\n"},
	} {
		data, err := os.ReadFile(doc.path)
		if err != nil {
			t.Fatal(err)
		}
		want := replaceBetween(t, string(data), doc.start, doc.end, doc.body)
		if want == string(data) {
			continue
		}
		if *updateConfigDocs {
			if err := os.WriteFile(doc.path, []byte(want), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		t.Errorf("%s is out of date with configKeyDefs; run go test ./internal/cli -run TestConfigKeyDocs -update-config-docs", doc.path)
	}
}
