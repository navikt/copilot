# Testing nav-pilot

nav-pilot har to testnivåer: strukturelle tester (raske, ingen avhengigheter) og E2E-tester (krever `copilot` CLI).

## Kjør tester

```bash
# Strukturelle tester — validerer agent-filer (< 1 sek)
./scripts/test/test-agent-phases.sh

# E2E-tester — kjører copilot CLI med agenter (~2-5 min per test)
./scripts/test/test-agent-phases.sh --e2e

# Verbose — viser agent-output for debugging
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
| Auth content | auth-agent | Output inneholder auth-relatert innhold (`Auth`, `token`, `OAuth`, `🔐`) |

## Fase-modellen

nav-pilot bruker en 4-fase modell. Hver fase har et emoji-prefiks og en eksplisitt stopp:

```
🔍 Fase 1: Intervju — kartlegger behov og blinde flekker
   Stiller spørsmål, identifiserer arketype
   ─────────────────────────────────────────
   ⏳ Venter på svar før Fase 2: Plan

📐 Fase 2: Plan — arkitektur og beslutninger
   Foreslår arkitektur, velger mønstre
   ─────────────────────────────────────────
   ⏳ Bekreft planen før Fase 3: Review

🔎 Fase 3: Review — kvalitetssikring
   Delegerer til @auth, @security-champion, @nais
   ─────────────────────────────────────────
   ⏳ Bekreft funn før Fase 4: Lever

🚀 Fase 4: Lever — genererer kode og dokumentasjon
   Implementerer basert på godkjent plan
```

## Hvordan fase-headers fungerer

Fase-headers styres av `<response_format>` XML-tag i `nav-pilot.agent.md`. Vi prøvde flere tilnærminger:

| Forsøk | Teknikk | Resultat |
|--------|---------|----------|
| 1 | `VIKTIG: Du SKAL alltid starte...` | ❌ Ignorert |
| 2 | `REGEL:` direktiv | ❌ Ignorert |
| 3 | `<response_format>` XML-tag | ✅ Fungerer |

**Lærdom**: Modellen behandler XML-tags (`<response_format>`, `<rules>`) som strukturelle krav med høyere prioritet enn fritekst-instruksjoner. Plasser dem tidlig i agent-filen, rett etter frontmatter.

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

Output lagres i temp-mappe for inspeksjon etter kjøring.

## Feilsøking

**E2E-tester feiler umiddelbart**: Sjekk at `copilot` CLI er installert og autentisert (`copilot --version`).

**Fase-header mangler**: Sjekk at `<response_format>` tag er intakt i `nav-pilot.agent.md` — det er den eneste teknikken som fungerer pålitelig.

**Output ser merkelig ut**: Bruk `-v` flagget og inspiser filene i temp-mappen som skrives ut på slutten.
