# ADR-mal for Nav

## Filnavn

`docs/adr/ADR-{nummer}-{kort-tittel}.md`

## Mal

```markdown
# ADR-{nummer}: {Tittel}

**Dato:** YYYY-MM-DD
**Status:** Foreslått | Godkjent | Forkastet | Erstattet av ADR-{n}
**Beslutningstakere:** {team eller personer}

## Kontekst

Beskriv situasjonen som fører til at en beslutning må tas. Inkluder:
- Hva er problemet eller muligheten?
- Hvorfor må vi ta en beslutning nå?
- Hvilke begrensninger gjelder?

## Beslutning

Vi har besluttet å {konkret valg}.

## Alternativer vurdert

### Alternativ A: {navn} ✅ (valgt)

**Beskrivelse:** ...

**Fordeler:**
- ...

**Ulemper:**
- ...

### Alternativ B: {navn}

**Beskrivelse:** ...

**Fordeler:**
- ...

**Ulemper:**
- ...

### Alternativ C: Gjøre ingenting

**Beskrivelse:** Beholde nåværende løsning.

**Fordeler:**
- Ingen endringskostnad

**Ulemper:**
- {konsekvensen av å ikke gjøre noe}

## Nav-spesifikke vurderinger

### Sikkerhet og personvern
- **Dataklassifisering:** Åpen / Intern / Fortrolig / Strengt fortrolig
- **Auth-mekanisme:** ID-porten / Azure AD / TokenX / Maskinporten
- **PII-håndtering:** {hvordan personopplysninger beskyttes}
- **Tilgangsstyring:** {accessPolicy-strategi}

### Plattform (Nais/GCP)
- **Infrastrukturkrav:** {PostgreSQL, Kafka, Redis, etc.}
- **Ressursbehov:** {CPU, minne, replicas}
- **Observerbarhet:** {metrikker, logging, tracing}
- **CI/CD-endringer:** {nye workflows, deploy-strategi}

### Team og organisasjon
- **Berørte team:** {liste over team som påvirkes}
- **Migrasjonsstrategi:** {hvordan vi kommer fra nåtilstand til måltilstand}
- **Tilbakerulling:** {hvordan vi ruller tilbake hvis det feiler}
- **Tidsramme:** {når skal dette være på plass}

## Konsekvenser

### Positive
- ...

### Negative
- ...

### Risiko
| Risiko | Sannsynlighet | Konsekvens | Mitigering |
|--------|--------------|------------|-----------|
| ... | Lav/Middels/Høy | ... | ... |

## Aksjonspunkter

- [ ] {oppgave} — {eier} — {frist}
- [ ] Oppdater Nais-manifest
- [ ] Sett opp observerbarhet
- [ ] Informer berørte team
- [ ] Oppdater dokumentasjon
```

## Tips

- Hold ADR-er korte og fokuserte — én beslutning per ADR
- «Gjøre ingenting» er alltid et alternativ
- Skriv for fremtidige lesere som ikke kjenner konteksten
- Oppdater status når beslutningen er tatt
- Bruk «Erstattet av ADR-{n}» når en beslutning revideres
