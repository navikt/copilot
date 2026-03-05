---
title: "Nyheter og trender — Februar 2026"
date: 2026-02-28
category: copilot
excerpt: "Agent HQ med Claude og Codex, GitHub Agentic Workflows, Copilots minnesystem, Copilot SDK, AGENTS.md-debatten, og stemningen i utviklermiljøet."
tags:
  - agent-hq
  - agentic-workflows
  - copilot-memory
  - copilot-sdk
  - agents-md
---

Februar 2026 var måneden da AI-kodingsagenter tok et nytt steg — fra verktøy du spør om hjelp, til agenter som jobber selvstendig i repoet ditt. Her er det viktigste som skjedde, og hva det betyr for oss i Nav.

## Velg din agent

GitHub åpnet 4. februar **Agent HQ** — en plattform der du kan sette Claude (Anthropic) og OpenAI Codex på issues og pull requests, ved siden av Copilot. Du nevner `@Claude` eller `@Codex` i en kommentar, og agenten starter en økt. Hver økt koster én premium-forespørsel.

Det interessante for oss er ikke bare flere modellvalg. Agent HQ kommer med sentraliserte tilgangskontroller på organisasjonsnivå, et nytt **Copilot-metrikk-dashboard** (public preview), og full revisjonslogging. GitHub lanserte også **Code Quality** i public preview — automatisk vurdering av vedlikeholdbarhet og pålitelighet som del av Copilots arbeidsflyt.

Google, Cognition og xAI er annonsert som kommende agentleverandører.

[Les mer på GitHub Blog →](https://github.blog/news-insights/company-news/pick-your-agent-use-claude-and-codex-on-agent-hq/)

## Arbeidsflyter skrevet i Markdown

GitHub Next lanserte 13. februar **Agentic Workflows** i technical preview — og konseptet er overraskende enkelt. Du skriver en arbeidsflyt i Markdown, definerer hva agenten skal gjøre i naturlig språk, og kjører den som en GitHub Action.

Sikkerhetsmodellen er gjennomtenkt: kun lesetilgang som standard, skriving krever eksplisitte «Safe Outputs», sandkassekjøring, og PR-er merges aldri automatisk. Et menneske godkjenner alltid.

Bak dette ligger et mønster GitHub kaller **Continuous AI** — regler i naturlig språk kombinert med agentbasert resonnering som kjører kontinuerlig. GitHub Next har testet seks bruksområder:

- **Docs vs. kode** — agenten finner og fikser avvik mellom dokumentasjon og kildekode
- **Prosjektrapporter** — ukentlige oppsummeringer generert automatisk
- **Oversettelser** — alle språkversjoner oppdateres når kildetekst endres
- **Avhengighetsdrift** — oppdager udokumenterte endringer i avhengigheter
- **Testdekning** — fra ~5 % til nær 100 % på 45 dager, for omtrent $80 i modellkostnader
- **Ytelse** — finner og fikser subtile ytelsesproblemer

Mønsteret som tegner seg: repoer vil ha **flåter av små agenter**, hver med ansvar for én oppgave, heller enn én stor generell agent.

[Automate repository tasks with GitHub Agentic Workflows →](https://github.blog/ai-and-ml/automate-repository-tasks-with-github-agentic-workflows/)

[Continuous AI in practice →](https://github.blog/ai-and-ml/generative-ai/continuous-ai-in-practice-what-developers-can-automate-today-with-agentic-ci/)

## Agenter som husker

Copilot fikk i januar et **minnesystem** som deles på tvers av kodingsagenten, CLI og code review. Ideen er at agenter skal bli bedre over tid — ikke starte blankt hver gang.

Slik fungerer det i praksis: Agenten lagrer fakta med kodehenvisninger når den oppdager noe nyttig. Før et minne brukes, verifiserer den kilden mot gjeldende kode. Stemmer det ikke, lagres en korrigert versjon. Oppdager code review en konvensjon? Da kan kodingsagenten bruke den neste gang, og CLI utnytter den ved feilsøking.

Tallene fra A/B-testing er overbevisende: **7 % høyere PR-merge-rate** (90 % vs. 83 %) og 2 % mer positiv tilbakemelding på kodegjennomgang, begge med p < 0,00001. Minner er knyttet til repoet og utløper etter 28 dager.

Et interessant funn: I adversariell testing der feilaktige og ondsinnede minner ble plantet med vilje, oppdaget agentene konsekvent feilene og korrigerte seg selv.

[Building an agentic memory system for GitHub Copilot →](https://github.blog/ai-and-ml/github-copilot/building-an-agentic-memory-system-for-github-copilot/)

## Copilot SDK: Bygg agenter i egne apper

GitHub lanserte 22. januar **Copilot SDK** i technical preview, med støtte for Node.js, Python, Go og .NET. SDK-et gir programmatisk tilgang til den samme agentsløyfen som driver Copilot CLI — planlegging, verktøybruk, flertrinnsutføring, MCP-servere og modellhåndtering.

For Nav er dette relevant fordi det åpner for å bygge interne verktøy med agentfunksjonalitet — uten å måtte implementere hele infrastrukturen selv.

[Build an agent into any app with the GitHub Copilot SDK →](https://github.blog/news-insights/company-news/build-an-agent-into-any-app-with-the-github-copilot-sdk/)

## AGENTS.md-debatten

To Hacker News-diskusjoner i februar viste at utviklermiljøet er delt i synet på hvordan man best gir kodingsagenter kontekst.

**Vercel** fant at en komprimert dokumentasjonsindeks rett i AGENTS.md **utkonkurrerer Claude Code Skills**. I 56 % av testene ble skills aldri aktivert. Grunnen er enkel: informasjonen er alltid tilgjengelig, uten avhengighet av asynkron lasting eller rekkefølge.

En akademisk evaluering viste ~4 % forbedring fra håndskrevne AGENTS.md-filer. Mange kalte det «massivt» for en enkel Markdown-fil. LLM-genererte AGENTS.md-filer ga derimot **minus 3 % effekt** — fordi de beskriver det åpenbare heller enn domenekunnskap agenten faktisk trenger.

Noen praktiske funn fra diskusjonene:

- **Førsteperson fungerer best**: «I will follow instructions» slår «You must follow instructions»
- **Progressiv struktur**: en slank toppnivå-AGENTS.md med nestede filer per app eller funksjon
- Legg bare til regler i AGENTS.md når agenten **faktisk feiler** — tilbakestill og kjør på nytt for å sjekke at det hjelper
- Flere anbefaler å flytte regler til deterministiske sjekker (kompilator, linter, AST) — agenter ignorerer instruksjoner i komplekse situasjoner

[AGENTS.md outperforms skills (Hacker News) →](https://news.ycombinator.com/item?id=46809708)

[Evaluating AGENTS.md (Hacker News) →](https://news.ycombinator.com/item?id=47034087)

## Anthropics agentrapport

Anthropic publiserte sin **2026 Agentic Coding Trends Report** med åtte trender. Den mest interessante for oss: **agentbasert kvalitetskontroll blir standard** — der AI-agenter gjennomgår AI-generert kode for sårbarheter, arkitektur og kvalitet. Andre trender inkluderer koordinerte agentteam, langkjørende agenter som bygger hele systemer, og at produktivitetsgevinster endrer økonomien i utvikling.

[2026 Agentic Coding Trends Report →](https://www.anthropic.com/research/agentic-coding-trends)

## Fra utviklermiljøet

Det er verdt å ta temperaturen på hva utviklere faktisk opplever:

**Det som fungerer godt:** En utvikler fikk en AI-agent til å oversette 14 000 linjer Python til TypeScript uten feil, på fire timer. Andre rapporterer at agenter som trekker ut «skills» fra vellykkede kjøringer blir merkbart bedre over tid. Poenget er at delegering av hele oppgaver — fra start til slutt — begynner å fungere.

**Det som bekymrer:** «Forestill deg å feilsøke 3 millioner linjer kode ingen mennesker har rørt.» Konteksttap er et gjennomgangstema — AI-kodere kan gjøre fantastiske ting, men uten utviklerens forståelse flyttes flaskehalsen heller enn å forsvinne. En utvikler opplevde at agenten byttet ut all SQLite med MariaDB, til tross for en AGENTS.md som eksplisitt sa «spør først».

Duncan Ogilvie oppsummerte erfaringen med kodingsagenter under begrepet **«Vibe Engineering»**: Kontekstvinduet er dyrebart — vær bevisst på hva som fyller det. Prosjektoppsett er avgjørende. TDD har blitt essensielt fordi testene er det som faktisk fanger feil. Og planlegging før prompting lønner seg alltid.
