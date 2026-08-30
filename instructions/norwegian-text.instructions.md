---
applyTo: "**/*.md"
---

# Norsk tekstkvalitet

Regler for norsk tekst i markdown-filer: agenter, instruksjoner, skills, dokumentasjon og README-er.

Språknøytrale skriveregler (lengde, tetthet, AI-markører, tegnsetting) står i `output-style.instructions.md` og gjelder allerede. Denne instruksjonen dekker bare det som er spesifikt for norsk.

For dypere tekstredaksjon, bruk `@forfatter`-agenten. Disse reglene gjelder automatisk ved redigering og code review.

## Klarspråk

Språkloven pålegger offentlige organer å bruke klart, korrekt språk tilpassa mottakerne.

- **Start med poenget.** Konklusjon først, bakgrunn etterpå.
- **Bruk verb, ikke substantiv av verb.** "Vi vurderer" ikke "gjennomføring av en vurdering".
- **Aktiv form.** "Vi bruker X" ikke "det benyttes X".
- **Kort over langt.** Vanlig ord over fancy ord. Kutt fyllord: "i bunn og grunn", "i stor grad", "på mange måter".
- **Skriv for leseren.** Hva trenger leseren å gjøre etter å ha lest dette? Kutt alt som ikke hjelper dem.

## Engelske AI-ord som siver inn i norsk

Direkte oversettelser fra engelsk som er langt vanligere i KI-generert norsk enn i vanlig norsk:

- "fordype seg i" (delve into): skriv bare innholdet
- "utnytte" / "leverere" (leverage): bruk "bruke"
- "understreke" (underscore): si poenget direkte
- "avgjørende" (crucial): overbrukt, si hvorfor det er viktig
- "landskap" (landscape): si "markedet", "feltet", "situasjonen"
- "fremme" (foster): si hva dere gjør konkret
- "navigere" (navigate): si "håndtere", "forholde seg til"
- "rike" / "sfære" (realm): si "område", "felt"
- "effektivisere" (streamline): si hva som blir enklere

## Anglismer

### Unødvendige anglismer, bruk norsk

| Anglisme | Norsk alternativ |
|----------|-----------------|
| "adressere et problem" | "løse", "fikse", "ta tak i" |
| "delivere" | "levere" |
| "ta eierskap til" | "ha ansvar for" |
| "per dags dato" | "nå", "i dag" |
| "involvere" (overbrukt) | "ta med", "inkludere" |
| "ha en god dialog" | "snakke med", "samarbeide med" |
| "i henhold til" (overbrukt) | "etter", "ifølge" |
| "basert på" (overbrukt) | "ut fra", "med utgangspunkt i" |

### Behold engelsk fagspråk

Ikke oversett: image, cluster, node, container, release, pod, namespace, secret, bug, bugfix, hotfix, patch, edge case, rollback, failover, backup, pipeline, workflow, runtime, framework, middleware, pull request, merge, commit, branch, endpoint, token, scope.

`deployment` som substantiv beholdes på engelsk (ikke "utrulling"). Verbet `deploye` er OK, og "rulle ut" er OK som verb.

## Sammensatte ord

Bindestrek ved engelsk+norsk:

```
✅ image-bygg, CI-pipeline, deploy-steg, Postgres-operatoren, Kafka-topicet, GitHub-repoet, PR-er
❌ Postgres operatoren, Kafka topicet, GitHub repoet (særskrivingsfeil)
```

## Nav, ikke NAV

"Nav" med stor forbokstav og små bokstaver. Aldri "NAV" (gammelt akronym).

## Overskrifter

Bare første ord og egennavn med stor bokstav, ikke engelsk tittelstil. (Kolon på slutten av overskrifter er forbudt av `output-style.instructions.md`.)

## Tone

- Skriv som til en kollega, ikke som en pressemelding
- "vi" og "du", ikke "bruker" og "man" i interne dokumenter
- Unngå superlativer og amerikansk stil
- Konsekvent bokmål, ikke bland inn nynorsk
- Vanlige nynorsk-feil fra KI: -ingar (skal være -inger), -leg (skal være -lig), kv- (skal være hv-), ei-/eig- (skal være e-/eg-), medan→mens, vorte→blitt, vart→ble, berre→bare, mykje→mye, difor→derfor
- Svensk som siver inn: engångs-→engangs-, ändring→endring (å/ä der bokmål har a/e)
