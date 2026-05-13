---
title: "Fra fastpris til AI Credits: Strategier for kostnadseffektiv KI-bruk"
date: 2026-05-13
author: starefosen
category: nav
excerpt: "Model-pinning, caching, context engineering og sju konkrete tips for å holde Copilot-kostnadene nede når bruksbasert fakturering starter 1. juni."
url: "https://github.com/navikt/copilot/issues/216"
tags:
  - billing
  - business-controls
  - models
  - cost-optimization
---

Nav har nå over 600 Copilot Business-brukere. Bruken øker måned for måned — flere agenter, lengre sesjoner, mer kontekst. Fra 1. juni endrer GitHub faktureringsmodellen: premium requests erstattes av AI Credits ut fra faktisk token-forbruk. Det betyr at modellvalg blir en direkte kostnadsfaktor.

## Hva endrer seg 1. juni

Hver Business-bruker får 1 900 AI Credits per måned ($19). Credits pooler på organisasjonsnivå — Nav får ca. 950 000 credits i måneden. Én credit = $0.01.

«Auto»-modus har innebygd rabatt og velger en passende modell for oppgaven. Problemet er at mange brukere velger Claude Opus manuelt for alt — også oppgaver der Sonnet gir like godt resultat.

| | Claude Opus 4.6 | Claude Sonnet 4.6 | Forskjell |
| --- | --- | --- | --- |
| Input per 1M tokens | $5.00 | $3.00 | Opus er 67 % dyrere |
| Output per 1M tokens | $25.00 | $15.00 | Opus er 67 % dyrere |
| Cached input per 1M tokens | $0.50 | $0.30 | Opus er 67 % dyrere |
| Typisk interaksjon (3K inn / 5K ut) | $0.14 | $0.084 | $0.056 mer per kall |
| 50 kall per dag | $7.00 | $4.20 | $2.80 spart per dag |
| Per bruker per måned (50 kall/dag) | ~$140 | ~$84 | ~$56 spart |

Med 600 brukere og et snitt på 10 kall per dag: Opus koster ca. $28/bruker/mnd — det er mer enn hele Business-kvoten på $19.

Med model-pinning styrer vi hvilken modell hver agent og prompt bruker, ut fra hva oppgaven krever. For de fleste oppgaver er Sonnet eller billigere modeller mer enn godt nok.

## Hva vi har gjort

Alle 12 agenter og 7 prompts i `.github/agents/` og `.github/prompts/` har fått `model:` i YAML-frontmatter. Modellvalget er gjort ut fra benchmarks og oppgavetype:

| Modell | Agenter/prompts | Pris (input/output per 1M tokens) | Begrunnelse |
| --- | --- | --- | --- |
| Claude Opus 4.6 | nav-pilot, security-champion | $5 / $25 | Agentic planning, 83 % OWASP-recall |
| GPT-5.3-Codex | code-review, rust, kafka, nais, kafka-topic, nais-manifest | $1.75 / $14 | Terminal-Bench 75 %, KubeBench-leder |
| Claude Sonnet 4.6 | forfatter | $3 / $15 | Best på norsk språk |
| Claude Haiku 4.5 | aksel, accessibility, auth, observability + 5 scaffold-prompts | $1 / $5 | Sjekklister og maler, 73 % SWE-bench |
| Gemini 2.5 Pro | research | $1.25 / $10 | Best reasoning per krone |

## Hva vi sparer

Den enkleste besparelsen: bruk Auto i stedet for å velge Opus manuelt.

| Scenario | Kostnad per interaksjon | Spart vs. Opus |
| --- | --- | --- |
| Opus for alt (brukervalg) | $0.14 | — |
| Auto-modus (med rabatt) | ~$0.06–$0.08 | ~40–55 % |
| Med pinning (riktig modell per oppgave) | $0.03–$0.08 | ~40–80 % |

Model-pinning gir mest for agenter og prompts der vi vet at en billigere modell holder. For vanlig chat er Auto det beste valget.

Promokreditter juni–august gir 3 000 credits per bruker i stedet for 1 900. Det gir Nav tid til å justere bruken.

## Slik holder du forbruket nede

### 1. Bruk Auto — ikke Opus for alt

Auto-modus har innebygd rabatt og velger riktig modell for oppgaven. Trenger du Opus, bruk agenter som `@nav-pilot` eller `@security-champion` — de har Opus pinnet fordi oppgavene krever det. For vanlig chat og kodearbeid er Auto eller Sonnet like bra til langt lavere pris.

### 2. Bruk caching — hold sesjonen åpen

Cached tokens koster **10 % av vanlige input-tokens**. Copilot cacher kontekst fra tidligere i sesjonen automatisk. Eksempel med GPT-5.3-Codex:

- Første spørring (10K input-tokens): $0.0175
- Neste spørring i samme sesjon (90 % cache hit): $0.0033

Lukker du sesjonen og starter på nytt, betaler du full pris igjen. Hold sesjoner åpne når du jobber med relaterte oppgaver.

### 3. Kodekomplettering er gratis

Ghost text, tab-completions og next edit suggestions bruker **ikke** AI Credits. Bruk dette mest mulig.

### 4. Context engineering i CLI

Copilot CLI og OpenCode laster inn AGENTS.md, instruksjonsfiler og åpne filer som kontekst — alt teller som input-tokens. Slik holder du konteksten kompakt:

**Copilot CLI:**

- `AGENTS.md` og `.github/copilot-instructions.md` lastes i sin helhet for hvert kall. Hold dem korte og relevante.
- Scoped instructions (`.github/instructions/*.instructions.md` med `applyTo:`) lastes bare når du jobber med matchende filer. Bruk dette i stedet for å legge alt i den globale filen.
- Kompakte checkpoints: CLI-en komprimerer kontekst automatisk i lange sesjoner, men du kan hjelpe ved å starte ny sesjon når du bytter oppgave.

**OpenCode:**

- `AGENTS.md` i prosjektroten lastes som kontekst. Kjør `/init` for å generere en tilpasset versjon — den skanner prosjektet og lager et kompakt sammendrag.
- Personlige regler i `~/.config/opencode/AGENTS.md` lastes alltid — hold den minimal.
- `/compact` komprimerer samtalehistorikken manuelt hvis sesjonen blir lang.

**Felles prinsipper:**

- Skriv instruksjoner som stikkord og lister, ikke lange avsnitt. LLM-er forstår konsise regler like godt.
- Fjern utdaterte eller overlappende instruksjoner — de bruker tokens uten å gi verdi.
- Bruk `applyTo:` (Copilot) eller separate agent-filer (OpenCode) for å avgrense kontekst til relevante filer.
- Husk: 2 000 tegn ≈ 350 tokens. En AGENTS.md på 10 000 tegn koster ca. $0,006 i input per kall med Sonnet.

### 5. Gi presis kontekst

Mindre kontekst = færre input-tokens = lavere kostnad. Konkret:

- Åpne bare filene du jobber med (Copilot sender åpne filer som kontekst)
- Skriv spesifikke spørsmål i stedet for «fiks koden min»
- Bruk `@workspace` bare når du trenger søk på tvers av prosjektet

### 6. Sjekk forbruket ditt

- **I VS Code**: Copilot-ikonet i statuslinjen → «View quota usage»
- **På GitHub.com**: Settings → Billing → Metered usage
- **Som admin**: Billing & licensing viser forbruk per bruker og team

### 7. Unngå unødvendig agentbruk

Agenter bruker flere tokens enn vanlig chat fordi de gjør flere kall. Spør deg:

- Trenger jeg en agent, eller holder et enkelt chat-spørsmål?
- Kan jeg løse dette med kodefullføring (code completion) i stedet?
- Er dette en «grønn sone»-oppgave der AI gir mest verdi?

## Hva vi planlegger videre

Model-pinning er første steg. Framover ser vi på:

- **Budsjetter per team:** Kredittgrenser per team, slik at forbruket fordeles rettferdig og ingen får overraskelser.
- **Personlige budsjetter:** Veiledende kredittbudsjett per bruker, med varsling når du nærmer deg grensa.
- **Forbruksoversikt:** Dashbord på min-copilot.ansatt.nav.no som viser hvem som bruker mest. Ikke for å henge ut noen, men for å lære av hverandre.
- **Erfaringsdeling fra storforbrukere:** De som bruker mest AI Credits, blir invitert til å dele hva de jobber med og hvordan de bruker agenter. Målet er å spre gode arbeidsmønstre.

Vi tror de mest aktive brukerne har verdifull innsikt i hva som fungerer — og hva som ikke gjør det. Den innsikten vil vi gjøre tilgjengelig for alle.

## Hva du merker

Ingenting — `model:` setter default, men du kan fortsatt velge modell manuelt i model picker. I juni oppgraderer vi til Opus 4.7 — da koster alle Opus-modeller det samme per token.

**Bruk [`@nav-pilot`](https://min-copilot.ansatt.nav.no/nav-pilot) for tunge oppgaver.** Nav-pilot er vår primæragent for arkitektur, planlegging og implementering. Den bruker model-pinning med Opus 4.6 og GPT-5.3-Codex som fallback — du får altså den kraftigste modellen automatisk når du trenger den, uten å velge Opus manuelt for alt annet. For vanlig chat og kodearbeid: bruk Auto.

**Kilder:**

- [Copilot model pinning — issue #216](https://github.com/navikt/copilot/issues/216) (navikt/copilot, mai 2026)
- [Models and pricing for GitHub Copilot](https://docs.github.com/en/copilot/reference/copilot-billing/models-and-pricing) (GitHub Docs)
- [GitHub Copilot is moving to usage-based billing](https://github.blog/news-insights/company-news/github-copilot-is-moving-to-usage-based-billing/) (GitHub Blog, april 2026)
