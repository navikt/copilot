# Modellvalg i Nav Copilot

Levende referansedokument for hvilke modeller vi bruker, hvorfor, og hvordan vi vurderer oppdateringer.

Kortversjonen for utviklere, med målinger og priser, står på [ki-utvikling.nav.no/modeller](https://ki-utvikling.nav.no/modeller).

## Gjeldende modellpinning

De fleste agenter og prompts har et eksplisitt `model:`-felt i YAML-frontmatter. `nav-pilot` har det ikke, men agentpakken bruker GPT-6 Sol når brukeren ikke har valgt en modell. En brukerpinne vinner fortsatt over pakkas standard. Valget følger oppgavetype, kostnad og ytelse, ikke leverandørpreferanse. Priser og kategori står i modelltabellen under.

**Pinnene under gjelder når agenten startes direkte.** Blir den startet som subagent av `@nav-pilot`, arver den modellen forelderen kjører på. Se [Pinner og delegering](#pinner-og-delegering).

### Agenter

| Agent                | Modell            | Begrunnelse                                                                                                                                                                                                               |
| -------------------- | ----------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `@nav-pilot`         | GPT-6 Sol         | Agentpakkas standard for Copilot, opencode og pi. GPT-5.6 Sol beholdes som fallback. En brukerpinne overstyrer standarden                                                                                                 |
| `@nav-pilot-opus`    | Claude Opus 5.5   | Høyrisikoplanlegging og kritisk kodegjennomgang. High effort traff de plantede linjene i fem av fem gjennomganger. GPT-6.1 Sol er fallback mens vi måler agenten direkte                                                  |
| `@security-champion` | GPT-6 Sol         | Sikkerhetskritiske vurderinger. Modellen fant personvern, tilgangskontroll og riktig TokenX-mønster i fem av fem kjøringer. Ett fasebrudd i `nav-pilot` følges under utrullingen                                          |
| `@code-review`       | Claude Opus 5.5   | Fant alle plantede feil på riktig linje i ti av ti gjennomganger på Low, Medium og High 30. september. Low holder og er billigst. GPT-6.1 Sol er fallback, deretter GPT-5.3-Codex                                         |
| `@kafka`             | GPT-6 Luna        | Verktøytung kodeagent. Luna Medium besto alle 30 sjekker i kodesuiten 30. september for omtrent 1,7 credits, mot omtrent 28 med GPT-6 Sol. Oppgavene var små, så GPT-6 Sol er fallback, deretter GPT-5.3-Codex            |
| `@research`          | GPT-6 Luna        | Leser og søker uten å skrive kode. Modellen besto ti av ti avgrensede krav og brukte omtrent 45 prosent færre credits enn GPT-5.6 Luna. Gjelder bare når agenten startes direkte, ikke når `@nav-pilot` delegerer til den |
| `@rust`              | GPT-6 Luna        | Verktøytung kodeagent. Luna Medium besto alle 30 sjekker i kodesuiten 30. september for omtrent 1,7 credits, mot omtrent 28 med GPT-6 Sol. Oppgavene var små, så GPT-6 Sol er fallback, deretter GPT-5.3-Codex            |
| `@aksel`             | Claude Sonnet 5.5 | Sterk på komponentstruktur og designsystem-konvensjoner. Sonnet 5 beholdes som fallback                                                                                                                                   |
| `@accessibility`     | Claude Sonnet 5.5 | God på WCAG-tolkning og semantisk HTML. Sonnet 5 beholdes som fallback                                                                                                                                                    |
| `@forfatter`         | Claude Sonnet 5.5 | Anthropic-modellene er best på norsk klarspråk. Sonnet 5 beholdes som fallback                                                                                                                                            |

### Prompts

| Prompt                 | Modell           | Begrunnelse                                                                 |
| ---------------------- | ---------------- | --------------------------------------------------------------------------- |
| `kafka-topic`          | GPT-6 Luna       | Fast scaffold-prompt. GPT-5.3-Codex beholdes som fallback                   |
| `nais-manifest`        | GPT-6 Luna       | Fast scaffold-prompt. GPT-5.3-Codex beholdes som fallback                   |
| `aksel-component`      | Gemini 3.8 Flash | Rask og billig for scaffolding av Aksel-komponenter                         |
| `ktor-endpoint`        | GPT-6 Luna       | Enkel strukturert mal. GPT-5.6 Luna beholdes som fallback under utrullingen |
| `nextjs-api-route`     | GPT-6 Luna       | Enkel strukturert mal. GPT-5.6 Luna beholdes som fallback under utrullingen |
| `spring-boot-endpoint` | GPT-6 Luna       | Enkel strukturert mal. GPT-5.6 Luna beholdes som fallback under utrullingen |
| `golang-service`       | GPT-6 Luna       | Enkel strukturert mal. GPT-5.6 Luna beholdes som fallback under utrullingen |

## Blokkeringsskjerm for GPT-6 og Opus 5.5

Målt 23. september 2026 med Copilot CLI 1.0.88-2, standard kontekstvindu og fem kjøringer per testarm. Utvalget kan finne tydelige blokkeringer, men kan ikke slå fast at kandidatene er minst like gode som kontrollmodellene.

| Kandidat               | Kontroll            | Resultat                                                                                                                                   | Anbefaling                                                                                                                          |
| ---------------------- | ------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------- |
| GPT-6 Luna Medium      | GPT-5.6 Luna Medium | Begge besto ti av ti avgrensede krav. GPT-6 Luna brukte 8,667 credits mot 15,894, med omtrent lik veggklokketid                            | Rull ut på `@research` og malpromptene. Behold GPT-5.6 Luna som fallback                                                            |
| GPT-6 Sol High         | GPT-5.6 Sol High    | GPT-6 Sol hoppet over intervjuet i én av fem kjøringer, men fant alle sikkerhets- og auth-krav. GPT-5.6 Sol fulgte fasekravet i fem av fem | Bruk som standard for `@nav-pilot` og verktøytunge kodeagenter. Følg fasebrudd og behold GPT-5.6 Sol og GPT-5.3-Codex som fallbacks |
| Claude Opus 5.5 Medium | Claude Opus 5 High  | **Ugyldig, se rettelsen under.** Medium var raskere og billigere, men oppga feil linjenumre i to av fem TSX-gjennomganger                  | Bruk ikke Medium til kodegjennomgang der presise linjer er viktig                                                                   |
| Claude Opus 5.5 High   | Claude Opus 5 High  | **Ugyldig, se rettelsen under.** Begge traff de plantede linjene i fem av fem. Opus 5.5 kostet 209,514 credits mot 186,763 og var tregere  | Rull ut på `@nav-pilot-opus` og `@code-review`. Behold Opus 5 og GPT-5.3-Codex som fallbacks                                        |

> **Rettelse 30. september 2026.** Opus-radene i tabellen er ugyldige. Alle tre Opus-armene, også kontrollen med Opus 5, kjørte GPT-5.3-Codex: bruksradene viser bare `gpt-5.3-codex` (55, 109 og 90 kall, se [models-per-arm.psv](golden-baselines/2026-09-23-blokkeringsskjerm/models-per-arm.psv)). Årsaken er to regler i Copilot CLI. En agentfil i `~/.copilot/agents/` går foran agentfila testoppsettet legger i arbeidsmappa, og en `model:`-pinne i agentfila går foran `--model`. Både den installerte `code-review.agent.md` og repoets versjon den dagen (520c36dd) var pinnet til GPT-5.3-Codex.
>
> Pinnene til Opus 5.5 på `@code-review` og `@nav-pilot-opus` hviler derfor ikke på noen gyldig måling i dag. Linjefeilene som ble tilskrevet Opus 5.5 Medium, kom fra GPT-5.3-Codex på Medium.
>
> GPT-6- og GPT-5.6-armene kjørte modellen de oppgir, men ikke repoets `nav-pilot.agent.md`. De målte kopien som lå installert i `~/.copilot/agents/` fra 15. september: versjonen fra 1ba0c234 (#776), fra før #898 og #905. Det ser vi i svarene: 19 av 20 transkripter fra test 2 ([t2-checkpoint.psv](golden-baselines/2026-09-23-blokkeringsskjerm/t2-checkpoint.psv)) slutter med «✅ Fase 1 ferdig — klar for Fase 2», en blokk som #905 hadde fjernet fra repoet før målingen. Sammenligningen mellom GPT-6 og GPT-5.6 står, fordi begge armene brukte samme agentfil. Men tallene gjelder den eldre agentfila, ikke den som ble rullet ut, og rettelsene i testoppsettet som er nevnt under, ble også utledet fra svar på den eldre fila.
>
> Testoppsettet installerer nå agenten under et eget navn og fjerner modellpinnen når `--model` er satt. Sammendraget avviser en kjøring der bruksradene viser en annen modell enn den som er oppgitt. Opus 5.5 ble målt på nytt på `@code-review` 30. september, se [Målinger 30. september 2026](#målinger-30-september-2026).

Råmålingen bruker eksakte `assistant_usage_events`, inkludert retries og subagenter. Fem kjøringer er ikke nok til å rangere modellene bredt. Utrullingen må derfor kunne reverseres uten at de eldre modellene først fjernes.

Vi tilpasset ikke agentpersonaene eller instruksjonene til de nye modellene før målingen. Bare testoppsettet ble rettet: Det måler nå Fase 2 på riktig tur og bruker faktiske intervjuspørsmål i stedet for en bestemt faseoverskrift. Kandidat og kontroll brukte samme agentfil, men det var den installerte kopien og ikke repoets (se rettelsen over).

`@code-review` ble flyttet til Opus 5.5 med en anbefaling om High effort. Målingen bak flyttingen var ugyldig (se rettelsen over). Den nye målingen 30. september støtter pinnen og viser at Low holder. Agent-frontmatter kan ikke håndheve innsatsnivå (effort), så en direkte start kan arve nivået fra sesjonen. Bruk GPT-6.1 Sol eller GPT-5.3-Codex som fallback ved regresjoner.

Kafka- og Rust-agentene flyttes til Sol, mens `kafka-topic` og `nais-manifest` flyttes til Luna. Blokkeringsskjermen målte samme oppgaveklasse, men ikke disse fire artefaktene direkte. Dette er derfor en kontrollert utrulling med fallbacks, ikke dokumentasjon på at de nye modellene er bedre på Kafka, Rust eller Nais-manifester.

Nye målinger kjøres som suiter i golden-harnesset og vises på modellsiden på ki-utvikling.nav.no. [golden-baselines/README.md](golden-baselines/README.md) forklarer hvordan du kjører en benchmark og legger til en modell.

## Målinger 30. september 2026

Vi kjørte to batcher med golden-harnesset på Copilot CLI 1.0.90-5, med fem kjøringer per testarm og ti på kodegjennomgang. Rådata ligger i [2026-09-30-batch1](golden-baselines/2026-09-30-batch1/) og [2026-09-30-batch2](golden-baselines/2026-09-30-batch2/). Credits er medianen per kjøring. Fem kjøringer finner tydelige feil, men er for få til å rangere modellene bredt.

- **`@kafka` og `@rust` bytter til GPT-6 Luna.** Kodesuiten har tre små feilrettinger i Go og TypeScript. Hver har to sjekker og kjøres fem ganger, så hver arm får 30 sjekker. Luna Medium besto alle 30 for 1,65 credits. GPT-6 Sol besto også alle 30, men brukte 26,9 til 29,4 credits. Oppgavene var små, så GPT-6 Sol er fallback.
- **High gjør ikke GPT-6 Luna bedre på koding.** High besto de samme 30 sjekkene for 1,84 credits.
- **Low holder for `@code-review`.** Opus 5.5 fant alle plantede feil på riktig linje i ti av ti gjennomganger på alle tre nivåer. Medianen var 23,9 credits på Low, 30,0 på Medium og 38,1 på High. Dette er den nye målingen som rettelsen over ventet på.
- **Bruk ikke High for `@nav-pilot`.** GPT-6 Sol High stoppet ikke etter fase 1 i to av fem kjøringer og brukte 70,7 credits. På Low stoppet den i fem av fem for 28,9 credits.
- **GPT-6 Sol er fortsatt standard for `@nav-pilot`.** GPT-6.1 Sol High og GPT-6 Astra skrev de åpne punktene i fase 1 som påstander, ikke som spørsmål, og besto test 2 i null av fem kjøringer. GPT-6.1 Sol Low besto i to av fem. Astra kostet i tillegg fem til seks ganger så mye som Sol-modellene: 154 credits per planleggingskjøring mot 28,9 for GPT-6 Sol Low.
- **Alle modellene skrev «AI» i norsk tekst.** Sonnet 5.5 gjorde det i tre av fem kjøringer, GPT-6.1 Sol i fem av fem og Astra i fire av fem. `@forfatter` har derfor fått en egen regel om å skrive «KI».

### GPT-6.1 Sol spør ikke i fase 1 (30. september 2026)

I batch 1 stoppet GPT-6.1 Sol High riktig etter fase 1 i fem av fem kjøringer og endret ingen filer. Men den skrev de åpne punktene, som personvern og tilgang, som en nummerert liste med påstander og ikke som spørsmål. Derfor feilet test 2, som teller spørsmål, og test 4 kom aldri til sin andre tur. Agentfila ber om spørsmål i fase 1 («Ask questions … All relevant blind spots raised as questions»). GPT-6.1 Sol Low gjorde det samme i test 2 i tre av fem kjøringer.

Vi har ikke løsnet sjekken. Tallene per transkript står i [t2-questions.psv](golden-baselines/2026-09-30-batch1/t2-questions.psv), og resultatene for High i [planning-gpt-6.1-sol-high-results.psv](golden-baselines/2026-09-30-batch1/planning-gpt-6.1-sol-high-results.psv).

## Målinger 1. oktober 2026

Batch 3 hadde fem kjøringer per testarm. Copilot CLI ble oppdatert mens planleggingsarmene startet, så de kjørte på en nyere versjon enn norsk-armene. Rådata ligger i [2026-10-01-batch3](golden-baselines/2026-10-01-batch3/), og hver sjekk som feilet, er klassifisert i [failures.psv](golden-baselines/2026-10-01-batch3/failures.psv). Credits er medianen per kjøring. Fem kjøringer finner tydelige feil, men er for få til å rangere modellene bredt.

- **Regelen om «KI» virker.** Ingen av modellene skrev «AI» i noen av de fem kjøringene. Før regelen besto Sonnet 5.5 sjekken i to av fem kjøringer og GPT-6.1 Sol i null av fem. GPT-6 Sol var ikke målt på norsk før, bortsett fra én testkjøring.
- **Sjekken for rød sone i planleggingen har en feil.** Alle de tre modellene skrev erklæringen om rød sone, men to av fem kjøringer per modell satte komma etter «Rød sone». Sjekken godtar bare lang tankestrek (—), kort tankestrek (–) eller kolon. Agentmalen bruker lang tankestrek, mens skrivereglene forbyr den og foreslår komma. Vi har ikke løsnet sjekken. På grunn av feilen skiller ikke test 4 modellene i denne batchen.
- **GPT-6 Sol Medium er ikke bedre enn Low på planlegging.** Medium stoppet etter fase 1 og stilte spørsmålene i fem av fem kjøringer, som Low, men brukte 41,3 credits mot 28,9. High stoppet ikke i to av fem kjøringer.
- **Claude Opus 5.5 planla riktig, men koster dobbelt så mye.** Opus besto alle sjekkene utenom feilen over, for 57,9 credits per kjøring.
- **GPT-6 Luna holder på kodegjennomgang, men ikke på planlegging.** På `review` fant Luna Medium alle plantede feil på riktig linje i fem av fem kjøringer for 1,3 credits. Opus 5.5 Low brukte 23,9. På planlegging spurte Luna i to av fem kjøringer bare hva fødselsnummeret skulle brukes til, ikke om personopplysninger. I batch 2 besto Luna Medium også alle sjekkene i `research` for 0,9 credits.
  - _Rettelse 6. oktober:_ Luna-resultatet på planlegging ble vurdert før «fødselsnummer» kom inn i mønsteret for blindsone 1 (`RE_BS1`, #1436). Kjøring 3 og 5 i [failures.psv](golden-baselines/2026-10-01-batch3/failures.psv) nevner fødselsnummer og ville bestått blindsone 1 med dagens sjekk. Transkriptene ble ikke tatt vare på, så vi kan ikke vurdere kjøringene på nytt.

**Tillegg 1. oktober (batch 3b).** Rådata ligger i [2026-10-01-batch3b](golden-baselines/2026-10-01-batch3b/), med klassifisering i [failures.psv](golden-baselines/2026-10-01-batch3b/failures.psv).

- **Luna holder på kodegjennomgang også med ti kjøringer.** GPT-6 Luna Medium fant alle plantede feil i ti av ti kjøringer og oppga riktig linje i ni av ti. I den tiende fant Luna feilen i TSX-fila, men pekte på linja over. Medianen var 1,3 credits per kjøring.
- **`@code-review` blir på Claude Opus 5.5.** GPT-6 Luna Medium holder godt til en rask gjennomgang. Med ti kjøringer besto den 39 av 40 sjekker for 1,3 credits per kjøring, mot 40 av 40 for 23,9 med Opus 5.5 Low. Den eneste bommen var et linjenummer én linje feil, så sjekk linjenumrene mot diffen.
- **Sjekken for rød sone er rettet.** Test 4 godtar nå komma etter «Rød sone» når 🔴 står foran. En setning som «koden er i rød sone, så …» godtas fortsatt ikke. I en ny måling besto GPT-6 Sol Medium alle sjekkene i fem av fem kjøringer, også test 4, for 37,5 credits per kjøring.

## GPT-6 Sol følger ubetingede regler bokstavelig (6. oktober 2026)

En bruker meldte at `@nav-pilot` startet et intervju om personvern og tilgang når den ble bedt om å vurdere en migrering til Jackson 3. Endringen var rent teknisk.

- **Feilen kom bare med riktig klient og modell.** Med standardmodellen i Copilot CLI så vi den ikke i noen av tolv kjøringer, fordelt på fire varianter av prompten. Med OpenCode og GPT-6 Sol, samme agentfil og skill-en kalt med `/jackson-3-migration`, startet agenten intervjuet i alle tre kjøringene i hver av tre målinger.
- **Regler uten vilkår blir fulgt bokstavelig.** Agentfila sa «Always verify privacy …» og «always ask #1 and #2 if the change touches user data». GPT-6 Sol leste en DTO med fnr som «touches user data» og spurte, selv om ingen data, mottaker eller tilgangsvei var ny. Da reglene fikk et konkret vilkår, forsvant spørsmålene. Vilkåret er at endringen legger til eller endrer et felt, en mottaker, et loggpunkt eller hvem som har tilgang. I alle tre kjøringene spurte agenten i stedet om konsumentene tåler det nye formatet.
- **Gjenskap med samme klient og modell før du retter.** En feilrapport om personaen kan ikke avkreftes med en annen klient eller modell. Testoppsettet har fått `--client opencode` for dette.

Rådata ligger i `golden-baselines/2026-10-06-personvern-opencode-gpt-6-sol-*`. [v3-before](golden-baselines/2026-10-06-personvern-opencode-gpt-6-sol-v3-before.txt) og [v3-after](golden-baselines/2026-10-06-personvern-opencode-gpt-6-sol-v3-after.txt) er målingen før og etter endringen. Tre kjøringer per arm er nok til å vise feilen, men for få til å si hvor ofte den skjer.

## Målinger 6. oktober 2026

Batch 4 sammenligner GPT-6.1 Sol og Claude Opus 5.5 med GPT-6 Sol som standardmodell for daglig bruk. Alle armene kjører på Low i Copilot CLI. GPT-6 Sol er kontrollen og er målt på nytt, fordi #1436 endret `agents/nav-pilot.agent.md`.

### Kriteriene ble satt før målingen

- **GPT-6.1 Sol erstatter GPT-6 Sol på `@nav-pilot`** bare hvis den består test 2 i fem av fem kjøringer, minst like ofte som kontrollen består test 3, 4, 5, 7 og 7b, og ikke koster mer enn kontrollen.
- **GPT-6.1 Sol anbefales som personlig standard** bare hvis den ikke er dårligere enn kontrollen på koding, kodegjennomgang, norsk og research.
- **Claude Opus 5.5 Low** må være like god som kontrollen på alt. Kostnaden dokumenteres uansett.
- **Sjekken for test 2 løsnes ikke.** Viser transkriptene at testrepoet nå svarer på personvern, skrives det ned før vurderingen og legges fram for eieren.

### Resultater

Rådata ligger i [2026-10-06-batch4](golden-baselines/2026-10-06-batch4/), og feilene i planleggingen er klassifisert i [failures.psv](golden-baselines/2026-10-06-batch4/failures.psv). Planlegging, koding, norsk og research har fem kjøringer per arm, kodegjennomgang ti. Credits er medianen per kjøring. Alle tallene er målt, ikke anslått.

| Testpakke          | GPT-6 Sol (kontroll) | GPT-6.1 Sol            | Claude Opus 5.5        |
| ------------------ | -------------------- | ---------------------- | ---------------------- |
| Planlegging, t2    | 5/5                  | 2/5                    | 5/5                    |
| Planlegging, t3    | 5/5                  | 4/5                    | 5/5                    |
| Planlegging, t4    | 4/5                  | 3/5                    | 5/5                    |
| Planlegging, t5    | 5/5                  | 5/5                    | 5/5                    |
| Planlegging, credits | 27,2               | 22,7                   | 50,8                   |
| Koding             | 30/30, 24,7 credits  | 30/30, 25,3 credits    | 30/30, 47,9 credits    |
| Kodegjennomgang    | 33/40, 15,4 credits  | 24/40, 17,1 credits    | 40/40, 24,4 credits    |
| Norsk              | 20/20, 15,1 credits  | 20/20, 14,5 credits    | 20/20, 30,8 credits    |
| Research           | 20/20, 13,0 credits  | 20/20, 14,7 credits    | 20/20, 35,8 credits    |

- **GPT-6.1 Sol erstatter ikke GPT-6 Sol på `@nav-pilot`.** Den besto test 2 i to av fem kjøringer. I de tre andre listet den de åpne punktene som påstander uten spørsmålstegn. Testrepoet svarer ikke på personvern, så sjekken er ikke løsnet. Test 3 var også svakere (4/5). Test 4 ble ikke vurdert i to kjøringer fordi test 2 feilet. Kontrollens 4/5 på test 4 er én kjøring der testoppsettet ikke fant noen plan for fase 2, ikke en modellfeil.
- **GPT-6.1 Sol anbefales ikke som personlig standard.** Den holdt på koding, norsk og research, men ikke på kodegjennomgang. I Kotlin-fila nevnte den ikke det svelgede unntaket ved riktig linje i åtte av ti kjøringer. Kontrollen bommet på det én gang.
- **Claude Opus 5.5 Low var minst like god som kontrollen på alle målte sjekker**, men kostet 1,6 til 2,8 ganger så mye per kjøring. Dyrest er den på research: 35,8 credits mot 13,0.
- **Test 7 og 7b og OpenCode-armen ble ikke kjørt.** Testpakkene brukte 2 197 credits, mot et anslag på 1 909. Medregnet 66 credits på testkjøringer på forhånd ble det 2 263. De gjenstående kjøringene ville tatt forbruket over grensen på 2 500 credits (anslaget pluss 25 prosent). Eieren satte denne stoppregelen da målingen startet. Den er ikke en del av kriteriene over. Resultatene for test 7 og 7b kan ikke endre utfallet for GPT-6.1 Sol, som allerede feiler på test 2.

Denne målingen endrer ingen pinner. Om `@nav-pilot` og agentpakkens standard skal endres, avgjøres for seg.

## Målinger 7. oktober 2026

Kodegjennomgangen målte til nå to filer med plantede feil. `review` har fått fire nye sjekker (rv5–rv8). De måler gjennomgang av en branch med åtte filer, prioritering, og om agenten lar være å slå alarm på en fil uten feil. Armene er GPT-6 Luna Medium, GPT-6.1 Sol Low og Claude Opus 5.5 Low, med ti kjøringer hver i Copilot CLI. GPT-6 Sol er ikke med, etter beslutning fra eieren.

### Kriteriene ble satt før målingen

En arm er en akseptabel reservemodell for `@code-review` bare hvis alle fire kravene holder:

1. rv5 (sikkerhet og personvern på riktig linje i riktig fil) består i minst 9 av 10 kjøringer.
2. rv7 (riktig prioritet) består i minst 9 av 10 kjøringer.
3. rv8 (fil uten feil): medianen for funn med høy prioritet er 0, og ingen kjøring har mer enn ett.
4. rv6 (designfeil) består i høyst to kjøringer færre enn Opus 5.5.

9 av 10 og 10 av 10 regnes som likt. Består GPT-6 Luna alle fire, blir den første reservemodell foran GPT-6.1 Sol. Claude Opus 5.5 beholder pinnen med mindre den selv feiler krav 1 eller 2. Da klassifiseres feilene i `failures.psv` før vi konkluderer, fordi feilen da like gjerne kan ligge i fiksturen.

Mønstrene for rv6 utledes fra tre pilotkjøringer med Opus 5.5 Low og låses i en egen commit før hovedkjøringene. Målingen endrer ingen pinner. Et forslag om reservemodell kommer i en egen PR.

### Resultater

Rådata ligger i [2026-10-07-review-suite](golden-baselines/2026-10-07-review-suite/), og hver sjekk som feilet, er klassifisert i [failures.psv](golden-baselines/2026-10-07-review-suite/failures.psv). Piloten og tre kontrollkjøringer av rv8 ligger i [2026-10-07-review-suite-pilot](golden-baselines/2026-10-07-review-suite-pilot/LESMEG.txt). Copilot CLI 1.0.93-4, ti kjøringer per arm. Bruksradene viser at alle tre armene kjørte modellen de oppgir. Credits er medianen per kjøring for hele testpakken (rv1–rv8).

Tabellen viser sjekkens tall først. Tallet i parentes er etter klassifiseringen, der en feil som skyldes sjekken og ikke modellen, regnes som bestått. Mønstrene er ikke endret etter kjøringene.

| Sjekk                       | Claude Opus 5.5 Low | GPT-6 Luna Medium | GPT-6.1 Sol Low |
| --------------------------- | ------------------- | ----------------- | --------------- |
| rv1–rv4 (to filer)          | 40/40               | 36/40             | 25/40           |
| rv5, sikkerhet og personvern | 7/10 (9/10)        | 1/10 (2/10)       | 3/10 (3/10)     |
| rv6, designfeil             | 8/10 (9/10)         | 1/10 (9/10)       | 2/10 (10/10)    |
| rv7, prioritet              | 10/10               | 7/10 (10/10)      | 9/10 (10/10)    |
| rv8, rader med høy prioritet, median (høyest) | 0 (2), etter klassifisering 0 (0) | 0,5 (1), etter klassifisering 0 (1) | 2 (3), etter klassifisering 1 (1) |
| Credits per kjøring         | 58,5                | 3,6               | 37,7            |

Hva klassifiseringen fant:

- **rv5:** GPT-6 Luna og GPT-6.1 Sol nevnte ikke at `accessPolicy.inbound` i nais.yaml slipper inn alle applikasjoner, i åtte og sju av ti kjøringer. Det er modellfeil. Opus nevnte den i alle ti, men oppga to ganger nøkkelen «inbound» i stedet for linjenummer. Én gang pekte Opus på linje 24, der spørringen kjøres, og ikke linje 23, der den settes sammen. Det regnes som modellfeil.
- **rv6:** Mønsteret ble utledet fra tre Opus-svar og kjenner bare ordene Opus brukte. GPT-modellene skrev for eksempel «Retry oppretter nye vedtak. Hvis lagringen lykkes og Kafka-publiseringen feiler …» på riktig linje. Det er samme feil, men med andre ord. Sjekken måler derfor ordvalg mer enn forståelse, og tallene for rv6 bør ikke brukes til å skille modellene.
- **rv7 og rv5:** Luna skrev «parameterbinding» og «settes direkte inn i SQL-strengen». Sjekken for SQL kjenner ikke disse ordene.
- **rv8:** Sjekken teller «blokkerende JDBC-kall» og «høy belastning» i 🟡-rader som høy prioritet. Alle Opus-radene var slike. Ekte 🔴-rader på fila uten feil gjaldt alle samme sak: `log.error(..., e)` kan få med fødselsnummer fra en databasefeil. Luna merket det 🔴 i fire av ti kjøringer, GPT-6.1 Sol i åtte av ti.

Vurdering mot kriteriene:

- **GPT-6 Luna Medium er ikke en akseptabel reservemodell.** Den feiler krav 1 (rv5 1/10, 2/10 etter klassifisering). Med sjekkens tall feiler den også krav 2, 3 og 4. Etter klassifiseringen holder krav 2, 3 og 4.
- **GPT-6.1 Sol Low er ikke en akseptabel reservemodell.** Den feiler krav 1 (rv5 3/10) og krav 3 (median 2, etter klassifisering 1). Krav 2 holder (9/10). Krav 4 holder bare etter klassifiseringen.
- **Claude Opus 5.5 feiler krav 1 med sjekkens tall (7/10).** Etter klassifiseringen er det 9/10, som regnes som likt med 10/10. To av de tre feilene var funn med nøkkel i stedet for linjenummer i nais.yaml. Krav 2 holder (10/10). Pinnen står derfor.
- **Ingen av de to GPT-modellene oppfyller kriteriene.** Rekkefølgen for reservemodeller endres ikke.

Forbruket var 1 002 credits på hovedkjøringene og 227 på piloten og kontrollkjøringene, til sammen 1 229. Stoppgrensen var 1 430.

Før neste måling bør rv6 få et mønster som også dekker GPT-modellenes ordvalg, utledet fra disse transkriptene. Rv8 bør ikke lese «blokkerende» i en 🟡-rad som prioritet.

## Pinner og delegering

Målt mot Copilot CLI 1.0.83-4, 7. september 2026.

### Pinnen gjelder bare på toppnivå

`model:`-feltet i en agents frontmatter blir brukt når agenten startes direkte:

```
copilot --agent research      # kjører på gpt-6-luna, som pinnen sier
```

Blir den samme agenten startet som subagent, arver den forelderens modell, og pinnen leses ikke:

```
copilot --agent nav-pilot --model gpt-5.6-terra -p "start subagenten research"
  -> ● Research (model: gpt-5.6-terra)

copilot --agent nav-pilot --model gpt-5.6-sol -p "start subagenten research"
  -> ● Research (model: gpt-5.6-sol)
```

Det betyr at `@nav-pilot` sin modell i praksis er modellen for hele delegeringstreet. Agentpakkas standard er GPT-6 Sol. Modellporten i `agents/nav-pilot.agent.md` bytter persona ved eskalering til `@nav-pilot-opus`, ikke modell, med mindre noen sier noe annet eksplisitt.

Verktøyet tar imot en modell hvis den som kaller ber om det. Da gjelder den:

```
copilot --agent nav-pilot --model gpt-5.6-sol \
  -p "start subagenten research, be eksplisitt om gpt-5.6-luna"
  -> ● Research (model: gpt-5.6-luna)
```

Det hviler på at modellen velger å oppgi den. En instruks i personaen om å gjøre det er samme slag som soft-sjekk 2b i golden-harnessen, som aldri er innfridd over tre persona-revisjoner og fire modeller. Regn ikke med den.

### Den deterministiske overstyringen

Klientens egen konfigurasjon setter modell per subagent, uavhengig av hva modellen finner på. `~/.copilot/settings.json`:

```json
{
  "subagents": {
    "agents": {
      "research": { "model": "gpt-5.6-luna" }
    }
  }
}
```

`copilot help config` dokumenterer `subagents.agents.<agent-name>` med `model`, `effortLevel` og `contextTier`, der hvert felt også tar `"inherit"`. `/subagents` setter det interaktivt.

**Nøkkelen er filnavnet, ikke `name:` i frontmatteren.** Målt med kontroll:

| Nøkkel                                      | Modell subagenten kjørte på |
| ------------------------------------------- | --------------------------- |
| `research` (filnavnet, `research.agent.md`) | gpt-5.6-luna                |
| `research-agent` (frontmatterens `name:`)   | gpt-5.6-sol                 |
| `tullball` (kontroll)                       | gpt-5.6-sol                 |

Seks av agentene våre har et `name:` som ikke er filnavnet: `accessibility`, `aksel`, `kafka`, `research`, `rust` og `security-champion` heter alle `<navn>-agent` i frontmatteren. Den som setter opp dette fra agentens eget navn får ingen feilmelding, bare ingen effekt.

Nav-pilot skriver ikke klientkonfigurasjon i dag, men skal gjøre det: [beslutning 4.8](nav-pilot-benchmark-og-beslutninger-2026-08.md#48-nav-pilot-skriver-nøkler-den-selv-eier-og-tar-dem-tilbake) ble omgjort 8. september. Nøkler nav-pilot eier skrives og tas tilbake. Fire spørsmål om eierskap, reversering, formatering og synlighet står ubesvart, og ingen kode skrives før de har svar ([#500](https://github.com/navikt/copilot/issues/500)).

### AI-kreditter skiller ikke modeller

Samme agent, samme oppgave, 10,0k input-tokens:

| Modell        | AI Credits |
| ------------- | ---------- |
| GPT-5.6 Luna  | 0,26       |
| GPT-5.6 Sol   | 0,26       |
| Claude Opus 5 | 0,26       |

Kredittene følger tokenforbruk, ikke modellklasse: en nav-pilot-tur på 52k tokens kostet omtrent 18. Tallet CLI-en viser kan altså ikke brukes til å vise gevinsten av et modellbytte.

Pristabellen lenger nede er GitHubs listepriser. Vi har ikke koblet dem mot det Nav faktisk faktureres, og vet ikke hvilken enhet den faktureringen er i. Kostnadsargumentene i dette dokumentet er derfor listepris ganget med et anslått forhold mellom input og output, ikke målt forbruk. Det står også i [Grunnlaget for Luna-byttene](#grunnlaget-for-luna-byttene-august-2026), og gjelder like fullt her.

## Grunnlaget for Luna-byttene (august 2026)

Golden-prompt-harnessen kjørte nav-pilot-personaen mot Claude Sonnet 4.6,
GPT-5.6 Sol, GPT-5.6 Luna og GPT-5.6 Terra på samme oppgave. Tall, metode og
forbehold står i
[benchmarken og beslutningene fra august 2026](nav-pilot-benchmark-og-beslutninger-2026-08.md).
Kortversjonen: ingen av modellene skilte seg signifikant fra Claude Sonnet 4.6
på den ene påkrevde påstanden som ble målt. Målingen viser altså ikke at Luna
er tryggere, den viser at kandidatene ikke lot seg skille med dette utvalget.
Når sikkerhet ikke skiller dem, avgjør kostnad.

Blandet pris under forutsetter **10 input-tokens per output-token. Det er et
anslag, ikke noe vi har målt**, og forholdet varierer med oppgaven.

| Modell            | Input | Output | Blandet $/1M ved 10:1 (anslag) |
| ----------------- | ----- | ------ | ------------------------------ |
| GPT-5.6 Luna      | $0.20 | $1.20  | 0,29                           |
| Claude Haiku 4.5  | $1.00 | $5.00  | 1,36                           |
| GPT-5.3-Codex     | $1.75 | $14.00 | 2,86                           |
| Claude Sonnet 4.6 | $3.00 | $15.00 | 4,09                           |

### Hva som flyttes

- `@research` går fra GPT-5.3-Codex til GPT-5.6 Luna. Agenten er lesetilgang
  alene: verktøylista er `read`, `search`, `web` og lesende GitHub-MCP-kall,
  uten `execute`, `edit` eller `runSubagent`. Den samler inn og oppsummerer,
  den skriver ikke kode. Luna ligger omtrent 90 prosent under Codex blandet.
- De fire malpromptene som sto på Claude Haiku 4.5, går til Luna:
  `ktor-endpoint`, `nextjs-api-route`, `spring-boot-endpoint` og
  `golang-service`. Alle fyller ut en fast mal. Luna er omtrent en femtedel av
  Haiku 4.5 blandet, og omtrent en fjortendedel av Sonnet 4.6.

### Hva som ikke flyttes hit

- Augustmålingen flyttet ikke `@code-review` eller `@accessibility` til Luna.
  De ble opprinnelig vurdert som lesende mønsteranvendere, men begge er
  verktøytunge agenter. `@code-review` har `execute`, mens `@accessibility` har
  `execute` og `edit`. Opus 5.5 ble målt på kodegjennomgang 30. september og fant alle plantede feil på riktig linje på Low, Medium og
  High. Low holder. GPT-6.1 Sol er fallback, deretter GPT-5.3-Codex. `@accessibility` bruker Claude Sonnet 5.5 med Sonnet 5 som fallback.
- `@forfatter` beholder Anthropic-modellen sin. Jobben er å skille bokmål fra
  nynorsk og luke ut norske AI-markører. Målingen sier ingenting om det, og
  gevinsten er nær null mot en kjent nedside.

> To rettelser i ettertid, som ikke endrer beslutningen over. Modellen disse tre
> sto på het Claude Sonnet 4.6; den er trukket tilbake av GitHub og pinnene er
> flyttet til Claude Sonnet 5 (#715). Og `@accessibility` hadde aldri
> `runSubagent`: navnet er ikke et verktøy noen klient kjenner, så grantet var en
> stille null og er fjernet (#689). Argumentet står likevel, siden `execute` og
> `edit` alene gjør agenten verktøytung.

- Resten av GPT-5.3-Codex-pinningene sto urørt etter augustmålingen. Kafka- og
  Rust-agentene samt `kafka-topic` og `nais-manifest` ble flyttet i den
  kontrollerte GPT-6-utrullingen i september 2026.

## Tilgjengelige modeller og bruksområder

Et kuratert utvalg av modellflåten: modellene vi faktisk vurderer, ikke alle
GitHub priser. Prisene under er GitHubs listepriser slik de sto
**3. oktober 2026**, hentet fra `apps/my-copilot/src/lib/model-pricing.ts`,
som dekker hele flåten. De endrer seg uten varsel, så
tallene her har et tidsstempel og ikke evig gyldighet.

| Modell                | Kategori    | Input    | Output   | Best for                                                                                                                                                                                                        |
| --------------------- | ----------- | -------- | -------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Claude Opus 5.5       | Powerful    | $4.00    | $20.00   | Lange agentoppgaver, kodebaseomfattende migreringer, høyrisiko planlegging og sikkerhetskritisk review. Lansert 22. september 2026                                                                              |
| Claude Opus 5         | Powerful    | $5.00    | $25.00   | Dyp resonnering, risikovurdering og sikkerhetskritisk kode med justerbar effort (low/medium/high). Lansert 24. juli 2026                                                                                        |
| Claude Opus 4.7 / 4.8 | Powerful    | $5.00    | $25.00   | Dyp risikovurdering, sikkerhetskritisk kode, kompleks arkitektur. Opus 4.5 og 4.6 falt ut av GitHubs prisliste 5. sep 2026                                                                                      |
| Claude Sonnet 4.6     | Versatile   | $3.00    | $15.00   | Daglig koding, norsk tekst, planlegging                                                                                                                                                                         |
| Claude Sonnet 5.5     | Versatile   | $2.00    | $10.00   | Aksel, tilgjengelighet og norsk tekst                                                                                                                                                                           |
| Claude Sonnet 5       | Versatile   | $2.00    | $10.00   | Fallback for Sonnet 5.5. ⚠️ Kampanjen vi noterte gikk ut 31. aug 2026, og standardprisen er ukjent. Se noten under tabellen                                                                                     |
| Claude Haiku 4.5      | Versatile   | $1.00    | $5.00    | Sjekklister, maler, scaffold-prompts                                                                                                                                                                            |
| GPT-5.3-Codex         | Powerful    | $1.75    | $14.00   | Kodeforståelse, terminal, infrastruktur                                                                                                                                                                         |
| GPT-5.6 Luna          | Lightweight | $0.20    | $1.20    | Raske rutineoppgaver, enkel autofullfør. OpenAI plasserer den i nano-sjiktet fra tidligere GPT-5-familier, men med høy reasoning-rating og justerbar effort                                                     |
| GPT-5.6 Terra         | Versatile   | $2.00    | $12.00   | Allround daglig koding i GPT-familien                                                                                                                                                                           |
| GPT-5.6 Sol           | Powerful    | $4.00    | $20.00   | Tung reasoning over store kodebaser. Listepris; kampanjen gikk ut 3. sep 2026. Lang kontekst over 272K: $8.00 / $30.00                                                                                          |
| GPT-6 Luna            | Lightweight | $0.10    | $0.50    | Raske rutineoppgaver og faste maler. Lang kontekst over 272K: $0.20 / $0.75                                                                                                                                     |
| GPT-6 Sol             | Powerful    | $2.00    | $10.00   | Daglig agentisk koding med validering i flere steg. Lang kontekst over 272K: $4.00 / $15.00                                                                                                                     |
| Gemini 2.5 Pro        | Powerful    | (utgått) | (utgått) | 🚫 Utfaset 31. juli 2026. Gemini 3.1 Pro, som overtok rollen, falt ut av prislista 5. sep 2026. Google har ingen Powerful-modell igjen hos GitHub. Bruk GPT-6 Sol eller Kimi K3 til research over lang kontekst |
| Gemini 3.5 Flash      | Lightweight | $1.50    | $9.00    | Rask og billig for enkle oppgaver                                                                                                                                                                               |
| Gemini 3.8 Flash      | Versatile   | $0.75    | $3.75    | Rask Aksel-scaffolding. Kampanjepris t.o.m. 31. des 2026                                                                                                                                                        |
| Gemini 3.6 Flash      | Versatile   | $0.75    | $3.75    | Agentiske workflows med parallell verktøybruk, men deaktivert i Nav. Kampanjepris t.o.m. 31. des 2026                                                                                                           |
| Kimi K2.7 Code        | Versatile   | $0.95    | $4.00    | Rimeligste alternativ for kode-agent-løkker (open-weight)                                                                                                                                                       |
| Kimi K3               | Powerful    | $3.00    | $15.00   | Rimeligste Powerful-modell på lista (open-weight). Ikke pinnet, ikke målt hos oss                                                                                                                               |

**Kampanjepriser.** GitHub merker enkelte rader med kampanjepris i fotnoter, og
fotnotene følger ikke med når vi synkroniserer pristabellen
([#503](https://github.com/navikt/copilot/issues/503)). Per 31. august 2026
gjelder det:

- **GPT-5.6 Sol: avklart, kampanjen er over.** Kampanjen løp ut 3. september
  2026, og synkroniseringen 4. september hentet listeprisen: $4.00 / $20.00 for
  standardvinduet og $8.00 / $30.00 over 272K. Det er nøyaktig de tallene vi
  regnet oss fram til fra «50 % off», så anslaget traff. Sol har ingen fotnote
  lenger, og tallene i tabellen over er nå publisert listepris, ikke utregning.
- **Gemini 3.6 Flash, Gemini 3.7 Flash og Gemini 3.8 Flash:** $0.75 input og $3.75 output t.o.m. 31. desember 2026. Standardprisen står ikke i fotnoten. Gemini 3.6 og 3.7 er deaktivert i Nav.
  Gemini 3.7 er heller ikke pinnet noe sted hos oss og står derfor ikke i tabellen over.
- **Claude Sonnet 5:** notatet vårt sa kampanje t.o.m. 31. august 2026. GitHubs
  pristabell viser fortsatt $2.00 / $10.00 og har ingen fotnote for Sonnet 5, så
  vi kan hverken bekrefte kampanjen eller finne standardprisen. Tallet skal
  verifiseres mot kilden før det brukes i et regnestykke.
- **GPT-5.6 Luna: ikke kampanjepris.** Sjekket særskilt fordi $0.20 / $1.20 er
  80 % under de $1.00 / $6.00 som stod i juli-artiklene våre, og fordi sju
  pinninger hviler på tallet. OpenAIs egen modellside for `gpt-5.6-luna` oppgir
  $0.20 input, $0.02 cachet input og $1.20 output som listepris, uten
  kampanjeformuleringer. GitHubs pristabell, som er kilden vi synkroniserer fra,
  har ingen fotnote på Luna-raden. Ingen av Luna-pinningene har altså en
  utløpsdato.

Se [prissiden](/priser) for oppdaterte priser på modellene Nav har aktivert.

## Grunnlaget for Sol-byttet (august 2026)

`@security-champion` og `@nav-pilot-opus` går fra Claude Opus 4.6 til GPT-5.6
Sol. **Byttet hviler på pris. Disse to agentene er ikke målt mot noen modell,
heller ikke mot den de flytter fra.** Opus 4.6 falt ut av GitHubs prisliste 5. september 2026; Opus 4.7, 4.8 og 5 ligger på samme $5.00 / $25.00, så
regnestykket under er uendret med en av dem i stedet.

Golden-prompt-harnessen kjørte nav-pilot-personaen mot Claude Sonnet 4.6,
GPT-5.6 Sol, GPT-5.6 Luna og GPT-5.6 Terra. Opus 4.6, modellen disse to
agentene faktisk kjører på i dag, var ikke med i målingen, og harnessen tester
personaen til `nav-pilot`, ikke `@security-champion` og ikke `@nav-pilot-opus`.
Det som ble målt er én regex-påstand på én prompt, og der skilte Sol seg ikke
fra Claude Sonnet 4.6 (Fisher p = 1,00). Det betyr umulig å skille, ikke
likeverdig. Tall, metode og forbehold står i
[benchmarken og beslutningene fra august 2026](nav-pilot-benchmark-og-beslutninger-2026-08.md).

### Prisen var en kampanjepris, og kampanjen er over

GitHub oppga i en fotnote på prissiden (anker
`#user-content-fn-gpt-56-sol-promo`) at GPT-5.6 Sol lå på **50 prosent avslag
til og med 3. september 2026**. Kampanjeprisen for standardvinduet var $2.00
input og $10.00 output, og vi regnet oss fram til $4.00 / $20.00 som full pris.
**Kampanjen er nå over.** Synkroniseringen 4. september hentet listeprisen
$4.00 / $20.00 for standardvinduet og $8.00 / $30.00 over 272K, altså nøyaktig
det utregningen ga. Tallene under er ikke lenger anslag.

Sammenlikningen under er blandet pris per million tokens ved **10 input-tokens
per output-token. Forholdet er et anslag, ikke noe vi har målt**, og varierer
med oppgaven.

| Modell                                     | Input | Output | Blandet $/1M ved 10:1 (anslag) |
| ------------------------------------------ | ----- | ------ | ------------------------------ |
| GPT-5.6 Sol, kampanje (utløpt 3. sep 2026) | $2.00 | $10.00 | 2,73                           |
| Claude Sonnet 4.6                          | $3.00 | $15.00 | 4,09                           |
| Kimi K3                                    | $3.00 | $15.00 | 4,09                           |
| GPT-5.6 Sol, listepris i dag               | $4.00 | $20.00 | 5,45                           |
| Claude Opus 4.7 / 4.8 / 5                  | $5.00 | $25.00 | 6,82                           |

### Hva byttet faktisk sparer

Den riktige sammenlikningen er mot Opus-sjiktet, som er der disse to agentene
kom fra. **Under 272K kontekst** er Sol billigere på begge akser: $4.00 mot
$5.00 og $20.00 mot $25.00. Det er 20 prosent billigere på begge akser, og den
gevinsten overlevde kampanjeslutt.

**Over 272K snur det.** Sol har et eget prisnivå for lang kontekst, $8.00 /
$30.00. Opus-modellene har ikke det og koster $5.00 / $25.00 uansett
kontekstlengde. Sol er altså dyrere enn Opus på begge akser over 272K, ikke
lenger som anslag, men som publisert listepris. Begge disse to agentene er
pitchet mot tung resonnering over store kodebaser, altså nettopp arbeidslasten
som oftest krysser 272K. Prisgevinsten gjelder korte kontekster, ikke lange.

Mot Claude Sonnet 4.6 er bildet et annet, og det skal ikke brukes som
begrunnelse: Sol ligger 33 prosent **over** Sonnet 4.6 blandet. Sonnet 4.6 er
heller ikke modellen disse agentene erstatter.

### Kostnadsregelen peker ikke på Sol

Regelen «når sikkerhet ikke skiller dem, avgjør kostnad» velger ikke Sol. Til
listepris ligger Sol på 5,45 blandet, mens GPT-5.3-Codex ligger på 2,86 og Kimi
K3 på 4,09. Begge er Powerful-modeller, og begge er billigere enn Sol. Fulgt
bokstavelig peker regelen på en av dem, ikke på Sol.

Sammenlikningen med Gemini 3.1 Pro (2,91) sto her fram til 5. september 2026.
Den modellen er borte fra GitHubs prisliste, og Google har ingen Powerful-modell
igjen der. Konklusjonen står likevel: det finnes fortsatt billigere
Powerful-modeller enn Sol.

Sol er valgt fordi den er nærmeste erstatter for Opus i resonneringssjiktet.
**Det er en vurdering, ikke en måling.** Vi har ingen tall som viser at Sol
resonnerer bedre enn GPT-5.3-Codex eller Kimi K3 på oppgavene disse to agentene
gjør, og ingen som viser at den holder Opus-nivået. Argumentet er ubelagt, og
skal leses som det.

Sol er tilgjengelig på Copilot Business. Målt 7. september 2026: `gpt-5.6-sol` står i modellkatalogen klienten henter for en konto med `copilot_plan: business`. Dokumentet sa tidligere at Sol krever Pro+, og motsa seg selv i notatet under, som lister Business blant planene GA-utrullingen dekket.

## Kriterier for å bytte modell

Vi bytter **ikke** modell automatisk når noe nytt lanseres. Et bytte krever at alle tre er oppfylt:

1. **Bekreftet ID.** Modellnavnet i `model:`-feltet er verifisert mot faktisk model picker-oppførsel, ikke bare dokumentasjon.
2. **Kostnad er lik eller lavere.** Eller: ytelsesgevinsten er dokumentert og rettferdiggjør økt kostnad.
3. **Testet på reell oppgave.** Minst én oppgave av typen agenten brukes til, ikke benchmark-tall fra leverandøren.

### Eksempel: GPT-5.3-Codex → GPT-5.6 Terra

| Kriterium    | Status                                                                              |
| ------------ | ----------------------------------------------------------------------------------- |
| Bekreftet ID | ❌ Ikke verifisert i model picker                                                   |
| Kostnad      | ⚖️ Jevnt, se regnestykket under                                                     |
| Testet       | ⚠️ Testet på nav-pilot-personaen (45 kjøringer), ikke på en kodegjennomgangsoppgave |

Terra koster $2.00 mot Codex $1.75 på input, men $12.00 mot $14.00 på output, så hvilken som er billigst avhenger av blandingen. Terra er billigere ved alt under åtte input-tokens per output-token, og 1,6 % dyrere ved 10:1 ($2,91 mot $2,86 per million tokens). **Forholdet 10:1 er et anslag, ikke noe vi har målt.** Konklusjonen tåler hele spennet uansett: forskjellen er noen få prosent i begge retninger, og kostnad er ikke lenger et argument mot Terra.

Regnestykket ser bort fra cachet input, der Codex ligger på $0.175 mot Terras $0.20. Cachet input dominerer agentiske løkker, så det trekker i motsatt retning av output-prisen. Skal noen bytte på kostnad alene, er det den blandingen som må måles først.

**Konklusjon:** ikke byttet. GPT-5.3-Codex beholdes inntil videre.

## Sjekkliste for nye modeller

> **Notat (24. juli 2026, oppdatert 31. august 2026):** Claude Opus 5 (`claude-opus-5`) er lansert av Anthropic og var kandidat til å erstatte Opus 4.6-pinningene på `@nav-pilot-opus` og `@security-champion`. Listeprisen er identisk med Opus 4.8 ($5.00/$25.00), og Anthropic oppgir vesentlig sterkere resonnering (mer enn dobling av Opus 4.8 på Frontier-Bench v0.1). Begge agentene står nå på GPT-5.6 Sol, som er billigere enn Opus-modellene på begge akser under 272K kontekst også etter at kampanjeprisen løp ut 3. september 2026; over 272K er Sol dyrere enn Opus på begge akser til gjeldende listepris. Opus 5 er fortsatt aktuell hvis en måling viser at den tyngre resonneringen er verdt prisforskjellen, men den er ikke testet mot vår egen golden-prompt. Utrullingen i Copilot er gradvis (GA for Pro+/Max/Business/Enterprise 24. juli), så modellen kan mangle i model picker en periode.

Når nye modeller slås på (som nå med Claude Opus 5, GPT-5.6-familien, Kimi K2.7 og Gemini 3.6 Flash):

- [ ] Bekreft eksakt modell-ID i model picker (ikke bare dokumentasjonsnavn)
- [ ] Sammenlign pris mot eksisterende pinnet modell for samme agent
- [ ] Sjekk om modellen er tilgjengelig på riktig Copilot-plan (Pro vs Pro+/Business)
- [ ] Test på en reell oppgave av typen agenten brukes til
- [ ] Oppdater tabell over pinning og begrunnelse i dette dokumentet
- [ ] Oppdater `model:`-feltet i agent/prompt-filen
- [ ] Sjekk `NAV_DISABLED_MODELS` i `apps/my-copilot/src/lib/model-policy.ts`. Prissiden viser alle modeller som aktivert, unntatt dem på denne lista. Står modellen der, fjern den. Slår Nav av en modell, legg den til

## Modell-ID-format

Slik ser navnekonvensjonene i `model:`-feltet ut i dag:

| Modell              | Format                               | Merk                                                                                   |
| ------------------- | ------------------------------------ | -------------------------------------------------------------------------------------- |
| `GPT-5.3-Codex`     | Bindestrek mellom versjon og variant | Fungerer                                                                               |
| `Claude Sonnet 4.6` | Mellomrom                            | Fungerer                                                                               |
| `Claude Opus 4.6`   | Mellomrom                            | Fungerer, men modellen falt ut av GitHubs prisliste 5. sep 2026. Bruk 4.7, 4.8 eller 5 |
| `GPT-5.6 Sol`       | Mellomrom                            | Verifisert gjennom golden-prompt-kjøringene (august 2026)                              |
| `Claude Opus 5`     | Mellomrom                            | Verifisert (GitHub changelog / Anthropic, 24. juli 2026). API-ID `claude-opus-5`       |
| `Gemini 3.5 Flash`  | Mellomrom                            | Fungerer                                                                               |
| `Gemini 3.6 Flash`  | Mellomrom                            | Antatt, ikke verifisert i praksis                                                      |
| `GPT-5.6 Luna`      | Mellomrom                            | Verifisert gjennom golden-prompt-kjøringene (august 2026)                              |
| `GPT-5.6 Terra`     | Mellomrom                            | Verifisert gjennom golden-prompt-kjøringene, men ikke pinnet                           |

Frem til en modell er verifisert i praksis, merkes den som «Antatt» og bør ikke brukes i produksjonspinning.
