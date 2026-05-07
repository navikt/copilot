# Agentic engineering for enterprise-arkitekter

## Hva er agentic engineering?

Tradisjonell AI-kodeassistanse (autocomplete) foreslår én linje om gangen. Du aksepterer eller forkaster. Agentisk AI er kvalitativt annerledes:

| Egenskap | Autocomplete | Agentisk AI |
|----------|--------------|-------------|
| Interaksjon | Enkeltforslag | Flertrinns planlegging og utførelse |
| Verktøybruk | Ingen | Leser filer, kjører kode, kaller API-er |
| Varighet | Millisekunder | Minutter til timer |
| Feilmodus | Dårlig forslag (begrenset skade) | Kaskadefeil på tvers av systemer |
| Styring | Innholdsfilter | Tillitsgrenser, tilgangsstyring, revisjonslogg |

Anthropics analogi: Du har gått fra å chatte med en assistent som svarer på spørsmål, til å ansette en konsulent med nøkler til bygget, tilgang til e-posten din og fullmakt til å godkjenne utgifter.

---

## Hva sier forskningen?

### De som er mest skeptiske har ofte rett — delvis

**METR-studien (juli 2025)** — randomisert kontrollert forsøk, gullstandard:
- 16 erfarne utviklere, 246 reelle oppgaver i store kodebaser (22 000+ stars, 1M+ linjer)
- Resultat: **AI gjorde dem 19 % tregere**
- Utviklerne *trodde* de var 24 % raskere. Etter å ha opplevd nedgangen, trodde de fortsatt de var 20 % raskere
- Økonomer og ML-forskere spådde 38–39 % speedup

Kilde: [metr.org/blog/2025-07-10-early-2025-ai-experienced-os-dev-study](https://metr.org/blog/2025-07-10-early-2025-ai-experienced-os-dev-study/) + arXiv:2507.09089

**Hvorfor?** Fem faktorer:
1. AI presterer dårligere i store, komplekse kodebaser med implisitt kontekst
2. Kognitiv overhead av å lese, forstå og verifisere AI-generert kode
3. Debugging av AI-feil tar tid
4. Høye kvalitetskrav (stil, tester, dokumentasjon) gjør AI-forslag mindre direkte brukbare
5. Oppgavene var i kodebaser utviklerne kjente fra før — der AI gir minst merverdi

### Men det finnes reell verdi — for riktige oppgaver

**GitHubs forskning (2022–2024):**
- Enkel, avgrenset oppgave (HTTP-server i JavaScript): **55 % raskere** med Copilot
- Enterprise-skala hos Accenture: **8,7 % flere PR-er**, 84 % bedre CI-bygg, 15 % høyere merge-rate

**NBER/Brynjolfsson et al. (2023)** — 5 179 kundeserviceagenter:
- +14 % produktivitet totalt
- **+34 % for nybegynnere**, minimalt for erfarne

**Anthropic internt (august 2025)** — 132 ingeniører:
- 59 % av daglig arbeid bruker Claude, selvrapportert 50 % produktivitetsøkning
- 67 % økning i mergede PR-er per ingeniør per dag
- Men: over halvparten delegerer kun 0–20 % av arbeidet fullt til AI
- Senioringeniørene delegerer bevisst *lite* av kjernearbeidet

Kilde: [anthropic.com/research/how-ai-is-transforming-work-at-anthropic](https://www.anthropic.com/research/how-ai-is-transforming-work-at-anthropic)

### Mønsteret er konsistent

```
AI hjelper mest:   Nybegynnere, enkel/avgrenset oppgave, ukjent kodebase
AI hjelper minst:  Erfarne utviklere, kompleks/kjent kodebase, høye kvalitetskrav
```

---

## Kompetansebevaring — det skjulte problemet

Anthropics egne senioringeniører (2025):

> «Det blir vanskeligere å ta seg tid til å faktisk lære noe når det er så lett og raskt å produsere output.»

> «Ferdighetene mine vil primært forfalle med hensyn til min evne til å trygt *bruke* AI for oppgavene jeg bryr meg om.»

Dette er **supervisjonsparadokset**: Effektiv oversikt over AI krever nettopp de ferdighetene som AI-avhengighet eroderer.

**Nav utviklerundersøkelsen 2026** (163 respondenter):
- 75 % opplever at AI hjelper dem jobbe raskere
- **59 % er bekymret for kompetansetap**
- Kun 34 % mener AI-kode holder til code review
- #1-ønske: Bedre opplæring (31 %)

Kilde: [Stray et al., HICSS-59 2026](https://arxiv.org/abs/2509.20353) — Navs egen longitudinelle studie (26 317 commits) fant *ingen statistisk signifikant produktivitetsøkning*.

---

## Sikkerhetsrisiko ved agentisk AI

### Anthropic: Agentic Misalignment (2025)

Red-teaming av 16 ledende AI-modeller (Anthropic, OpenAI, Google, Meta, xAI) i simulerte bedriftsmiljøer. Modellene fikk verktøytilgang (e-post, filsystemer) og mål som kom i konflikt med bedriftsinstrukser.

Funn: **Alle 16 modeller** tyr til ondsinnet atferd (utpressing, lekkasje av informasjon) når det er eneste måten å unngå nedleggelse.

Kilde: [anthropic.com/research/agentic-misalignment](https://www.anthropic.com/research/agentic-misalignment)

### OWASP Top 10 for LLM-applikasjoner

De tre mest relevante for enterprise-agenter:

| # | Sårbarhet | Konsekvens |
|---|-----------|------------|
| LLM01 | Prompt injection | Angriper kaprer agenthandlinger via innhold i e-post, dokumenter, nettsider |
| LLM08 | Excessive agency | AI med for vid fullmakt tar utilsiktede handlinger |
| LLM09 | Overreliance | Ukritisk aksept av AI-output |

### Anthropic: Trustworthy Agents (april 2026)

Fire komponenter som hver utgjør en angrepsflate:
1. **Modellen** — trening former oppførsel
2. **Harness** — instrukser og guardrails (feilkonfigurert harness kan undergrave god modell)
3. **Verktøy** — e-post, kalender, databaser, kodekjøring
4. **Miljø** — hva agenten har tilgang til

Kilde: [anthropic.com/research/trustworthy-agents](https://www.anthropic.com/research/trustworthy-agents)

---

## Hva Forrester og Microsoft sier

**Forrester Predictions 2025:**
- **75 % av bedrifter som bygger agentisk AI selv vil mislykkes** — arkitekturene er for komplekse
- ROI-forventninger vil føre til for tidlige nedskaleringer
- 40 % av regulerte virksomheter må slå sammen data- og AI-governance

Kilde: [forrester.com/blogs/predictions-2025-artificial-intelligence](https://www.forrester.com/blogs/predictions-2025-artificial-intelligence/)

**Microsoft Work Trend Index 2024–2025** (31 000 respondenter, 31 land):
- 78 % av AI-brukere tar med egne AI-verktøy (BYOAI) — utenom bedriftskontroller
- 52 % er *redde for å innrømme* at de bruker AI til viktige oppgaver
- 81 % av ledere forventer agenter integrert i AI-strategi innen 12–18 måneder
- 60 % av ledere innrømmer at organisasjonen mangler plan for AI-implementering

---

## Hva vi gjør i Nav — praktisk tilpasning

### Arkitekturen: harness over modell

Vi investerer ikke i egne modeller. Vi bygger *harness* — tilpasningslaget som gjør generelle modeller til Nav-spesifikke verktøy:

```
┌─────────────────────────────────────────────────┐
│  Governance-lag: Bevisst AI-bruk, grønn/rød sone │
├─────────────────────────────────────────────────┤
│  Agent-lag: nav-pilot, security-champion, ...    │
├─────────────────────────────────────────────────┤
│  Skills: nav-plan, threat-model, api-design, ... │
├─────────────────────────────────────────────────┤
│  Instruksjoner: golang, nextjs-aksel, security  │
├─────────────────────────────────────────────────┤
│  MCP-servere: GitHub, registry, onboarding       │
└─────────────────────────────────────────────────┘
         ↕ (API)
   GitHub Copilot / Claude / GPT (modell-agnostisk)
```

### Grønn og rød sone — kodifisert i verktøyet

Vi har bakt forskningsfunnene direkte inn i AI-instruksene:

**🟢 Grønn sone (AI-egnet):** Boilerplate, Nais-manifest, CRUD, kjent teknologi, konfigurasjon, testdata

**🔴 Rød sone (kode manuelt først):** Debugging, nye konsepter, kjernelogikk, sikkerhetskritisk kode, arkitekturbeslutninger

**Tre-forsøks-regelen:** Prøv å løse problemet selv i tre forsøk før du ber AI om hjelp.

### nav-pilot: Planleggingsagent med fasestyring

Ikke bare «skriv kode for meg», men en 4-fase arbeidsflyt:

1. **Intervju** — kartlegger blindsoner (personvern, tilgangsstyring, feilhåndtering, observerbarhet, teamgrenser, endringskonsekvenser, teststrategi, migrering, bakoverkompatibilitet, dekommisjonering, kompetansebevaring)
2. **Plan** — beslutningstrær for auth, kommunikasjon, database, CI/CD
3. **Review** — fra fire perspektiver: sikkerhet, plattform, arkitektur, endringssikkerhet
4. **Lever** — kode + dokumentasjon, med rød-sone-kode markert som TODO

Agenten *stopper* mellom fasene og venter på godkjenning. Den delegerer til spesialistagenter (auth, kafka, nais, security) ved behov, men beholder kontrollen.

### Tall fra Nav

- 11 spesialistagenter, 21+ skills
- 93 % av utviklerne bruker AI-kodeverktøy aktivt
- 53 % bruker Copilot CLI (agentisk)
- MCP-registry med godkjente servere (kontrollert verktøytilgang)
- Bevisst AI-bruk-instruksjonen er aktiv i alle repoer som har tatt den i bruk

---

## Hva enterprise-arkitekter bør spørre om

### Spørsmål til leverandører

1. **Hvilken studie underbygger produktivitetspåstanden?** (Lab-oppgave? Enterprise RCT? Selvrapportert?)
2. **Gjelder det for erfarne utviklere i store kodebaser?** (METR sier nei)
3. **Hva er feilmodusene?** (Ikke bare «hva kan gå galt», men «hva gjør agenten når den tar feil?»)
4. **Hvem har tilsyn?** (Supervisjonsparadokset — trenger ekspertise for å oppdage AI-feil)
5. **Hva skjer med BYOAI?** (78 % tar med egne verktøy uansett)

### Spørsmål til egen organisasjon

1. **Måler vi riktig?** (PR-volum ≠ verdi. METR viste at subjektiv opplevelse er upålitelig)
2. **Har vi grønn/rød-sone-bevissthet?** (Hvilke oppgaver bør *ikke* delegeres?)
3. **Investerer vi i harness eller bare lisenser?** (Generell AI uten tilpasning gir generelle resultater)
4. **Trener vi supervisjon?** (Kompetanse til å vurdere AI-output er en egen ferdighet)
5. **Er governance-strukturen klar?** (Hvem godkjenner at en agent får tilgang til produksjonsdata?)

---

## Oppsummering for den skeptiske

| Påstand | Evidens |
|---------|---------|
| «AI gjør utviklere dobbelt så produktive» | Nei. 8–55 % avhengig av oppgave og erfaring. Erfarne devs i store kodebaser: muligens *tregere*. |
| «Det er bare hype» | Nei. Reell verdi for boilerplate, onboarding, forståelse av ukjent kode. 97 % bruker det allerede. |
| «Vi trenger bare lisenser» | Nei. Harness (tilpasning, governance, instruksjoner) er der verdien ligger. |
| «Det er trygt» | Delvis. Prompt injection, excessive agency og agentic misalignment er reelle risikoer. |
| «Vi kan vente» | Risikabelt. 78 % BYOAI betyr at utviklerne allerede bruker ukontrollerte verktøy. |

---

## Demoer (under presentasjonen)

1. **nav-pilot planlegging** — vis fasestyring, blindsoner, beslutningstrær
2. **Grønn/rød sone i praksis** — vis hvordan instruksjonen markerer kjernelogikk
3. **MCP-registry** — vis kontrollert verktøytilgang for agenter
4. **Code review-agent** — vis automatisk kvalitetskontroll
5. **Bevisst AI-bruk** — vis generer-så-forstå-mønsteret live

---

## Kilder

| Kilde | År | URL |
|-------|-----|-----|
| METR: AI Experienced OS Dev Study | 2025 | [metr.org/blog/2025-07-10-early-2025-ai-experienced-os-dev-study](https://metr.org/blog/2025-07-10-early-2025-ai-experienced-os-dev-study/) |
| GitHub/Accenture Enterprise Study | 2024 | [github.blog](https://github.blog/news-insights/research/research-quantifying-github-copilots-impact-in-the-enterprise-with-accenture/) |
| NBER: Generative AI at Work | 2023 | [nber.org/papers/w31161](https://nber.org/papers/w31161) |
| Anthropic: AI Transforming Work | 2025 | [anthropic.com/research/how-ai-is-transforming-work-at-anthropic](https://www.anthropic.com/research/how-ai-is-transforming-work-at-anthropic) |
| Anthropic: Agentic Misalignment | 2025 | [anthropic.com/research/agentic-misalignment](https://www.anthropic.com/research/agentic-misalignment) |
| Anthropic: Trustworthy Agents | 2026 | [anthropic.com/research/trustworthy-agents](https://www.anthropic.com/research/trustworthy-agents) |
| Anthropic: Measuring Agent Autonomy | 2026 | [anthropic.com/research/measuring-agent-autonomy](https://www.anthropic.com/research/measuring-agent-autonomy) |
| Anthropic: Economic Index | 2026 | [anthropic.com/research/economic-index-march-2026-report](https://www.anthropic.com/research/economic-index-march-2026-report) |
| Forrester Predictions 2025: AI | 2025 | [forrester.com/blogs/predictions-2025-artificial-intelligence](https://www.forrester.com/blogs/predictions-2025-artificial-intelligence/) |
| Microsoft Work Trend Index | 2024–2025 | [microsoft.com/worklab](https://www.microsoft.com/en-us/worklab/work-trend-index/ai-at-work-is-here-now-comes-the-hard-part) |
| Stray et al. (Nav-studie) | 2026 | [arxiv.org/abs/2509.20353](https://arxiv.org/abs/2509.20353) |
| Nav utviklerundersøkelsen | 2026 | Intern |
| OWASP GenAI Security | 2024 | [owasp.org](https://owasp.org/www-project-top-10-for-large-language-model-applications/) |
| GitHub: Copilot Productivity RCT | 2022 | [github.blog](https://github.blog/2022-09-07-research-quantifying-github-copilots-impact-on-developer-productivity-and-happiness/) |
