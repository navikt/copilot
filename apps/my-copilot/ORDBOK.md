# Ordbok – Copilot-portalen

Terminologi brukt i statistikkdashboardet og verktøykatalogen. Engelske faguttrykk brukes der det ikke finnes et godt norsk alternativ.

## Engelske termer vi beholder

| Engelsk           | Kommentar                                          |
| ----------------- | -------------------------------------------------- |
| agent mode        | Copilots agent-modus — ikke oversett               |
| ask mode          | Copilots spørremodus — ikke oversett               |
| chat              | Copilot Chat                                       |
| CLI               | Command Line Interface                             |
| code review       | Gjennomgang av kode i pull requests                |
| commit            | Git-operasjon — brukes som verb og substantiv      |
| dashboard         | Visualiseringspanel (Grafana, statistikk)          |
| GDPR              | EU-forordning for personvern                       |
| inline            | Inline kodeforslag i editoren                      |
| merge             | Slå sammen en pull request                         |
| pull request (PR) | Endringsforslag i Git                              |
| review            | Gjennomgang — brukes som verb og substantiv        |
| skill             | Copilot-ferdighet — ikke oversett                  |
| sandbox           | Isoleringsmiljø for agenter (cplt)                 |
| tokens            | Tekstenheter AI-modellen bruker (ca. 1 per 4 tegn) |
| prompt injection  | Angrepsteknikk mot AI-agenter                      |
| org policy        | Organisasjonsnivå-regler i GitHub                  |
| inference context | Data sendt til AI-modellen for behandling          |

## Agent-begreper

Begrepsavklaringer for agentisk KI. Disse termene beholder vi på engelsk der det er etablert fagspråk, men de trenger en tydelig definisjon. Skriv «agent» og «agentisk» med liten forbokstav.

| Begrep           | Forklaring                                                                                                                                                          |
| ---------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| agent            | KI som selv planlegger og utfører flere steg for å løse en oppgave — den kaller verktøy, leser resultatet og bestemmer neste steg. Mer enn ren chat.                |
| agentisk KI      | Samlebegrep for KI-systemer som handler på egen hånd mot et mål, i stedet for å svare på ett og ett spørsmål. Brukes som adjektiv: «en agentisk arbeidsflyt».       |
| agency           | Hvor stor handlefrihet agenten har — hvilke verktøy den får bruke og hvilke beslutninger den tar selv. Vi oversetter ikke ordet; bruk «handlefrihet» i norsk tekst. |
| autonomi         | Hvor selvstendig agenten kjører uten at et menneske godkjenner hvert steg. Høy autonomi betyr færre stopp for bekreftelse.                                          |
| harness          | Rammeverket rundt modellen som styrer verktøy, kontekst og sikkerhet — for eksempel sandboxen og reglene agenten kjører innenfor. Ikke oversett.                    |
| excessive agency | Sikkerhetsbegrep: agenten har fått mer handlefrihet, tilgang eller autonomi enn oppgaven krever, og kan gjøre skade. Behold engelsk; forklar ved første bruk.       |

## Norske oversettelser

| Engelsk              | Norsk               | Eksempel i UI                        |
| -------------------- | ------------------- | ------------------------------------ |
| acceptance rate      | aksepteringsrate    | «Aksepteringsrate: 32 %»             |
| accepted             | akseptert           | «Aksepterte forslag»                 |
| active users         | aktive brukere      | «Daglig aktive brukere»              |
| adoption             | adopsjon            | Seksjonstittel: «Adopsjon»           |
| code suggestions     | kodeforslag         | «Genererte forslag», «Kodeforslag»   |
| daily                | daglig              | «Daglige CLI-brukere»                |
| editor               | editor              | «Utviklingsverktøy» (i tab-tittel)   |
| features             | funksjoner          | «Funksjonsbruk»                      |
| generations          | genereringer        | «1 234 genereringer»                 |
| interactions         | interaksjoner       | «Totale interaksjoner»               |
| key metrics          | nøkkeltall          | Seksjonstittel: «Nøkkeltall»         |
| lines of code        | kodelinjer          | «Kodelinjer foreslått vs akseptert»  |
| monthly              | månedlig            | «Månedlig aktive brukere»            |
| overview             | oversikt            | Tab: «Oversikt»                      |
| premium requests     | premiumforespørsler | Tab: «Premiumforespørsler»           |
| programming language | programmeringsspråk | «Statistikk for programmeringsspråk» |
| ranking              | rangering           | Tabellkolonne: «Rangering»           |
| requests             | forespørsler        | «CLI-forespørsler»                   |
| sessions             | sesjoner            | «CLI-sesjoner»                       |
| statistics           | statistikk          | Sidetittel: «Statistikk»             |
| suggested            | foreslått           | «Foreslått lagt til»                 |
| suggestions          | forslag             | «Copilot review-forslag»             |
| token usage          | tokenforbruk        | Undertittel: «Tokenforbruk»          |
| trend                | trend               | «Adopsjonstrender»                   |

## Verktøykatalog

| Engelsk    | Norsk           | Eksempel i UI               |
| ---------- | --------------- | --------------------------- |
| edge cases | grensetilfeller | «Test med grensetilfeller»  |
| examples   | eksempler       | Seksjonstittel: «Eksempler» |
| install    | installer       | «Installer med ett klikk»   |
| scaffold   | lag / opprett   | «Lag Aksel-komponent»       |
| tags       | emneord         | Filteroverskrift: «Emneord» |
| use case   | brukseksempel   | «Se brukseksempler»         |

## Skriveregler

- **Sammensatte ord**: Skriv sammen der det er naturlig: «editorbruk», «kodelinjer», «aksepteringsrate». Bruk bindestrek ved engelsk+norsk: «CLI-brukere», «agent-forespørsler», «PR-er».
- **Korte forklaringer**: HelpText-tooltips skal forklare hva metrikken betyr, ikke hvilke API-felt den kommer fra.
- **Ikke overdriv**: Unngå «Oversikt over...» og «Detaljert oversikt over...» — gå rett på sak.
- **Tall**: Bruk norsk tallformat med mellomrom som tusenskilletegn: «151 354».
- **Prosent**: Skriv «20–40 %» med mellomrom før prosenttegnet.
