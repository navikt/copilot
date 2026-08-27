# 📦 Spike: samlingene og agentpakke-kontrakten

Konvergenshistorien bak agentpakke-arbeidet står i tre kilder ([#435](https://github.com/navikt/copilot/issues/435) Q2 «resolved — supersede», [#437](https://github.com/navikt/copilot/issues/437) «Related, not blocked», [beslutningene §1](agentpakke-beslutninger.md)): Navs egne samlinger blir agentpakker, `navikt/copilot` skipper sin egen `.nav-pilot/agentpakke.json`, og samlingsmekanismen pensjoneres innenfor kontraktens deprekeringsvindu. Dette dokumentet viser at historien **ikke er uttrykkbar i kontrakten slik den står**, koster de mulige veiene ut, og anbefaler én. Det implementerer ingenting; implementasjonen følger beslutningen.

Regelen fra [beslutningsdokumentet](agentpakke-beslutninger.md) gjelder her også: hver påstand er sjekket mot filene, og der kontrakt og kode er uenige, er koden fasit. Tallene under er målt mot arbeidstreet på `main` (`0fbf6ac4`), og utkastene er validert mot schemaet **slik [#461](https://github.com/navikt/copilot/pull/461) etterlater det** (branch `m2-wp7-clean`).

## 1. Formene, målt

### Fellespoolene

`navikt/copilot` skipper fire delte pooler som samlingene kuraterer delmengder av:

| Pool | Artefakter | Navn |
| --- | --- | --- |
| `agents/` | 13 | `*.agent.md` |
| `skills/` | 32 | `<navn>/SKILL.md` |
| `instructions/` | 16 | `*.instructions.md` |
| `prompts/` | 7 | `*.prompt.md` |
| **Sum** | **68** | 141 filer, 844 KiB målt som det install kopierer |

### De fem samlingene

`collections/{frontend,fullstack,kotlin-backend,nextjs-frontend,platform}/manifest.json`, hver et flatt navnevalg fra poolene:

| Samling | Agents | Skills | Instructions | Prompts | Sum | Filer | KiB |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `frontend` | 5 | 12 | 9 | 2 | 28 | 73 | 388 |
| `fullstack` | 7 | 28 | 16 | 7 | 58 | 123 | 705 |
| `kotlin-backend` | 4 | 24 | 10 | 4 | 42 | 86 | 470 |
| `nextjs-frontend` | 5 | 12 | 11 | 3 | 31 | 76 | 420 |
| `platform` | 4 | 15 | 7 | 2 | 28 | 63 | 358 |

Overlappen er ikke en tendens, den er mengdelære:

- **Fire ekte delmengdeforhold:** `frontend` ⊆ `nextjs-frontend` ⊆ `fullstack`, og `kotlin-backend` ⊆ `fullstack`.
- **`platform` ∖ `fullstack` = {`skills/rust-development`}** — én artefakt. `fullstack` pluss rust-skillen er unionen av alle fem.
- Unionen er 59 av poolens 68 artefakter; **17 artefakter er felles for alle fem** (bl.a. `code-review`- og `nav-pilot`-agentene, `nav-plan`/`nav-troubleshoot`-skillsene, `security-owasp` i begge former).
- **9 poolartefakter står i ingen samling:** agentene `auth`, `kafka`, `nais`, `nav-pilot-opus`, `observability`, `rust` og skillsene `ai-news-research`, `aksel-spacing`, `jackson-3-migration`. De nås i dag bare via bruker-scopets «(all)»-install eller à la carte (`nav-pilot add`).

### Hva en install gjør i dag

`cmdInstallFromSource` (`cli/nav-pilot/internal/cli/install.go`) leser `collections/<navn>/manifest.json` for en manifestløs kilde og kopierer nøyaktig de navngitte artefaktene inn i scopet (`installItems`); bruker-scope dropper prompts (`SupportsType`). State registrerer samlingsnavnet (`StateFile.Collection`) og fillisten. `sync` er state-drevet: den oppdaterer bare filene i `state.Files` (`resolveSyncFiles`, `cli/nav-pilot/internal/cli/sync.go`), og varsler om *nytt* innhold bare når `scopeTracksEverything` er sann — i dag kun «(all)»-installs i bruker-scope, og pakkeinstalls der `state.Collection == pakke.Name`. Den interaktive flyten har allerede en artefaktvelger der fravalg lagres som `fileStatusIgnored` og overlever reinstall (`interactive.go`, `buildPickerDefaults`).

Delmengder finnes altså på **to** nivåer i dag: samlingsnivået (navngitte kurasjoner i repoet) og statenivået (per-bruker ignore-lister). Bare det første er truet av kontrakten.

## 2. Mismatchen, presist

Kontrakten ([README.agentpakke.md](README.agentpakke.md)): *«Et repo med manifest erstatter samlingsmodellen: det tilbyr nøyaktig ett installerbart navn, nemlig manifestets `name`.»* Schemaet har ingen subsetting-konstruksjon, og PRD-ens Q2 er avgjort som supersede med tillegget *«the manifest has no collections field»* ([#435 §10 Q2](https://github.com/navikt/copilot/issues/435)).

Koden lever samtidig på den motsatte formen: `SynthesizeLegacy` (`cli/nav-pilot/internal/agentpakke/legacy.go`) syntetiserer **ett manifest per samling, navngitt etter samlingen** — fem installerbare identiteter fra én kilde, pluss `nav-pilot`-defaulten. Presist sagt: koden motsier ikke kontrakten *i dag*, for syntesen gjelder bare manifestløse kilder, som kontrakten eksplisitt overlater til legacy-modellen. Motsigelsen inntreffer den dagen `navikt/copilot` selv skipper et manifest — og konvergenshistorien *krever* at den dagen kommer. Da skjer, uten noen annen endring, dette (alle tre er kodefakta i `install.go` og `sync.go`):

1. `nav-pilot install frontend` slutter å virke: `cmdInstallAuto` setter kandidatlisten til `[]string{pakkeName}` når kilden har manifest (`install.go:209`), så alle fem samlingsnavn blir «not found».
2. Den interaktive samlingsvelgeren forsvinner — kontrakten sier selv at flyten «hopper over samlingsvelgeren» for en manifestkilde.
3. Eksisterende installs fryser som delmengder: sync fortsetter å oppdatere filene i state (de finnes fortsatt i poolene), men `scopeTracksEverything` blir falsk (`"frontend" != "nav-pilot"`), så nytt innhold varsles aldri, og det finnes ingen kommando som tar et «frontend»-scope over på pakka uten full reinstall.

Tier 2-kontekster (`payloads.full/focused`) er ikke en utvei: de er en *launch*-dimensjon på ferdigbygde, digest-bundne trær, valgt per økt med `--payload-context` — ikke en *install*-dimensjon på markdown-pooler. Å uttrykke fem samlinger som kontekster krever en payload-generator dette repoet ikke har, og gir uansett feil livssyklus (ingen filer i `.github/`, ingen state, ingen sync).

## 3. Alternativene

Utkastene ligger som ekte filer og validerer mot post-#461-schemaet: [`agentpakke-spike-utkast-a.json`](agentpakke-spike-utkast-a.json) og [`agentpakke-spike-utkast-b.json`](agentpakke-spike-utkast-b.json).

### A — én pakke, ingen subsets

Manifestet er `SynthesizeLegacy("")` serialisert til JSON — det finnes allerede i koden, med literaler testene holder i takt med kallstedene:

```json
{
  "contractVersion": "1",
  "name": "nav-pilot",
  "description": "Nav's default agents, skills, instructions, and prompts",
  "owner": { "repo": "navikt/copilot", "team": "nav-pilot maintainers" },
  "clients": {
    "copilot": { "primaryAgents": ["nav-pilot"] },
    "opencode": { "primaryAgents": ["nav-pilot", "nav-pilot-opus"], "defaultModel": "github-copilot/auto" },
    "pi": { "primaryAgents": ["nav-pilot"] }
  },
  "layout": { "agents": "agents", "skills": "skills", "instructions": "instructions", "prompts": "prompts" }
}
```

- **Kontraktkostnad: null.** Ingen schemaendring, ingen dokumentendring utover å oppdatere samlingsdokumentasjonen. Additivt per definisjon.
- **Installstørrelse:** alt = 68 artefakter / 141 filer / 844 KiB. Målt mot dagens samlinger vokser en repo-install med 18–78 filer (verst: `frontend`, +40 artefakter / +68 filer / +456 KiB committet i `.github/`).
- **Kjøretidskostnaden er avgrenset:** 14 av 16 instruksjoner er glob-scopet (`applyTo: "**/*.kt"` osv.) og aktiveres bare mot matchende filer; de to `applyTo: "**"`-instruksjonene (`code-review`, `deliberate-ai-use`) ligger allerede i alle fem samlingers felleskjerne, så «alt» tilfører **null** alltid-på-instruksjoner. Skills lastes on demand, og alle agenter utenfor `primaryAgents` materialiseres som subagenter.
- **Hva knekker:** punktene 1–3 i §2 — med mindre migrasjonen i §4 leveres samtidig.
- **Hva brukerne mister:** de fem navngitte kurasjonene. Hva de får: de 9 foreldreløse artefaktene, og én identitet å forholde seg til.

### B — navngitte subsets i kontrakten

Et additivt rotfelt, her kalt `subsets`, som løfter samlingsmanifestenes form inn i agentpakke-manifestet (fullt utkast med alle fem reelle listene i [`agentpakke-spike-utkast-b.json`](agentpakke-spike-utkast-b.json)):

```json
"subsets": {
  "frontend": {
    "description": "Rammeverk-uavhengig frontend-pakke …",
    "agents": ["accessibility", "aksel", "code-review", "forfatter", "nav-pilot"],
    "skills": ["…"], "instructions": ["…"], "prompts": ["…"]
  }
}
```

`nav-pilot install <subset>` installerer delmengden; `nav-pilot install <name>` installerer alt; state registrerer subsetnavnet.

- **Schema:** additivt — `additionalProperties: true` gjør at utkast B *allerede* validerer (bekreftet mot post-#461-schemaet). Det er også svakheten: schemaet kan ikke skille «subsets med mening» fra støy; semantikken må inn i dokumentet og den semantiske valideringen. En eldre binær ignorerer feltet og feiler lukket på subsetnavnet («not found») — ingen feilinstall, men heller ingen degradering til noe nyttig.
- **Validator:** ny semantisk sjekk — hvert navn i hver subset må finnes i `layout`-poolene (samme sjekk `nav-pilot validate` i dag *ikke* gjør for `collections/`).
- **Install:** liten — subsetlisten har nøyaktig samme form som samlingsmanifestet `loadManifest` allerede konsumerer. Men `stateCollection`, kryss-kilde-vakten og `scopeTracksEverything` trenger alle en subsetregel (subsetnavn ≠ pakkenavn), og `list`/interaktiv flyt får subsetvelgeren tilbake.
- **Dokumenter:** «nøyaktig ett installerbart navn» må omskrives, og PRD-ens Q2-vedtak («the manifest has no collections field») må **eksplisitt omgjøres** — B er samlingsmodellen flyttet inn i manifestet, ikke pensjonert.
- **Strukturelt:** subsets er meningsløse for Tier 2 — en delmengde av et digest-bundet payload-tre validerer ikke. Feltet blir dermed en permanent Tier 1-eksklusiv konstruksjon i en kontrakt hvis uttalte retning er at tierne konvergerer ([beslutningene §1](agentpakke-beslutninger.md)). Hver fremtidig konsument må implementere det likt, for alltid, for å bevare fem lister hvis samlede informasjonsinnhold er fire delmengdeforhold og én rust-skill (§1).

### C — flere pakker per repo

To varianter, begge brytende:

- **C1: N manifestfiler** (f.eks. `.nav-pilot/agentpakker/<navn>.json`). Manifeststien er en konvensjon konsumenten har innebygget (`agentpakke.ManifestPath`), og kontrakten sier selv at å endre en slik krever bump av `contractVersion`. I tillegg blir kilde→pakke 1:N overalt: `validate`-output, `list`, kryss-kilde-vakten (kildeidentiteten navngir ikke lenger én pakke), `source`-nøkkelen i config.
- **C2: ett manifest med `pakker: []`.** `name` er påkrevd og entall; å restrukturere rota er per kontraktens egne regler en betydningsendring av eksisterende felt — bump.

For Navs faktiske innhold er C ren duplisering: alle fem samlingene deler identiske personaer, klienter og layout — bare artefaktlistene varierer. Fem pakker betyr fem kopier av `clients`-blokken for null variasjon, pluss all B-kostnaden, pluss oppløsningsambiguiteten. Det er formen `SynthesizeLegacy` simulerer i dag, og grunnen til at den skal *bort*, ikke kontraktfestes.

### D — splitt i egne repoer

Fem repoer, hver med sitt utkast-A-formede manifest og sin innholdsdelmengde. Kontrakten er urørt, men:

- **Vedlikehold:** innholdet er delte pooler — `fullstack` inneholder tre av de fire andre samlingene i sin helhet. Enten dupliseres hver skill-endring over inntil fem repoer, eller det bygges en build-time-komposisjonsgenerator (H1) dette repoet ikke har og PRD-en eksplisitt utsatte.
- **Discovery:** standardkilden er `navikt/copilot`; fem repoer krever at brukerne kjenner reponavn, og katalogspørsmålet (Q3) er uløst. CODEOWNERS, CI og release-prosess ×5.

D kjøper ingenting de andre ikke gir billigere.

### Oppsummert

| | Kontraktendring | Kode | Hva brukerne mister | Hva som skalerer dårlig |
| --- | --- | --- | --- | --- |
| **A** — én pakke | ingen | migrasjonssti (§4) | navngitte kurasjoner (bevares som ignore-lister) | installstørrelse: +456 KiB verst |
| **B** — subsets | additiv, men omgjør Q2 | validator + install + state-regler + velger | ingenting | permanent Tier 1-eksklusiv kontraktflate |
| **C** — flere pakker | **brytende** (sti- eller rotkonvensjon) | 1:N-oppløsning overalt | ingenting | 5× duplisert klientblokk, ambiguitet |
| **D** — splitt repoer | ingen | komposisjonsgenerator eller 5× vedlikehold | én kilde, discovery | alt innholdsarbeid |

## 4. Anbefaling: A — én pakke, med state-migrasjon i samme release

**Anbefalingen er A**, med én ufravikelig forutsetning: manifestet og migrasjonen skipper i **samme** release.

Begrunnelsen, i rekkefølgen den bærer:

1. **Samlingene inneholder nesten ingen informasjon.** Fire delmengdeforhold og én artefakt skiller de fem listene; felleskjernen er 17 av 28–58 artefakter; `frontend` og `nextjs-frontend` deler 90 % (Jaccard). En kontraktkonstruksjon (B) eller en repostruktur (C/D) for å bevare dét er flate kjøpt for omtrent én bit kurasjon.
2. **Delmengden brukeren faktisk har, er allerede representerbar uten kontrakt.** State-laget uttrykker delmengder i dag: `fileStatusIgnored` pluss artefaktvelgeren. Migrasjonen er derfor en engangs-adopsjon i sync/install: et scope med `state.Collection` ∈ {de fem samlingsnavnene} skrives om til pakkenavnet, og resten av poolen markeres `ignored`. Brukerens install er byte-for-byte uendret, sync-semantikken består, `scopeTracksEverything` blir sann (nye artefakter varsles, som for «(all)» i dag), og velgeren viser fravalgene ved neste reinstall. Mekanismen finnes; det nye er bare adopsjonssteget — samme mønster som kildeadopsjonen for pre-source-tracking-scopes i `sync`.
3. **A er det eneste alternativet som lar tierne fortsette å konvergere.** Subsets kan ikke uttrykkes for digest-bundne payloads, så B og C institusjonaliserer en splittelse beslutningene (§1) sier er midlertidig.
4. **Kostnaden ved «alt» er målt og liten.** Null nye alltid-på-instruksjoner (§3A), on-demand-skills, subagent-materialisering. Det som gjenstår er 141 filer i `.github/` — reelt, men engangs-diff, og repo-scope-brukere kan fortsatt velge bort i velgeren.

De forkastede, på sak: **B** taper ikke på kostnad i kode (den er liten) men på varighet — den gjeninnfører samlingsmodellen som permanent kontraktflate for å bevare fem lister som i dag kan bli ren dokumentasjon, og den omgjør et PRD-vedtak uten at noe nytt behov har oppstått siden vedtaket. **C** er brytende for en gevinst B gir additivt, og modellerer variasjon Navs innhold ikke har. **D** bytter et kontraktproblem mot et innholdsdupliserings- og discoveryproblem som er dyrere enn det opprinnelige.

Samlingslistene selv pensjoneres til dokumentasjon: [README.collections.md](README.collections.md) blir «anbefalte utvalg per teamtype» (tabeller, ingen mekanikk), og `collections/`-katalogen slettes ved utløpet av deprekeringsvinduet. En valgfri senere forbedring — *ikke* en del av beslutningen — er å la velgeren tilby forhåndsutfylte utvalg; det er ren TUI, uten kontraktspor.

## 5. Hvis vi ikke gjør noe

Om seks måneder, konkret:

- `navikt/copilot` er fortsatt manifestløs, og **hver eneste default-install i Nav går gjennom `SynthesizeLegacy`** — adapteren hvis egen dokumentasjon sier at den skal forsvinne. Literalene i `legacy.go` må holdes i takt med hardkodede kallsteder i `internal/provider` og `internal/source` av tester, og hver nye manifest-lesende funksjon (payload-rostere fra #461, `acceptsUserContext`-forslaget, modell-frontmatter i WP5) legger enda et felt til synkroniseringsbyrden.
- Migrasjonens fase 2 (PRD §7A) er blokkert, dermed fase 3: **deprekeringsklokka for samlingsmekanismen kan ikke engang starte**, for vinduet forutsetter en kunngjøring, og kunngjøringen forutsetter at erstatningen finnes.
- Suksessmetrikken «a second agentpakke can onboard on Tier 1 using only the published contract docs» forblir umålt mot det eneste innholdet vi selv eier — kontrakten er i praksis testet av én Tier 2-konsument og null Tier 1-konsumenter.
- Statefiler med `Collection: "frontend"` osv. fortsetter å akkumulere, så den som til slutt pensjonerer samlingene arver *denne* beslutningen pluss en større migrasjonsflate: fem samlingsnavn, «(all)», «(à la carte)» — og på det tidspunktet finnes det reelle andrekonsumenter, så enhver kontraktjustering som da viser seg nødvendig koster bump pluss 90 dager i stedet for en PR.

Ingenting *knekker* av å vente. Det er nettopp problemet: tilstanden er stabil nok til å bli permanent.

## 6. Hva som må besluttes i vinduet, og hva som kan vente

Vinduspremisset fortjener en presisering, for det er sterkere formulert i omløp enn kontrakten dekker: kontraktens egne kompatibilitetsregler gjør **additive** felt frie *også etter* at andrekonsumenten finnes. Alternativ B kan altså legges til om et år uten bump og uten vindu — en eldre binær ignorerer feltet og feiler lukket på subsetnavn. «Nå eller aldri» gjelder bare det brytende.

**Må avgjøres nå (før `navikt/copilot`-manifestet skipper, som selv lukker vinduet):**

1. **Om «nøyaktig ett installerbart navn per repo» består som kontraktinvariant.** Det er den ene setningen C bryter, og C er gratis bare nå. Anbefalingen er ja — behold invarianten, forkast C eksplisitt.
2. **Rekkefølgen:** manifestet kan ikke skippe før migrasjonsstien for samlingsbrukerne finnes (§4-forutsetningen). Dette er ikke en schemabeslutning, men en releasebeslutning — å skippe utkast A alene bryter `install frontend` samme dag.
3. **A eller B som utgangspunkt** — ikke fordi B blir dyr senere, men fordi valget avgjør om Q2-vedtaket står, hva README.collections.md skal bli, og hva migrasjonen i pkt. 2 migrerer *til*.

**Kan vente:**

- B som *tillegg* (hvis et reelt behov for navngitte delmengder dukker opp etter A — additivt, når som helst).
- Velger-forhåndsutvalg og annen TUI-komfort.
- Selve slettingen av `collections/` og `SynthesizeLegacy` — den følger deprekeringsvinduet som starter når manifestet skipper.

## 7. Funn under lesing som korrigerer eller presiserer eksisterende dokumenter

- **`nav-pilot-opus` er deklarert opencode-primærpersona uten å være installerbar via noen samling.** `SynthesizeLegacy` setter `primaryAgents: ["nav-pilot", "nav-pilot-opus"]` for opencode, men agenten står i *ingen* av de fem samlingsmanifestene — bare «(all)»-installs i bruker-scope får filen. En `fullstack`-install materialiserer altså en allowlist som navngir en agent den aldri installerte. Under A forsvinner avviket av seg selv; det er også et konkret eksempel på at samlingslistene og manifestdeklarasjonene allerede drifter fra hverandre.
- **«Koden motsier kontrakten» er upresist som nåtidspåstand.** `SynthesizeLegacy` virker bare på manifestløse kilder, som kontrakten eksplisitt unntar. Den riktige formuleringen er at koden *beviser et behov* (fem installerbare identiteter fra én kilde) kontrakten ikke kan uttrykke — og at motsigelsen materialiserer seg først den dagen repoet skipper manifest. Beslutningen må tas *fordi* den dagen er planlagt, ikke fordi noe er galt i dag.
- **Vindusargumentet er overdrevet for additiv vekst** (§6): bare brytende former (C, konvensjonsendringer) er bundet til vinduet. #461s egen begrunnelse sier det samme — «the window binds from the second consumer onward» gjelder brytende endringer.
- **De fem samlingene deler version-feltet `"2025.07"` uendret** mens poolinnholdet har fortsatt å endre seg — samlingsmanifestenes `version` er i praksis dødt (state bruker kildens release-versjon, `src.Version`). Ingen del av beslutningen henger på feltet, men det er nok et tegn på at kurasjonslaget ikke vedlikeholdes aktivt.

## Se også

- [README.agentpakke.md](README.agentpakke.md) — kontrakten («nøyaktig ett installerbart navn»)
- [agentpakke-beslutninger.md](agentpakke-beslutninger.md) — begrunnelsene, særlig §1 om tier-konvergens
- [README.collections.md](README.collections.md) — legacy-modellen dette dokumentet avgjør skjebnen til
- [`agentpakke-spike-utkast-a.json`](agentpakke-spike-utkast-a.json), [`agentpakke-spike-utkast-b.json`](agentpakke-spike-utkast-b.json) — utkastene, validert mot post-#461-schemaet
- [#435](https://github.com/navikt/copilot/issues/435) (PRD, Q2), [#437](https://github.com/navikt/copilot/issues/437), [#461](https://github.com/navikt/copilot/pull/461)
