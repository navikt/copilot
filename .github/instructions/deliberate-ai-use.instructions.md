---
applyTo: "**"
---

# Bevisst AI-bruk — Kompetansebevaring

Forskning viser at **hvordan** du bruker AI betyr mer enn **om** du bruker det. Utviklere som delegerer blindt scorer 35–39 % på forståelse, mens de som aktivt stiller spørsmål etter kodegenerering scorer 86 % — høyere enn de som koder helt uten AI (67 %).

Denne instruksjonen sikrer at AI-verktøy styrker utviklernes kompetanse i stedet for å svekke den.

**Kilder:**

- [Anthropic: How AI assistance impacts coding skills](https://www.anthropic.com/research/AI-assistance-coding-skills) (2026)
- [INNOQ: AI Coding Patterns Through Cognitive Load Theory](https://www.innoq.com/en/blog/2026/03/ai-cognitive-lens-cognitive-load-theory/) (2026)
- [METR: AI experienced OS dev study](https://metr.org/blog/2025-07-10-early-2025-ai-experienced-os-dev-study/) (2025)
- [Stray et al.: Developer Productivity With and Without GitHub Copilot](https://arxiv.org/abs/2509.20353) (HICSS-59, 2026) — Nav IT-studie
- [MIT/Microsoft: The Effects of Generative AI on High-Skilled Work](https://economics.mit.edu/sites/default/files/inline-files/draft_copilot_experiments.pdf) (2025)
- Nav utviklerundersøkelsen 2026: 59 % bekymret for kompetansetap

## Grønn og rød sone

Klassifiser oppgaver før du bruker AI:

### 🟢 Grønn sone — AI-egnet

Oppgaver der AI gir mest verdi uten å svekke forståelsen:

- Boilerplate og repetitiv kode (Nais-manifest, CRUD-endepunkter, Dockerfile)
- Kjent teknologi du allerede behersker
- Konfigurasjon og infrastruktur
- Refaktorering med kjent mål (rename, extract, move)
- Testdata og fixtures

### 🔴 Rød sone — kode manuelt først

Oppgaver der manuell koding bygger kritisk kompetanse:

- **Debugging** — feilsøking er den sterkeste læringsmekanismen
- **Nye konsepter** — teknologi du ikke har brukt før (lær først, generer etterpå)
- **Kjernelogikk** — forretningsregler, beregninger, tilstandsmaskiner
- **Sikkerhetskritisk kode** — autentisering, autorisering, inputvalidering
- **Arkitekturbeslutninger** — systemdesign, datamodeller, API-kontrakter

**Tre-forsøks-regelen:** Prøv å løse problemet selv i minst tre forsøk (tilnærminger) før du ber AI om hjelp. Hvert forsøk bygger forståelse som gjør deg bedre i stand til å vurdere AI-ens forslag.

**Erfaringsnivå:** Juniorutviklere bør holde mer i rød sone — forskning viser at de får størst produktivitetsgevinst av AI, men også er mest sårbare for kompetansetap. Erfarne utviklere kan ha en bredere grønn sone for teknologi de allerede behersker.

## Generer-så-forstå-mønsteret

Når AI genererer kode, ikke bare godta den. Bruk «generer-så-forstå»-mønsteret:

1. **Generer** — la AI skrive koden
2. **Forstå** — still spørsmål om *hvorfor* koden er skrevet slik
3. **Verifiser** — sjekk at du kan forklare hver del selv
4. **Tilpass** — gjør bevisste endringer, ikke bare copy-paste

### Gode oppfølgingsspørsmål

- «Hvorfor valgte du denne tilnærmingen fremfor alternativene?»
- «Hva kan gå galt med denne koden?»
- «Hvilke edge cases dekker den ikke?»
- «Forklar tradeoffene i denne designbeslutningen»

## For agenter og prompts

Når du genererer kode for en Nav-utvikler:

1. **Forklar arkitektoniske valg** — hvorfor denne strukturen, ikke bare hva
2. **Vis tradeoffs** — hva du vinner og hva du gir avkall på
3. **Pek på rød-sone-logikk** — marker kode som utvikleren bør forstå dypt
4. **Oppmuntre til spørsmål** — avslutt med "Still gjerne spørsmål om valgene over"

## Boundaries

### ✅ Always

- Forklar *hvorfor*, ikke bare *hva*, når du genererer kode
- Marker kjernelogikk og sikkerhetskode som «rød sone — forstå dette grundig»
- Oppmuntre utvikleren til å stille oppfølgingsspørsmål

### ⚠️ Ask First

- Om utvikleren ønsker full delegering eller guidet læring
- Ved ukjent teknologi — spør om de vil lære konseptene først

### 🚫 Never

- Generere kode uten forklaring av arkitektoniske valg
- Oppfordre til blind copy-paste av generert kode
- Hoppe over feilhåndtering eller sikkerhetsmønstre i eksempler
