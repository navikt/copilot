# Claude Sonnet 5.5 på tilgjengelighetsoppgavene (uu1–uu5), med GPT-6 Luna som sammenligning

Kjørt 12. oktober 2026 med Copilot CLI 1.0.95: `--agent accessibility --model claude-sonnet-5.5 --repeat 5`, uten `--effort`. Bruksradene viser `claude-sonnet-5.5` på Medium for alle kall, så ingen kjøring er forkastet. Transkriptene ligger i [`transkripter/`](transkripter/). Stier til den midlertidige arbeidsmappa er byttet ut med `<arbeidsmappe>`.

Bakgrunn: raden «Aksel, tilgjengelighet og norsk tekst» i `model-policy.ts` sier «Ikke målt i den siste modelltesten». Norsk tekst er alt målt for Sonnet 5.5 (`norsk`, 20/20 1. oktober). Denne kjøringen dekker resten: `@accessibility` på `StatusPanel.tsx`, sjekkene uu1–uu5.

Etter den første kjøringen ble GPT-6 Luna kjørt med samme oppsett (`--model gpt-6-luna`, uten `--effort`, Medium i bruksradene) som sammenligning. Luna er valgt fordi den er den billigste modellen som er standard for tre agenter (`@kafka`, `@research` og `@worker`), og derfor den mest sannsynlige erstatteren for Sonnet på denne raden. Alle 154 bruksrader viser `gpt-6-luna`.

Kjøringene har ikke en `--suite`, så den er ikke med i `summary.json` eller på /modeller.

## Resultater

| Sjekk | Hva den krever | Sonnet 5.5 | GPT-6 Luna |
| --- | --- | --- | --- |
| uu1 | Finner minst tre av fire plantede feil | 5/5 | 5/5 |
| uu2 | Viser til WCAG-kriterier med nummer | 5/5 | 0/5 |
| uu3 | Egendefinert ARIA-rolle: spør eller viser til Aksel før den skriver | 4/5 | 0/5 |
| uu4 | Enkelt spørsmål: svarer direkte, uten underagenter | 5/5 | 5/5 |
| uu5 | Vanlig UU-retting: endrer fila | 5/5 | 5/5 |
| Totalt | | 24/25 | 15/25 |

## Kreditter

| Kjøring | 1 | 2 | 3 | 4 | 5 | Median | Totalt |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Sonnet 5.5 | 29,03 | 28,78 | 28,71 | 26,50 | 26,51 | 28,71 | 139,53 |
| GPT-6 Luna | 3,05 | 3,21 | 3,63 | 2,73 | 3,75 | 3,21 | 16,38 |

## Feilet kjøring, lest for hånd

| Modell | Sjekk | Kjøring | Vurdering |
| --- | --- | --- | --- |
| Sonnet 5.5 | uu3 | 5 | Reell feil. Agenten skrev en egen `role="listbox"` rett inn i fila og nevnte Aksel `Select` eller `RadioGroup` først etterpå. Agentfila sier at egne ARIA-roller skal bekreftes før de skrives (accessibility.agent.md:249–250). |
| GPT-6 Luna | uu2 | 1–5 | Reell feil. Gjennomgangene finner feilene, men ingen av de fem nevner WCAG eller et kriterienummer i det hele tatt. |
| GPT-6 Luna | uu3 | 1–5 | Reell feil. Alle fem skrev en egen `role="listbox"` i fila uten å spørre først. Kjøring 1 sier etterpå at en egen listbox «betyr at vi selv må vedlikeholde tastatur- og skjermleseratferden», men nevner ikke Aksel i svaret. |

## Hva dette viser

- Sonnet 5.5 klarer tilgjengelighetsoppgavene (uu1–uu5): 24 av 25, med én reell feil i én av fem kjøringer på «spør først»-regelen.
- GPT-6 Luna koster om lag en niendedel, men bommer på to av fem sjekker i alle kjøringene: den viser ikke til WCAG og spør ikke før den skriver en egen ARIA-rolle.
- Fem kjøringer per modell er lite, men forskjellen på uu2 og uu3 (9 av 10 mot 0 av 10) er ikke støy. Luna er ikke en erstatter for Sonnet 5.5 på denne raden med dagens agentfil.
