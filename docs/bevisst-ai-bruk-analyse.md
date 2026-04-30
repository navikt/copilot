# Bevisst AI-bruk: Analyse og tiltak for kompetansebevaring

> Analyse av hvordan navikt/copilot-repoet påvirker utviklerkompetanse, basert på forskning og Navs egen utviklerundersøkelse 2026.

## Bakgrunn

Navs utviklerundersøkelse 2026 avdekket tydelige bekymringer:

- **59 %** av utviklerne er bekymret for at AI svekker dyp teknisk forståelse
- **Kun 34 %** mener AI-generert kode holder god nok kvalitet til å passere code review uten ekstraarbeid
- **#1 ønske** er mer opplæring og veiledning i effektiv bruk av AI-verktøy

Nav IT har også forsket på dette direkte: en longitudinell studie (Stray et al., HICSS-59 2026) med 26 317 commits fra 703 repoer fant **ingen statistisk signifikant produktivitetsøkning** for Copilot-brukere, til tross for sterk *opplevd* gevinst. Dette bekrefter at vi må fokusere på kvalitet og kompetanse, ikke bare hastighet.

## Forskning: Hvordan du bruker AI betyr mer enn om du bruker det

### Anthropic RCT (2026)

Studie med 52 ingeniører som lærte ny teknologi (Trio-biblioteket i Python).

**Nøkkelfunn:**

| Interaksjonsmønster | Forståelsesscore | Hastighet |
| --- | --- | --- |
| Full delegering til AI | 35–39 % | Raskest |
| Iterativ AI-debugging | 24 % | Tregest |
| Hybrid kode + forklaring | 68 % | Middels |
| Konseptuell utforskning | 65 % | Middels |
| **Generer-så-forstå** | **86 %** | Litt tregere |
| Uten AI (kontrollgruppe) | 67 % | Middels |

Generer-så-forstå-mønsteret — der utvikleren lar AI generere kode og deretter aktivt stiller spørsmål om *hvorfor* — scorer høyere enn å kode helt uten AI.

**Kilde:** [How AI assistance impacts the formation of coding skills](https://www.anthropic.com/research/AI-assistance-coding-skills)

### METR-studie (2025)

Erfarne open source-utviklere var **19% tregere** med AI, men **estimerte at de var 20% raskere** — et gap på 39 prosentpoeng mellom opplevd og faktisk produktivitet.

**Kilde:** [Early 2025 AI experienced OS dev study](https://metr.org/blog/2025-07-10-early-2025-ai-experienced-os-dev-study/)

### INNOQ: Kognitiv lastteori (2026)

Forklarer resultatene gjennom kognitiv lastteori:

- **Full delegering** fjerner all kognitiv belastning — inkludert den *produktive* belastningen som bygger forståelse
- **Generer-så-forstå** frigjør kapasitet fra syntaks/boilerplate (ekstranøs last) og bruker den på å bygge mentale modeller (germane last)
- **AI-debugging** er verst fordi det outsourcer den kognitive prosessen som bygger feilsøkingskompetanse

**Kilde:** [Understanding AI Coding Patterns Through Cognitive Load Theory](https://www.innoq.com/en/blog/2026/03/ai-cognitive-lens-cognitive-load-theory/)

### Nav IT longitudinell studie (HICSS-59, 2026)

Studie utført i Nav av Viktoria Stray et al. med 26 317 commits fra 703 repoer over 6 måneder. 25 Copilot-brukere ble sammenlignet med 14 ikke-brukere.

**Nøkkelfunn:**

- Ingen statistisk signifikant forskjell i commit-basert produktivitet mellom Copilot-brukere og kontrollgruppen
- Sterk *opplevd* produktivitetsgevinst — brukerne føler seg raskere, men målingene viser det ikke
- Validerer METR-studiens funn om gap mellom opplevd og faktisk produktivitet, nå med Nav-spesifikke data
- Viser at kvantitative metrikker (commits, linjer) ikke fanger hele bildet — kvalitative gevinster er reelle

**Implikasjon for rammeverket:** Støtter at vi bør fokusere på *kvalitativ kompetansebygging* (grønn/rød sone) heller enn ren hastighetsoptimalisering.

**Kilde:** [Developer Productivity With and Without GitHub Copilot](https://arxiv.org/abs/2509.20353)

### MIT/Microsoft/Accenture RCT (2025)

Storskala randomisert kontrollert studie med ~5000 utviklere hos Microsoft og Accenture.

**Nøkkelfunn:**

- 26 % flere PRs/uke for utviklere med AI-tilgang
- Juniorutviklere fikk størst gevinst — erfarne utviklere nådde et platå
- Kvaliteten på koden (målt via bugs og reverts) var uendret

**Implikasjon for rammeverket:** Juniorutviklere er mest sårbare for kognitiv offloading — de har mest å vinne produktivitetsmessig, men også mest å tape kompetansemessig. Grønn/rød sone bør tilpasses erfaringsnivå.

**Kilde:** [The Effects of Generative AI on High-Skilled Work](https://economics.mit.edu/sites/default/files/inline-files/draft_copilot_experiments.pdf)

### Faros AI-rapport (2025)

PR merge rates økte, men review-tid økte 91%. Total produktivitet på selskapsnivå var uendret — raskere generering, tregere verifisering.

**Kilde:** [The State of AI in Software Engineering](https://www.faros.ai/blog/ai-software-engineering)

## Analyse av navikt/copilot

### Styrker (kompetansebevarende)

Repoet har et sofistikert undervisningslag:

- **nav-pilot**: Obligatoriske 4-faser (intervju → plan → review → lever) med eksplisitte stopp
- **code-review**: «Teach, don't just flag» — forklarer *hvorfor* hvert funn er viktig
- **security-owasp**: 850+ linjer med ✅/❌-mønstre inkludert angrepsforklaringer
- **36 filer** med eksplisitte boundary-seksjoner (Always / Ask First / Never)
- **15:1 ratio** av undervisningsinnhold til ren kodegenerering

### Svakheter (kompetanseeroderende)

Før disse endringene hadde repoet blinde flekker:

1. **Alle 7 prompt-maler** inneholdt null forklaringer av *hvorfor* — ren kodegenerering
2. **Ingen omtale** av kompetansebevaring, AI-frie soner, eller bevisst tenkning
3. **Ingen «rød sone»-markører** — ingenting fortalte utviklere «denne typen arbeid bør gjøres manuelt»
4. **Survey-innsikt** (59% bekymring) var frakoblet fra verktøyene

## Tiltak implementert

### 1. Ny instruksjon: `deliberate-ai-use.instructions.md`

Global instruksjon som definerer:

- **🟢 Grønn sone** — AI-egnet: boilerplate, kjent teknologi, konfigurasjon, refaktorering
- **🔴 Rød sone** — kode manuelt: debugging, nye konsepter, kjernelogikk, sikkerhet
- **Tre-forsøks-regelen**: prøv selv før du ber AI om hjelp
- **Generer-så-forstå-mønsteret**: generer → forstå → verifiser → tilpass

### 2. «Forstå koden»-seksjoner i alle 7 prompt-maler

Hver prompt avsluttes nå med instruksjoner til AI om å forklare:

- Arkitektoniske valg og *hvorfor* dette mønsteret
- Tradeoffs — hva du vinner og gir avkall på
- Feilmodi — hva som kan gå galt
- 🔴 Rød-sone-markører på sikkerhet og kjernelogikk

### 3. Agentoppdateringer

- **nav-pilot**: Ny «Kompetansebevaring»-rad i blinde-flekker-tabellen + nye Always-regler
- **code-review**: Ny «AI-generert kode»-sjekk som verifiserer at utvikleren forstår designbeslutningene

## Videre arbeid

- Måle effekten i neste utviklerundersøkelse (2027)
- Vurdere «AI-frie blokker» som praksis i teamene
- Koble survey-resultatene tettere til verktøyutviklingen
- Vurdere en dedikert «deliberate practice»-skill for AI-fri koding

## Kilder

- [Anthropic: How AI assistance impacts coding skills](https://www.anthropic.com/research/AI-assistance-coding-skills) (2026)
- [INNOQ: AI Coding Patterns Through Cognitive Load Theory](https://www.innoq.com/en/blog/2026/03/ai-cognitive-lens-cognitive-load-theory/) (2026)
- [METR: Early 2025 AI experienced OS dev study](https://metr.org/blog/2025-07-10-early-2025-ai-experienced-os-dev-study/) (2025)
- [Stray et al.: Developer Productivity With and Without GitHub Copilot — A Longitudinal Mixed-Methods Case Study](https://arxiv.org/abs/2509.20353) (HICSS-59, 2026) — **Studie gjort i Nav IT**
- [MIT/Microsoft/Accenture: The Effects of Generative AI on High-Skilled Work](https://economics.mit.edu/sites/default/files/inline-files/draft_copilot_experiments.pdf) (2025) — RCT med ~5000 utviklere
- [AgentPatterns.ai: Skill Atrophy](https://agentpatterns.ai/human/skill-atrophy/)
- [ACM CACM: The AI Deskilling Paradox](https://cacm.acm.org/news/the-ai-deskilling-paradox/) (2025)
- [Faros AI: The State of AI in Software Engineering](https://www.faros.ai/blog/ai-software-engineering) (2025)
- Nav utviklerundersøkelsen 2026 (intern)
