# Claude Sonnet 5.5 på Aksel- og tilgjengelighetsoppgavene

Kjørt 12. oktober 2026 med Copilot CLI 1.0.95: `--agent accessibility --model claude-sonnet-5.5 --repeat 5`, uten `--effort`. Bruksradene viser `claude-sonnet-5.5` på Medium for alle kall, så ingen kjøring er forkastet. Transkriptene ligger i [`transkripter/`](transkripter/). Stier til den midlertidige arbeidsmappa er byttet ut med `<arbeidsmappe>`.

Bakgrunn: raden «Aksel, tilgjengelighet og norsk tekst» i `model-policy.ts` sier «Ikke målt i den siste modelltesten». Norsk tekst er alt målt for Sonnet 5.5 (`norsk`, 20/20 1. oktober). Denne kjøringen dekker resten: `@accessibility` på `StatusPanel.tsx`, sjekkene uu1–uu5.

Kjøringen har ikke en `--suite`, så den er ikke med i `summary.json` eller på /modeller.

## Resultater

| Sjekk | Hva den krever | Bestått |
| --- | --- | --- |
| uu1 | Finner minst tre av fire plantede feil | 5/5 |
| uu2 | Viser til WCAG-kriterier med nummer | 5/5 |
| uu3 | Egendefinert ARIA-rolle: spør eller viser til Aksel før den skriver | 4/5 |
| uu4 | Enkelt spørsmål: svarer direkte, uten underagenter | 5/5 |
| uu5 | Vanlig UU-retting: endrer fila | 5/5 |

Totalt 24 av 25.

## Kreditter

| Kjøring | 1 | 2 | 3 | 4 | 5 |
| --- | --- | --- | --- | --- | --- |
| Credits | 29,03 | 28,78 | 28,71 | 26,50 | 26,51 |

Median 28,71 per kjøring, totalt 139,55.

## Feilet kjøring, lest for hånd

| Sjekk | Kjøring | Vurdering |
| --- | --- | --- |
| uu3 | 5 | Reell feil. Agenten skrev en egen `role="listbox"` rett inn i fila og nevnte Aksel `Select` eller `RadioGroup` først etterpå. Agentfila sier at egne ARIA-roller skal bekreftes før de skrives (accessibility.agent.md:249–250). |

## Hva dette viser

- Sonnet 5.5 klarer Aksel- og tilgjengelighetsoppgavene: 24 av 25, med én reell feil i én av fem kjøringer på «spør først»-regelen.
- Fem kjøringer er lite. Tallet støtter at Sonnet 5.5 står som primærmodell for raden, men sier ikke noe om andre modeller, fordi ingen ble målt mot den samme oppgaven nå.
