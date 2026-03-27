# Spørreundersøkelse: AI-kodeverktøy i Nav (2026)

## Bakgrunn

Spørreundersøkelse for ~500 teknologer i Nav. Målet er å forstå hvordan folk bruker AI-kodeverktøy, hva de bruker dem til, og hvor AI-agenter gir eller ikke gir verdi. GitHub Copilot er det godkjente AI-kodeverktøyet i Nav, tilgjengelig i flere miljøer. Undersøkelsen tar ~5 minutter (12 spørsmål).

### Teoretisk grunnlag

- **SPACE-rammeverket** (Forsgren, Storey, Maddila, Zimmermann & Noda, 2021) — Satisfaction, Performance, Activity, Communication, Efficiency. Vi dekker fire av fem dimensjoner direkte (S, P, A, E). Communication dekkes indirekte gjennom spørsmålet om code review.
- **Seksfaktormodellen fra «Beyond the Commit»** (Chen et al., ICSE-SEIP 2026) — Self-sufficiency, Cognitive load, Task completion, Peer review, Long-term expertise, Ownership. Fem av seks er Likert-spørsmål (Q4–Q9). Self-sufficiency er slått sammen med Task completion.

### Designprinsipper

- Maks 12 spørsmål, ~5 minutter
- Skip-logikk for ikke-brukere (Q1–Q2 + Q9–Q12: 5 obligatoriske + 1 valgfritt)
- Ett valgfritt fritekstspørsmål
- Anonyme svar
- Resultatene deles med deltakerne

---

## Spørsmål

### Del 1: Profil (segmentering)

**Q1.** Hvor mange års erfaring har du som teknolog?

- 0–2
- 3–5
- 6–10
- 11+

**Q2.** Hvilke AI-kodemiljøer bruker du i dag? *(velg alle som passer)*

- Copilot i VS Code (completions, chat, agent mode)
- Copilot i IntelliJ / JetBrains
- Copilot på github.com (PR-oppsummeringer, code review m.m.)
- Copilot CLI (terminal)
- GitHub Copilot Extensions / MCP-servere
- Claude Code (Anthropic terminal-agent)
- OpenCode (open source terminal-agent)
- Annet: ___
- **Jeg bruker ikke AI-kodeverktøy** → *hopp til Q9*

---

### Del 2: Bruksmønster (bare for de som bruker AI-verktøy)

**Q3.** Hvor gir AI-kodeverktøy deg mest verdi i det daglige? *(velg opptil 3)*

- Code completions / kodegenerering
- Forklare eller forstå eksisterende kode
- Skrive tester
- Feilsøking
- Refaktorering
- Skrive dokumentasjon og kommentarer
- Hjelp med code review
- Generere boilerplate / scaffolding
- Lære nye språk, rammeverk eller API-er
- Delegere flerstegoppgaver til en autonom agent
- Annet: ___

---

### Del 3: Effekt — tilfredshet og seksfaktormodellen (5-punkts Likert-skala)

Svaralternativer: Helt uenig / Uenig / Nøytral / Enig / Helt enig

**Q4. Tilfredshet (SPACE-S):** «Alt i alt er jeg fornøyd med AI-kodeverktøyene vi har tilgang til i Nav.»

**Q5. Kognitiv belastning:** «AI-kodeverktøy reduserer mental innsats på repetitive oppgaver og boilerplate, slik at jeg kan fokusere på vanskeligere problemer.»

**Q6. Oppgavegjennomføring:** «AI-kodeverktøy hjelper meg å komme videre når jeg står fast, og fullføre oppgaver raskere enn uten.»

**Q7. Code review:** «Kode jeg lager med AI-hjelp holder god nok kvalitet til at den ikke skaper ekstra arbeid i code review.»

**Q8. Teknisk kompetanse:** «Jeg er bekymra for at AI-verktøy kan svekke min egen dype forståelse av koden og teknologiene jeg jobber med.»

Reversert skåring — fanger opp bekymring rundt langsiktig kompetanseutvikling.

**Q9. Eierskap:** «Jeg er trygg på å ta fullt ansvar for kode som er generert eller vesentlig hjulpet av AI.»

**Q10. Juss og sikkerhet:** «Usikkerhet rundt personvern eller interne sikkerhetsregler hindrer meg i å bruke AI-kodeverktøy fullt ut i det daglige arbeidet.»

Reversert skåring — høy enighet peker på en barriere vi kan gjøre noe med gjennom tydeligere retningslinjer og bedre verktøystøtte.

---

### Del 4: Barrierer og muligheter (alle respondenter)

**Q11.** Hvis du kunne endra én ting med AI-kodeverktøyene i Nav, hva ville det vært? *(velg én)*

- Bedre forståelse av kodebasen vår og interne rammeverk
- Mer opplæring og veiledning i effektiv bruk
- Tydeligere retningslinjer for sikkerhet og personvern
- Færre begrensninger på tilgang til verktøy
- Støtte for flere AI-verktøy eller miljøer
- Ingenting — jeg er fornøyd med dagens situasjon
- Jeg foretrekker å kode uten AI
- Annet: ___

**Q12.** *(Valgfritt, fritekst)* Hva er din mest minneverdige opplevelse — positiv eller negativ — med AI-kodeverktøy, og hva ville gjort dem mer nyttige?

---

## Supplerende metoder

1. **Semistrukturerte intervjuer** — 5–8 utviklere, bevisst utvalgt fra ytterpunktene: de som bruker agenter aktivt og har funnet gode arbeidsmåter, de som ikke bruker AI, og seniorer med dyp kjennskap til kodebasen. Temaer som er for nyanserte for en spørreundersøkelse:
   - **Mestring** — Hvordan beholder utviklere mestringsfølelse og faglig vekst når arbeidet skifter mot å vurdere AI-generert kode framfor å skrive sjøl?
   - **Teknisk gjeld** — Hjelper AI med å betale ned teknisk gjeld, eller gir det mer kode som er vanskeligere å vedlikeholde?
   - **AI-fatigue** — Er skiftet fra å skrive kode til å styre og vurdere AI-output energigivende eller slitsomt?
   - **Utviklingsfaser** — Hvor i utviklingsløpet (design, implementasjon, testing, vedlikehold) gir AI-verktøy verdi, og hvor gjør de skade?
   - **Spre det som funker** — Hva skiller utviklere som får mye ut av agenter (vaner, repo-oppsett, arbeidsmåter), og hvordan kan vi overføre det til resten?
2. **API-metrikker** — Koble Copilot-bruksdata (aksepteringsrate, aktive brukere, frekvens) mot svar fra undersøkelsen. Bruksfrekvens ble droppa fra undersøkelsen fordi vi måler det via API-et.
3. **Lukke løkka** — Dele resultater og tiltak med alle deltakerne.

## Referanser

- Chen et al., «Beyond the Commit: Developer Perspectives on Productivity with AI Coding Assistants», ICSE-SEIP 2026 (arxiv.org/abs/2602.03593) — Kilde for seksfaktormodellen. Q5–Q9 dekker fem av seks faktorer; self-sufficiency er slått sammen med task completion (Q6). Resultatene kan sammenlignes direkte med Chen et al.
- Forsgren, Storey, Maddila, Zimmermann & Noda, «The SPACE of Developer Productivity», ACM Queue, 2021 — Rammeverk for utviklerproduktivitet i fem dimensjoner. Q4 dekker Satisfaction; Activity og Efficiency dekkes av Q3 og Q6.
- GitHub/Accenture Enterprise Copilot Study, github.blog, 2024 — Kontekst for benchmarking. Tilnærminga med å kombinere API-metrikker og spørreundersøkelser inspirerte «Supplerende metoder»-seksjonen vår.
- Australian Government M365 Copilot Trial, digital.gov.au, 2024 — Designmønstre for spørreundersøkelser (Likert-skalaer, frekvensmatriser, pre/post-struktur) tilpassa vår kontekst. Merk: studien gjaldt M365 Copilot (kontorverktøy), ikke kodeverktøy.
- Stack Overflow Developer Survey 2025, survey.stackoverflow.co/2025/ — Kontekst for adopsjonsrater og bruksmønster. Q3 bygger på AI-seksjonen deres; adopsjonsrater fra Q2 kan sammenlignes med de globale tallene.
