---
name: token-saver
description: Reduser token- og kostforbruk i agentøkter. Bruk når du vil komprimere shell-output, vurdere RTK, begrense tool-bruk eller stramme inn agent-workflows uten å bryte parsing eller CI.
license: MIT
metadata:
  domain: general
  tags: [token-optimization, cost-optimization, rtk, shell-output, ci-safety, agent-workflow, nav-pilot]
---

# Token-saver — balansert playbook

Mål: mindre støy, lavere kost og samme kvalitet.

## Bruk når

- du vil komprimere menneskelesbar shell-output
- du vurderer RTK eller lignende filterlag
- du vil redusere tool-/modellbruk uten å svekke sikkerhet eller CI

For ren svarkomprimering uten workflow-endringer, bruk også `$terse-mode`.

## Kjernegrep

1. Én oppgave per tråd.
2. Tool-first: bruk deterministiske verktøy før bred resonnering.
3. Kort output som standard; utvid bare ved behov.
4. Hold modell, verktøy og konfig stabile gjennom sesjonen.
5. Aktiver bare verktøy som trengs.

## Trygg RTK-adopsjon

1. Start med opt-in, ikke global default.
2. Wrap bare menneskelesbar, interaktiv output.
3. Filtrer aldri parser-sensitive flyter:
   - strict JSON
   - stdout brukt av scripts/CI
   - kommandoer med kontraktsfestet linjeformat
4. Legg filtrering i launch-/provider-laget, ikke spredt i mange kommandoer.
5. Behold fallback til normal kjøring hvis verktøyet mangler.
6. Mål effekt før bredere utrulling.

## Juster filtre underveis

1. Start med få, trygge filtre for høyfrekvente kommandoer.
2. Unngå aggressive caps som skjuler review- eller feilkontekst.
3. Revider filtre når repoets arbeidsflyt endrer seg.
4. Bruk `rtk discover` for å finne hvor RTK faktisk kan spare mest.
5. Bruk `rtk gain` for å se om filtrene faktisk gir verdi.

## Aldri gjør dette

- Aldri optimaliser på bekostning av sikkerhetskritisk tydelighet.
- Aldri filtrer outputs som brukes av automasjon/parsing.
- Aldri gjør irreversible handlinger mindre tydelige.

## Verifiser

```bash
rtk discover
rtk gain
```

Kjør deretter repoets standard verifisering med og uten RTK der det er relevant.

## Mål på effekt

- Færre turns per sammenlignbar oppgave.
- Redusert støy i shell-output.
- Ingen økning i CI-feilrate.
- Færre unødvendige modell-eskaleringer.

## Modellvalg

- Bruk lett modell for discovery, sammendrag og rutineoppgaver.
- Bruk sterkere modell bare ved sikkerhet, tunge tradeoffs eller kompleks arkitektur.

## Sjekkliste

1. Er scope tydelig og avgrenset?
2. Er bare nødvendige verktøy aktivert?
3. Er outputformat kort og handlingsorientert?
4. Er RTK kun brukt der menneskelesbar komprimering er trygg?
5. Er verifisering kjørt før ferdigmelding?
