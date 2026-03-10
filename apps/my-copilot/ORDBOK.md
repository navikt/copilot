# Ordbok – Copilot Statistikk

Terminologi brukt i statistikkdashboardet. Engelske faguttrykk brukes der det ikke finnes et godt norsk alternativ.

## Engelske termer vi beholder

| Engelsk           | Kommentar                                          |
| ----------------- | -------------------------------------------------- |
| agent mode        | Copilots agent-modus — ikke oversett               |
| ask mode          | Copilots spørremodus — ikke oversett               |
| chat              | Copilot Chat                                       |
| CLI               | Command Line Interface                             |
| code review       | Gjennomgang av kode i pull requests                |
| inline            | Inline kodeforslag i editoren                      |
| merge             | Slå sammen en pull request                         |
| pull request (PR) | Endringsforslag i Git                              |
| review            | Gjennomgang — brukes som verb og substantiv        |
| tokens            | Tekstenheter AI-modellen bruker (ca. 1 per 4 tegn) |

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

## Skriveregler

- **Sammensatte ord**: Skriv sammen der det er naturlig: «editorbruk», «kodelinjer», «aksepteringsrate». Bruk bindestrek ved engelsk+norsk: «CLI-brukere», «agent-forespørsler», «PR-er».
- **Korte forklaringer**: HelpText-tooltips skal forklare hva metrikken betyr, ikke hvilke API-felt den kommer fra.
- **Ikke overdriv**: Unngå «Oversikt over...» og «Detaljert oversikt over...» — gå rett på sak.
- **Tall**: Bruk norsk tallformat med mellomrom som tusenskilletegn: «151 354».
- **Prosent**: Skriv «20–40 %» med mellomrom før prosenttegnet.
