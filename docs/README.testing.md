# Testing nav-pilot

nav-pilot har to testnivåer. Strukturelle tester leser agent-filene og krever ingenting installert. E2E-tester kjører `copilot` CLI mot ekte agenter og tar tid.

## Kjør tester

```bash
# Strukturelle tester, validerer agent-filer (< 1 sek)
./scripts/test/test-agent-phases.sh

# E2E-tester, kjører copilot CLI med agenter (~2-5 min per test)
./scripts/test/test-agent-phases.sh --e2e

# Verbose, viser agent-output for debugging
./scripts/test/test-agent-phases.sh --e2e -v
```

## Hva testes

### Strukturelle tester (17 sjekker)

Validerer agent-filene uten å kjøre Copilot:

| Sjekk | Hva | Hvorfor |
|-------|-----|---------|
| `<response_format>` tag | nav-pilot.agent.md har XML-tag | Modellen behandler XML-tags som strukturelle krav |
| Fase 1-4 definert | Alle 4 faser finnes med emoji | Sikrer at fase-definisjoner ikke fjernes ved redigering |
| Fase-separator | `─────` mønster | Visuell pause mellom faser |
| Imperativt språk | MUST/REGEL/SKAL | Instruksjoner må være tydelige, ikke rådgivende |
| Spesialist-fremdrift | Alle 5 spesialist-agenter har progress-indikatorer | Konsistent brukeropplevelse på tvers av agenter |

### E2E-tester (3 sjekker)

Kjører `copilot --agent <name> -p "prompt" --allow-all` og sjekker output:

| Test | Agent | Forventet |
|------|-------|-----------|
| Phase header | nav-pilot | Output inneholder `Fase 1` eller `Fase 2` med emoji |
| Planning phase | nav-pilot | Output inneholder `Intervju` eller `Plan` |

## Fase-modellen

nav-pilot bruker fire faser. Hver fase har et emoji-prefiks, og alle utenom den siste stopper og venter på brukeren.

| Fase | Gjør | Venter på |
|------|------|-----------|
| 🔍 Fase 1: Intervju | Stiller spørsmål, kartlegger behov og blindsoner, identifiserer arketype | Svar før Fase 2 |
| 📐 Fase 2: Plan | Foreslår arkitektur, velger mønstre | Bekreftet plan før Fase 3 |
| 🔎 Fase 3: Review | Delegerer til @security-champion, laster $nav-auth og $nais | Bekreftede funn før Fase 4 |
| 🚀 Fase 4: Lever | Genererer kode og dokumentasjon fra godkjent plan | Ingenting, siste fase |

## Hvordan fase-headers fungerer

Fase-headers styres av `<response_format>` XML-tag i `nav-pilot.agent.md`. Vi prøvde tre tilnærminger:

| Forsøk | Teknikk | Resultat |
|--------|---------|----------|
| 1 | `VIKTIG: Du SKAL alltid starte...` | ❌ Ignorert |
| 2 | `REGEL:` direktiv | ❌ Ignorert |
| 3 | `<response_format>` XML-tag | ✅ Fungerer |

Lærdommen: modellen behandler XML-tags som `<response_format>` og `<rules>` som strukturelle krav med høyere prioritet enn fritekst. Plasser dem tidlig i agent-filen, rett etter frontmatter.

## Legge til nye tester

### Strukturell test

Legg til en `grep`-sjekk i seksjonen "Structural Tests":

```bash
if grep -q "mitt_mønster" "$AGENT_FILE"; then
  pass "Min nye sjekk"
else
  fail "Min nye sjekk" "Mønster ikke funnet"
fi
```

### E2E-test

Legg til et nytt `run_agent` + `check_file` par:

```bash
log "Test N: beskrivelse"
FILE=$(run_agent "test-name" "agent-name" "prompt til agenten")
check_file "forventet oppførsel" "$FILE" "(regex|mønster)"
```

Output lagres i en temp-mappe for inspeksjon etter kjøring.

## Feilsøking

| Symptom | Sjekk |
|---------|-------|
| E2E-tester feiler umiddelbart | At `copilot` CLI er installert og autentisert (`copilot --version`) |
| Fase-header mangler | At `<response_format>` tag er intakt i `nav-pilot.agent.md`. Det er den eneste teknikken som fungerer pålitelig |
| Output ser merkelig ut | Kjør med `-v` og inspiser filene i temp-mappen som skrives ut på slutten |
