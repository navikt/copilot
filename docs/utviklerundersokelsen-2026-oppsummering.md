# Oppsummering: Utviklerundersøkelsen 2026

> Nøkkelfunn fra spørreundersøkelsen «AI-kodeverktøy i Nav» (mars 2026). 163 respondenter av ~500 inviterte teknologer.

## Respondentprofil

- **163 svar** (svarprosent ~33 %)
- 57 % har 11+ års erfaring, 27 % har 6–10 år
- 93 % bruker AI-kodeverktøy aktivt (12 av 163 gjør det ikke)
- 74 % bruker to eller flere AI-verktøy parallelt

### Mest brukte verktøy

| Verktøy | Brukere | Andel |
|---------|---------|-------|
| Copilot (github.com) | 95 | 58 % |
| Copilot (IntelliJ) | 88 | 54 % |
| Copilot CLI | 86 | 53 % |
| Copilot (VS Code) | 54 | 33 % |
| Extensions / MCP | 25 | 15 % |
| Claude Code | 22 | 13 % |

---

## Hovedfunn

### Det som fungerer

| Påstand | Enig/Helt enig | Andel |
|---------|---------------|-------|
| Hjelper meg fullføre raskere | 122 | 75 % |
| Fornøyd med verktøyene | 119 | 73 % |
| Reduserer kognitiv belastning | 109 | 67 % |
| Eierskap til AI-generert kode | 101 | 62 % |

### Det som bekymrer

| Påstand | Enig/Helt enig | Andel |
|---------|---------------|-------|
| Bekymret for kompetansetap | 96 | 59 % |
| AI-kode god nok for review | 56 | 34 % |

### Spenningsfeltet

Dataen viser et tydelig paradoks:
- **75 % opplever at AI hjelper dem jobbe raskere** — men Navs egen longitudinelle studie (Stray et al., HICSS-59 2026, 26 317 commits) fant *ingen statistisk signifikant produktivitetsøkning*
- **59 % er bekymret for kompetanseeffekter** — og Anthropics RCT (2026) bekrefter at bruksmønsteret avgjør om kompetansen styrkes eller svekkes
- **Kun 34 % mener AI-kode holder til review** — signaliserer at kvalitetssjekker (sensors i harness-termer) oppfattes som utilstrekkelige

---

## Bruksmønstre: Hvor gir AI mest verdi?

Respondentene valgte opptil 3 områder:

| Bruksområde | Antall | Andel |
|-------------|--------|-------|
| Forstå eksisterende kode | 78 | 48 % |
| Code completions | 70 | 43 % |
| Feilsøking | 66 | 40 % |
| Skrive tester | 47 | 29 % |
| Hjelp med code review | 40 | 25 % |
| Refaktorering | 39 | 24 % |
| Generere boilerplate | 28 | 17 % |
| Lære nye språk / API-er | 23 | 14 % |
| Delegere til autonom agent | 21 | 13 % |
| Dokumentasjon | 18 | 11 % |

**Merk:** De tre mest verdifulle bruksområdene (forstå kode, completions, feilsøking) handler om *forståelse og navigering*, ikke kodegenerering.

---

## Hva ønsker utviklerne endret?

| Ønske | Antall | Andel |
|-------|--------|-------|
| Bedre opplæring | 50 | 31 % |
| Bedre forståelse av kodebase/rammeverk | 24 | 15 % |
| Flere AI-verktøy/miljøer | 21 | 13 % |
| Bedre sikkerhet/personvern | 21 | 13 % |
| Fornøyd som det er | 16 | 10 % |
| Foretrekker uten AI | 15 | 9 % |

**#1 ønske er opplæring** — dette understøtter tiltak som grønn/rød sone-rammeverket og generer-så-forstå-mønsteret.

---

## Kobling til harness-arbeidet

Undersøkelsen gir direkte input til harness-utviklingen:

| Funn | Harness-implikasjon |
|------|---------------------|
| 59 % bekymret for kompetansetap | Governance-laget (bevisst AI-bruk) er riktig prioritert |
| 34 % mener AI-kode holder til review | Inferential sensors (code-review-agenter) bør tettere integreres |
| 31 % ønsker bedre opplæring | Inferential guides (agenter som forklarer *hvorfor*) har riktig retning |
| 48 % bruker AI til å forstå kode | Generer-så-forstå-mønsteret treffer reelt behov |

---

## Metodikk

- **Design:** 12 spørsmål, ~5 minutter, anonym
- **Teoretisk grunnlag:** SPACE-rammeverket (Forsgren et al., 2021) + seksfaktormodellen (Chen et al., ICSE-SEIP 2026)
- **Skala:** 5-punkt Likert (Helt enig → Helt uenig)
- **Populasjon:** ~500 teknologer i Nav
- **Detaljert undersøkelsesdesign:** Se [ai-coding-engagement-survey-2026.md](ai-coding-engagement-survey-2026.md)
