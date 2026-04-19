---
name: nav-deep-interview
description: Strukturert intervju som avdekker blinde flekker i Nav-prosjekter — personvern, auth, avhengigheter og observerbarhet
license: MIT
metadata:
  domain: general
  tags: planning requirements interview onboarding
---

# Deep Interview — Nav Project Clarification

Kjør et strukturert intervju for å avdekke blinde flekker *før* implementering starter. Basert på vanlige feil og oversikter i Nav-prosjekter.

## Workflow

1. **Identifiser arketype** — hva slags ting bygges?
2. **Still domene-spesifikke spørsmål** — basert på arketypen
3. **Oppsummer funn** — krav, risiko, ikke-mål
4. **Generer output** — strukturert kravdokument

## Steg 1: Arketype

Still dette spørsmålet først:

> Hva slags ting bygger du?
> - Backend API (Kotlin/Ktor eller Spring Boot)
> - Hendelsekonsument (Kafka / Rapids & Rivers)
> - Frontend for innbygger (Next.js + ID-porten)
> - Frontend for saksbehandler (Next.js + Azure AD)
> - Batchjobb (Naisjob)
> - Fullstack (frontend + BFF + backend)

## Steg 2: Domene-spesifikke spørsmål

Still spørsmål fra **alle fire domener**. Tilpass rekkefølge basert på arketype.

### Personvern og data

Disse spørsmålene glemmes oftest. Still dem først.

| # | Spørsmål | Hvorfor |
|---|----------|---------|
| D1 | Behandler tjenesten personopplysninger? Hvilke kategorier? | Bestemmer dataklassifisering og lagringsregler |
| D2 | Hvem har tilgang til dataene — innbygger, saksbehandler, system? | Bestemmer auth og tilgangskontroll |
| D3 | Hva er formålet med behandlingen? (Hjemmel) | Nødvendig for GDPR-vurdering |
| D4 | Hvor lenge skal data lagres? Finnes det sletteregler? | Påvirker database-design og retensjon |
| D5 | Skal data deles med andre tjenester? Hvilke? | Påvirker API-design og accessPolicy |
| D6 | Trenger dere audit-logging av hvem som har sett/endret data? | Påkrevd for sensitive personopplysninger |

Se [data-classification.md](./references/data-classification.md) for Navs dataklassifiseringsnivåer.

### Plattform og autentisering

| # | Spørsmål | Hvorfor |
|---|----------|---------|
| P1 | Hvem initierer forespørsler — bruker, annen tjeneste, batch, ekstern? | Bestemmer auth-mekanisme |
| P2 | Hvilke andre tjenester kaller dere? Hvilke cluster? | Bestemmer outbound accessPolicy og token exchange |
| P3 | Er tjenesten eksponert eksternt (internett) eller bare internt? | Bestemmer ingress og nettverkspolicy |
| P4 | Hva skjer når en avhengighet er nede? | Påvirker retry-strategi og circuit breaker |
| P5 | Trenger dere asynkron kommunikasjon (hendelser)? | Kafka-oppsett eller ikke |
| P6 | Finnes det eksisterende tjenester dere kan gjenbruke? | Unngå duplikering |

### Observerbarhet og drift

| # | Spørsmål | Hvorfor |
|---|----------|---------|
| O1 | Hvilke forretningsmetrikker viser at tjenesten fungerer? | Definerer Prometheus-metrikker |
| O2 | Hva skal trigge varsling? | Definerer alerting-regler |
| O3 | Hvordan vet dere at en deploy gikk bra? | Smoke test-strategi |
| O4 | Hvem er on-call? Finnes det en vaktordning? | Påvirker varslings-setup |
| O5 | Forventet trafikkmønster? (Jevnt, kontor­tid, sesong?) | Påvirker skalering og ressurser |

### Team og prosess

| # | Spørsmål | Hvorfor |
|---|----------|---------|
| T1 | Nytt prosjekt eller utvidelse/modernisering av eksisterende? | Påvirker scaffold vs. migrasjonsstrategi |
| T2 | Avhenger dere av andre team? Hvilke? | Koordineringsbehov |
| T3 | Er det en deadline (regulatorisk, politisk, annet)? | Påvirker scope og prioritering |
| T4 | Har teamet erfaring med stakken? | Påvirker kompleksitetsnivå |

### Modernisering og migrasjon

Still disse spørsmålene hvis T1 avdekker modernisering/refaktorering:

| # | Spørsmål | Hvorfor |
|---|----------|---------|
| M1 | Hva finnes i dag — hvilken teknologi, arkitektur og datamodell? | Kartlegger utgangspunktet |
| M2 | Hva er feil med dagens løsning? (Ytelse, vedlikeholdbarhet, sikkerhet?) | Avklarer motivasjon og prioritet |
| M3 | Må gammel og ny løsning kjøre parallelt? Hvor lenge? | Bestemmer migreringsstrategi (big bang vs. gradvis) |
| M4 | Finnes det andre team eller tjenester som konsumerer API-er/hendelser som endres? | Koordineringsbehov og bakoverkompatibilitet |
| M5 | Hva er rollback-planen hvis migreringen feiler? | Må defineres FØR implementering starter |
| M6 | Hvordan vet dere at migreringen er ferdig? (Exit criteria) | Definerer når gammel løsning kan dekommisjoneres |
| M7 | Finnes det data som må migreres? Hvor mye? | Påvirker migreringspipeline og nedetid |
| M8 | Har dere karakteriseringstester som låser dagens adferd? | Nødvendig for trygg refaktorering |

## Steg 3: Sjekk blinde flekker

Etter intervjuet, sjekk om noen av disse vanlige blinde flekkene ble avdekket. Hvis ikke — still oppfølgingsspørsmål.

Se [blind-spots.md](./references/blind-spots.md) for en komplett liste over vanlige oversikter.

**Kritiske blinde flekker (still alltid):**

- [ ] Er auth-mekanismen verifisert mot caller-typen?
- [ ] Er `accessPolicy.inbound` definert i Nais-manifestet?
- [ ] Er PII-logging unngått?
- [ ] Er HikariCP pool-størrelse tilpasset container (3-5, ikke 10)?
- [ ] Er det satt opp structured logging (JSON)?
- [ ] Finnes det en strategi for dependency failure?

**Ekstra blinde flekker for modernisering (still hvis T1 = modernisering):**

- [ ] Er bakoverkompatibilitet vurdert? (Kan gammel kode lese nytt skjema?)
- [ ] Er rollback-plan definert og testet?
- [ ] Er exit criteria for migreringen definert?
- [ ] Er berørte konsumenter identifisert og informert?
- [ ] Er dekommisjoneringsplan for gammel løsning laget?
- [ ] Er feature toggle satt opp for gradvis utrulling?
- [ ] Er rekonsiliering planlagt (sammenligning av gammel og ny)?

### Endringskonsekvensanalyse (still alltid for brownfield)

Kartlegg påvirkning systematisk. Disse spørsmålene avdekker risiko som ofte oppdages for sent:

| # | Spørsmål | Formål |
|---|----------|--------|
| K1 | Hvem kaller dine API-er i dag? (sjekk accessPolicy.inbound) | Identifiser direkte konsumenter |
| K2 | Hvem konsumerer dine Kafka-hendelser? | Finn downstream-avhengigheter |
| K3 | Har andre tjenester tilgang til databasen din? | Avdekk delt database-risiko |
| K4 | Påvirker endringen data som vises til innbyggere? | Brukerreise-konsekvens |
| K5 | Påvirker endringen data som brukes til utbetaling? | ⚠️ Høy risiko — krever ekstra review |
| K6 | Finnes det dashboards eller varsler som må oppdateres? | Operasjonell påvirkning |
| K7 | Krever endringen koordinert deploy med andre team? | Logistisk risiko |
| K8 | Finnes det API-kontrakter (eksplisitte eller implisitte) som endres? | Kontraktsbrudd |
| K9 | Hva er testtilstanden for koden som endres? (dekning, type tester) | Endringssikkerhet |
| K10 | Hvordan verifiserer dere at endringen fungerer i produksjon? | Observerbarhet |

## Steg 4: Output

Generer et strukturert kravdokument:

```markdown
# Prosjekt: [navn]

## Arketype
[Backend API / Hendelsekonsument / Frontend / ...]

## Scope
- [Hva som skal bygges]
- [Ikke-mål: hva som er utenfor scope]

## Dataklassifisering
- Nivå: [Åpen / Intern / Fortrolig / Strengt fortrolig]
- PII-kategorier: [fnr, navn, helseopplysninger, ...]
- Hjemmel: [...]

## Arkitekturbeslutninger
| Beslutning | Valg | Begrunnelse |
|-----------|------|-------------|
| Auth | [mekanisme] | [hvorfor] |
| Kommunikasjon | [sync/async] | [hvorfor] |
| Database | [PostgreSQL/ingen/BigQuery] | [hvorfor] |
| Observerbarhet | [metrikker] | [hva måles] |

## Migrasjonsstrategi (kun ved modernisering)
| Aspekt | Beslutning |
|--------|-----------|
| Endringstype | [Nybygg / Modernisering / Refaktorering] |
| Utrulling | [Big bang / Gradvis med toggle / Parallellkjøring] |
| Rollback | [Toggle av / Revert migrasjon / Beholde gammel] |
| Exit criteria | [Når er migreringen ferdig?] |
| Dekommisjonering | [Plan for å fjerne gammel løsning] |

## Identifiserte risikoer
1. [risiko + foreslått mitigering]

## Endringskonsekvenser (kun ved brownfield)
| Berørt komponent | Type påvirkning | Risiko | Handling |
|-----------------|----------------|--------|---------|
| [tjeneste/team] | [API/Kafka/DB/UI] | [Lav/Middels/Høy] | [Informere/Migrere/Teste] |

## Teststrategi
| Lag | Testnivå | Hva dekkes |
|-----|----------|-----------|
| [Forretningslogikk] | [Unit] | [Beregninger, validering] |
| [API] | [Slice/integrasjon] | [Ruting, feilkoder, auth] |
| [Database] | [Integrasjon] | [CRUD, migreringer] |
| [Brukerreise] | [E2E] | [Kritiske flyter] |

## Avhengigheter
- [team/tjeneste + hva som trengs]

## Neste steg
- [ ] [konkret aksjonspunkt]
```

## Boundaries

### ✅ Always

- Still personvernspørsmål først — de glemmes oftest
- Verifiser auth-mekanisme mot caller-type
- Dokumenter ikke-mål eksplisitt
- Identifiser minst én risiko

### ⚠️ Ask First

- Foreslå arkitektur som avviker fra teamets eksisterende stack
- Anbefale å endre auth-mekanisme på eksisterende tjenester

### 🚫 Never

- Anta at auth ikke trengs
- Hoppe over personvernspørsmål
- Foreslå at PII kan logges
- Ignorere spørsmål om avhengigheter til andre team
