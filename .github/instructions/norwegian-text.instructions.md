---
applyTo: "**/*.md"
---

# Norsk tekstkvalitet

Regler for norsk tekst i markdown-filer: agenter, instruksjoner, skills, dokumentasjon og README-er.

For dypere tekstredaksjon, bruk `@forfatter`-agenten. Disse reglene gjelder automatisk ved redigering og code review.

## Klarspråk

Språkloven pålegger offentlige organer å bruke klart, korrekt språk tilpassa mottakerne.

- **Start med poenget.** Konklusjon først, bakgrunn etterpå.
- **Bruk verb, ikke substantiv av verb.** "Vi vurderer" ikke "gjennomføring av en vurdering".
- **Aktiv form.** "Vi bruker X" ikke "det benyttes X".
- **Kort over langt.** Vanlig ord over fancy ord. Kutt fyllord: "i bunn og grunn", "i stor grad", "på mange måter".
- **Skriv for leseren.** Hva trenger leseren å gjøre etter å ha lest dette? Kutt alt som ikke hjelper dem.

## AI-markører

Erstatt eller fjern mønstre som avslører KI-generert tekst.

### Svulstige ord — fjern eller skriv om

"banebrytende", "revolusjonerende", "innovativ", "robust", "helhetlig", "sømløs", "holistisk", "spiller en avgjørende rolle", "representerer et betydelig skritt fremover", "digital transformasjon", "muliggjør", "tilrettelegger for", "effektivisere prosessen", "sette brukeren i sentrum".

### Åpnings- og avslutningsfraser — kutt

"Det er verdt å merke seg", "det er viktig å påpeke", "i dagens verden", "i en verden der", "i en tid der", "la oss utforske", "la oss dykke ned i", "oppsummert kan man si at", "kort sagt", "avslutningsvis", "det finnes flere aspekter ved dette", "resultatene taler for seg selv".

### Retoriske AI-mønstre

- **"Ikke bare X, men også Y"** — skriv om til to setninger eller velg det viktigste
- **"Det handler ikke om X, men om Y"** — si bare Y
- **"I en tid der..." + avsluttende perspektiv** — kutt innramminga
- **Tredeling (trikolon)** i serie — én gang OK, gjentatt er AI-tegn
- **Falsk muntlighet** — uformell åpning som brått skifter til polert byråkratspråk
- **Rettferdiggjøringsavsnitt** — hele avsnitt som forklarer hvorfor noe er viktig uten ny informasjon

### Strukturelle AI-tegn

- Overgangsord som avsnittåpner ("Videre", "Dessuten", "I tillegg") — bruk sjelden
- Overskrifter som alle ender med kolon — varier
- Identisk grammatisk struktur i alle kulepunkter — varier
- Oppsummeringssetning på slutten av seksjoner som gjentar det du allerede har skrevet — kutt
- Tvungen balanse mellom alternativer ("begge har sine fordeler") — velg det beste
- Overforklaring av ting målgruppa allerede vet
- Perfekt mal-struktur (krok → kontekst → helt → resultat → det store bildet → konklusjon) — bryt opp, start med nyheten

### Engelske AI-ord som siver inn i norsk

- "fordype seg i" (delve into) — skriv bare innholdet
- "utnytte" / "leverere" (leverage) — bruk "bruke"
- "understreke" (underscore) — si poenget direkte
- "avgjørende" (crucial) — overbrukt, si hvorfor det er viktig
- "landskap" (landscape) — si "markedet", "feltet", "situasjonen"
- "fremme" (foster) — si hva dere gjør konkret
- "navigere" (navigate) — si "håndtere", "forholde seg til"

## Anglismer

### Unødvendige anglismer — bruk norsk

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

Ikke oversett: image, cluster, node, container, deployment, release, pod, namespace, secret, bug, bugfix, hotfix, patch, edge case, rollback, failover, backup, pipeline, workflow, runtime, framework, middleware, pull request, merge, commit, branch, endpoint, token, scope.

## Sammensatte ord

Bindestrek ved engelsk+norsk:

```
✅ image-bygg, CI-pipeline, deploy-steg, Postgres-operatoren, Kafka-topicet, GitHub-repoet, PR-er
❌ Postgres operatoren, Kafka topicet, GitHub repoet (særskrivingsfeil)
```

## Nav — ikke NAV

"Nav" med stor forbokstav og små bokstaver. Aldri "NAV" (gammelt akronym).

## Overskrifter

- Bare første ord og egennavn med stor bokstav (ikke engelsk stil)
- Dropp kolon på slutten av overskrifter

## Tone

- Skriv som til en kollega, ikke som en pressemelding
- "vi" og "du", ikke "bruker" og "man" i interne dokumenter
- Unngå superlativer og amerikansk stil
- Konsekvent bokmål, ikke bland inn nynorsk
