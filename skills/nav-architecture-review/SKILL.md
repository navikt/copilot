---
name: nav-architecture-review
description: Generer Architecture Decision Records (ADR) med flerperspektiv-review tilpasset Nav
license: MIT
metadata:
  domain: general
  tags: architecture adr review planning
---

# Architecture Review — ADR Generator

Generer et Architecture Decision Record (ADR) med review fra tre perspektiver: arkitektur, sikkerhet og plattform. Følger Navs Architecture Advice Process.

## Workflow

1. **Forstå konteksten** — hva er endringen og hvorfor?
2. **Evaluer fra tre perspektiver** — arkitektur, sikkerhet, plattform
3. **Identifiser alternativer** — minst to alternativer + valgt løsning
4. **Generer ADR** — formelt dokument med Nav-spesifikke vurderinger
5. **Liste aksjonspunkter** — hva må gjøres for å realisere beslutningen

## Steg 1: Kontekst

Still disse spørsmålene:

- Hva er endringen? (én setning)
- Hvorfor gjøres den? (forretningsbehov, teknisk gjeld, regulatorisk)
- Hva er konsekvensen av å *ikke* gjøre noe?
- Hvilke team påvirkes?

## Steg 2: Fler-perspektiv-review

Evaluer fra tre perspektiver. For hvert perspektiv: identifiser bekymringer, risiko og anbefalinger.

### Perspektiv 1: Arkitektur

- Passer dette i Navs overordnede arkitektur?
- Finnes det enklere alternativer?
- Introduserer det unødvendig kompleksitet?
- Er det i tråd med team-autonomi-prinsippet?
- Gjenbruker det eksisterende plattform-kapabiliteter?
- **Navs prinsipper:** Team First, essential complexity, Product Development

### Perspektiv 2: Sikkerhet

> **Dypere sikkerhetsgjennomgang:** For trusselmodellering, OWASP-sjekklister og compliance-vurderinger,
> delegér til `@security-champion-agent` som har komplett sikkerhetsmateriale.

- Hvilke data behandles? Klassifiseringsnivå?
- Er autentisering og autorisasjon riktig?
- Er tilgangsstyring minimalt nødvendig (least privilege)?
- Er PII beskyttet (logging, lagring, transit)?
- Følger dette Navs Golden Path for sikkerhet?

### Perspektiv 3: Plattform

- Fungerer dette på Nais (Kubernetes/GCP)?
- Er ressurskrav realistiske?
- Er observerbarhet ivaretatt?
- Er CI/CD-pipeline enkel og vedlikeholdbar?
- Er det avhengigheter til on-prem eller legacy?

### Perspektiv 4: Migrasjon (kun ved endring av eksisterende system)

- Er endringen bakoverkompatibel? Kan gammel kode kjøre med nytt skjema?
- Finnes det en rollback-plan som ikke medfører datatap?
- Er det definert exit criteria for når migreringen er ferdig?
- Er feature toggle satt opp for gradvis utrulling?
- Er berørte konsumenter (andre team/tjenester) identifisert og informert?
- Er det satt opp rekonsiliering for å verifisere datakonsistens?
- Er dekommisjoneringsplan for gammel løsning definert?
- Er migrasjons-observerbarhet på plass (gammel vs ny path, avviksteller)?

## Steg 3: Alternativer

Identifiser minst to alternativer:

| Alternativ | Fordeler | Ulemper | Nav-vurdering |
|-----------|---------|---------|---------------|
| A: [valgt] | ... | ... | ... |
| B: [forkastet] | ... | ... | ... |
| C: Gjøre ingenting | ... | ... | ... |

## Steg 4: Generer ADR

Bruk malen fra [adr-template.md](./references/adr-template.md).

```markdown
# ADR-{nummer}: {tittel}

**Dato:** {dato}
**Status:** Foreslått | Godkjent | Forkastet | Erstattet av ADR-{n}
**Beslutningstakere:** {team/personer}

## Kontekst

{Hva er problemet/muligheten? Hvorfor må vi ta en beslutning nå?}

## Beslutning

{Hva har vi besluttet å gjøre?}

## Alternativer vurdert

### Alternativ A: {navn} (valgt)
- **Fordeler:** ...
- **Ulemper:** ...
- **Nav-vurdering:** ...

### Alternativ B: {navn}
- **Fordeler:** ...
- **Ulemper:** ...
- **Nav-vurdering:** ...

## Nav-spesifikke vurderinger

### Sikkerhet
- Dataklassifisering: {nivå}
- Auth-mekanisme: {valgt mekanisme}
- PII-håndtering: {strategi}

### Plattform
- Nais-konfigurasjon: {endringer}
- Ressursbehov: {estimat}
- Observerbarhet: {strategi}

### Team-påvirkning
- Berørte team: {liste}
- Migrasjonsstrategi: {plan}
- Tilbakerulle-strategi: {plan}

### Migrasjon (ved endring av eksisterende system)
- Bakoverkompatibilitet: {vurdering}
- Utrullingsstrategi: {big bang / gradvis / parallell}
- Feature toggle: {toggle-navn og strategi}
- Rollback-trigger: {hva utløser rollback}
- Exit criteria: {når er migreringen ferdig}
- Dekommisjonering: {plan for gammel løsning}

## Konsekvenser

### Positive
- ...

### Negative
- ...

### Risiko
- ...

## Aksjonspunkter

- [ ] {konkret oppgave med eier}
```

## Steg 5: Aksjonspunkter

Avslutt alltid med konkrete aksjonspunkter:

- [ ] Implementer beslutningen
- [ ] Oppdater Nais-manifest
- [ ] Oppdater CI/CD-pipeline
- [ ] Informer berørte team
- [ ] Oppdater dokumentasjon
- [ ] Sett opp observerbarhet

## Navs arkitekturprinsipper

Se [nav-principles.md](./references/nav-principles.md) for detaljerte prinsipper.

**Kort oppsummert:**

1. **Team First** — Autonome team med sirkel av autonomi
2. **Essensiell kompleksitet** — Unngå accidental complexity
3. **Produktutvikling** — Kontinuerlig utvikling, produktorganisert gjenbruk
4. **DORA-metrikker** — Mål og forbedre team-ytelse
5. **Architecture Advice Process** — Søk råd fra berørte parter, men ta beslutningen selv

## Teknisk gjeld-vurdering

Bruk denne strukturen til å identifisere, dokumentere og prioritere teknisk gjeld. Spesielt nyttig ved modernisering og refaktorering.

### Identifisering

Still disse spørsmålene for å finne teknisk gjeld:

| Område | Spørsmål |
|--------|----------|
| Avhengigheter | Finnes det utdaterte avhengigheter med kjente sårbarheter? |
| Arkitektur | Er det moduler med uklare ansvarsområder eller tett kobling? |
| Kode | Finnes det duplisert logikk, store klasser/filer, eller manglende abstraksjoner? |
| Tester | Er testdekningen lav i kritisk forretningslogikk? Mangler integrasjonstester? |
| Infrastruktur | Brukes utdatert base image, mangler observerbarhet, eller er CI/CD treg? |
| Dokumentasjon | Mangler ADR-er for viktige beslutninger? Er runbooks utdaterte? |
| Sikkerhet | Er auth-mekanismen oppdatert? Logges PII? |

### Prioriteringsmodell

Prioriter gjeldsposter med tre dimensjoner:

```
Prioritet = Alvorlighetsgrad × Frekvens × Nedslagsfelt

Alvorlighetsgrad (1-3):
  1 = Irriterende men håndterbart
  2 = Bremser utvikling merkbart
  3 = Blokkerer ny funksjonalitet eller utgjør sikkerhetsrisiko

Frekvens (1-3):
  1 = Sjeldent (årlig)
  2 = Regelmessig (månedlig)
  3 = Ofte (ukentlig eller daglig)

Nedslagsfelt (1-3):
  1 = Én komponent
  2 = Flere komponenter i samme tjeneste
  3 = På tvers av tjenester/team
```

### Output: Gjeldstabell

```markdown
| # | Gjeldspost | Alv. | Frekv. | Nedslagsfelt | Prioritet | Anbefalt tiltak |
|---|-----------|------|--------|-------------|-----------|----------------|
| G1 | Utdatert auth-bibliotek | 3 | 3 | 2 | 18 | Oppgrader til nyeste navikt/oasis |
| G2 | Manglende testdekning i beregningsmodul | 2 | 3 | 1 | 6 | Skriv karakteriseringstester |
| G3 | Duplisert valideringslogikk | 1 | 2 | 2 | 4 | Ekstraher til felles modul |
```

**Leverabel:** Gjeldstabell med prioritert rekkefølge og anbefalt tiltak per post.

## Boundaries

### ✅ Always

- Inkluder minst to alternativer (inkludert «gjøre ingenting»)
- Vurder fra alle tre perspektiver
- Dokumenter Nav-spesifikke vurderinger (auth, data, plattform)
- Avslutt med konkrete aksjonspunkter

### ⚠️ Ask First

- ADR som påvirker andre teams tjenester
- Beslutninger som avviker fra Navs standardmønstre
- Introduksjon av ny teknologi

### 🚫 Never

- Fatt beslutning uten å vurdere sikkerhet
- Ignorer plattform-konsekvenser
- Hopp over alternativer — det finnes alltid minst to valg
