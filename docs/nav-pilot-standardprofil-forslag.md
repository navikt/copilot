# Forslag: ekstern standardprofil for nav-pilot

**Status: forslag, ikke gjeldende oppførsel.** Ingenting i dette dokumentet er implementert. `nav-pilot` henter ingen profil i dag, og ingen av nøklene, filene eller endepunktene som beskrives her finnes utenfor forslaget. Dokumentet er skrevet for å bli motsagt: siste seksjon ([§12](#12-når-dette-er-feil-valg)) er en samling argumenter mot å bygge det.

**Regel for dokumentet, lånt fra [agentpakke-beslutninger.md](agentpakke-beslutninger.md):** hver påstand om koden skal kunne sjekkes mot fil og linje. Der oppdraget som utløste dokumentet sier noe annet enn koden, er koden fasit, og avviket noteres ([§13](#13-påstander-som-ikke-overlevde-kontrollen)).

Alle linjenummer er lest på `b618c07b` (`origin/main`, 31.08.2026).

---

## 1. Behovet, i én setning

Nav skal kunne flytte standardmodellen uten å bygge og distribuere en ny binær, brukeren skal beholde sitt eget valg hvis hun har tatt ett, og hun skal få vite om flyttingen én gang per endring i stedet for ved hver start.

## 2. Sammendrag og anbefaling

Anbefalingen er **en rå fil i `navikt/copilot`, hentet i samme budsjett som release-sjekken, med `defaultsVersion` som nudge-utløser, og ingen egen transport**. Profilen legges under agentpakke-manifestet i presedensen, ikke over, den feiler mykt der manifestet feiler lukket, og den deler `cache.json` med staleness-sjekken framfor å få sin egen fil. Endringen som faktisk flytter standardmodellen for flertallet av brukerne er derimot ikke profilen: det er å erstatte `"defaultModel": "inherit"` for `copilot` i det innebygde manifestet ([legacy.go:69-72](../cli/nav-pilot/internal/agentpakke/legacy.go)) med en konkret modell-id, som etter [#490](https://github.com/navikt/copilot/pull/490) har et lesested som faktisk brukes ([copilot_launch.go:80-91](../cli/nav-pilot/internal/provider/copilot_launch.go)). Den endringen er én linje og krever ingen henting. Profilen er verdifull først når den linjen skal kunne flyttes uten ny binær, og rekkefølgen bør være: sett verdien først, bygg hentemekanismen etterpå, når det finnes noe som faktisk er verdt å flytte.

## 3. Hva som faktisk skjer i dag

Dette er nødvendig for resten, fordi flere av innvendingene mot forrige forsøk hviler på antakelser som ikke stemmer lenger.

### 3.1 Standardbrukeren har ingen Nav-satt modell i det hele tatt

Standardklienten er `copilot` ([config.go:242](../cli/nav-pilot/internal/cli/config.go)). Standardpersonaen for copilot er `nav-pilot` ([legacy.go:69-72](../cli/nav-pilot/internal/agentpakke/legacy.go)). Den agentfila har **ingen `model:`-linje** ([agents/nav-pilot.agent.md](../agents/nav-pilot.agent.md), linje 1-8). Og det innebygde manifestet deklarerer `InheritModel` for copilot, som `pakkeDeclaredModel` oversetter til tom streng ([staged_launch.go:76-86](../cli/nav-pilot/internal/provider/staged_launch.go)).

Konsekvensen: en bruker som ikke har satt noe, starter copilot uten `--model` og uten en `model:`-linje i personaen. Serveren velger. Nav uttrykker ingen preferanse noe sted på den vanligste stien.

### 3.2 #490 landet, og den endret bildet mer enn frontmatter-halvdelen tilsier

[#490](https://github.com/navikt/copilot/pull/490) er i main (`b493564e`). To ting derfra betyr noe her.

`BuildAgentFrontmatter` skriver nå en `model:`-linje når den får en modell ([frontmatter.go:196-207](../cli/nav-pilot/internal/source/frontmatter.go)), og `openCodeAgentModel` oversetter Nav-agentenes visningsnavn til opencode-id-er ([export.go:322-347](../cli/nav-pilot/internal/artifacts/export.go)). Det gjør modell per agent virksomt på opencode. Ni av ti Nav-agenter har en `model:`-linje, men den tiende er `nav-pilot`, altså den som faktisk startes.

Den viktigere halvdelen er den oppdraget ikke nevner: #490 la inn et fallback fra manifestet på Tier 1 copilot ([copilot_launch.go:80-91](../cli/nav-pilot/internal/provider/copilot_launch.go)). Før det var standardkonfigurasjonen den eneste stien der Nav ikke kunne deklarere noe som helst. Nå finnes lesestedet. Det er inert bare fordi manifestet deklarerer `inherit`.

### 3.3 #498 har ikke landet

[#498](https://github.com/navikt/copilot/pull/498) er **åpen**, ikke merget (`gh pr view 498`: `"state":"OPEN"`, `"mergedAt":null`). Alt oppdraget bygger på #498 er derfor betinget. Det som står i PR-en er likevel verdt å ta med, fordi det er målt mot `opencode` 1.18.25 og motsier den intuitive modellen:

- I TUI-en, som er den nav-pilot faktisk starter, slår agentens `model:`-frontmatter `--model`-flagget.
- I `opencode run` slår flagget frontmatteren.
- Konfigurasjonens toppnivå-`model` er sesjonsstandarden og ligger under agenten i begge inngangspunkter.

#498 flytter derfor Nav-standarden for opencode fra et flagg til opencodes egen konfigurasjon (`EnsureOpenCodeSessionModel`), og lar copilot være i fred. Hvis #498 lander, er det en tredje skriveflate å holde i synk. Hvis den ikke lander, er `--model`-flagget fortsatt der, og det er virkningsløst for enhver opencode-agent som har en egen `model:`-linje. Ingen av delene endrer designet under, men begge endrer hvor verdien til slutt havner.

### 3.4 Innhold når allerede brukerne én gang om dagen

Ved start sjekker `nav-pilot` om den installerte pakka er utdatert og tilbyr synk ([interactive.go:205-235](../cli/nav-pilot/internal/cli/interactive.go)). Sjekken har 24 timers TTL ([staleness.go:13](../cli/nav-pilot/internal/artifacts/staleness.go)). Agentfiler er innhold. En modellendring som ligger i en `model:`-linje når altså brukerne innen et døgn allerede i dag, uten noen ny mekanisme, mot ett ja-klikk.

Det er den enkeltopplysningen som svekker behovet mest, og den står her framfor i fotnotene.

## 4. De fem innvendingene mot forrige forsøk

Branchen `feat/model-default-profile` (`2bf00734`) har en komplett implementasjon: profil-JSON i repoet, henting i release-budsjettet, cache i `StalenessCache`, validering mot innebygd skjema, mykt fall, lest i `openCodeDefaultModel()`. Den ble frarådet merget. Slik står innvendingene nå.

| # | Innvending | Status |
| --- | --- | --- |
| 1 | Nådde bare opencode | **Løst, men ikke av grunnen oppdraget oppgir.** Se under. |
| 2 | Verdien var allerede `github-copilot/auto` | **Står, og er styrket.** Se under. |
| 3 | Lesestedet skal slettes | **Delvis feil premiss.** Se under. |
| 4 | 14 % av kredittbruken | **Står uimotsagt.** Kan ikke etterprøves i repoet. |
| 5 | To rekonstruksjonssteder for `StalenessCache` | **Står, og det er tre, ikke to.** |

**1. «Nådde bare opencode.»** Løst. Etter #490 finnes det et lesested for copilot Tier 1 ([copilot_launch.go:82](../cli/nav-pilot/internal/provider/copilot_launch.go)), og alle fire launch-stiene leser nå manifestet på samme måte: Tier 1 copilot (linje 82), Tier 1 opencode gjennom `ToOpenCodeModel` ([provider.go:96-106](../cli/nav-pilot/internal/provider/provider.go)) og `openCodeDefaultModel` ([pakke.go:82-88](../cli/nav-pilot/internal/provider/pakke.go)), Tier 2 opencode ([staged_launch.go:218](../cli/nav-pilot/internal/provider/staged_launch.go)) og Tier 2 copilot ([staged_launch.go:252](../cli/nav-pilot/internal/provider/staged_launch.go)). Men merk hva som løste det: det var manifest-fallbacket, ikke frontmatter-linja. `nav-pilot.agent.md` har fortsatt ingen `model:`, så frontmatter-halvdelen av #490 rører ikke standardpersonaen. Oppdraget krediterer feil halvdel av PR-en.

**2. «Verdien var allerede `github-copilot/auto`.»** Står, og for copilot er situasjonen verre: verdien er `inherit`, altså ingen verdi ([legacy.go:71](../cli/nav-pilot/internal/agentpakke/legacy.go)). Kommentaren over den linja sier eksplisitt at å velge en konkret id «hører til den som eier rutingsbeslutningen, i en commit som handler om rutingsbeslutningen». Den commiten er ikke skrevet. Å bygge en hentemekanisme for en verdi ingen har bestemt seg for å endre er å bygge leveransen før lasten.

Forrige forsøk gjorde dette akutt: for å få profilen til å virke i det hele tatt måtte den behandle «manifestet deklarerer nøyaktig `OpenCodeDefaultModel`» som «manifestet har ingen mening» (diffen på [pakke.go](../cli/nav-pilot/internal/provider/pakke.go) i `2bf00734`). Det er en lag-sammenblanding som oppstår nettopp fordi den innebygde verdien og profilens verdi er den samme.

**3. «Lesestedet skal slettes.»** Premisset er upresist. Det som forsvinner er den innebygde adapteren `Default()`, «når navikt/copilot leverer sitt eget manifest og samlingsmekanismen pensjoneres» ([legacy.go:15-23](../cli/nav-pilot/internal/agentpakke/legacy.go)). Lesestedene i §3.2 leser `source.ActivePakke()`, ikke adapteren, og de blir stående. Det som flytter seg er hvor verdien kommer fra: fra Go-literaler til en innsjekket `.nav-pilot/agentpakke.json`.

Det gjør innvendingen mildere og designproblemet skarpere. Når navikt/copilot leverer sitt eget manifest, er standardmodellen allerede en fil i et repo som hentes og valideres. Da har vi to filer i samme repo som begge sier hva standardmodellen er, med ulik TTL og ulik feilmodus. Det er den virkelige kostnaden ved profilen, og §12 handler mest om den.

**4. «14 % av kredittbruken.»** Tallet finnes ikke i repoet og kan ikke verifiseres herfra; det nærmeste er datagrunnlaget i `apps/copilot-metrics`. Det står ubestridt, og det står her i klartekst framfor i en fotnote: **en mekanisme som sentralt setter agent-standarder sikter mot den mindre delen av forbruket.** Se [§11](#11-hva-dette-ikke-løser).

**5. «To rekonstruksjonssteder.»** Det er tre, alle bygger `StalenessCache` fra bunnen og mister derfor et nytt felt i stillhet:

- [staleness.go:133](../cli/nav-pilot/internal/artifacts/staleness.go), etter mislykket henting
- [staleness.go:141](../cli/nav-pilot/internal/artifacts/staleness.go), etter vellykket henting
- [update.go:130](../cli/nav-pilot/internal/cli/update.go), etter selvoppgradering

Forrige forsøk fant og oppdaterte alle tre. Det er ikke en garanti for at den fjerde blir det. Se [§7.4](#74-fella-med-tre-rekonstruksjonssteder).

## 5. Nudgen

### 5.1 Hvorfor versjon, ikke tidsstempel, og ikke boolean

Utløseren er et bevisst hevet `defaultsVersion` i profilen. Et tidsstempel endrer seg ved hver redigering, inkludert rene tekstrettinger i `note`-feltet, og ville nudget hele Nav for en kommafeil. Et heltall som Nav hever når endringen er verdt å spørre om, lar den som redigerer bestemme hva som er verdt oppmerksomhet. Det er monotont, så en revert som hever versjonen videre oppfører seg riktig, og klokkeskjev på klienten betyr ingenting.

Å lagre en boolean («er varslet») ville trengt en nullstilling for hver endring, altså en skriveoperasjon Nav ikke kan utføre på brukerens maskin. Å lagre selve verdien gjør nullstillingen implisitt: den lagrede versjonen er ulik den nye versjonen, og det er hele testen. Dette er samme resonnement som `rtk_prompted_client` bruker: den lagrer *hvilke klienter* som er spurt, ikke *om* noen er spurt ([config_cmd.go:138-145](../cli/nav-pilot/internal/cli/config_cmd.go)), fordi et ja/nei ikke kunne svare på «er denne brukeren spurt for opencode».

Lagre versjonen, ikke modell-id-en. Modell-id-en er avledet: to profiler kan sette samme id via ulike veier, og en id kan endres kosmetisk. Versjonen er det Nav faktisk styrer.

### 5.2 Hva som lagres, og hvor

Én ny konfignøkkel, `defaults_notified_version`, i samme form som `rtk_prompted_*`:

- felt i `Config` og `ResolvedConfig` ([domain.go:30-31 og 56-57](../cli/nav-pilot/internal/domain/domain.go) er formen)
- oppføring i `configKeyDefs` med `flag: ""`, altså ingen CLI-flagg ([config_cmd.go:138-153](../cli/nav-pilot/internal/cli/config_cmd.go))
- **ikke** i `configPageKeys`, som allerede holder `version` og `rtk_prompted_*` utenfor innstillingssiden ([config_page.go:14-28](../cli/nav-pilot/internal/cli/config_page.go))
- kommentert ut i konfigmalen, som `rtk_prompted_at` ([config_cmd.go:254-260](../cli/nav-pilot/internal/cli/config_cmd.go))

Verdien er profilens `defaultsVersion` som streng. Skriving skjer **før** meldingen skrives ut, og en feilet skriving advarer i stedet for å stoppe, presis som `savePromptState` ([rtk_setup.go:108-120](../cli/nav-pilot/internal/cli/rtk_setup.go)). Rekkefølgen er ikke tilfeldig: skriver vi etterpå og prosessen dør imellom, får brukeren samme melding ved neste start, altså akkurat naggingen nøkkelen finnes for å hindre.

### 5.3 De fire tilfellene

**Fersk installasjon, ingen konfigfil.** Ingen nudge. Det finnes ingen tidligere standard å ha flyttet seg fra. Klienten skriver den gjeldende `defaultsVersion` og sier ingenting. Merk at dette krever at konfigfila opprettes; hvis brukeren aldri har kjørt `config set`, må skrivingen opprette fila. Det gjør `cmdConfigSet` allerede ([rtk_setup.go:113](../cli/nav-pilot/internal/cli/rtk_setup.go) er presedensen).

**Første start etter at dette leveres, for en eksisterende bruker.** Ingen nudge. Nøkkelen er tom, og en tom nøkkel betyr «vi vet ikke hva denne brukeren har sett», ikke «brukeren har sett null». Å behandle tom som null ville hilst hele Nav med et varsel om en standard de alltid har hatt. Skriv verdien stille, si ingenting. Dette er den ene regelen som er lettest å implementere feil og vanskeligst å ta tilbake.

**Standarden flytter seg to ganger før brukeren starter.** Én melding, om den nyeste verdien. Vi hopper ikke over noe og vi køer ikke opp meldinger: sammenligningen er «lagret versjon ulik profilens versjon», ikke en teller. Mellomliggende versjoner har aldri vært brukerens standard, og en melding om en modell hun aldri kjørte er støy.

**Brukeren har satt `model` eksplisitt, og den er en annen.** Én melding, med en annen ordlyd: standarden har flyttet seg, ditt valg gjelder fortsatt, og slik følger du Nav-standarden igjen (`nav-pilot config set model ""`). Nøkkelen skrives på samme måte, så også hun får den bare én gang per endring. Å tilby et interaktivt bytte her er en egen beslutning og hører ikke hjemme i førsteversjonen: det gjør en informasjonslinje om til et samtykkepunkt i en flyt som allerede har ett ([interactive.go:212-226](../cli/nav-pilot/internal/cli/interactive.go)).

**Hvor meldingen skrives:** på stderr, før klienten startes, i samme stil som `ResolvedModelNotice` ([pakke.go:99-105](../cli/nav-pilot/internal/provider/pakke.go)) og de øvrige advarslene. Ikke en prompt. Ikke noe som krever et tastetrykk.

### 5.4 Ikke-interaktive kjøringer

`nav-pilot` kjøres også fra skript og fra CI. En melding på stderr er ufarlig der, men skrivingen av nøkkelen er ikke: en CI-kjøring som skriver `defaults_notified_version` til en efemer HOME oppnår ingenting, og en som skriver til en delt HOME kan konsumere nudgen for et menneske. Dette er ikke løst i forslaget, og det skal ikke skjules: enten hopper vi over både melding og skriving når stdout ikke er en TTY, eller vi aksepterer at nudgen kan gå tapt. Anbefaling: hopp over begge deler når det ikke er en TTY. Det er én betingelse, og den feiler mot «ingen melding» framfor mot «tapt melding».

## 6. Presedens

Rekkefølgen under er lest ut av koden, ikke gjengitt fra en plan. Den er høyeste først.

| Lag | Hvor den bor | Sted i koden |
| --- | --- | --- |
| 1. CLI-flagg `--model` | prosessargument | [config.go:305-307](../cli/nav-pilot/internal/cli/config.go) skriver over filverdien |
| 2. Konfigfil `model` | `~/.nav-pilot/config.toml` | [config.go:257-259](../cli/nav-pilot/internal/cli/config.go) |
| 3. Agentens frontmatter `model:` | den materialiserte agentfila | [export.go:318](../cli/nav-pilot/internal/artifacts/export.go), virkningen er klientavhengig |
| 4. Agentpakke-manifestets `defaultModel` | `.nav-pilot/agentpakke.json`, eller den innebygde adapteren | [copilot_launch.go:82](../cli/nav-pilot/internal/provider/copilot_launch.go), [staged_launch.go:218](../cli/nav-pilot/internal/provider/staged_launch.go) og [:252](../cli/nav-pilot/internal/provider/staged_launch.go), [pakke.go:82-88](../cli/nav-pilot/internal/provider/pakke.go) |
| 5. Profilen (foreslått) | hentet dokument, cachet | eksisterer ikke |
| 6. Innkompilert standard | `OpenCodeDefaultModel` | [provider.go:65](../cli/nav-pilot/internal/provider/provider.go) |

**Tre presiseringer oppdragets liste mangler.**

Lag 1 og 2 er ikke to lag ved lesestedet. De er kollapset til `ResolvedConfig.Model` av `resolve()` ([config.go:240-320](../cli/nav-pilot/internal/cli/config.go)) lenge før noen modellbeslutning tas. Alle launch-stiene ser bare `r.Model != ""`. Det er en fordel: profilen trenger ikke vite forskjell på et flagg og en fil, bare på «brukeren har sagt noe» og «brukeren har ikke sagt noe».

Lag 3 og 4 er ikke ordnet på tvers av klienter. På copilot er frontmatteren per agent og `--model` per sesjon, og hvem som vinner avgjøres av copilot CLI, ikke av oss. På opencodes TUI vinner frontmatteren over flagget (målt i [#498](https://github.com/navikt/copilot/pull/498), se §3.3). På `opencode run` vinner flagget. Tabellen over er nav-pilots egen resolusjonsrekkefølge, ikke en garanti om hva klienten gjør med det den får. Enhver tekst som lover brukeren en total ordning på tvers av klientene er feil.

Lag 5 hører **under** lag 4. Et fremmed agentpakke som deklarerer en modell har gjort et bevisst valg for sitt eget innhold, og en Nav-publisert standard skal ikke overstyre det. Forrige forsøk snudde dette i praksis ved å behandle den innebygde verdien som «ingen mening» (§4, innvending 2). Den riktige løsningen er ikke et unntak i `openCodeDefaultModel`, men at det innebygde manifestet slutter å deklarere en verdi profilen skal kunne flytte: enten `inherit`, eller ingen deklarasjon, slik at profilen fyller et faktisk tomrom.

## 7. Henting og cache

### 7.1 Gjenbruk, ikke nytt

`internal/artifacts/staleness.go` gjør allerede jobben: 24 timers intervall ([:13](../cli/nav-pilot/internal/artifacts/staleness.go)), 1 times ny-forsøk etter feil ([:117-119](../cli/nav-pilot/internal/artifacts/staleness.go)), aldri blokkerende, atomisk skriving via temp og rename ([:62-95](../cli/nav-pilot/internal/artifacts/staleness.go)). Hentefunksjonen injiseres ovenfra med 5 sekunders timeout ([aliases.go:227-241](../cli/nav-pilot/internal/cli/aliases.go)).

Profilen skal ha samme TTL, samme cachefil (`~/.nav-pilot/cache.json`) og samme utløser. Ikke en egen fil: to skrivere av samme katalog med hver sin TTL er to sjanser til å klabbe over hverandre, og profilen har ingen grunn til å oppdateres oftere enn release-sjekken.

### 7.2 Når hentes den

I samme kall som release-sjekken lykkes, altså høyst én gang i døgnet, på start. Ikke ved `doctor`, ikke ved `config`, ikke ved `export`.

### 7.3 Feilmodusene

| Situasjon | Oppførsel |
| --- | --- |
| Offline, varm cache | Forrige kjente profil brukes. Ingen melding. |
| Offline, kald cache | Innkompilert standard. Ingen melding. Ingen forsinkelse ut over de 5 sekundene release-sjekken allerede bruker. |
| Henting feiler, varm cache | Forrige profil rir videre. Ny-forsøk om en time, ikke om et døgn ([:117-119](../cli/nav-pilot/internal/artifacts/staleness.go)). |
| Profilen er ugyldig JSON eller feiler skjemaet | Forrige profil rir videre. Ingen melding til brukeren; hun kan ikke gjøre noe med det. |
| Profilen mangler helt (404) | Forrige profil rir videre. Å slette fila er dermed ikke en måte å ødelegge starter på. |
| Profilen deklarerer en ukjent `profileVersion` | Hele dokumentet ignoreres. Slik holder en fremtidig inkompatibel profil seg ufarlig for dagens binærer. |

Merk at release-sjekken **ikke** skal utløse en ekstra henting når den selv feiler. En feilet release-oppslag er det sterkeste tilgjengelige signalet om at det ikke finnes nett, og en andre forespørsel bruker bare opp startens budsjett på samme timeout. Forrige forsøk kom fram til det samme og skrev det ned i `staleness.go`; det resonnementet er verdt å beholde.

### 7.4 Fella med tre rekonstruksjonssteder

`StalenessCache` bygges fra bunnen tre steder (§4, innvending 5). Et nytt felt faller stille bort i to av dem. Å be folk huske det er ikke et design.

To billige mottiltak, i prioritert rekkefølge:

1. **Gjør cachen additiv.** En `WriteCacheUpdate(func(*StalenessCache))` som leser, muterer og skriver, slik at ingen kallsted konstruerer strukten. Da er det umulig å miste et felt. Dette er samme form som `mutateOpenCodeConfig` i #498, altså et mønster huset allerede har valgt for nøyaktig dette problemet.
2. **Hvis 1 er for mye:** en test som skriver en cache med alle felt satt, kjører hver av de tre stiene, og feiler hvis et felt er tomt etterpå. Den fanger den fjerde stien når noen legger den til.

Anbefaling: 1. Den fjerner problemet i stedet for å oppdage det, og den er kortere enn testen.

### 7.5 Hvorfor profilen feiler mykt der manifestet feiler lukket

Agentpakke-manifestet feiler lukket, og det er skrevet ned: «manifestet finnes, men er ikke i samsvar, så ingenting bør installeres, synkes eller startes fra det» ([load.go:26-28](../cli/nav-pilot/internal/agentpakke/load.go)).

Forskjellen er hva de to dokumentene styrer. Manifestet bestemmer **hva som kjører**: hvilke agenter som materialiseres, hvilken persona som startes, hvilket innhold som havner i brukerens katalog. Et manifest vi ikke stoler på er innhold vi ikke stoler på, og da er «ikke kjør» det eneste trygge svaret. Profilen bestemmer bare **hvilken av flere fungerende modeller** som velges når brukeren ikke har valgt. Forrige svar er alltid et trygt svar. Det finnes ingen tilstand der «vi klarte ikke lese profilen» gjør det farlig å starte klienten, og derfor finnes det ingen begrunnelse for å la den stoppe noe.

Den praktiske konsekvensen er en regel som må stå i koden og ikke bare her: **ingenting i profilstien returnerer en feil oppover.** Ingen `error`, ingen advarsel til brukeren, ingen ny grunn til at en start feiler.

## 8. Hvor profilen bor

Fire kandidater, veid mot rollback-hastighet, observerbarhet, rate-limit og gjennomgangsport.

**Rå fil i `navikt/copilot`** (`raw.githubusercontent.com/navikt/copilot/main/nav-pilot-profile.json`, formen forrige forsøk valgte). Rollback er `git revert` og en merge, altså minutter, pluss CDN-cachen på raw-URL-er som er i størrelsesorden minutter. Gjennomgangsporten er PR-en. Revisjonssporet er git-historikken, som er akkurat det en endring som treffer alle utviklere i Nav trenger. Observerbarhet: ingen. Vi ser aldri hvem som hentet hva. Rate-limit: `raw.githubusercontent.com` er ikke `api.github.com` og har ikke de 60 anonyme forespørslene i timen som release-sjekken lever med ([update.go:22](../cli/nav-pilot/internal/cli/update.go)); med én forespørsel per bruker per døgn er det uansett ikke i nærheten av noe tak.

**Release-asset.** Å endre den krever en release. Det er nøyaktig det mekanismen finnes for å slippe. Forkastet.

**Endepunkt på `apps/mcp-registry`.** Appen er deployet med offentlig ingress `https://mcp-registry.nav.no` ([.nais/prod-gcp.yaml](../apps/mcp-registry/.nais/prod-gcp.yaml)), har Prometheus på ([app.yaml:29-31](../apps/mcp-registry/.nais/app.yaml)) og logger utvalgte endepunkt ([middleware.go:32](../apps/mcp-registry/middleware.go)). Den gir observerbarhet: vi ville se hentefrekvens og hvor mange som faktisk plukker opp en ny versjon. Men rollback går gjennom en deploy, ikke en revert, og appen blir en tilgjengelighetsavhengighet for noe som per definisjon aldri skal blokkere. Den siste innvendingen er svakere enn den ser ut, siden alt feiler mykt, men den første er reell: en deploy er tregere enn en merge.

**`apps/my-copilot`.** Samme argumenter som mcp-registry, uten fordelen: det er en Next.js-frontend, ikke et innholdsendepunkt.

**Anbefaling: rå fil i repoet.** Rollback-hastighet og gjennomgangsport veier tyngst for en knapp som treffer alle, og observerbarhet kan hentes billigere fra klientsiden: `internal/telemetry` har allerede en recorder-form med `RecordStalenessCheck(komponent, scope, resultat)` ([telemetry.go:67](../cli/nav-pilot/internal/telemetry/telemetry.go)), og en `RecordProfileVersion` i samme form gir oss «hvilken profilversjon er i bruk der ute» uten et endepunkt. Hvis det senere viser seg at vi trenger å vite hvem som ikke har plukket opp en endring, er mcp-registry riktig sted, og profilen kan flyttes dit uten at klientdesignet endrer seg.

**Én ting anbefalingen ikke løser: hvem som får trykke.** Repoet har ingen `.github/CODEOWNERS`. En fil på `main` som styrer standardmodellen for alle utviklere i Nav bør ikke ha samme gjennomgangsport som en typo i en README. Legg til en CODEOWNERS-oppføring for profilfila i samme PR som innfører den. Det er én linje, og uten den er «gjennomgangsporten er PR-en» en påstand og ikke en kontroll.

## 9. Blast radius

**Hva en dårlig redigering gjør.** Verste realistiske tilfelle er en syntaktisk gyldig profil som setter en modell-id som ikke finnes, eller som er dyrere enn den forrige. Skjemaet fanger ikke det: `ModelValuePattern` ([domain.go:161](../cli/nav-pilot/internal/domain/domain.go)) sjekker tegnsettet, ikke at modellen eksisterer. Klienten sender den videre og serveren avviser. Effekten er at brukere som ikke har satt egen modell ikke får startet en sesjon, i inntil 24 timer etter at fila er rettet, minus den som starter oftere.

**Hvordan det oppdages.** Ikke automatisk. Det er den ærligste beskrivelsen. To ting reduserer eksponeringen billig:

1. **Valider mot den kuraterte lista i CI.** `domain.KnownCopilotModels` ([domain.go:112](../cli/nav-pilot/internal/domain/domain.go)) er allerede fasit for hvilke modeller Nav kjenner, og `IsKnownCopilotModel` er eksportert ([provider.go:125](../cli/nav-pilot/internal/provider/provider.go)). En CI-sjekk som avviser en profil med en ukjent id fjerner hele denne feilklassen ved porten framfor hos brukeren. Dette er det viktigste enkelttiltaket i seksjonen, og det koster en `mise`-oppgave.
2. **La klienten avvise en ukjent id fra profilen.** Samme sjekk, andre side. En profil-id som ikke er i den kuraterte lista ignoreres og forrige verdi rir videre. Det gjør en dårlig redigering til en no-op i stedet for et utfall.

Med begge på plass krever et utfall at noen setter en modell som er kjent, kuratert og likevel feil. Det er en beslutningsfeil, ikke en mekanismefeil, og den løses ved å revertere.

**Hvordan brukeren kommer seg løs uten ny binær.** Tre veier, i økende grad av inngripen:

- `nav-pilot config set model <noe som virker>`. Brukerens eget valg slår profilen (§6), umiddelbart, uten å vente på TTL-en.
- Slette `~/.nav-pilot/cache.json`. Neste start henter på nytt, og hvis fila er rettet er brukeren tilbake.
- Ingenting. Innen 24 timer henter klienten den rettede profilen selv.

Den første er den som må stå i feilmeldingen hvis vi noen gang skriver en. Den andre bør stå i [RUNBOOKS.md](../cli/nav-pilot/RUNBOOKS.md).

## 10. Forholdet til kill switch ([#485](https://github.com/navikt/copilot/issues/485))

Issue #485 er åpen og innledes med «Ikke implementer nå. Notert for senere.» Den peker selv på dette arbeidet: «Hører sammen med arbeidet med å flytte standardvalg som modell ut av binæren og inn i en ekstern profil som oppdateres uten ny installasjon. Samme henteveien, samme cache, samme spørsmål om hva som skjer når brukeren er offline eller sitter på en gammel pin.»

**Bygg transporten én gang, men ikke det samme dokumentet.** En delt `httpGet`-med-timeout og en delt `cache.json` er riktig gjenbruk: det er infrastruktur, den er allerede der, og to kopier av den ville drifte fra hverandre. Å legge begge nyttelastene i samme JSON-dokument er derimot feil, av én grunn: de har motsatt feilmodus. Profilen må ignorere et dokument den ikke kan lese. Kill switchen må stoppe hvis den ikke kan lese det, eller den er ubrukelig nettopp for den den skal stoppe. Én parser med to motstridende kontrakter er en bug som venter.

**Hva som endrer seg hvis begge rir på samme henting.** Tre ting, og alle tre er grunner til å ikke gjøre det uten å tenke:

1. **TTL-en blir kill switchens TTL, ikke profilens.** 24 timer er greit for en modellstandard. Det er ikke greit for en lekkasje. Enten senkes intervallet for begge, som gir mer trafikk for ingen gevinst på profilsiden, eller så trenger kill switchen sin egen kadens, og da er hentingene uansett ikke én.
2. **En feilet henting må bety to ting samtidig.** «Behold forrige standard» og «ingen får jobbe». Det kan den ikke.
3. **Offline-svaret er allerede det åpne spørsmålet i #485,** og det er ikke opp til dette dokumentet å avgjøre.

**Konklusjon: del transporten, ikke dokumentet, og ikke TTL-en.** Konkret: `fetchJSON(url, timeout)` deles, feltet i `cache.json` er separat per dokument, og hver har sin egen parser med sin egen kontrakt. Skriv det ned i `profile.go` når den skrives, slik forrige forsøk gjorde: «Kill switchen vil ha samme transport og må ikke gjenbruke denne regelen: den må feile lukket. Lever den som sin egen lesning.»

## 11. Hva dette ikke løser

**Det treffer ikke de omtrent 60 prosentene av kredittbruken som ligger i modeller utviklere velger interaktivt.** De tre største postene er modeller som velges i klienten, i en sesjon, og som ikke står i noen agentfil. En profil som setter agent-standarder rører dem ikke, uansett hvor godt den virker. Mekanismen sikter mot det som er målt til rundt 14 prosent.

Det er ikke et argument mot å gjøre den 14 prosenten riktig. Det er et argument mot å presentere dette som et kostnadstiltak. Hvis kostnad er målet, ligger tiltakene et annet sted:

- **Gjøre det billige valget synlig i øyeblikket det tas.** Prisinformasjon der modellen velges, ikke i [modellvalg.md](modellvalg.md).
- **Måle først.** `apps/copilot-metrics` har dataene. En fordeling per modell per valgmåte, altså agent-deklarert mot interaktivt valgt, ville sagt om det er noen igjen å hente på agent-siden i det hele tatt.
- **En budsjettsignal i klienten.** `apps/copilot-api` har allerede budsjettdata ([budget.go](../apps/copilot-api/budget.go)).

Ingen av dem er denne mekanismen, og ingen av dem blir enklere av å bygge den.

Videre, eksplisitt utenfor dette forslaget: profilen distribuerer ikke innhold, den setter ikke `reasoning_effort` eller andre parametre, den er ikke en policy-kanal, og den er ikke et sted å legge noe som må virke for å kunne stole på klienten.

## 12. Når dette er feil valg

Fire argumenter mot, i synkende styrke. En gjennomgang som ender med å avvise forslaget bør avvise det på disse.

**1. Leveringsveien finnes allerede, og den er raskere enn folk tror.** `nav-pilot` sjekker allerede en gang i døgnet om innholdet er utdatert og tilbyr synk ([interactive.go:205-235](../cli/nav-pilot/internal/cli/interactive.go)). En modellendring i en `model:`-linje eller i et innsjekket agentpakke-manifest når brukerne innen ett døgn, mot ett ja-klikk, uten en eneste ny linje kode. Profilens eneste reelle fordel over den veien er at den virker uten samtykke og uten klikket. Det er en smal fordel, og den er ikke gratis: den fjerner også brukerens mulighet til å si nei.

**2. Verdien som skal gjøres flyttbar er ikke bestemt ennå.** Manifestet deklarerer `inherit` for copilot og `github-copilot/auto` for opencode ([legacy.go:69-79](../cli/nav-pilot/internal/agentpakke/legacy.go)). `auto` er en serverside-ruter som allerede følger kost/kvalitet-fronten uten noen klientmekanikk. `inherit` er ingen verdi. Å bygge en mekanisme for å flytte en verdi når ingen har bestemt hvilken verdi den skal flyttes til, er å bygge før behovet finnes. Den ærlige rekkefølgen er motsatt: bestem verdien, se om den trenger å flytte seg oftere enn en release, bygg mekanismen da.

**3. To dokumenter i samme repo som sier det samme.** Når navikt/copilot leverer sitt eget agentpakke-manifest ([legacy.go:15-23](../cli/nav-pilot/internal/agentpakke/legacy.go)), blir standardmodellen en innsjekket JSON-fil som hentes og valideres. Da har vi `.nav-pilot/agentpakke.json` og `nav-pilot-profile.json` i samme repo, med hver sin TTL, hver sin feilmodus og hver sin presedens, og begge sier hva standardmodellen er. Det er den typen duplisering som er billig å innføre og dyr å fjerne. En bedre variant av dette forslaget kan være «manifestet får en TTL og hentes uten synk», framfor et andre dokument ved siden av.

**4. Effekten er liten og målt.** Rundt 14 prosent av kredittbruken går gjennom agent-deklarerte modeller. Hvis begrunnelsen for arbeidet er kostnad, er dette feil sted å bruke innsatsen ([§11](#11-hva-dette-ikke-løser)).

**Hva som ville snudd meg.** Ett av disse, ikke en sum av dem:

- En konkret hendelse der Nav måtte flytte standardmodellen raskere enn en release, og ikke kunne. Én slik hendelse gjør hele §12 irrelevant.
- Kill switchen ([#485](https://github.com/navikt/copilot/issues/485)) blir besluttet bygget. Da finnes transporten uansett, og profilen koster et felt i stedet for en mekanisme. Det snur rekkefølgen: bygg #485 først, la profilen arve.
- Måledata som viser at synk-prompten faktisk avvises av mange nok til at innhold ikke når fram. Det er en påstand som kan sjekkes med telemetrien som allerede finnes ([telemetry.go:63-66](../cli/nav-pilot/internal/telemetry/telemetry.go)), og den bør sjekkes før noen skriver kode.

## 13. Påstander som ikke overlevde kontrollen

Fra oppdraget som utløste dokumentet, og fra den forrige gjennomgangen.

| Påstand | Hva koden sier |
| --- | --- |
| «PR #498 gjorde at nav-pilot skriver Nav-standarden inn i opencodes egen konfigurasjon» | **#498 er åpen, ikke merget** (`gh pr view 498`). Alt som hviler på den er betinget. |
| «#490 gjorde at materialiserte opencode-agenter bærer modellen sin, og det er det som løser innvending 1» | Halvparten stemmer. `BuildAgentFrontmatter` skriver `model:` ([frontmatter.go:196-207](../cli/nav-pilot/internal/source/frontmatter.go)), men `nav-pilot.agent.md` har ingen `model:`-linje, så standardpersonaen påvirkes ikke. Det som løser innvending 1 er manifest-fallbacket på Tier 1 copilot ([copilot_launch.go:82](../cli/nav-pilot/internal/provider/copilot_launch.go)), som oppdraget ikke nevner. |
| «To rekonstruksjonssteder for `StalenessCache`» | **Tre**: [staleness.go:133](../cli/nav-pilot/internal/artifacts/staleness.go), [staleness.go:141](../cli/nav-pilot/internal/artifacts/staleness.go), [update.go:130](../cli/nav-pilot/internal/cli/update.go). |
| «#485 er en PR» | #485 er et **issue**, åpent, merket «Ikke implementer nå». |
| «Lesestedet ligger på en kodesti som skal slettes» | Det som slettes er den innebygde adapteren `Default()` ([legacy.go:15-23](../cli/nav-pilot/internal/agentpakke/legacy.go)). Lesestedene leser `source.ActivePakke()` og blir stående. |
| «Verdien som gjøres oppdaterbar var allerede `github-copilot/auto`» | Stemmer for opencode ([legacy.go:78](../cli/nav-pilot/internal/agentpakke/legacy.go)). For copilot, som er standardklienten, er verdien `inherit`, altså ingen verdi ([legacy.go:71](../cli/nav-pilot/internal/agentpakke/legacy.go)). Innvendingen er sterkere enn den ble formulert. |
| «Rate-limit på anonyme api.github.com» er relevant for profilen | Bare hvis profilen legges på `api.github.com`. Anbefalingen i [§8](#8-hvor-profilen-bor) bruker `raw.githubusercontent.com`, som ikke deler det taket. Det er release-sjekken som lever med det ([update.go:22](../cli/nav-pilot/internal/cli/update.go)). |
| «14 % / 60 % av kredittbruken» | Kan ikke etterprøves i repoet. Står ubestridt, og er gjengitt i [§11](#11-hva-dette-ikke-løser) som premiss, ikke som verifisert tall. |
