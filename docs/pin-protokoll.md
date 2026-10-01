# Protokoll for modellmålinger

En modellpinne endres ikke på grunnlag av golden-suiten eller en utforskende
pilot. Skriv ned sammenligning, risikogrense og utvalgsstørrelse **før** en
beslutningsmåling. Se [#584](https://github.com/navikt/copilot/issues/584).

## Utforskende pilot

Beslutningen 1. oktober 2026 er å beholde GPT-6 Sol som standard for
`@nav-pilot`. Piloten i [#1409](https://github.com/navikt/copilot/issues/1409)
undersøker to spørsmål hver for seg:

| Spørsmål | Sammenligning | Omfang |
| --- | --- | --- |
| Hvor mye arbeid trenger orkestratoren? | GPT-6 Sol Low mot GPT-6 Sol Medium | Planlegging, oppgavefordeling og kontroll, ikke bare kodesvaret |
| Når holder en billigere worker? | GPT-6 Luna Medium mot GPT-6 Sol Medium | Avgrenset Kotlin/Ktor- og TypeScript/Next.js-koding og feilsøking |

Ingen av armene innebærer å bytte standardmodellen til `@nav-pilot`.
Sammenlign ikke Luna med Sol samtidig som innsatsnivået endres. Et
subagentkall kan arve foreldermodellen selv om worker-agenten har en annen
pinne. Før målingen må vi enten starte workeren direkte eller verifisere en
eksplisitt overstyring i bruksradene. En arm der modellen eller innsatsnivået
ikke kan bekreftes, kan ikke sammenlignes.

Første runde er utforskende og har et tak på **1 500 AI credits**, inkludert
retries og subagenter. Ikke start et nytt forsøk uten et kostnadsanslag som
får plass i gjenværende budsjett. Et enkelt modellkall kan likevel overskride
anslaget; et absolutt tak krever en grense hos leverandøren. Begynn med én
Kotlin- og én TypeScript-oppgave for worker-sammenligningen. Når fiksturene
er validert, kan vi sammenligne de to modellene på fem uavhengige forsøk
per oppgave. Undersøk Sol Low mot Medium på egne orkestreringsoppgaver
bare hvis det er budsjett igjen. Dette kan avdekke feil og anslå kostnad.
Verken to oppgaver eller fem forsøk per arm gir grunnlag for modellrangering
eller ny pinne. Ikke publiser piloten som et sammenlignbart resultat på
`/modeller`.

## Måling og kontroll

Enheten er **en fullført oppgave per forsøk**: alle nye akseptansekrav og
tidligere fungerende atferd består tester som agenten ikke kan endre.
Rapporter også delvis beståtte krav, regresjoner og credits per forsøk, men
ikke tell sjekker i samme forsøk som uavhengige observasjoner. En timeout,
CLI-feil eller ødelagt testmiljø får egen status og er ikke fullførte
oppgaver. Manglende bruksdata betyr at kostnaden er ukjent, også når
testutfallet er kjent.

Før et betalt forsøk må starttilstanden feile på det nye kravet og bestå
regresjonstestene, en kjent retting bestå alt og en kjent ugyldig patch
avvises. Bruk samme oppgaver, instruksjoner, verktøytilgang, klientversjon,
isolerte miljø og ressursgrenser i begge armer. Lagre revisjonene av fixture,
agent og instruksjoner, faktisk modell og innsatsnivå, testutfall, tid og
fullstendig bruk per forsøk. Les feilende patcher før årsaker tilskrives
modellen. Agenten skal ikke ha tilgang til secrets eller produksjonsdata.

## Før en eventuell pinningsbeslutning

For rutineoppgaver er den **foreslåtte akseptgrensen** høyst fem prosentpoeng
lavere andel fullførte oppgaver enn Sol. Ingen regresjon i auth,
tilgangskontroll eller bevart atferd kan godtas. En observert slik feil
utløser gjennomgang av oppgaven og patchen; null observerte feil beviser
ikke null risiko.

Denne grensen er **ikke** en statistisk test av pilotens fem forsøk. Før en
senere beslutningsmåling må vi fastsette oppgaveutvalg, hvilken modell og
innsats som faktisk er referanse, ensidig usikkerhetsnivå, utvalgsstørrelse
utledet fra marginen og øvre kostnadstak. Kan vi ikke betale for et utvalg
som besvarer spørsmålet, er utfallet «ikke avgjort». Vurder de to spørsmålene
i hver sin analyse; planleggingsresultater kan ikke overføres til worker-
oppgaver eller omvendt. Trinnvise krav i
[#1410](https://github.com/navikt/copilot/issues/1410) er et eget eksperiment.
