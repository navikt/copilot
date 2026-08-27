# 📋 Agentpakke — beslutninger

Dette dokumentet forklarer **hvorfor nav-pilot oppfører seg som den gjør** rundt agentpakker: hvilke valg som er tatt bevisst, hva vi har valgt *ikke* å gjøre, hva som fortsatt er åpent, og hvilke begrensninger vi har akseptert med vitende og vilje. Det er skrevet for oss som jobber på nav-pilot, ikke for pakkeforfattere.

Selve kontrakten — hva en agentpakke *er*, og hva nav-pilot krever av den — står i [README.agentpakke.md](README.agentpakke.md) og i [`cli/nav-pilot/schemas/agentpakke-v1.json`](../cli/nav-pilot/schemas/agentpakke-v1.json). Her gjentas den ikke; her står begrunnelsene bak den.

**Regel for dette dokumentet:** hver påstand skal kunne sjekkes mot kode eller en sitert kilde. Der en plan eller et issue sier noe annet enn koden, er koden fasit, og avviket noteres ([§8](#8-der-kildene-er-uenige)). En begrunnelse som ikke fantes i noen kilde, er merket som skrevet ned her og nå ([§7](#7-begrunnelser-som-ikke-sto-skrevet-noe-sted-før-dette-dokumentet)) framfor å bli framstilt som en eldre beslutning.

## Status (27.08.2026)

| Arbeidspakke | Krav i [#437](https://github.com/navikt/copilot/issues/437) | PR | Status |
| --- | --- | --- | --- |
| M1 — kontrakt, kildevalg, `validate` | — | [#436](https://github.com/navikt/copilot/pull/436) | i main |
| WP1 — payload-verifisering | G1 | [#454](https://github.com/navikt/copilot/pull/454) | i main |
| WP2 — datadrevne personaer | C1, C2, C3 | [#455](https://github.com/navikt/copilot/pull/455) | i main |
| WP3a — staging | G2 (staging-halvdelen) | [#456](https://github.com/navikt/copilot/pull/456) | i main |
| WP3b — staged launch | G2 (launch), G3, C4, deler av F1 | [#458](https://github.com/navikt/copilot/pull/458) | åpen |
| WP4 — løfte install-sperren for Tier 2, med revisjonspin | «not installable yet»-stoppen | — | planlagt |
| WP7 — kontraktskorreksjon: roster per payload | G4 (P1, focused-persona) | [#461](https://github.com/navikt/copilot/pull/461) | i main |
| WP5 / WP6 — `model`-frontmatter, differensialtest | F1-resten, G4 | — | ikke startet |

**Rekkefølgen er bindende: revisjonspinnen ligger *inne i* WP4, ikke etter den.** Først runtime-gatene ([#462](https://github.com/navikt/copilot/issues/462)) og roster-rettelsen ([#461](https://github.com/navikt/copilot/issues/461)); så løftes install-sperren i samme arbeidspakke som pinnen. Grunnen er hele poenget: så lenge pinnen mangler, kloner hver Tier 2-launch den bevegelige standardbranchen, og en pakkeforfatter endrer hva som kjører ved brukerens neste launch uten noe samtykkepunkt ved install. Å løfte sperren først ville gjort det tilgjengelig for alle som installerer, framfor bare for dem som bevisst konfigurerer en kilde. Ikke stokk om på dette for å få noe merget raskere.

[#458](https://github.com/navikt/copilot/pull/458) erstatter [#457](https://github.com/navikt/copilot/pull/457), som lå stablet på #456 og ikke kunne retargetes etter at den ble merget. Samme arbeid, pluss rettelsene fra gjennomgangen — der #457-teksten og koden er uenige, er det #458 som gjelder ([§8](#8-der-kildene-er-uenige)).

Referansen alt måles mot er **grillmester v0.3.0, SHA `3573b93cc8b7568516117263562d073cae9ee7fc`**. Team eSyfo har navngitt den som den gjennomgåtte baselinen for G4, og har bedt om at en flytting av baselinen er en eksplisitt felles beslutning framfor stille drift ([kommentar i #437](https://github.com/navikt/copilot/issues/437)). Alle referanselinjenummer i kode og under peker på den SHA-en.

## 1. Retningen: Tier 2 er ikke en sidegren

**Navs egne samlinger skal selv bli agentpakker.** Manifestet erstatter samlingsmodellen (`collections/<navn>/manifest.json`) i tre faser: manifest valgfritt nå → `navikt/copilot` blir standard-agentpakka → samlingsmekanismen pensjoneres innenfor kontraktens deprekeringsvindu ([#435, «Q2 resolved as supersede»](https://github.com/navikt/copilot/issues/435)). Fase 2 er eksplisitt *ikke* blokkert av M2 ([#437](https://github.com/navikt/copilot/issues/437), «Related, not blocked»).

Konsekvensen for alt arbeid på Tier 2: **Tier 2 er den tidlige forekomsten av formen alt konvergerer mot, ikke en parallell løype.** Praktiske følger, som gjelder også når du leser eldre planer:

- Ikke bygg en parallell flyt for Tier 2. Samme install-pipeline, samme state-form, samme vokabular.
- Ikke anta at en klients tier er fast. `Tier()` utledes av manifestets form ved hver kjøring, og en pakke som senere får en `layout` skal plukkes opp uten migrasjon.
- Ikke anta at en pakke er *enten* Tier 1 *eller* Tier 2. Blandede pakker er gyldige i dag, og `guardTier2Only` (`cli/nav-pilot/internal/cli/agentpakke.go`) treffer bare pakker som er payload-only (`Layout == nil && HasTier(TierPayload)`).
- Vokabularet (`Collection:` i status-output, `"collection"` i state-JSON) står igjen med vilje. Å døpe det om treffer output og state-kompatibilitet for hver eneste eksisterende bruker, og hører hjemme i milepælen der alle installasjoner er agentpakker — ikke i en Tier 2-spesifikk endring.

Alt som forutsetter at Tier 1 og Tier 2 er permanent atskilte modeller, er feil.

## 2. Bevisste avvik fra grillmester-referansen

Avvikene er få og hver enkelt er begrunnet i koden der sjekken gjøres. Sammendraget:

| Avvik | Hvor det er begrunnet | Hvor kravet håndheves i stedet |
| --- | --- | --- |
| Modesjekk på kilden er subset + exec-bit, ikke eksakt `S_IMODE` | `payload.go`, `verifyPayloadFile` | Eksakte modes på det stagede treet, `VerifyPayloadExact` |
| Ingen pin av `target`-navn | `payload.go`, `PayloadManifest.Target` | Digestkjeden |
| `O_NOFOLLOW`/`O_NONBLOCK` + fstat på deskriptoren (referansen har ingen av delene) | `payload.go`, `openPayloadFile` | — (strengere enn referansen) |
| Kopiering styres av manifestets `files`, ikke av en walk av kildetreet | `stage.go`, filhode | — (strengere enn referansen) |
| `--project-dir` tas ikke i bruk *på launchen* (`--no-audit` tas i bruk, se [§2.5](#25-flagg-fra-referansen)) | `staged_launch.go`, filhode | Arbeidskatalogen *er* prosjektomfanget — ingen launch-sti setter `cmd.Dir`. Versjonsproben er unntaket og setter det, se [§2.5](#25-flagg-fra-referansen) |
| Copilots numeriske build-suffiks (`1.0.81-14`) godtas; referansen avviser det som prerelease | `runtime_gate.go`, `copilotBuildSuffixPattern` | Sammenligningen kjører på `major.minor.patch`; ekte prereleases (`-next.3`, `-beta`, `-rc.1`) avvises fortsatt |

### 2.1 Modesjekk på kilden: subset pluss exec-bit

Referansens `_verify_manifested_payload` sammenligner `S_IMODE` eksakt. nav-pilot krever i stedet at rettighetene på disk er en **delmengde** av det manifestet deklarerer, og at exec-biten er lik.

Begrunnelsen: en klone laget under `umask 0077` gir 0600 og 0700 for innholdsidentiske filer. En eksakt sjekk ville avvist enhver konform payload på slike maskiner — og de er vanlige på Nav-utstyr. En umask kan bare *fjerne* bits, aldri legge til, så retningen er trygg å tolerere. Motsatt vei er den ikke: en 0644-fil funnet som 0666, eller en 0755-fil funnet som 0777, er ikke et umask-artefakt, og avvises. Setuid, setgid og sticky avvises separat og eksplisitt, fordi Gos `FileMode.Perm()` maskerer til 0777 og en setuid 04755-fil ellers ville verifisert rent.

Merk at 0700 fortsatt er kjørbar: å klare exec-biten av 0755 krever en umask som inneholder 0111, som samtidig ville gjort hver katalog brukeren lager uåpnbar. En deklarert 0755-fil helt uten exec-bit er derfor et ekte avvik, ikke et umask-artefakt, og er fatalt.

Eksakte modes håndheves der de er håndhevbare: `StagePayload` chmod-er hver stagede fil til deklarert mode og kjører `VerifyPayloadExact` på resultatet. På et tre nav-pilot selv har laget, betyr «subset» at chmod-en ikke tok — og det er en bug verdt å feile på. `VerifyPayloadExact` skal aldri pekes mot en kildekloning ([#454](https://github.com/navikt/copilot/pull/454), [statusen i #437](https://github.com/navikt/copilot/issues/437)).

### 2.2 Ingen pin av `target`-navn

Referansen sjekker payload-manifestets `target` mot en hardkodet liste over sine egne byggemål. Agentpakke-kontrakten har ingen target-navn — en pakke navngir payloadene sine med klient × kontekst — så `Target` parses og eksponeres, men brukes ikke til å avvise noe. Det er digestkjeden som binder treet, ikke etiketten (`payload.go`, `PayloadManifest.Target`).

### 2.3 Hardere åpning av payload-filer

`openPayloadFile` åpner med `O_NOFOLLOW|O_NONBLOCK` og stat-er *deskriptoren*, ikke stien. Referansen gjør `lstat` og deretter `read_bytes`, uten noen av delene. Grunnen er vinduet mellom `lstat` og `open`: uten `O_NOFOLLOW` følges en innsatt symlink, og en lenke hvis mål tilfeldigvis matcher digesten verifiserer rent; uten `O_NONBLOCK` henger en innsatt fifo hele prosessen inne i `open(2)`. Staging leser kildefilene sine gjennom samme funksjon, så vinduet er lukket også på kopien.

### 2.4 Kopiering styres av manifestet

Referansen kopierer med `shutil.copytree(symlinks=True)` — den walker kilden. nav-pilot kopierer fra manifestets `files`-map: manifestet, ikke katalogen, er autoriteten på hva payloaden inneholder. En fil som smugles inn i kilden mellom verifisering og staging blir dermed aldri kopiert, framfor å bli kopiert og så avvist. I tillegg re-hashes hver fil *mens* den skrives, slik at vinduet mellom kildeverifiseringen og lesingen som kopierer bytene er lukket (`stage.go`, filhode og `stageFile`).

### 2.5 Flagg fra referansen

Fra referansens `build_launch_command` (`scripts/grillmester.py`, linje 647–689) tar vi ett av de to cplt-flaggene i bruk og utelater ett:

- **`--no-audit`** (linje 663) — **tas i bruk** på den stagede stien, på referansens plass: først i cplt-vektoren, før `--agent`. Vi leste det først som launcher-policy og en versjonsspesifikk workaround, og utelot det. Team eSyfo korrigerte det i svaret på G4-spørsmålet ([kommentar i #437](https://github.com/navikt/copilot/issues/437#issuecomment-5437575432)): på cplt-baselinen grillmester v0.3.0 er testet mot, kan cplts parent-side audit kjøre repo-kontrollerte Git-hjelpere *utenfor* sandboxen. Uten flagget er en staget Tier 2-launch dårligere isolert enn launcheren den skal være ekvivalent med. Å fjerne det igjen krever dokumentasjon på at en gjennomgått cplt-baseline retter oppførselen, pluss en minimumsversjonssperre som håndhever den — ikke at flagget ser overflødig ut.
- **`--project-dir`** (linje 666–667) — **utelates fortsatt på launchen**. nav-pilot starter i arbeidskatalogen: ingen launch-sti setter `cmd.Dir`, så cplt og klienten arver brukerens cwd. eSyfo aksepterer utelatelsen på nøyaktig det vilkåret, og at differansetestene asserterer oppførselen.

Ett unntak, og det er ikke en omgjøring: **versjonsproben** setter `--project-dir` mot en tom 0700-tempkatalog, slik referansen gjør i `_sandboxed_client_version` (linje 884–886). En launch er scopet til brukerens prosjekt med vilje; et `--version`-spørsmål skal ikke være scopet til noe. Uten flagget kjører proben i brukerens cwd, engasjerer repoets `.cplt.toml`-tillitsflyt og gir klienten lese- og skrivetilgang til repoet for å svare på et versjonsspørsmål. Proben setter også `--yes --quiet` på referansens plass (`_client_probe`, linje 879) — uten `--yes` stopper cplt på launch-bekreftelsen og proben feiler hver gang, på hver maskin.

Beslutningen om `--no-audit` er altså endret, og den er Team eSyfos ([§5.2](#52-grensen-for-equivalent-invocation-g4--eier-team-esyfo), [§8](#8-der-kildene-er-uenige)); den om `--project-dir` er vår, nå med eSyfos aksept.

### 2.6 Annet vi ikke speiler

- **Referansens cloud-launcher stager ikke i det hele tatt** — den peker klienten på payloaden der den ligger, inne i en immutabel Homebrew-bundle. Verify → kopier → re-verify er hentet fra samme prosjekts *lokale* modus (`scripts/grillmester_local.py`, `_materialize_opencode_config`). nav-pilot må stage fordi kilden kan være en midlertidig klone med umask-modes framfor manifestets. Ikke let etter staging i `build_launch_command`.
- **`ensure_opencode_runtime_support`s pre-seeding av `~/.config/opencode/.gitignore`** er ikke tatt med: det er en skriving til den delte config-katalogen, altså launcher-policy. Ikke-blokkerende for implementasjonen, men del av G4-ordlyden ([§5.2](#52-grensen-for-equivalent-invocation-g4--eier-team-esyfo)).

## 3. Staging-modellen

Kode: `cli/nav-pilot/internal/agentpakke/stage.go`, kallstedet i `cli/nav-pilot/internal/cli/pakke_launch.go` (`tryPakkeLaunch`).

**Én unik katalog per launch.** `os.MkdirTemp` under `~/.nav-pilot/staged/` (som følger `configPath()`, slik at `NAV_PILOT_CONFIG` også flytter staging-roten). En fast katalog per pakke ville vært feil: å tømme og skrive den på nytt trekker config-katalogen ut under en økt som allerede kjører mot den gjennom `OPENCODE_CONFIG_DIR`. To samtidige stagings av samme kilde deler ingenting utover roten.

**Sekvensen er verify → kopier (med re-hash) → chmod → eksakt re-verify.** Manifestet leses og parses én gang; de samme bytene skrives inn i det stagede treet og re-verifiseres mot det, så ingen andre lesing kan smugle inn et annet manifest underveis. `os.OpenFile`s mode-argument maskeres av prosessens umask — under `umask 0077` blir en fil opprettet som «0644» faktisk 0600 — så det er `chmod` gjennom deskriptoren som gjør deklarert mode til faktisk mode.

**Det stagede treet bærer sitt eget manifest.** Kilden kan være en midlertidig klone som slettes så snart staging returnerer, så treet er selvbeskrivende og den eksakte re-verifiseringen har et input som ikke avhenger av at kilden overlever. En payload som selv deklarerer en fil med det navnet, avvises før noe opprettes.

**Formen garanteres av konstruksjon, ikke av sjekk.** Symlinker, hardlinker ut av treet, enhetsnoder, fifoer og sockets kan ikke oppstå: hver node er enten en katalog nav-pilot lager, eller en vanlig fil åpnet med `O_CREATE|O_EXCL|O_NOFOLLOW`. Den eksakte re-verifiseringen er derfor en sjekk av vårt eget arbeid, ikke tillitsgrensen.

**Fail-closed, uten fallback.** Enhver feil i ethvert steg etterlater intet staget tre og intet brukbart resultat. `tryPakkeLaunch` returnerer feilen; en Tier 2-launch faller *aldri* tilbake til legacy-stien (G2).

**Katalogmodes verifiseres ikke, og det er ingenting å verifisere dem mot.** Payload-manifestet deklarerer modes for filer, ikke kataloger. nav-pilot velger derfor selv: 0700 for staging-roten, hvert staget tre og hver katalog i det — eiers egen, fordi et staget tre er en privat projeksjon for én prosess. Referansen gjør det samme. Under enhver umask som fortsatt lar brukeren gå inn i sine egne kataloger, kommer de ut som 0700; under en umask som klarer eier-bitene, feiler første `create` inni dem og staging avbrytes, som er riktig utfall for en maskin konfigurert slik.

**Opprydding: ved exit, pluss et 24-timers feiesluk.** Kallstedet `defer`-er `CleanupStaged` rundt klientprosessen, og skriver en advarsel hvis den feiler (et tre som overlever er verifisert config liggende på disk — si fra). Før hver staging kjøres `GCStaged(root, 24h)` best effort. Hva aldersregelen *gir*: et tre lekket av en krasj samles inn innen et døgn. Hva den *ikke* gir: den sier ingenting om bruk. mtime settes når staging skriver siste fil og røres ikke etterpå, så «eldre enn 24 t» betyr «staget for mer enn 24 t siden», ikke «ubrukt» — se [§6](#6-kjente-begrensninger-vi-har-valgt-med-vilje).

**`CleanupStaged` nekter for en sti som ikke er et staget tre.** Funksjonen er `RemoveAll` bak en navnevakt (`nav-pilot-staged-*`), slik at en bug i et kallsted ikke kan bli en rekursiv sletting av en hjemmekatalog. Samme vakt gjelder i `GCStaged`, som i tillegg hopper over alt den ikke selv har laget.

**Ikke alle payloads verifiseres ved launch.** Launch kjører `Load` (schema) og `StagePayload` (full verifisering av den payloaden som faktisk startes). Den kjører ikke `ValidateSource`, som digest-verifiserer *alle* deklarerte payloads (`load.go`, `ValidateSource` → `VerifyPayload` per payload) — å verifisere payloads du ikke starter, koster uten å gi noe. `nav-pilot validate` er alle-payloads-porten, og kjøres av pakkerepoets egen CI.

## 4. Launch-beslutningene

Kode: `cli/nav-pilot/internal/provider/staged_launch.go`, `cli/nav-pilot/internal/provider/pakke.go`, `cli/nav-pilot/internal/provider/runtime_gate.go`, `cli/nav-pilot/internal/cli/pakke_launch.go` ([#458](https://github.com/navikt/copilot/pull/458)).

**Staged Copilot krever cplt — uten fallback og uten prompt.** Legacy-stien for copilot godtar fortsatt en vanlig, usandboxet `copilot`-binær. På Tier 2-stien gjør den det ikke: en payload-launch er *definert* av sandbox-flaggene (`--allow-read <staged>`), payloaden er tredjeparts konfigurasjon vi ikke har skrevet, og å falle tilbake til en usandboxet klient ville stille droppet nettopp den isolasjonen payloaden ble staget for. Referansen krever også cplt. Brukere uten cplt får en feil som navngir `brew install navikt/tap/cplt`. Dette er en innstramming målt mot dagens copilot-støtte, men bare på Tier 2-stien.

**Flagget heter `--payload-context`, ikke `--context`.** `--context` er allerede navnet på Copilots kontekst-tier. Uten verdi brukes manifestets `defaultContext`. Å sende flagget på en launch som *ikke* går fra en Tier 2-payload er en **feil**, ikke en stille no-op — samme policy som ukjente config-nøkler får (`payloadContextUnsupported`). En ukjent kontekst gir en feil som lister de deklarerte. Det finnes ingen TOML-nøkkel for payload-kontekst; det er et per-launch-valg.

**Manifestet er autoritativt for personaer.** `PrimaryAgent` (Tier 1, legacy launch) og `PrimaryAgentFor` (Tier 2, staged launch) leser den aktive pakka og har **ingen** fallback til nav-pilots egen default. En fallback ville injisert Navs nav-pilot-persona inn i en fremmed pakkes launch, samtidig som materialiseringen korrekt degraderer den til subagent (`internal/artifacts/export.go`). Den tomme strengen kan ikke observeres på launch-stiene: schemaet krever `primaryAgents` med `"minItems": 1` på hver Tier 1-klientoppføring og på hver Tier 2-payload, sjekket av `agentpakke.Load` før et manifest i det hele tatt festes til en kilde — så den tomme saken er *urepresenterbar*, ikke bare usannsynlig. `stagedPrimaryAgent` feiler likevel høyt framfor å sende et tomt `--agent`, i tilfelle et framtidig kallsted bryter invarianten `SetActivePakke` dokumenterer.

**Rosteret hører til payloaden, ikke til klienten (WP7).** For en Tier 2-oppføring ligger `primaryAgents` på hver `payloads.<kontekst>`, er påkrevd der, og leses bare derfra — `PrimaryAgentFor(client, context)` faller aldri tilbake til klientnivået, og en Tier 2-oppføring som likevel har et klientnivå-roster får det ignorert.

Bakgrunnen er Team eSyfos svar på G4-spørsmålene ([kommentar i #437](https://github.com/navikt/copilot/issues/437#issuecomment-5437575432)): ved det pinnede referansepunktet lister klientoppføringen `grillmester` først, mens *begge* `focused`-payloadene med vilje bare inneholder `barista` og `grill-inspektor`. En `focused`-launch skulle altså sendt `--agent grillmester` mot et tre som ikke har den agenten. eSyfo avviste både å endre `full`-defaulten og å utvide focused-payloaden for å passe kjøreren, og ba om «a context-specific default/allowed-agent declaration (or an equivalent jointly reviewed solution)».

Vi leste det som en **strukturell feil i kontrakten, ikke et hull**: rosteret hører til den enheten som faktisk bærer agentene. For Tier 1 er det repoets `layout`, for Tier 2 er det payload-treet. En Tier 2-klientoppføring som deklarerer ett roster for payloads med ulike rostere sier noe usant om de bytene den binder. Derfor ble feltet *flyttet*, ikke supplert.

To alternativer ble vurdert og forkastet:

- **Valgfri overstyring på payloaden, med fallback til klientnivået.** Forkastet fordi den bevarer den usanne påstanden på klientnivå permanent, og legger en fallback-regel oppå den som hver framtidig konsument må implementere likt.
- **Én `defaultAgent`-streng per payload.** Forkastet fordi den bare uttrykker standardpersonaen og ikke hvilke agenter konteksten i det hele tatt tilbyr — den samme informasjonen `primaryAgents` allerede bærer på Tier 1, i et annet format, for samme formål.

**nav-pilot kryssjekker ikke deklarerte agentnavn mot filene i payloaden.** Hvordan en agentfil heter er klientspesifikk konvensjon (`.agent.md`, `agents/<navn>.md`, en `name:`-frontmatter), og det er digestkjeden som binder payloaden — ikke gjetning ut fra filnavn. Et navn i `primaryAgents` som payloaden ikke har, feiler i klienten, ikke i nav-pilot.

**Kontraktsversjonen ble stående på `"1"`, og 90-dagersvinduet ble ikke brukt.** Dette er en korreksjon gjort *før første konsument*: Tier 2 kunne ikke installeres, ingen bruker hadde en Tier 2-kilde, og det eneste manifestet som fantes var grillmesters — hvis eiere var med på å utforme endringen. Hadde det funnes én konsument, ville dette krevd bump av `contractVersion` **og** de 90 dagene; vinduet binder fra det øyeblikket konsument nummer to finnes. Det står også i kontraktens [Kompatibilitet-kapittel](README.agentpakke.md#én-korreksjon-før-første-konsument-august-2026), slik at ingen senere leser reglene der og konkluderer at vinduet ble brutt. Konsekvensen på deres side: grillmester regenererer manifestet og navngir et nytt pinnet referanse-SHA — manifestet ved `3573b93cc8b7568516117263562d073cae9ee7fc` validerer ikke lenger, med en feil som navngir hver payload som mangler feltet.

**Den aktive pakka bor i `internal/source`, ikke i `internal/provider`.** `internal/artifacts` trenger de samme deklarasjonene for materialisert frontmatter og kan ikke importere provider; `internal/source` er den laveste pakka begge allerede importerer. Provider-sømmen ligger der planen la den, og delegerer ([#455](https://github.com/navikt/copilot/pull/455)).

**Modellrekkefølge:** brukerens pin vinner, deretter pakkas `defaultModel`, ellers ingenting. Literalen `"inherit"` betyr *ingen* `--model` i det hele tatt, som også er det referansen sender (`build_launch_command` videresender ingen modell).

**Klientformene** er transkribert fra referansen med linjenumrene sitert i testen som asserterer dem, slik at G4-differansen har noe å sammenligne mot framfor noe å oppdage:

- opencode: `OPENCODE_CONFIG_DIR` mot det stagede treet, videreført gjennom cplt med `--pass-env`, pluss `--allow-read <staged>` så sandboxen får lese treet. `--allow-read` kommer før `--pass-env`, fordi det er rekkefølgen referansen sender dem i (WP3-planen skrev motsatt rekkefølge; referansen vant).
- copilot: `--plugin-dir <staged>` foran `--agent <pakke>:<agent>` — persona-navnet er plugin-kvalifisert med pakkas navn.
- `--mode plan` fortsetter å mappe til opencodes innebygde, lesende `plan`-agent, som på legacy-stien.
- opencodes subkommandoer håndteres som i referansen: `--agent`/`--model` bindes bare til inngangspunkter som godtar dem. Ingen videresendte argumenter → bare bindingen; `run …` → `run <binding> …`; en annen opencode-subkommando → videresendt urørt, uten `--agent`; alt annet → binding foran. Transkribert fra `_opencode_client_arguments` (linje 692–704) og `OPENCODE_COMMANDS` (linje 37–63), med linjenumrene sitert i `openCodeClientArgs`.

**En staget økt arver ikke brukerens egne Copilot-instruksjoner.** `CopilotEnv` er nå et skall rundt `copilotEnv(otelLogLevel, injectUserInstructions bool)` (`internal/provider/copilot_launch.go`); den stagede copilot-launchen sender `pakkeAcceptsUserContext("copilot")`, som er `false` i dag. Begrunnelsen: referansen sender miljøet urørt, og en pakke bør se konteksten forfatteren faktisk har testet mot, framfor hva som måtte ligge i brukerens `~/.copilot`. En `COPILOT_CUSTOM_INSTRUCTIONS_DIRS` brukeren selv har eksportert, arves fortsatt urørt fra `os.Environ()` — det er brukerens eget valg, ikke noe nav-pilot injiserer. Om en pakke skal kunne velge dette inn, er et manifestspørsmål ([§5.3](#53-skal-en-pakke-kunne-be-om-brukerens-egen-kontekst--eier-nav-pilot)); beslutningspunktet er med vilje ett funksjonskall, slik at feltet blir en linjes endring den dagen det finnes.

**Spørsmålet om usandboxet launch stilles først når svaret kan brukes.** «Ville kjørt usandboxet» er bare sant på legacy-stien — en Tier 2-launch krever cplt og nekter uten — og hvilken av de to det er, vet man ikke før `tryPakkeLaunch` har resolvet kilden og lest tieren. Bekreftelsen er derfor flyttet inn i `launchClientConfirming` (`internal/cli/interactive.go`), slik at ingen blir bedt om å bekrefte en launch som like etter avvises.

**Ingenting på denne stien skriver til brukerens delte klientconfig (G2).** Den stagede opencode-launchen hopper over både `EnsureOpenCodeNavContext` og `EnsureOpenCodeOTelConfig` — begge skriver inn i `~/.config/opencode` — og redigerer heller aldri payloaden, hvis bytes er digestbundet. OTel reiser fortsatt som miljøvariabler, som ikke er config-mutasjon (se konsekvensen i [§6](#6-kjente-begrensninger-vi-har-valgt-med-vilje)).

**Fail-closed begynner ved tier-porten, ikke før den.** Kilden resolves ved hver launch, fordi det er slik nav-pilot får vite om denne launchen i det hele tatt er Tier 2. En resolve-feil lander *før* tier-porten, der ingenting ennå sier at det er en payload i bildet — kilden deklarerer kanskje ingen. Å være offline, eller ha et utdatert reponavn i config, skal derfor ikke blokkere en launch som virket før Tier 2-staging fantes: nav-pilot advarer og tar legacy-stien, slik `EnsureOpenCodeNavContext` alltid har gjort. Forbi tier-porten, der payloaden er kjent, er alt fail-closed uten fallback.

Dette er en rettelse: #457 beskrev den fatale varianten som en akseptert konsekvens for copilot-brukere med egen kilde. Den framstillingen var feil på to punkter — legacy-opencode kloner den *state-registrerte* repoen, ikke den konfigurerte, og en feil der er bare en advarsel — og den fatale varianten rammet begge klienter, altså enhver offline bruker med `source` satt, inkludert `navikt/copilot` skrevet eksplisitt. Rettet i #458 framfor dokumentert.

**Runtime-portene er nav-pilots, ikke kontraktens.** Kode: `cli/nav-pilot/internal/provider/runtime_gate.go`. Team eSyfo gjorde håndheving obligatorisk ([kommentar i #437](https://github.com/navikt/copilot/issues/437#issuecomment-5437575432)): «Runtime client compatibility ranges and a reviewed cplt minimum should be enforced, not only validated as manifest syntax.» En staged launch kjører derfor to porter før den bygger noe som helst — cplt-gulvet alltid, `compatibility` når klientoppføringa deklarerer et område.

**cplt-gulvet er en konstant i nav-pilot, ikke et manifestfelt.** `minStagedCpltStamp` er hentet fra referansens `SUPPORTED_CPLT_RELEASE` (`scripts/grillmester.py`, linje 27). Det kunne vært et felt i kontrakten, men da ville eieren blitt feil: hvilken cplt-versjon som er trygg nok til å kjøre en sandboxet payload, er *nav-pilots og cplts* vurdering, ikke pakkeforfatterens. En pakke kunne ellers senket gulvet under det nav-pilot har gjennomgått — presis den porten som skal beskytte brukeren mot pakka. Å flytte konstanten følger samme fellesbeslutningsregel som referansepinnen. Den er samtidig utgangsrampen for `--no-audit` ([§2.5](#25-flagg-fra-referansen)): den dagen en gjennomgått cplt-baseline retter parent-side audit, er endringen én diff — hev stempelet, fjern flagget, legg ved dokumentasjonen.

**«Vet ikke» er fatalt, ikke en advarsel.** En probe som feiler, eller versjonsutdata nav-pilot ikke kan parse, avviser launchen på lik linje med en versjon som faktisk er utenfor området. Tre grunner: portene ligger *forbi* tier-porten, der alt er fail-closed uten fallback; referansen er fatal på samme sted (`check_cplt`, `_strict_client_version_output`); og en versjonssjekk som blir grønn på manglende data er nøyaktig feilmodusen [#452](https://github.com/navikt/copilot/pull/452) rettet i nav-pilots egen cplt-skew-sjekk. Et område som ikke kan håndheves, er ikke håndhevet — og da har eSyfos krav ikke noe innhold. Deklarerer manifestet ingen `compatibility`, er det ingenting å håndheve: da probes klienten ikke i det hele tatt.

## 5. Åpne spørsmål

Disse er **ikke** avgjort. Ikke behandle dem som avgjorte, og ikke «rydde opp» i dem ved å velge ett svar i forbifarten.

### 5.1 Parity for `focused`-kontekst (G4) — eier: Team eSyfo

Referanselauncheren når `focused` bare gjennom `grillmester local` (loopback, macOS). For cloud-launchen har vi foreslått at focused-scenarioene asserterer byte-parity på payloaden pluss nøyaktig samme config-pekende mekanisme som `full`, uten local-modusens herdingsflagg — og at local-modusens invokasjonskontrakt forblir M4-scope. Spørsmålet er stilt i [statuskommentaren i #437](https://github.com/navikt/copilot/issues/437) og gjelder ordlyden i G4, ikke hva vi bygger. Blokkerer G4-signoff, ikke implementasjon.

### 5.2 Grensen for «equivalent invocation» (G4) — eier: Team eSyfo

Vi planlegger å assertere config-plassering, persona inkludert `<pakke>:<agent>`-kvalifiseringen, modellhåndtering, `--pass-env OPENCODE_CONFIG_DIR`, `--allow-read <staged>` og cplt-agentvalg. Vi tar i bruk `--no-audit` og utelater fortsatt `--project-dir` ([§2.5](#25-flagg-fra-referansen)), normaliserer modes ved staging ([§2.1](#21-modesjekk-på-kilden-subset-pluss-exec-bit)), og hopper over `.gitignore`-pre-seedingen ([§2.6](#26-annet-vi-ikke-speiler)). Spørsmålet til eSyfo var om noen av dem er bærende for deres guardrail-historie. **Besvart for flaggene:** `--no-audit` er bærende og tas i bruk, `--project-dir` kan utelates så lenge arbeidskatalogen er prosjektomfanget ([kommentar i #437](https://github.com/navikt/copilot/issues/437#issuecomment-5437575432)). Resten av G4-ordlyden er fortsatt stilt samme sted som 5.1.

### 5.3 Skal en pakke kunne be om brukerens egen kontekst? — eier: nav-pilot

**Slik det er i dag:** en staget launch blander ikke inn brukerens eget `~/.copilot`-innhold. `buildStagedCopilotSpec` kaller `copilotEnv(r.OtelLogLevel, pakkeAcceptsUserContext("copilot"))`, og `pakkeAcceptsUserContext` returnerer `false` for alle klienter — det finnes ikke noe manifestfelt å lese ennå. Uten en deklarasjon blandes ingenting inn: en tredjeparts pakke skal ikke stille motta Nav-innhold forfatteren aldri har testet mot ([§4](#4-launch-beslutningene)).

**Forslaget** er et felt `acceptsUserContext` på klientoppføringen, navngitt i kommentaren til `pakkeAcceptsUserContext` (`internal/provider/staged_launch.go`). Da blir funksjonen en linjes manifestlesing, som `pakkeDeclaredModel`. Schemaet er med vilje urørt i #458.

**Statusen på forslaget:** det er *vårt* forslag. Det er ikke lagt fram for Team eSyfo, og det finnes ingen avtale om det. Feltet står ikke i [`agentpakke-v1.json`](../cli/nav-pilot/schemas/agentpakke-v1.json), og per i dag heller ikke skrevet ned i #435 eller #437 — kodekommentaren peker på #437 som stedet det skal tas opp. Er det tatt opp der når du leser dette, er den kommentaren kilden; ellers er dette dokumentet og koden det eneste stedet forslaget finnes. Et slikt felt er additivt og krever ikke bump av `contractVersion`.

### 5.4 WP4-beslutninger som ennå bare finnes i planen

Install-sperren for payload-only-pakker (`guardTier2Only`) står fortsatt, og [README.agentpakke.md](README.agentpakke.md#begrensninger-i-dag) beskriver den korrekt som dagens status. WP4-planen løfter den uten ny state-form og uten nytt vokabular ([§1](#1-retningen-tier-2-er-ikke-en-sidegren)), men ingenting av det er implementert eller merget. Behandle det som retning, ikke som beslutning. Det samme gjelder `nav-pilot export opencode`-stoppen for ikke-kanoniske layouts, som #437 lister som «re-scope or implement» og som ikke er tatt stilling til.

## 6. Kjente begrensninger vi har valgt med vilje

Ikke «fiks» disse ved et uhell — de er valgt, og de har begrunnelser.

- **Aldersregelen i `GCStaged` er bevisst naiv.** Den betyr «staget for mer enn `maxAge` siden», ikke «ubrukt». En økt som lever lenger enn 24 timer får treet sitt feid bort under seg, og klienten begynner å feile på å lese sin egen config. Det er en *synlig* feil, ikke en utrygg en, og en døgnlang interaktiv økt er ikke verdt pid-liveness-maskineri. Merket i koden med oppgraderingsstien (`ponytail:`-kommentar i `stage.go` og i `pakke_launch.go`).
- **Én linje kan ikke dekkes av en test, og blir stående likevel.** `O_EXCL|O_NOFOLLOW` på opprettelsen av den stagede fila (`stage.go`, `stageFile`) er uoppnåelig fra en test: destinasjonen ligger i en `MkdirTemp`-katalog opprettet mikrosekunder tidligere, hvis navn ingen annen prosess kjenner. Flaggene blir stående slik at unikhets-invarianten er *håndhevet* framfor *antatt* — hvis antagelsen en gang ryker, feiler staging framfor å skrive gjennom det som måtte ligge der. 13 av 14 sjekker i [#456](https://github.com/navikt/copilot/pull/456) er mutasjonstestet; den fjortende er opplyst framfor skjult.
- **`stageCopyHook` er en testsøm i produksjonskoden.** Feilstien midt i kopieringen er ellers uoppnåelig fra ethvert input en test kan konstruere — hver fil kopien rører ble bevist eksisterende, regulær og korrekt hashet øyeblikk før. Hooken lar en test mutere kilden mellom verifisering og kopi, altså det ekte TOCTOU-tilfellet, og gir dermed både re-hashen og fail-closed-oppryddingen noe som faktisk kan feile.
- **`VerifyPayload` returnerer bare første brudd**, i motsetning til resten av valideringen som samler alle funn. Forbi første avvik er payloaden utroverdig, og en pakkeforfatter får ingenting igjen for en full liste over hva mer som er galt med et tre som uansett ikke stages.
- **Ingen OTel-config-injeksjon i staged opencode-modus.** `experimental.openTelemetry` skrives ikke, fordi det ville betydd å redigere enten den delte configen eller den digestbundne payloaden. OTel-miljøvariablene settes fortsatt.
- **To `Unreachable:`-grener** rundt `rec.perm()` i `payload.go` og `stage.go` er beholdt som defensiv feilretur, fordi `ParsePayloadManifest` allerede har avvist alle andre modes.
- **Kosmetisk rest i `openCodeDefaultModel`:** kjøres `config setup` med en `inherit`-pakke aktiv, merkes den innebygde modell-id-en «Nav default». Ingen M2-flyt setter en pakke før setup, så ingenting når dit i dag (`internal/provider/pakke.go`).
- **Tier-cachen er provisorisk og fjernes med WP4.** `tierCacheTTL` (6 timer, `internal/cli/pakke_launch.go`) finnes bare for å slippe en klone ved hver launch. Revisjonspinnen — en lokalt cachet, immutabel revisjon valgt av et install- eller update-steg, krevd av Team eSyfo ([kommentar i #437](https://github.com/navikt/copilot/issues/437#issuecomment-5437575432)) — svarer på et strengere spørsmål for hver installerte kilde, og lar tier-cachen stå igjen bare for launch-før-install. Da slettes den, framfor å bli to cacher med hver sin TTL og ingen forklaring på hvorfor begge finnes.

## 7. Begrunnelser som ikke sto skrevet noe sted før dette dokumentet

Tre valg var tatt uten at begrunnelsen fantes i kode, plan eller issue. De er skrevet ned her, i denne omgangen — ikke hentet fra et tidligere dokument. Behandle dem som begrunnelser vi står inne for, ikke som noe som har vært gjennom en gjennomgang.

- **`stagedMaxAge = 24h` er et avgrensningstall, ikke et målt tall.** Det skal overleve enhver plausibel økt, og samtidig være kort nok til at en krasjet økt ikke etterlater verifisert config på disk i det uendelige. Ingenting er målt. Ikke les presisjon inn i tallet — og ikke bygg noe som forutsetter at det er finstemt.
- **Staging-roten er `~/.nav-pilot/staged/` og ikke `os.UserCacheDir()`** fordi operativsystemet kan rydde en cache-katalog under en økt som kjører, og fordi nav-pilots egen state allerede bor på ett sted. En config-katalog som feies bort midt i en økt er nøyaktig den feilen per-launch-modellen finnes for å unngå.
- **Prefikset er fast (`nav-pilot-staged-`) og ikke `<pakke>-<klient>-<kontekst>-`** fordi det er det som gjør navnevakten i `CleanupStaged` mulig: en vakt kan ikke bygges på et navn som varierer per pakke. Kostnaden er reell og verdt å notere: en kataloglisting røper ikke lenger hvilken pakke som kjører.

## 8. Der kildene er uenige

- **Staging-katalog.** M2-planen beskrev en fast katalog per pakke/klient/kontekst som «tømmes og skrives på nytt per launch». Koden bruker en unik `MkdirTemp`-katalog per launch, av hensyn til samtidige økter ([§3](#3-staging-modellen)). Koden gjelder; M2-planens formulering er utdatert.
- **Rekkefølgen på cplt-argumentene for opencode.** WP3-planen skrev `--pass-env` før `--allow-read`; referansen sender dem motsatt, og koden følger referansen ([§4](#4-launch-beslutningene)).
- **Brukerens egne Copilot-instruksjoner.** #457 gjenbrukte `CopilotEnv` som den var, slik at brukerens `~/.copilot`-innhold ble blandet inn i en Tier 2-økt. #458 snur det: ingenting injiseres, og valget er samlet i ett kall ([§4](#4-launch-beslutningene), [§5.3](#53-skal-en-pakke-kunne-be-om-brukerens-egen-kontekst--eier-nav-pilot)). WP3-planens klassifisering av dette som «launcher policy» er dermed forlatt.
- **Hva en resolve-feil gjør.** #457-teksten kaller den en akseptert regresjon for copilot-brukere med egen kilde; koden i #458 advarer og faller tilbake til legacy-stien, og #457-framstillingen var feil på to punkter ([§4](#4-launch-beslutningene)).
- **`--no-audit` som launcher-policy.** #458 og en tidligere §2.5 klassifiserte flagget som launcher-policy vi ikke tok i bruk, med referansens egen kommentar som kilde. Team eSyfo svarte at det er bærende ved den testede cplt-baselinen ([kommentar i #437](https://github.com/navikt/copilot/issues/437#issuecomment-5437575432)). Deres svar gjelder: den stagede stien sender `--no-audit`, og vår klassifisering er forlatt ([§2.5](#25-flagg-fra-referansen)).
- **Hvor `primaryAgents` hører hjemme.** M1-kontrakten ([#436](https://github.com/navikt/copilot/pull/436)) plasserte rosteret på klientoppføringen, også for Tier 2. Team eSyfos G4-svar viste at payload-rostere skiller seg per kontekst, og at plasseringen dermed var feil. Kontrakten gjelder slik den er nå: rosteret ligger på payloaden for Tier 2 ([§4](#4-launch-beslutningene)). M1-formen er forlatt, ikke deprekert — den hadde ingen konsumenter.
- **Omstokking av opencode-argumenter.** WP3-planen kuttet referansens `_opencode_client_arguments` som «legges inn hvis noen melder fra». #458 implementerer den ([§4](#4-launch-beslutningene)); planen er utdatert på dette punktet.

## Se også

- [Agentpakke-kontrakten](README.agentpakke.md) — hva en agentpakke er, og hva nav-pilot krever av den
- [`cli/nav-pilot/schemas/agentpakke-v1.json`](../cli/nav-pilot/schemas/agentpakke-v1.json) — kontrakten selv
- [#435](https://github.com/navikt/copilot/issues/435) — PRD-en, med kontraktbeslutningene per revisjon
- [#437](https://github.com/navikt/copilot/issues/437) — M2: krav G1–G4, C1–C4, F1, og statusdialogen med Team eSyfo
- [cli/nav-pilot/DESIGN.md](../cli/nav-pilot/DESIGN.md) — internt design, sømmer og migrasjonsplan
