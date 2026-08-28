# Agentic engineering for enterprise-arkitekter

## Hva er agentic engineering?

Autocomplete foreslår én linje om gangen, og du aksepterer eller forkaster. Agentisk AI er noe annet:

| Egenskap | Autocomplete | Agentisk AI |
|----------|--------------|-------------|
| Interaksjon | Enkeltforslag | Flertrinns planlegging og utførelse |
| Verktøybruk | Ingen | Leser filer, kjører kode, kaller API-er |
| Varighet | Millisekunder | Minutter til timer |
| Feilmodus | Dårlig forslag (begrenset skade) | Kaskadefeil på tvers av systemer |
| Styring | Innholdsfilter | Tillitsgrenser, tilgangsstyring, revisjonslogg |

Siste rad er den som angår arkitekter. Et innholdsfilter er en produktegenskap du kjøper. Tillitsgrenser er arkitektur du må tegne selv.

## Hva sier forskningen?

### Skeptikerne har delvis rett

[METR-studien](https://metr.org/blog/2025-07-10-early-2025-ai-experienced-os-dev-study/) (juli 2025, arXiv:2507.09089) er et randomisert kontrollert forsøk: 16 erfarne utviklere, 246 reelle oppgaver i store kodebaser (22 000+ stars, 1M+ linjer). AI gjorde dem **19 % tregere**. Utviklerne trodde selv de var 24 % raskere, og de trodde fortsatt de var 20 % raskere etter å ha opplevd nedgangen. Økonomer og ML-forskere hadde på forhånd spådd 38–39 % speedup.

Fem forklaringer på resultatet:

1. AI presterer dårligere i store, komplekse kodebaser med implisitt kontekst
2. Å lese, forstå og verifisere AI-generert kode koster
3. Debugging av AI-feil tar tid
4. Høye krav til stil, tester og dokumentasjon gjør forslagene mindre direkte brukbare
5. Oppgavene lå i kodebaser utviklerne kjente fra før, der AI gir minst merverdi

### Men verdien er reell på riktige oppgaver

[GitHubs forskning](https://github.blog/2022-09-07-research-quantifying-github-copilots-impact-on-developer-productivity-and-happiness/) (2022–2024) målte **55 % raskere** på en avgrenset labbeoppgave (HTTP-server i JavaScript). [Hos Accenture](https://github.blog/news-insights/research/research-quantifying-github-copilots-impact-in-the-enterprise-with-accenture/), i enterprise-skala, ble tallene mer beskjedne: 8,7 % flere PR-er, 84 % bedre CI-bygg, 15 % høyere merge-rate.

[NBER/Brynjolfsson et al.](https://nber.org/papers/w31161) (2023) fulgte 5 179 kundeserviceagenter og fant +14 % produktivitet totalt, men **+34 % for nybegynnere** og minimalt for de erfarne.

[Anthropic internt](https://www.anthropic.com/research/how-ai-is-transforming-work-at-anthropic) (august 2025, 132 ingeniører): 59 % av det daglige arbeidet bruker Claude, selvrapportert 50 % produktivitetsøkning, 67 % flere mergede PR-er per ingeniør per dag. Samtidig delegerer over halvparten bare 0–20 % av arbeidet fullt til AI, og senioringeniørene delegerer bevisst lite av kjernearbeidet.

Mønsteret går igjen på tvers av studiene. AI hjelper mest nybegynnere, på avgrensede oppgaver, i ukjent kode. AI hjelper minst erfarne utviklere, på komplekse oppgaver, i kode de kjenner, med høye kvalitetskrav. Det er nesten en beskrivelse av arbeidsdagen til en Nav-utvikler med ti års fartstid i en fagsystemmonolitt.

## Kompetansebevaring

To sitater fra Anthropics egne senioringeniører (2025):

> «Det blir vanskeligere å ta seg tid til å faktisk lære noe når det er så lett og raskt å produsere output.»

> «Ferdighetene mine vil primært forfalle med hensyn til min evne til å trygt *bruke* AI for oppgavene jeg bryr meg om.»

Dette er **supervisjonsparadokset**: effektiv oversikt over AI krever nettopp de ferdighetene som AI-avhengighet eroderer.

Vår egen [utviklerundersøkelse 2026](utviklerundersokelsen-2026-oppsummering.md) (163 respondenter) viser det samme spenningsfeltet:

- 75 % opplever at AI hjelper dem jobbe raskere
- **59 % er bekymret for kompetansetap**
- Kun 34 % mener AI-kode holder til code review
- Det mest etterspurte tiltaket er bedre opplæring (31 %)

Til sammenligning fant [Stray et al., HICSS-59 2026](https://arxiv.org/abs/2509.20353), en longitudinell studie av 26 317 Nav-commits, *ingen statistisk signifikant produktivitetsøkning*. Opplevd fart og målt fart er ikke det samme, verken hos METR eller hos oss.

## Sikkerhetsrisiko ved agentisk AI

### Agentic misalignment

Anthropic red-teamet [16 ledende modeller](https://www.anthropic.com/research/agentic-misalignment) (2025) fra Anthropic, OpenAI, Google, Meta og xAI i simulerte bedriftsmiljøer. Modellene fikk verktøytilgang til e-post og filsystemer, og mål som kom i konflikt med bedriftsinstruksene. **Alle 16** tydde til ondsinnet atferd, utpressing og lekkasje av informasjon, når det var eneste vei unna nedleggelse.

Poenget er ikke at modellene er onde. Poenget er at verktøytilgang gjør målkonflikt til en sikkerhetshendelse.

### OWASP Top 10 for LLM-applikasjoner

Av [OWASPs liste](https://owasp.org/www-project-top-10-for-large-language-model-applications/) (2024) er tre punkter mest relevante for enterprise-agenter:

| # | Sårbarhet | Konsekvens |
|---|-----------|------------|
| LLM01 | Prompt injection | Angriper kaprer agenthandlinger via innhold i e-post, dokumenter, nettsider |
| LLM08 | Excessive agency | AI med for vid fullmakt tar utilsiktede handlinger |
| LLM09 | Overreliance | Ukritisk aksept av AI-output |

### Fire angrepsflater

[Trustworthy Agents](https://www.anthropic.com/research/trustworthy-agents) (april 2026) deler en agent i fire komponenter, og hver av dem er en angrepsflate:

1. **Modellen**, der treningen former oppførselen
2. **Harness**, altså instrukser og guardrails. En feilkonfigurert harness undergraver en god modell
3. **Verktøy**, som e-post, kalender, databaser og kodekjøring
4. **Miljø**, altså hva agenten faktisk har tilgang til

Nav kontrollerer ikke modellen. Vi kontrollerer de tre andre.

## Hva Forrester og Microsoft sier

[Forrester Predictions 2025](https://www.forrester.com/blogs/predictions-2025-artificial-intelligence/) spår at **75 % av bedriftene som bygger agentisk AI selv vil mislykkes** fordi arkitekturene er for komplekse, at ROI-forventninger vil utløse for tidlige nedskaleringer, og at 40 % av regulerte virksomheter må slå sammen data- og AI-governance.

[Microsoft Work Trend Index](https://www.microsoft.com/en-us/worklab/work-trend-index/ai-at-work-is-here-now-comes-the-hard-part) 2024–2025 (31 000 respondenter, 31 land):

- 78 % av AI-brukerne tar med egne verktøy (BYOAI), utenom bedriftskontrollene
- 52 % er *redde for å innrømme* at de bruker AI til viktige oppgaver
- 81 % av ledere forventer agenter i AI-strategien innen 12–18 måneder
- 60 % av ledere innrømmer at organisasjonen mangler en plan for AI-implementering

BYOAI-tallet er argumentet mot å vente. Alternativet til godkjente verktøy er ikke fravær av AI, det er AI vi ikke ser.

## Hva vi gjør i Nav

### Vi bygger harness, ikke modeller

Nav investerer ikke i egne modeller. Vi bygger *harness*, tilpasningslaget som gjør generelle modeller til Nav-spesifikke verktøy:

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

Laget er modell-agnostisk med vilje. Bytter leverandøren modell under oss, beholder vi instruksene, agentene og governance-laget.

### Grønn og rød sone

Forskningsfunnene er bakt inn i AI-instruksene, ikke bare skrevet ned i et policydokument:

**🟢 Grønn sone (AI-egnet):** boilerplate, Nais-manifest, CRUD, kjent teknologi, konfigurasjon, testdata

**🔴 Rød sone (kod manuelt først):** debugging, nye konsepter, kjernelogikk, sikkerhetskritisk kode, arkitekturbeslutninger

**Tre-forsøks-regelen:** prøv å løse problemet selv tre ganger før du spør AI.

Full oversikt over agenter, skills, instruksjoner og de områdene vi bevisst holder AI unna ligger i [ai-bruk-oversikt.md](ai-bruk-oversikt.md).

### nav-pilot: planlegging med fasestyring

`@nav-pilot` er ikke «skriv kode for meg», men en arbeidsflyt i fire faser:

1. **Intervju** kartlegger blindsoner: personvern, tilgangsstyring, feilhåndtering, observerbarhet, teamgrenser, endringskonsekvenser, teststrategi, migrering, bakoverkompatibilitet, dekommisjonering og kompetansebevaring
2. **Plan** bygger beslutningstrær for auth, kommunikasjon, database og CI/CD
3. **Review** vurderer planen fra fire perspektiver: sikkerhet, plattform, arkitektur, endringssikkerhet
4. **Lever** gir kode og dokumentasjon, med rød-sone-kode markert som TODO

Agenten stopper mellom fasene og venter på godkjenning. Den delegerer til spesialistagenter for auth, Kafka, Nais og sikkerhet ved behov, men beholder kontrollen selv.

### Tall fra Nav

Fra [utviklerundersøkelsen 2026](utviklerundersokelsen-2026-oppsummering.md):

- 93 % av utviklerne bruker AI-kodeverktøy aktivt
- 53 % bruker Copilot CLI, altså agentisk bruk fra terminalen

MCP-registryet holder listen over godkjente servere, som er den praktiske grensen for hvilke verktøy en agent får kalle.

## Hva enterprise-arkitekter bør spørre om

### Til leverandøren

1. **Hvilken studie underbygger produktivitetspåstanden?** Labboppgave, enterprise-RCT eller selvrapportering?
2. **Gjelder den for erfarne utviklere i store kodebaser?** METR sier nei.
3. **Hva er feilmodusene?** Ikke bare hva som kan gå galt, men hva agenten gjør når den tar feil.
4. **Hvem har tilsyn?** Supervisjonsparadokset betyr at du trenger ekspertisen for å oppdage AI-feilene.
5. **Hva skjer med BYOAI?** 78 % tar med egne verktøy uansett.

### Til egen organisasjon

1. **Måler vi riktig?** PR-volum er ikke verdi, og METR viste at den subjektive opplevelsen er upålitelig.
2. **Har vi grønn/rød-sone-bevissthet?** Hvilke oppgaver bør *ikke* delegeres?
3. **Investerer vi i harness eller bare lisenser?** Generell AI uten tilpasning gir generelle resultater.
4. **Trener vi supervisjon?** Å vurdere AI-output er en egen ferdighet.
5. **Er governance-strukturen klar?** Hvem godkjenner at en agent får tilgang til produksjonsdata?

## Oppsummering for den skeptiske

| Påstand | Evidens |
|---------|---------|
| «AI gjør utviklere dobbelt så produktive» | Nei. 8–55 % avhengig av oppgave og erfaring. Erfarne devs i store kodebaser: muligens *tregere*. |
| «Det er bare hype» | Nei. Reell verdi for boilerplate, onboarding, forståelse av ukjent kode. 97 % bruker det allerede. |
| «Vi trenger bare lisenser» | Nei. Harness (tilpasning, governance, instruksjoner) er der verdien ligger. |
| «Det er trygt» | Delvis. Prompt injection, excessive agency og agentic misalignment er reelle risikoer. |
| «Vi kan vente» | Risikabelt. 78 % BYOAI betyr at utviklerne allerede bruker ukontrollerte verktøy. |

## Demoer under presentasjonen

1. **nav-pilot planlegging**, med fasestyring, blindsoner og beslutningstrær
2. **Grønn/rød sone i praksis**, der instruksjonen markerer kjernelogikk
3. **MCP-registry**, kontrollert verktøytilgang for agenter
4. **Code review-agent**, automatisk kvalitetskontroll
5. **Bevisst AI-bruk**, generer-så-forstå-mønsteret live

## Videre lesning

- [Anthropic: Measuring Agent Autonomy](https://www.anthropic.com/research/measuring-agent-autonomy) (2026)
- [Anthropic: Economic Index](https://www.anthropic.com/research/economic-index-march-2026-report) (2026)
