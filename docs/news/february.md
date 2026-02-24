# Nyheter og trender — Februar 2026

De viktigste nyhetene om AI-kodingsagenter og GitHub Copilot denne måneden, med relevans for Nav.

---

## 1. GitHub Agent HQ: Claude og Codex i public preview

GitHub åpnet 4. februar agentplattformen **Agent HQ** for Claude (Anthropic) og OpenAI Codex. Utviklere med Copilot Pro+ eller Enterprise kan nå sette flere agenter på samme issue eller PR, nevne `@Copilot`, `@Claude` eller `@Codex` i kommentarer, og starte agentøkter rett fra VS Code.

Hver agentøkt koster én premium-forespørsel. Google, Cognition og xAI kommer som agentleverandører senere.

For organisasjoner betyr dette:

- Sentraliserte tilgangskontroller og policyer på organisasjonsnivå
- **GitHub Code Quality** (public preview) som vurderer vedlikeholdbarhet og pålitelighet
- Automatisert førsteutkast-kodegjennomgang i Copilots arbeidsflyt
- Copilot-metrikk-dashboard (public preview) for bruk og effekt på tvers av organisasjonen
- Full revisjonslogging

**Kilde:** [Pick your agent: Use Claude and Codex on Agent HQ](https://github.blog/news-insights/company-news/pick-your-agent-use-claude-and-codex-on-agent-hq/) (github.blog, 4. februar 2026)

---

## 2. Continuous AI og GitHub Agentic Workflows

GitHub Next lanserte 13. februar **GitHub Agentic Workflows** i technical preview — en ny form for automatisering der arbeidsflyter skrives i Markdown og kjøres som GitHub Actions med kodingsagenter.

### Lagdelt sikkerhet

Sikkerheten er bygget i lag:

- Kun lesetilgang som standard — skriving krever eksplisitte **Safe Outputs**
- Sandkassekjøring, godkjenningslister for verktøy og nettverksisolasjon
- PR-er merges aldri automatisk — et menneske må alltid godkjenne
- All aktivitet logges

### Continuous AI-mønsteret

**Continuous AI** er mønsteret bak: regler skrevet i naturlig språk, kombinert med agentbasert resonnering som kjører kontinuerlig i repoet. GitHub Next har testet seks bruksområder:

1. **Dokumentasjon vs. kode** — agenten finner og fikser avvik mellom docs og kode
2. **Prosjektrapporter** — ukentlige oppsummeringer med aktivitet, trender og anbefalinger
3. **Oversettelser** — oppdaterer alle språkversjoner automatisk når kildetekst endres
4. **Avhengighetsdrift** — fanger opp udokumenterte endringer i avhengigheter
5. **Testdekning** — fra ~5 % til nær 100 % på 45 dager for ~$80 i modellkostnader
6. **Ytelse** — finner og fikser subtile ytelsesproblemer

Mønsteret som tegner seg er at repoer vil ha **flåter av små agenter** — hver med ansvar for én oppgave — heller enn én stor generell agent.

**Kilder:**

- [Automate repository tasks with GitHub Agentic Workflows](https://github.blog/ai-and-ml/automate-repository-tasks-with-github-agentic-workflows/) (github.blog, 13. februar 2026)
- [Continuous AI in practice: What developers can automate today with agentic CI](https://github.blog/ai-and-ml/generative-ai/continuous-ai-in-practice-what-developers-can-automate-today-with-agentic-ci/) (github.blog, 5. februar 2026)

---

## 3. Copilots minnesystem

GitHub lanserte i januar **kryssagent-minne** i public preview for Copilot coding agent, CLI og code review. Systemet lar agenter lære og bli bedre over tid, på tvers av verktøyene.

### Slik fungerer det

- **Minne som verktøykall**: Agenter lagrer fakta med kodehenvisninger når de oppdager noe som kan være nyttig senere
- **Verifisering ved bruk**: Før et minne brukes, sjekker agenten kilden mot gjeldende kode. Stemmer det ikke, lagres en korrigert versjon
- **Deling på tvers**: Code review oppdager en konvensjon → kodingsagenten bruker den → CLI utnytter den ved feilsøking

### Resultater fra A/B-testing

- **7 % økning** i PR-merge-rate (90 % med minner vs. 83 % uten)
- **2 % økning** i positiv tilbakemelding på kodegjennomgang
- Begge statistisk signifikante med p < 0,00001
- Minner er knyttet til repoet, utløper etter 28 dager, og krever skrivetilgang

I adversariell testing — der feilaktige og ondsinnede minner ble plantet med vilje — oppdaget agentene konsekvent feilene og korrigerte seg selv.

**Kilde:** [Building an agentic memory system for GitHub Copilot](https://github.blog/ai-and-ml/github-copilot/building-an-agentic-memory-system-for-github-copilot/) (github.blog, 15. januar 2026)

---

## 4. Copilot SDK

GitHub lanserte 22. januar **Copilot SDK** i technical preview, med støtte for Node.js, Python, Go og .NET. SDK-et gir programmatisk tilgang til den samme agentsløyfen som driver Copilot CLI — planlegging, verktøybruk, flertrinnsutføring, MCP-servere og modellhåndtering.

Copilot CLI har samtidig fått vedvarende minne, uendelige økter, intelligent kompaktering, utforsk/planlegg/gjennomgå-arbeidsflyter, egendefinerte agenter og asynkron oppgavedelegering.

**Kilde:** [Build an agent into any app with the GitHub Copilot SDK](https://github.blog/news-insights/company-news/build-an-agent-into-any-app-with-the-github-copilot-sdk/) (github.blog, 22. januar 2026)

---

## 5. AGENTS.md-debatten: Skills vs. kontekst

To Hacker News-diskusjoner viste en pågående debatt om hvordan man best gir kodingsagenter kontekst.

### AGENTS.md slår Skills i evalueringer

Vercel fant at en komprimert dokumentasjonsindeks rett i AGENTS.md **utkonkurrerer Claude Code Skills**. I 56 % av testene ble skills aldri aktivert. Tre grunner: ingen beslutningspunkt (infoen er alltid der), konsistent tilgjengelighet (vs. asynkron skill-lasting) og ingen rekkefølgeproblemer.

### Akademisk evaluering

En forskningsartikkel fant ~4 % forbedring fra håndskrevne AGENTS.md-filer — som mange kalte «massivt» for en enkel Markdown-fil. LLM-genererte AGENTS.md-filer ga derimot **-3 % effekt**, fordi de beskriver det åpenbare heller enn domenekunnskap agenten faktisk trenger.

### Praktiske funn

- **Førsteperson fungerer best**: «I will follow instructions» slår «You must follow instructions»
- **Progressiv struktur**: slank toppnivå AGENTS.md + nestede filer per funksjon eller app
- AGENTS.md blir i praksis det nye CONTRIBUTING.md
- Flere anbefaler å flytte regler til **deterministiske sjekker** (kompilator/AST) — agenter ignorerer instruksjoner i komplekse situasjoner
- Anbefalt arbeidsflyt: legg til i AGENTS.md bare når agenten **feiler**, tilbakestill og kjør på nytt for å sjekke at det hjelper

**Kilder:**

- [AGENTS.md outperforms skills in our agent evals (Vercel)](https://news.ycombinator.com/item?id=46809708) (Hacker News)
- [Evaluating AGENTS.md: are they helpful for coding agents?](https://news.ycombinator.com/item?id=47034087) (Hacker News)

---

## 6. Anthropics agentrapport for 2026

Anthropic publiserte sin **2026 Agentic Coding Trends Report** med åtte trender:

1. Utviklingssyklusen endres dramatisk
2. Enkle agenter utvikler seg til koordinerte team
3. Langkjørende agenter bygger hele systemer
4. Menneskelig tilsyn skalerer gjennom smart samarbeid
5. Agenter sprer seg til nye overflater og brukergrupper
6. Produktivitetsgevinster endrer økonomien i utvikling
7. Ikke-tekniske bruksområder vokser
8. **Dobbeltbruksrisiko krever sikkerhet først** — agentbasert kvalitetskontroll blir standard, der AI-agenter gjennomgår AI-generert kode for sårbarheter, arkitektur og kvalitet

**Kilde:** [2026 Agentic Coding Trends Report](https://www.anthropic.com/research/agentic-coding-trends) (Anthropic)

---

## 7. Stemningen i utviklermiljøet

### Det som fungerer

- AI-agent oversatte 14 000 linjer Python til TypeScript uten feil på fire timer
- Agenter som trekker ut «skills» fra vellykkede kjøringer og blir bedre over tid
- Delegering av komplekse oppgaver fra start til slutt

### Det som bekymrer

- **Kvalitet i skala**: «Forestill deg å feilsøke 3 millioner linjer kode ingen mennesker har rørt»
- **Konteksttap**: «AI-kodere kan gjøre fantastiske ting, men uten utviklerens forståelse flyttes bare flaskehalsen»
- **Instruksjonskompleksitet**: Systeminstruksjoner blir mer og mer innviklede etter hvert som agenter lærer
- **Agenter ignorerer regler**: én utvikler opplevde at agenten byttet ut all SQLite med MariaDB — til tross for 25 linjers AGENTS.md som sa «spør først»

### «Vibe Engineering»

Duncan Ogilvie oppsummerte i januar 2026 erfaringer med kodingsagenter:

- Kontekstvinduet er dyrebart — vær bevisst på hva som fyller det
- Prosjektoppsett er avgjørende — riktig struktur gjør agenten bedre
- TDD er blitt essensielt — testene er det som fanger feil
- DevDocs for å overleve konteksttilbakestillinger
- Planlegging før prompting
- Sub-agenter som neste steg

**Kilder:**

- [Reddit-diskusjoner om agentisk koding](https://www.reddit.com/r/ArtificialInteligence/) (r/ArtificialIntelligence)
- [Vibe Engineering: What I've Learned Working with AI Coding Agents](https://www.linkedin.com/pulse/vibe-engineering-what-ive-learned-working-ai-coding-agents-ogilvie/) (LinkedIn, januar 2026)

---

## Relevans for Nav

Flere av trendene er direkte relevante for Nav:

| Trend             | Hva det betyr for Nav                                         |
| ----------------- | ------------------------------------------------------------- |
| Copilot-minne     | Agenten kan lære seg Navs konvensjoner og kodebaser over tid  |
| Agentic Workflows | Automatisert vedlikehold av docs, testdekning og kodekvalitet |
| Agent HQ          | Sammenligne ulike agenter for ulike oppgavetyper              |
| AGENTS.md         | Nav kan investere i gode AGENTS.md-filer i sine repoer        |
| Sikkerhet først   | Sammenfaller med Navs krav til sikkerhet og personvern        |
| Metrikk-dashboard | Bedre innsikt i faktisk bruk og effekt av Copilot             |
