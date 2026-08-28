# 📋 Beslutninger om agentpakker

Dette dokumentet forklarer **hvorfor nav-pilot oppfører seg som den gjør** rundt agentpakker: hvilke valg som er tatt bevisst, hva vi har valgt *ikke* å gjøre, hva som fortsatt er åpent, og hvilke begrensninger vi har akseptert med vitende og vilje. Det er skrevet for oss som jobber på nav-pilot, ikke for pakkeforfattere.

Selve kontrakten står i [README.agentpakke.md](README.agentpakke.md) og i [`cli/nav-pilot/schemas/agentpakke-v1.json`](../cli/nav-pilot/schemas/agentpakke-v1.json), altså hva en agentpakke *er* og hva nav-pilot krever av den. Her gjentas den ikke. Her står begrunnelsene bak den.

**Regel for dette dokumentet:** hver påstand skal kunne sjekkes mot kode eller en sitert kilde. Der en plan eller et issue sier noe annet enn koden, er koden fasit, og avviket noteres ([§8](#8-der-kildene-er-uenige)). En begrunnelse som ikke fantes i noen kilde, er merket som skrevet ned her og nå ([§7](#7-begrunnelser-som-ikke-sto-skrevet-noe-sted-før-dette-dokumentet)) framfor å bli framstilt som en eldre beslutning.

## Status (28.08.2026)

| Arbeidspakke | Krav i [#437](https://github.com/navikt/copilot/issues/437) | PR | Status |
| --- | --- | --- | --- |
| M1: kontrakt, kildevalg, `validate` | ingen | [#436](https://github.com/navikt/copilot/pull/436) | i main |
| WP1: payload-verifisering | G1 | [#454](https://github.com/navikt/copilot/pull/454) | i main |
| WP2: datadrevne personaer | C1, C2, C3 | [#455](https://github.com/navikt/copilot/pull/455) | i main |
| WP3a: staging | G2 (staging-halvdelen) | [#456](https://github.com/navikt/copilot/pull/456) | i main |
| WP3b: staged launch | G2 (launch), G3, C4, deler av F1 | [#458](https://github.com/navikt/copilot/pull/458) | i main |
| WP4′: løfte install-sperren for Tier 2, pinne revisjonen, fjerne per-launch-staging | «not installable yet»-stoppen, revisjonspinnen ([#437](https://github.com/navikt/copilot/issues/437#issuecomment-5437575432)), tier-cachen ([#469](https://github.com/navikt/copilot/issues/469)) | ingen | under arbeid |
| WP7: kontraktskorreksjon, roster per payload | G4 (P1, focused-persona) | [#461](https://github.com/navikt/copilot/pull/461) | i main |
| WP5 / WP6: `model`-frontmatter, differensialtest | F1-resten, G4 | ingen | ikke startet |

**Rekkefølgen er bindende. Revisjonspinnen ligger *inne i* WP4′, ikke etter den.** Først runtime-gatene ([#462](https://github.com/navikt/copilot/issues/462)) og roster-rettelsen ([#461](https://github.com/navikt/copilot/issues/461)), så løftes install-sperren i samme arbeidspakke som pinnen. Så lenge pinnen mangler, kloner hver Tier 2-launch den bevegelige standardbranchen, og da kan en pakkeforfatter endre hva som kjører ved brukerens neste launch uten noe samtykkepunkt ved install. Å løfte sperren først ville gjort det tilgjengelig for alle som installerer, framfor bare for dem som bevisst konfigurerer en kilde. Ikke stokk om på dette for å få noe merget raskere.

[#458](https://github.com/navikt/copilot/pull/458) erstatter [#457](https://github.com/navikt/copilot/pull/457), som lå stablet på #456 og ikke kunne retargetes etter at den ble merget. Samme arbeid, pluss rettelsene fra gjennomgangen. Der #457-teksten og koden er uenige, er det #458 som gjelder ([§8](#8-der-kildene-er-uenige)).

To ting har hittil delt én SHA, og skilles her.

**Den gjennomgåtte G4-baselinen er grillmester `5cc546c127ac224fbf89b5299ad31675011307f6`** ([navikt/grillmester#63](https://github.com/navikt/grillmester/pull/63)). Det er revisjonen begge prosjektene differensialtester mot. Forrige baseline var `3573b93cc8b7568516117263562d073cae9ee7fc` (grillmester v0.3.0). Team eSyfo navnga den og ba om at en flytting er en eksplisitt felles beslutning framfor stille drift. Flyttingen hit er den beslutningen ([kommentar i #437](https://github.com/navikt/copilot/issues/437#issuecomment-5451860344)). Baselinen står her og ikke som en konstant i koden, fordi ingenting i nav-pilot leser SHA-en under kjøring. Ved neste flytting: oppdater denne linjen, og bare den.

**Referanselinjenumrene i kildekommentarene er noe annet, og følger ikke baselinen.** «Transkribert fra `_bounded_command_output`, `grillmester.py` linje 770 ved `3573b93c`» er en opplysning om hva vi faktisk leste da koden ble skrevet. Den blir ikke usann av at baselinen flytter seg. Å skrive om SHA-en uten å lese filen på nytt gjør den derimot selvsikkert feil, og det er verre enn en riktig referanse til en eldre revisjon. Slike sitater blir stående på den SHA-en de ble transkribert fra. Flytt et sitat bare når du har lest filen på nytt og faktisk kontrollert linjenumrene.

Ved flyttingen til `5cc546c1` ble alle sitatene kontrollert på nytt. `scripts/grillmester.py`, `scripts/grillmester_local.py` og `plugin/manifest.json` er byte-identiske i de to revisjonene, så hvert linjenummer under peker på samme innhold begge steder, og `SUPPORTED_CPLT_RELEASE` er uendret. Det som faktisk endret seg, er `.nav-pilot/agentpakke.json`, altså selve agentpakke-manifestet.

## 1. Retningen: Tier 2 er ikke en sidegren

**Navs egne samlinger skal selv bli agentpakker.** Manifestet erstatter samlingsmodellen (`collections/<navn>/manifest.json`) i tre faser: manifest valgfritt nå, så blir `navikt/copilot` standard-agentpakka, og til slutt pensjoneres samlingsmekanismen innenfor kontraktens deprekeringsvindu ([#435, «Q2 resolved as supersede»](https://github.com/navikt/copilot/issues/435)). Fase 2 er eksplisitt *ikke* blokkert av M2 ([#437](https://github.com/navikt/copilot/issues/437), «Related, not blocked»).

Konsekvensen for alt arbeid på Tier 2: **Tier 2 er den tidlige forekomsten av formen alt konvergerer mot, ikke en parallell løype.** Følgene gjelder også når du leser eldre planer.

- Ikke bygg en parallell flyt for Tier 2. Samme install-pipeline, samme state-form, samme vokabular.
- Ikke anta at en klients tier er fast. `Tier()` utledes av manifestets form ved hver kjøring, og en pakke som senere får en `layout` skal plukkes opp uten migrasjon.
- Ikke anta at en pakke er *enten* Tier 1 *eller* Tier 2. Blandede pakker er fortsatt gyldige, og predikatet `payloadOnly` (`cli/nav-pilot/internal/cli/agentpakke.go`) treffer bare pakker som er payload-only (`Layout == nil && HasTier(TierPayload)`). Det er også de eneste som pinnes i denne releasen. Tier 2-klienten i en blandet pakke nekter å starte framfor å falle tilbake ([§4](#4-launch-beslutningene)).
- Vokabularet (`Collection:` i status-output, `"collection"` i state-JSON) står igjen med vilje. Å døpe det om treffer output- og state-kompatibilitet for hver eneste eksisterende bruker, og hører hjemme i milepælen der alle installasjoner er agentpakker, ikke i en Tier 2-spesifikk endring.

Alt som forutsetter at Tier 1 og Tier 2 er permanent atskilte modeller, er feil.

## 2. Bevisste avvik fra grillmester-referansen

Avvikene er få, og hvert enkelt er begrunnet i koden der sjekken gjøres.

| Avvik | Hvor det er begrunnet | Hvor kravet håndheves i stedet |
| --- | --- | --- |
| Modesjekk på kilden er subset + exec-bit, ikke eksakt `S_IMODE` | `payload.go`, `verifyPayloadFile` | Eksakte modes på det stagede treet, `VerifyPayloadExact` |
| Ingen pin av `target`-navn | `payload.go`, `PayloadManifest.Target` | Digestkjeden |
| `O_NOFOLLOW`/`O_NONBLOCK` + fstat på deskriptoren (referansen har ingen av delene) | `payload.go`, `openPayloadFile` | Ingenting, dette er strengere enn referansen |
| Kopiering styres av manifestets `files`, ikke av en walk av kildetreet | `stage.go`, filhode | Ingenting, dette er strengere enn referansen |
| `--project-dir` tas ikke i bruk *på launchen* (`--no-audit` tas i bruk, se [§2.5](#25-flagg-fra-referansen)) | `staged_launch.go`, filhode | Arbeidskatalogen *er* prosjektomfanget, og ingen launch-sti setter `cmd.Dir`. Versjonsproben er unntaket, se [§2.5](#25-flagg-fra-referansen) |
| Copilots numeriske build-suffiks (`1.0.81-14`) godtas, referansen avviser det som prerelease | `runtime_gate.go`, `copilotBuildSuffixPattern` | Sammenligningen kjører på `major.minor.patch`, og ekte prereleases (`-next.3`, `-beta`, `-rc.1`) avvises fortsatt |

### 2.1 Modesjekk på kilden: subset pluss exec-bit

Referansens `_verify_manifested_payload` sammenligner `S_IMODE` eksakt. nav-pilot krever i stedet at rettighetene på disk er en **delmengde** av det manifestet deklarerer, og at exec-biten er lik.

Grunnen er umask. En klone laget under `umask 0077` gir 0600 og 0700 for innholdsidentiske filer, og en eksakt sjekk ville avvist enhver konform payload på slike maskiner. De er vanlige på Nav-utstyr. En umask kan bare *fjerne* bits, aldri legge til, så den retningen er trygg å tolerere. Motsatt vei er den ikke: en 0644-fil funnet som 0666, eller en 0755-fil funnet som 0777, er ikke et umask-artefakt, og avvises. Setuid, setgid og sticky avvises separat og eksplisitt, fordi Gos `FileMode.Perm()` maskerer til 0777 og en setuid 04755-fil ellers ville verifisert rent.

Merk at 0700 fortsatt er kjørbar. Å klare exec-biten av 0755 krever en umask som inneholder 0111, som samtidig ville gjort hver katalog brukeren lager uåpnbar. En deklarert 0755-fil helt uten exec-bit er derfor et ekte avvik, ikke et umask-artefakt, og er fatalt.

Eksakte modes håndheves der de er håndhevbare. `StagePayload` chmod-er hver staget fil til deklarert mode og kjører `VerifyPayloadExact` på resultatet. På et tre nav-pilot selv har laget, betyr «subset» at chmod-en ikke tok, og det er en bug verdt å feile på. `VerifyPayloadExact` skal aldri pekes mot en kildekloning ([#454](https://github.com/navikt/copilot/pull/454), [statusen i #437](https://github.com/navikt/copilot/issues/437)).

### 2.2 Ingen pin av `target`-navn

Referansen sjekker payload-manifestets `target` mot en hardkodet liste over sine egne byggemål. Agentpakke-kontrakten har ingen target-navn, siden en pakke navngir payloadene sine med klient × kontekst. `Target` parses og eksponeres, men brukes ikke til å avvise noe. Det er digestkjeden som binder treet, ikke etiketten (`payload.go`, `PayloadManifest.Target`).

### 2.3 Hardere åpning av payload-filer

`openPayloadFile` åpner med `O_NOFOLLOW|O_NONBLOCK` og stat-er *deskriptoren*, ikke stien. Referansen gjør `lstat` og deretter `read_bytes`, uten noen av delene. Grunnen er vinduet mellom `lstat` og `open`. Uten `O_NOFOLLOW` følges en innsatt symlink, og en lenke hvis mål tilfeldigvis matcher digesten verifiserer rent. Uten `O_NONBLOCK` henger en innsatt fifo hele prosessen inne i `open(2)`. Staging leser kildefilene sine gjennom samme funksjon, så vinduet er lukket også på kopien.

### 2.4 Kopiering styres av manifestet

Referansen kopierer med `shutil.copytree(symlinks=True)` og walker altså kilden. nav-pilot kopierer fra manifestets `files`-map, fordi manifestet er autoriteten på hva payloaden inneholder, ikke katalogen. En fil som smugles inn i kilden mellom verifisering og staging blir dermed aldri kopiert, framfor å bli kopiert og så avvist. I tillegg re-hashes hver fil *mens* den skrives, slik at vinduet mellom kildeverifiseringen og lesingen som kopierer bytene er lukket (`stage.go`, filhode og `stageFile`).

### 2.5 Flagg fra referansen

Fra referansens `build_launch_command` (`scripts/grillmester.py`, linje 647-689) tar vi ett av de to cplt-flaggene i bruk og utelater ett.

- **`--no-audit`** (linje 663) tas i bruk på den stagede stien, på referansens plass: først i cplt-vektoren, før `--agent`. Vi leste det først som launcher-policy og en versjonsspesifikk workaround, og utelot det. Team eSyfo korrigerte det i svaret på G4-spørsmålet ([kommentar i #437](https://github.com/navikt/copilot/issues/437#issuecomment-5437575432)). På cplt-baselinen grillmester v0.3.0 er testet mot, kan cplts parent-side audit kjøre repo-kontrollerte Git-hjelpere *utenfor* sandboxen. Uten flagget er en staget Tier 2-launch dårligere isolert enn launcheren den skal være ekvivalent med. Å fjerne det igjen krever dokumentasjon på at en gjennomgått cplt-baseline retter oppførselen, pluss en minimumsversjonssperre som håndhever den. At flagget ser overflødig ut, holder ikke.
- **`--project-dir`** (linje 666-667) utelates fortsatt på launchen. nav-pilot starter i arbeidskatalogen: ingen launch-sti setter `cmd.Dir`, så cplt og klienten arver brukerens cwd. eSyfo aksepterer utelatelsen på nøyaktig det vilkåret, og at differansetestene asserterer oppførselen.

Ett unntak, og det er ikke en omgjøring. **Versjonsproben** setter `--project-dir` mot en tom 0700-tempkatalog, slik referansen gjør i `_sandboxed_client_version` (linje 884-886). En launch er scopet til brukerens prosjekt med vilje. Et `--version`-spørsmål skal ikke være scopet til noe. Uten flagget kjører proben i brukerens cwd, engasjerer repoets `.cplt.toml`-tillitsflyt og gir klienten lese- og skrivetilgang til repoet for å svare på et versjonsspørsmål. Proben setter også `--yes --quiet` på referansens plass (`_client_probe`, linje 879). Uten `--yes` stopper cplt på launch-bekreftelsen, og proben feiler hver gang, på hver maskin.

Beslutningen om `--no-audit` er altså endret, og den er Team eSyfos ([§5.2](#52-grensen-for-equivalent-invocation-g4-eier-team-esyfo), [§8](#8-der-kildene-er-uenige)). Den om `--project-dir` er vår, nå med eSyfos aksept.

### 2.6 Annet vi ikke speiler

- **Referansens cloud-launcher stager ikke i det hele tatt.** Den peker klienten på payloaden der den ligger, inne i en immutabel Homebrew-bundle. Verify, kopier, re-verify er hentet fra samme prosjekts *lokale* modus (`scripts/grillmester_local.py`, `_materialize_opencode_config`). nav-pilot må stage fordi kilden kan være en midlertidig klone med umask-modes framfor manifestets. Ikke let etter staging i `build_launch_command`.
- **`ensure_opencode_runtime_support`s pre-seeding av `~/.config/opencode/.gitignore`** er ikke tatt med. Det er en skriving til den delte config-katalogen, altså launcher-policy. Ikke-blokkerende for implementasjonen, men del av G4-ordlyden ([§5.2](#52-grensen-for-equivalent-invocation-g4-eier-team-esyfo)).

## 3. Materialiseringsmodellen og tillitsgrensen

Kode: `cli/nav-pilot/internal/agentpakke/stage.go`, kallstedene i `cli/nav-pilot/internal/cli/pakke_install.go` (`materializeRevision`) og `cli/nav-pilot/internal/cli/pakke_launch.go` (`tryPakkeLaunch`).

**Payload-bytes skrives på ett sted: materialiseringen.** Den kjører ved install, ved første launch av en payload-only-kilde som ikke er installert, og ved hver launch av en kilde som er en lokal sti ([§4](#4-launch-beslutningene)). Launch skriver ingenting. Den slår opp pinnen, verifiserer treet eksakt og peker klienten på det. Per-launch-staging finnes ikke lenger.

**Én katalog per revisjon, navngitt av innholdet.** `~/.nav-pilot/pakker/<eier>-<repo>/<sha>/`, med roten avledet av `configPath()` slik den stagede roten var, så `NAV_PILOT_CONFIG` flytter den fortsatt. Repo-id-en småskrives i katalognavnet, en absolutt sti gjør den ikke. Katalognavnet og identitetssjekkene må være enige: `sameSourceRepo` folder store og små bokstaver for repo-id-er fordi GitHub gjør det, og hele pin-stien er bygd på den sammenligningen. Skriver brukeren kilden om med annen bokstavering, ville et byte-eksakt katalognavn sagt noe annet enn vaktene. Identitetssjekkene sier «samme install» mens materialiseringen havner under en ny katalog, og den gamle blir utilgjengelig for både launch og uninstall (som går ut fra staten, med den nye bokstaveringen) og pruningen (som leser den nye katalogen). En sti sammenlignes som checkouten den peker på, ikke ved folding, og røres derfor ikke.

Katalogen er permanent og skrives aldri om. En ny revisjon får en ny SHA-katalog, og den forrige blir stående til den ryddes bort ([§6](#6-kjente-begrensninger-vi-har-valgt-med-vilje)). Det eneste unntaket er en lokal kilde, som ikke pinnes og hvis revisjon derfor bygges om hver gang. Det er dette som gjør en fast katalog per pakke riktig nå. Innvendingen mot den var at å tømme og skrive den på nytt trekker config-katalogen ut under en økt som allerede kjører mot den gjennom `OPENCODE_CONFIG_DIR`. En innholdsadressert katalog blir aldri skrevet på nytt, så innvendingen har ingenting å treffe ([§8](#8-der-kildene-er-uenige)).

**Revisjonen publiseres av én `os.Rename`.** Hver deklarert kontekst av hver payload-bærende klient stages inn i en søskenkatalog `.tmp-*`, og først når alle er ferdige får treet SHA-navnet sitt. En avbrutt materialisering etterlater derfor aldri en halv revisjon en launch kan finne, og staten skrives først etter at renamen har publisert et komplett tre. Taper prosessen kappløpet om renamen, bruker den vinnerens katalog framfor å feile.

**En revisjon som allerede finnes, adopteres bare hvis den fortsatt verifiserer.** Før `materializeRevision` gjenbruker en katalog, kjører den hele verifiseringen, altså det pinnede manifestet pluss den eksakte hash-walken for hver kontekst av hver payload-bærende klient, og bygger katalogen på nytt når den ikke holder. Uten den sjekken kiler én ødelagt publisering seg fast for godt: `install` skriver «Installed», `sync` svarer «up to date», re-install av samme SHA gir tilbake det samme ødelagte treet, og bare `uninstall` kommer ut av det. Sjekken fanger *korrupsjon*, ikke *foreldethet*, fordi treet verifiseres mot manifestet som ligger i det ([§3.1](#31-tillitsgrensen-etter-pinnen)). Derfor bygges lokale kilder om ubetinget framfor å lene seg på den ([§4](#4-launch-beslutningene)).

**Manifestet pinnes sammen med payloadene**, på konvensjonsplassen `<sha>/.nav-pilot/agentpakke.json`. Revisjonen er selvbeskrivende: launch fester pakka fra revisjonskatalogen, og persona, `defaultModel` og `compatibility`-porten leser den pinnede formen, ikke det standardbranchen måtte si i dag.

**Klonen er borte før noen launch skjer.** Install resolver kilden én gang, materialiserer og forkaster klonen. Ingen `.git`, ingen rå checkout blir liggende. At treet bærer sitt eget manifest er nå *grunnen* til at pinnen virker, ikke en bieffekt av at kilden kan forsvinne. En payload som selv deklarerer en fil med det navnet, avvises fortsatt før noe opprettes.

**Pinnen identifiseres av pinnen, ikke av navnet.** Hverken `sync` eller `uninstall` kjenner igjen en pinnet install på pakkas visningsnavn. Et navn kan endres oppstrøms uten at installasjonen blir en annen, og to pakker fra ulike repoer kan hete det samme. En navnesammenligning slapp begge tilfellene tilbake i fil-synkens blindvei, der `sync` melder «No customization files found to sync.» og returnerer suksess over en install som aldri kan komme videre.

De to kommandoene stiller likevel ikke samme spørsmål. `uninstall` går ut fra formen `pinnedState` beskriver, altså registrert kilde, registrert SHA og ingen faktisk installerte filer (`!installsContent`), pluss brukerscope, og fjerner revisjoner bare da. `sync` bruker `pinnedSync`: en revisjonskatalog som faktisk finnes (`pinnedRevisionOnDisk`), *eller* en pin-formet state for en kilde som fortsatt er payload-only. Ingen av halvdelene holder alene. Formen alene er tvetydig, siden en Tier 1-install også kan spore null filer. Kilden alene er det også, siden en pakke kan få en `layout` oppstrøms uten at pinnen hver launch leser slutter å være en pin. Og katalogen alene faller bort i det noen sletter den, som er nettopp det tredje utfallet under.

`sync` har derfor fire utfall på en pin, ikke to: pinnet SHA lik kildens («up to date»), kilden har flyttet seg (rapportert, og `--apply` pinner den nye revisjonen), revisjonskatalogen er borte (rapportert som borte, og `--apply` bygger den opp igjen og skriver `Restored …`), og kilden peker et annet sted enn pinnen. Det siste nektes, fordi kildebytte er en `install`. Et femte tilfelle er en nektelse i samme ånd: har den pinnede kilden sluttet å være payload-only, finnes det ingen revisjonsbump å gjøre i denne releasen, og `sync` sier det framfor å melde «up to date».

`uninstall` navngir hver katalog den fjerner, i både tørrkjøring og ekte kjøring. En pin installerer ingen filer, så revisjonene er alt kommandoen faktisk sletter, og en tørrkjøring som bare lister en tom filløkke ville beskrevet en kommando som ikke gjør noe. Men den er ikke lenger det eneste som fjerner dem. En Tier 1-install som skriver over en pin frigjør revisjonene den erstatter (`releasePin`, kjørt fra den delte state-skrivingen), stille. Det er den ene stien der de ellers ville blitt uleselige for alt: staten som var det eneste sporet av dem, er borte, og uninstall er portet på at staten fortsatt *er* en pin. Stien nås ved å følge nav-pilots egen beskjed, siden hver Tier 2-nektelse som ikke kan løses automatisk navngir `nav-pilot install --user <navn>`, og den kommandoen tar den vanlige Tier 1-veien.

**En ignorer-markør er ikke installert innhold.** `nav-pilot ignore <element> --user` legger til en `Files`-oppføring uten hash, og en state som bare bærer slike er fortsatt en pin. Derfor spør `pinnedState` etter `!installsContent` framfor «null oppføringer». Alle stedene som stiller spørsmålet, stiller det likt, og det er hele poenget: en markør skjuler ikke revisjonene for `uninstall`, den hindrer ikke en Tier 1-install i å frigi dem, og den får dem heller ikke slettet. `releasePin` utløses bare av en state som faktisk installerer innhold. Å slette payload-trærne en fungerende install starter fra ville vært verre enn å la dem ligge, for launchen ville nektet, og auto-pinnen kan ikke bygge dem opp igjen over en state som sporer filer.

Dette overlever uendret fra staging-modellen:

**Sekvensen er verify → kopier (med re-hash) → chmod → eksakt re-verify.** Manifestet leses og parses én gang, og de samme bytene skrives inn i det materialiserte treet og re-verifiseres mot det, så ingen annen lesing kan smugle inn et annet manifest underveis. `os.OpenFile`s mode-argument maskeres av prosessens umask, så under `umask 0077` blir en fil opprettet som «0644» faktisk 0600. Det er `chmod` gjennom deskriptoren som gjør deklarert mode til faktisk mode.

**Formen garanteres av konstruksjon, ikke av sjekk.** Symlinker, hardlinker ut av treet, enhetsnoder, fifoer og sockets kan ikke oppstå. Hver node er enten en katalog nav-pilot lager, eller en vanlig fil åpnet med `O_CREATE|O_EXCL|O_NOFOLLOW`. Det samme `StagePayload`-kallet gjør fortsatt jobben, men kjører én gang per revisjon i stedet for én gang per launch.

**Fail-closed, uten fallback.** Enhver feil i ethvert steg etterlater ingen publisert revisjon og intet brukbart resultat. `tryPakkeLaunch` returnerer feilen, og en Tier 2-launch faller *aldri* tilbake til legacy-stien (G2).

**Katalogmodes verifiseres ikke, og det er ingenting å verifisere dem mot.** Payload-manifestet deklarerer modes for filer, ikke kataloger. nav-pilot velger derfor selv 0700 for pakkeroten, hver revisjon og hver katalog i den. Eiers egen, fordi treet er brukerens eget og ingen annen bruker på maskinen har noe der å gjøre. Referansen gjør det samme. Under enhver umask som fortsatt lar brukeren gå inn i sine egne kataloger, kommer de ut som 0700. Under en umask som klarer eier-bitene, feiler første `create` inni dem og materialiseringen avbrytes, som er riktig utfall for en maskin konfigurert slik.

**Ikke alle payloads verifiseres ved launch.** Install digest-verifiserer alle deklarerte payloads, fordi `validatePakkeSource` går veien om `ValidateSource` (`load.go`, `ValidateSource` → `VerifyPayload` per payload). Launch gjør ikke det. Den kjører `Load` (schema) på det pinnede manifestet og `VerifyPayloadExact` på det ene treet som faktisk startes, siden det koster uten å gi noe å verifisere payloads du ikke starter. `nav-pilot validate` er fortsatt alle-payloads-porten, og kjøres av pakkerepoets egen CI.

### 3.1 Tillitsgrensen etter pinnen

Før pinnen var treet klienten leste bygd mikrosekunder tidligere, i en katalog ingen annen prosess kjente navnet på, verifisert eksakt og så overlevert. Vinduet mellom verifisering og bruk var millisekunder.

Etter pinnen er treet bygd ved install eller ved første launch, verifisert eksakt der og da, og blir liggende i `~/.nav-pilot/pakker/` (0700) i dager eller uker. **Vinduet er ubegrenset. Det er en reell svekkelse, og den lar seg ikke formulere til noe annet.**

`VerifyPayloadExact` på launch henter dette inn igjen, ved å walke treet begge veier (`payload.go`, `verifyTree`):

- sha256 **og eksakte rettighetsbits** for hver fil manifestet lister (`verifyPayloadFile`, exact)
- **filer manifestet ikke lister avvises**, de ignoreres ikke, så en plantet fil overlever ikke
- symlinker hvor som helst i treet, inkludert symlinkede kataloger, og alt som ikke er en vanlig fil
- filer manifestet deklarerer, men treet mangler
- en fil byttet mellom `lstat` og `open`, gjennom `O_NOFOLLOW|O_NONBLOCK` på deskriptoren pluss fstat (`openPayloadFile`), en herding som ligger der allerede og er nøyaktig det et aldrende tre trenger
- payload-manifestet selv, lest under samme mistillit: lstat-et, avvist hvis det er en symlink eller ikke en vanlig fil, og størrelsesbegrenset (`readPayloadManifestFile`)

**Hva den ikke dekker:** payload-manifestet leses fra det treet det verifiserer. Endrer noen en fil *og* digesten dens i manifestet, verifiserer treet rent. Det var sant om det stagede treet også, men der var manifestet kopiert fra en kilde verifisert sekunder før, så utnyttelsen var et mikrosekundkappløp. Etter pinnen er den en rolig redigering.

**Dette er akseptert, og begrunnelsen hører hjemme her, ikke i ny kode.** Den eneste aktøren i det vinduet er en prosess som kjører som brukeren. En slik prosess kan like gjerne skrive om `~/.copilot/agents/*.agent.md` (Tier 1 installerer dem uverifisert, og ingenting sjekker dem igjen), `~/.nav-pilot/config.toml`, eller `cplt` på `$PATH`. nav-pilot har aldri forsvart seg mot tukling fra brukeren selv. Digestkjeden finnes for å binde *kilderepoet*, og der strammer pinnen inn framfor å slakke, siden en pinnet revisjon er immutabel og standardbranchen ikke er det. Millisekundvinduet var et artefakt av staging-modellen, ikke en designet kontroll, og det finnes ingenting i kontrakten å forankre manifestet i. `provenance.digest` er eksplisitt ikke verifisert ([README.agentpakke.md](README.agentpakke.md#begrensninger-i-dag)).

Ikke lukk dette med en mekanisme som binder manifestet til noe utenfor treet. Den ville trengt en ny state-form, som [§1](#1-retningen-tier-2-er-ikke-en-sidegren) forbyr.

## 4. Launch-beslutningene

Kode: `cli/nav-pilot/internal/provider/staged_launch.go`, `cli/nav-pilot/internal/provider/pakke.go`, `cli/nav-pilot/internal/provider/runtime_gate.go`, `cli/nav-pilot/internal/cli/pakke_launch.go` ([#458](https://github.com/navikt/copilot/pull/458)).

**Staged Copilot krever cplt, uten fallback og uten prompt.** Legacy-stien for copilot godtar fortsatt en vanlig, usandboxet `copilot`-binær. På Tier 2-stien gjør den det ikke. En payload-launch er *definert* av sandbox-flaggene (`--allow-read <staged>`), payloaden er tredjeparts konfigurasjon vi ikke har skrevet, og å falle tilbake til en usandboxet klient ville stille droppet nettopp den isolasjonen payloaden ble staget for. Referansen krever også cplt. Brukere uten cplt får en feil som navngir `brew install navikt/tap/cplt`. Dette er en innstramming målt mot dagens copilot-støtte, men bare på Tier 2-stien.

**Flagget heter `--payload-context`, ikke `--context`.** `--context` er allerede navnet på Copilots kontekst-tier. Uten verdi brukes manifestets `defaultContext`. Å sende flagget på en launch som *ikke* går fra en Tier 2-payload er en **feil**, ikke en stille no-op, samme policy som ukjente config-nøkler får (`payloadContextUnsupported`). En ukjent kontekst gir en feil som lister de deklarerte. Det finnes ingen TOML-nøkkel for payload-kontekst. Det er et per-launch-valg.

**Manifestet er autoritativt for personaer.** `PrimaryAgent` (Tier 1, legacy launch) og `PrimaryAgentFor` (Tier 2, staged launch) leser den aktive pakka og har **ingen** fallback til nav-pilots egen default. En fallback ville injisert Navs nav-pilot-persona inn i en fremmed pakkes launch, samtidig som materialiseringen korrekt degraderer den til subagent (`internal/artifacts/export.go`). Den tomme strengen er *urepresenterbar*, ikke bare usannsynlig: schemaet krever `primaryAgents` med `"minItems": 1` på hver Tier 1-klientoppføring og på hver Tier 2-payload, sjekket av `agentpakke.Load` før et manifest festes til en kilde. `stagedPrimaryAgent` feiler likevel høyt framfor å sende et tomt `--agent`, i tilfelle et framtidig kallsted bryter invarianten `SetActivePakke` dokumenterer.

**Rosteret hører til payloaden, ikke til klienten (WP7).** For en Tier 2-oppføring ligger `primaryAgents` på hver `payloads.<kontekst>`, er påkrevd der, og leses bare derfra. `PrimaryAgentFor(client, context)` faller aldri tilbake til klientnivået, og en Tier 2-oppføring som likevel har et klientnivå-roster får det ignorert.

Bakgrunnen er Team eSyfos svar på G4-spørsmålene ([kommentar i #437](https://github.com/navikt/copilot/issues/437#issuecomment-5437575432)). Ved det pinnede referansepunktet lister klientoppføringen `grillmester` først, mens *begge* `focused`-payloadene med vilje bare inneholder `barista` og `grill-inspektor`. En `focused`-launch skulle altså sendt `--agent grillmester` mot et tre som ikke har den agenten. eSyfo avviste både å endre `full`-defaulten og å utvide focused-payloaden for å passe kjøreren, og ba om «a context-specific default/allowed-agent declaration (or an equivalent jointly reviewed solution)».

Vi leste det som en **strukturell feil i kontrakten, ikke et hull**. Rosteret hører til den enheten som faktisk bærer agentene. For Tier 1 er det repoets `layout`, for Tier 2 er det payload-treet. En Tier 2-klientoppføring som deklarerer ett roster for payloads med ulike rostere sier noe usant om de bytene den binder. Derfor ble feltet *flyttet*, ikke supplert.

To alternativer ble vurdert og forkastet:

- **Valgfri overstyring på payloaden, med fallback til klientnivået.** Forkastet fordi den bevarer den usanne påstanden på klientnivå permanent, og legger en fallback-regel oppå den som hver framtidig konsument må implementere likt.
- **Én `defaultAgent`-streng per payload.** Forkastet fordi den bare uttrykker standardpersonaen og ikke hvilke agenter konteksten i det hele tatt tilbyr, altså den samme informasjonen `primaryAgents` allerede bærer på Tier 1, i et annet format, for samme formål.

**nav-pilot kryssjekker ikke deklarerte agentnavn mot filene i payloaden.** Hvordan en agentfil heter er klientspesifikk konvensjon (`.agent.md`, `agents/<navn>.md`, en `name:`-frontmatter), og det er digestkjeden som binder payloaden, ikke gjetning ut fra filnavn. Et navn i `primaryAgents` som payloaden ikke har, feiler i klienten, ikke i nav-pilot.

**Kontraktsversjonen ble stående på `"1"`, og 90-dagersvinduet ble ikke brukt.** Korreksjonen kom *før første konsument*: Tier 2 kunne ikke installeres, ingen bruker hadde en Tier 2-kilde, og det eneste manifestet som fantes var grillmesters, hvis eiere var med på å utforme endringen. Med én konsument ville dette krevd bump av `contractVersion` **og** de 90 dagene. Vinduet binder fra det øyeblikket konsument nummer to finnes. Det står også i kontraktens [Kompatibilitet-kapittel](README.agentpakke.md#én-korreksjon-før-første-konsument-august-2026), slik at ingen senere leser reglene der og konkluderer at vinduet ble brutt. På eSyfos side betyr det at grillmester regenererer manifestet og navngir et nytt pinnet referanse-SHA. Det er gjort i [navikt/grillmester#63](https://github.com/navikt/grillmester/pull/63), som er den nåværende G4-baselinen. Manifestet ved `3573b93cc8b7568516117263562d073cae9ee7fc` validerer ikke lenger, med en feil som navngir hver payload som mangler feltet.

**Den aktive pakka bor i `internal/source`, ikke i `internal/provider`.** `internal/artifacts` trenger de samme deklarasjonene for materialisert frontmatter og kan ikke importere provider. `internal/source` er den laveste pakka begge allerede importerer. Provider-sømmen ligger der planen la den, og delegerer ([#455](https://github.com/navikt/copilot/pull/455)).

**Modellrekkefølge:** brukerens pin vinner, deretter pakkas `defaultModel`, ellers ingenting. Literalen `"inherit"` betyr *ingen* `--model` i det hele tatt, som også er det referansen sender (`build_launch_command` videresender ingen modell).

**Klientformene** er transkribert fra referansen med linjenumrene sitert i testen som asserterer dem, slik at G4-differansen har noe å sammenligne mot framfor noe å oppdage.

- opencode: `OPENCODE_CONFIG_DIR` mot det stagede treet, videreført gjennom cplt med `--pass-env`, pluss `--allow-read <staged>` så sandboxen får lese treet. `--allow-read` kommer før `--pass-env`, fordi det er rekkefølgen referansen sender dem i (WP3-planen skrev motsatt rekkefølge, referansen vant).
- copilot: `--plugin-dir <staged>` foran `--agent <pakke>:<agent>`. Persona-navnet er plugin-kvalifisert med pakkas navn.
- `--mode plan` fortsetter å mappe til opencodes innebygde, lesende `plan`-agent, som på legacy-stien.
- opencodes subkommandoer håndteres som i referansen, altså at `--agent` og `--model` bare bindes til inngangspunkter som godtar dem. Ingen videresendte argumenter gir bare bindingen, `run …` gir `run <binding> …`, en annen opencode-subkommando videresendes urørt uten `--agent`, og alt annet får bindingen foran. Transkribert fra `_opencode_client_arguments` (linje 692-704) og `OPENCODE_COMMANDS` (linje 37-63), med linjenumrene sitert i `openCodeClientArgs`.

**En staget økt arver ikke brukerens egne Copilot-instruksjoner.** `CopilotEnv` er nå et skall rundt `copilotEnv(otelLogLevel, injectUserInstructions bool)` (`internal/provider/copilot_launch.go`), og den stagede copilot-launchen sender `pakkeAcceptsUserContext("copilot")`, som er `false` i dag. Referansen sender miljøet urørt, og en pakke bør se konteksten forfatteren faktisk har testet mot, framfor hva som måtte ligge i brukerens `~/.copilot`. En `COPILOT_CUSTOM_INSTRUCTIONS_DIRS` brukeren selv har eksportert, arves fortsatt urørt fra `os.Environ()`, siden det er brukerens eget valg og ikke noe nav-pilot injiserer. Om en pakke skal kunne velge dette inn, er et manifestspørsmål ([§5.3](#53-skal-en-pakke-kunne-be-om-brukerens-egen-kontekst-eier-nav-pilot)). Beslutningspunktet er med vilje ett funksjonskall, slik at feltet blir en linjes endring den dagen det finnes.

**Manglende sandbox er en advarsel, ikke et spørsmål.** «Ville kjørt usandboxet» er bare sant på legacy-stien, siden en Tier 2-launch krever cplt og nekter uten, og hvilken av de to det er, vet man ikke før `tryPakkeLaunch` har resolvet kilden og lest tieren. Advarselen ligger derfor fortsatt i `launchClientConfirming` (`internal/cli/interactive.go`) og ikke hos kallstedet: `decideLaunch` sender bare `launchWarnUnsandboxed` videre som et flagg, og teksten skrives først når `tryPakkeLaunch` har gitt fra seg launchen til legacy-stien, slik at ingen får høre at launchen er usandboxet rett før den avvises. Fram til #476 var dette en bekreftelse (`launchConfirmUnsandboxed`) på samme sted. Den ble fjernet fordi folk kjører nav-pilot for å få klienten i gang, og en bekreftelse hvis eneste fornuftige svar er «ja» er et tastetrykk, ikke en beslutning. Den som vil ha sandboxen får den ved å installere cplt, som advarselen navngir, og `auto_launch = false` er innstillingen for aldri å starte noe i det hele tatt.

**Ingenting på denne stien skriver til brukerens delte klientconfig (G2).** Den stagede opencode-launchen hopper over både `EnsureOpenCodeNavContext` og `EnsureOpenCodeOTelConfig`, som begge skriver inn i `~/.config/opencode`, og redigerer heller aldri payloaden, hvis bytes er digestbundet. OTel reiser fortsatt som miljøvariabler, som ikke er config-mutasjon (se konsekvensen i [§6](#6-kjente-begrensninger-vi-har-valgt-med-vilje)).

**En pinnet launch resolver ikke.** Finnes det en pin i brukerscope-staten for den konfigurerte kilden, og revisjonskatalogen ligger der, leser launchen både manifestet og payloaden derfra og går aldri på nettet. Den flytter heller aldri pinnen. En standardbranch som har beveget seg plukkes opp av `nav-pilot sync`, ikke av å starte klienten på nytt. Feiler `attachPakke` på den pinnede katalogen, ved en nedgradert binær eller en major-bump i `contractVersion`, er det fatalt. Alternativet ville vært å falle stille tilbake til å klone en bevegelig branch, altså å gjøre en kontraktsfeil om til nøyaktig den oppførselen pinnen finnes for å fjerne. En revisjonskatalog som *mangler* er derimot ingen feil, bare fravær av pin. Det er stien for installasjoner fra før pinnen, for Tier 1-kilder og for kilder uten manifest. Blandede pakker er unntaket, se under. Hver pinnet launch skriver én dempet linje som navngir revisjonen, slik at hvilke bytes som kjører ikke er skjult.

**En payload-only kilde uten pin pinnes ved første launch, den avvises ikke.** Resolve én gang, materialiser, skriv pinnen, én linje output, kjør. Hver senere launch er et rent oppslag. Å materialisere uten å skrive pinnen er ikke et alternativ, for da ville neste launch klonet på nytt, og pinnen aldri tatt.

**En launch kan opprette en pin, men aldri ødelegge en installasjon.** Pinnen erstatter scopets state, så auto-pinnen nekter når brukerscopet har en installasjon å miste: state som sporer filer i det hele tatt, uansett hvilket repo de kom fra, eller en registrert pin på en *annen* kilde. Meldingen navngir `nav-pilot install --user <navn>`. Vakten ser bevisst etter mer enn «fremmed repo med filer», og lukker dermed to hull. Et repo som dropper `layout`-en sin og blir payload-only ville fått Tier 1-filene sine foreldreløse av en ren launch, med det eneste sporet av dem slettet i samme slengen. Og `nav-pilot --source B` på en maskin pinnet til A ville fjernet hver revisjon av A som bieffekt. Fjerning hører hjemme i `install` og `sync --apply`, der den eksplisitte kommandoen er samtykket. En launch har ikke noe samtykkepunkt, og stopper derfor og navngir den kommandoen som har det.

**Tier 2-klienten i en blandet pakke nekter å starte, den faller ikke tilbake.** Blandede pakker (`layout` **og** `payloads`) pinnes ikke i denne releasen, og etter at per-launch-staging er borte finnes det ikke lenger noen staget sti å falle tilbake på. Valget sto mellom å la klienten gå videre på legacy-stien, eller å stoppe launchen. Legacy-stien ville materialisert Tier 1-innhold inn i klientens egen config, altså gitt brukeren noe helt annet enn det isolerte, verifiserte payload-treet manifestet deklarerer for den klienten, uten at noe i output sa fra. Den stille semantiske nedgraderingen er verre enn en høylytt nektelse nettopp fordi brukeren ikke kan se den skje. Den ser ut som en vellykket launch. Launchen stopper derfor fail-closed, med en melding om at pinnen ikke dekker blandede pakker i denne releasen. Tier 1-klienten i den samme pakka er upåvirket og tar legacy-stien som før.

**Lokale kilder pinnes ikke.** En `source` som er en absolutt sti (`/sti/til/pakke`) er formen en pakkeforfatter utvikler mot, og en pin ville frosset den. Første launch materialiserte arbeidstreet, hver senere launch leste den samme revisjonen, og for en katalog uten git er «SHA-en» den bokstavelige `"unknown"`, så `sync` sammenlignet `"unknown"` med `"unknown"` og meldte «up to date» i all framtid. En lokal kilde materialiseres derfor på nytt ved hver launch, verifisert som før, bare aldri pinnet. Det er nøyaktig den redigér-og-start-løkka forfatteren hadde før pinnen fantes. `pinnable` gjør her samme unntak som `tierCacheable` allerede gjorde, av samme grunn: den som redigerer sin egen checkout skal se endringen ved neste launch. Følgene er tre. Ingen pin skrives. `install` av en lokal Tier 2-kilde nektes med en melding som navngir install fra repoet i stedet, siden den ellers ville skrevet en pin som navnga `"unknown"`. Og kostnaden ved å materialisere ved hver launch tas på denne ene stien.

**Fail-closed begynner ved tier-porten, ikke før den.** Dette gjelder uendret, og det gjelder nå de launchene som faktisk må resolve: en upinnet kilde, en Tier 1-kilde, en kilde uten manifest. En pinnet launch resolver ikke og møter aldri porten. Ellers resolves kilden, fordi det er slik nav-pilot får vite om denne launchen i det hele tatt er Tier 2. En resolve-feil lander *før* tier-porten, der ingenting ennå sier at det er en payload i bildet, siden kilden kanskje ikke deklarerer noen. Å være offline, eller ha et utdatert reponavn i config, skal derfor ikke blokkere en launch som virket før Tier 2-staging fantes. nav-pilot advarer og tar legacy-stien, slik `EnsureOpenCodeNavContext` alltid har gjort. Forbi tier-porten, der payloaden er kjent, er alt fail-closed uten fallback.

Dette er en rettelse. #457 beskrev den fatale varianten som en akseptert konsekvens for copilot-brukere med egen kilde. Den framstillingen var feil på to punkter. Legacy-opencode kloner den *state-registrerte* repoen, ikke den konfigurerte, og en feil der er bare en advarsel. Og den fatale varianten rammet begge klienter, altså enhver offline bruker med `source` satt, inkludert `navikt/copilot` skrevet eksplisitt. Rettet i #458 framfor dokumentert.

**Tier-cachen blir stående, innsnevret og dokumentert ([#469](https://github.com/navikt/copilot/issues/469)).** `tierCacheTTL` (6 timer, `internal/cli/pakke_launch.go`) finnes for å slippe en `git clone --depth 1` ved hver launch bare for å lære en tier. Etter pinnen står den igjen med nøyaktig én sti: en egen kilde som resolver til Tier 1, eller som ikke har noe manifest. `tryPakkeLaunch` kjører fortsatt for den, fordi porten øverst i funksjonen bare stopper launcher uten kilde satt, og uten cachen ville den klonet ved hver eneste launch. Det gjelder også `source = "navikt/copilot"` skrevet eksplisitt, som `persistInstalledSource` skriver etter enhver `install --source`. En pinnet kilde treffer ikke cachen, fordi pinnen svarer før den konsulteres, og en upinnet payload-only kilde resolver uansett én gang for å bli pinnet. **En husket `TierPayload`-oppføring blir dermed bare skrevet, aldri lest til noe. Det er bare ikke-payload-svaret som fortsatt er bærende.**

Slettetriggeren er skrevet ned slik at dette ikke må vurderes fra bunnen igjen. Den dagen Tier 1-installasjoner også pinnes ([§1](#1-retningen-tier-2-er-ikke-en-sidegren), fase 2), forsvinner det siste kallstedet, og cachen går med det sammen med `~/.nav-pilot/tier-cache.json`. Å bruke state som permanent tier-hukommelse i stedet er utelukket av regelen i [§1](#1-retningen-tier-2-er-ikke-en-sidegren): en pakke som senere får en `layout` skal plukkes opp uten migrasjon, og det er TTL-en som gjør at tier-svaret kan endre seg.

**Runtime-portene er nav-pilots, ikke kontraktens.** Kode: `cli/nav-pilot/internal/provider/runtime_gate.go`. Team eSyfo gjorde håndheving obligatorisk ([kommentar i #437](https://github.com/navikt/copilot/issues/437#issuecomment-5437575432)): «Runtime client compatibility ranges and a reviewed cplt minimum should be enforced, not only validated as manifest syntax.» En staged launch kjører derfor to porter før den bygger noe som helst, cplt-gulvet alltid og `compatibility` når klientoppføringa deklarerer et område.

**cplt-gulvet er en konstant i nav-pilot, ikke et manifestfelt.** `minStagedCpltStamp` er hentet fra referansens `SUPPORTED_CPLT_RELEASE` (`scripts/grillmester.py`, linje 27). Det kunne vært et felt i kontrakten, men da ville eieren blitt feil. Hvilken cplt-versjon som er trygg nok til å kjøre en sandboxet payload, er *nav-pilots og cplts* vurdering, ikke pakkeforfatterens. En pakke kunne ellers senket gulvet under det nav-pilot har gjennomgått, altså presis den porten som skal beskytte brukeren mot pakka. Å flytte konstanten følger samme fellesbeslutningsregel som referansepinnen. Den er samtidig utgangsrampen for `--no-audit` ([§2.5](#25-flagg-fra-referansen)): den dagen en gjennomgått cplt-baseline retter parent-side audit, er endringen én diff, altså hev stempelet, fjern flagget, legg ved dokumentasjonen.

**«Vet ikke» er fatalt, ikke en advarsel.** En probe som feiler, eller versjonsutdata nav-pilot ikke kan parse, avviser launchen på lik linje med en versjon som faktisk er utenfor området. Tre grunner. Portene ligger *forbi* tier-porten, der alt er fail-closed uten fallback. Referansen er fatal på samme sted (`check_cplt`, `_strict_client_version_output`). Og en versjonssjekk som blir grønn på manglende data er nøyaktig feilmodusen [#452](https://github.com/navikt/copilot/pull/452) rettet i nav-pilots egen cplt-skew-sjekk. Et område som ikke kan håndheves, er ikke håndhevet, og da har eSyfos krav ikke noe innhold. Deklarerer manifestet ingen `compatibility`, er det ingenting å håndheve, og da probes klienten ikke i det hele tatt.

## 5. Åpne spørsmål

Disse er **ikke** avgjort. Ikke behandle dem som avgjorte, og ikke «rydde opp» i dem ved å velge ett svar i forbifarten.

### 5.1 Parity for `focused`-kontekst (G4), eier Team eSyfo

Referanselauncheren når `focused` bare gjennom `grillmester local` (loopback, macOS). For cloud-launchen har vi foreslått at focused-scenarioene asserterer byte-parity på payloaden pluss nøyaktig samme config-pekende mekanisme som `full`, uten local-modusens herdingsflagg, og at local-modusens invokasjonskontrakt forblir M4-scope. Spørsmålet er stilt i [statuskommentaren i #437](https://github.com/navikt/copilot/issues/437) og gjelder ordlyden i G4, ikke hva vi bygger. Blokkerer G4-signoff, ikke implementasjon.

### 5.2 Grensen for «equivalent invocation» (G4), eier Team eSyfo

Vi planlegger å assertere config-plassering, persona inkludert `<pakke>:<agent>`-kvalifiseringen, modellhåndtering, `--pass-env OPENCODE_CONFIG_DIR`, `--allow-read <staged>` og cplt-agentvalg. Vi tar i bruk `--no-audit` og utelater fortsatt `--project-dir` ([§2.5](#25-flagg-fra-referansen)), normaliserer modes ved staging ([§2.1](#21-modesjekk-på-kilden-subset-pluss-exec-bit)), og hopper over `.gitignore`-pre-seedingen ([§2.6](#26-annet-vi-ikke-speiler)). Spørsmålet til eSyfo var om noen av dem er bærende for deres guardrail-historie. **Besvart for flaggene:** `--no-audit` er bærende og tas i bruk, `--project-dir` kan utelates så lenge arbeidskatalogen er prosjektomfanget ([kommentar i #437](https://github.com/navikt/copilot/issues/437#issuecomment-5437575432)). Resten av G4-ordlyden er fortsatt stilt samme sted som 5.1.

### 5.3 Skal en pakke kunne be om brukerens egen kontekst, eier nav-pilot

**Slik det er i dag:** en staget launch blander ikke inn brukerens eget `~/.copilot`-innhold. `buildStagedCopilotSpec` kaller `copilotEnv(r.OtelLogLevel, pakkeAcceptsUserContext("copilot"))`, og `pakkeAcceptsUserContext` returnerer `false` for alle klienter, siden det ikke finnes noe manifestfelt å lese ennå. Uten en deklarasjon blandes ingenting inn. En tredjeparts pakke skal ikke stille motta Nav-innhold forfatteren aldri har testet mot ([§4](#4-launch-beslutningene)).

**Forslaget** er et felt `acceptsUserContext` på klientoppføringen, navngitt i kommentaren til `pakkeAcceptsUserContext` (`internal/provider/staged_launch.go`). Da blir funksjonen en linjes manifestlesing, som `pakkeDeclaredModel`. Schemaet er med vilje urørt i #458.

**Statusen på forslaget:** det er *vårt* forslag. Det er ikke lagt fram for Team eSyfo, og det finnes ingen avtale om det. Feltet står ikke i [`agentpakke-v1.json`](../cli/nav-pilot/schemas/agentpakke-v1.json), og per i dag heller ikke skrevet ned i #435 eller #437. Kodekommentaren peker på #437 som stedet det skal tas opp. Er det tatt opp der når du leser dette, er den kommentaren kilden. Ellers er dette dokumentet og koden det eneste stedet forslaget finnes. Et slikt felt er additivt og krever ikke bump av `contractVersion`.

### 5.4 `nav-pilot export opencode` for ikke-kanoniske layouts, eier nav-pilot

Export leser de kanoniske stiene (`agents/`, `skills/`, `instructions/`, `prompts/`) direkte, og avviser en agentpakke som legger innholdet et annet sted. [#437](https://github.com/navikt/copilot/issues/437) lister det som «re-scope or implement», og det er ikke tatt stilling til. WP4-beslutningene som tidligere sto her, er ikke lenger åpne: install-sperren løftes og revisjonen pinnes, uten ny state-form og uten nytt vokabular ([§3](#3-materialiseringsmodellen-og-tillitsgrensen), [§4](#4-launch-beslutningene)).

## 6. Kjente begrensninger vi har valgt med vilje

Ikke «fiks» disse ved et uhell. De er valgt, og de har begrunnelser.

- **Høyst to revisjoner beholdes per pinnet kilde:** den pinnede og den den erstattet. En økt på den forrige revisjonen overlever altså én oppdatering. Ved oppdatering nummer to blir treet fjernet under en svært gammel økt, og klienten begynner å feile på å lese sin egen config. Det er en *synlig* feil, ikke en utrygg en. Ingen TTL, ingen aldersheuristikk, ingen liveness-sporing, dette er med vilje ikke en ny `GCStaged`. Aldersregelen der betydde «materialisert for mer enn 24 timer siden», ikke «ubrukt», og en regel som teller revisjoner sier i det minste noe sant om hva den beholder. En lokal kilde beholder bare den ene revisjonen launchen er i ferd med å bruke: pruningens andre kallsted er install-stien en lokal kilde aldri når, og uninstall går ut fra en state den aldri skriver, så et git-checkout ville ellers lagt igjen en katalog per commit forfatteren starter fra.
- **Kontekster deler ikke bytes.** Hver deklarert kontekst materialiseres uavhengig, så innhold to kontekster har felles lagres én gang per kontekst. For grillmester er det 2 klienter × 2 kontekster, der `full` og `focused` overlapper mye. Ingenting er målt. Merket i koden med oppgraderingsstien (`ponytail:`-kommentar, hardlink per digest).
- **Blandede pakker pinnes ikke i denne releasen, og Tier 2-klienten i dem nekter å starte.** Pinnen gjelder payload-only-pakker, slik at diffen holder seg unna Tier 1-skrivestien hver eksisterende bruker går gjennom. Tier 1-klienten i den samme pakka er upåvirket. Begrunnelsen for nektelsen framfor en fall-through står i [§4](#4-launch-beslutningene).
- **Installasjoner fra før pinnen migreres ikke.** En state uten `SourceSHA`, eller med en SHA det ikke finnes noen revisjonskatalog for, er ingen feil. Launchen resolver som før, og en `install` pinner den. Sporer den samme staten filer, pinner launchen ingenting. Den nekter og navngir install, slik at filene ikke mister sitt eneste spor uten at brukeren har bedt om det ([§4](#4-launch-beslutningene)). Rester under `~/.nav-pilot/staged/` fra en eldre binær ryddes heller ikke, siden ingenting leser dem igjen og en migrasjonslinje koster mer enn plassen de tar.
- **Lokale kilder betaler materialisering ved hver launch**, samme kostnad per launch som staging-modellen hadde. Den er akseptert fordi den kjøper forfatteren redigér-og-start-løkka pinnen ellers ville tatt fra dem ([§4](#4-launch-beslutningene)).
- **En hard drept prosess kan lekke ett `.tmp-*`-tre.** Pruningen rører aldri en `.tmp-*`-katalog. Det er staging-treet til en materialisering som pågår nå, for eksempel en launch som kappløper `sync --apply`, eller to første launcher på ulike SHA-er. Å slette et slikt tre midt i skrivingen feiler enten den andre prosessen, eller publiserer et halvslettet tre som revisjon hvis timingen lander mellom staging og rename. `materializeRevision` rydder sitt eget staging-tre på hver feilsti, så bare et `kill -9` etterlater ett. At det ikke feies på en aldersregel er poenget, ikke en forglemmelse: en TTL her ville vært `GCStaged` tilbake i en annen drakt, med den samme egenskapen at den ikke vet noe om bruk.
- **Én linje kan ikke dekkes av en test, og blir stående likevel.** `O_EXCL|O_NOFOLLOW` på opprettelsen av den stagede fila (`stage.go`, `stageFile`) er uoppnåelig fra en test, fordi destinasjonen ligger i en `MkdirTemp`-katalog opprettet mikrosekunder tidligere, hvis navn ingen annen prosess kjenner. Flaggene blir stående slik at unikhets-invarianten er *håndhevet* framfor *antatt*. Ryker antagelsen en gang, feiler staging framfor å skrive gjennom det som måtte ligge der. 13 av 14 sjekker i [#456](https://github.com/navikt/copilot/pull/456) er mutasjonstestet, og den fjortende er opplyst framfor skjult.
- **`stageCopyHook` er en testsøm i produksjonskoden.** Feilstien midt i kopieringen er ellers uoppnåelig fra ethvert input en test kan konstruere, siden hver fil kopien rører ble bevist eksisterende, regulær og korrekt hashet øyeblikk før. Hooken lar en test mutere kilden mellom verifisering og kopi, altså det ekte TOCTOU-tilfellet, og gir dermed både re-hashen og fail-closed-oppryddingen noe som faktisk kan feile.
- **`VerifyPayload` returnerer bare første brudd**, i motsetning til resten av valideringen som samler alle funn. Forbi første avvik er payloaden utroverdig, og en pakkeforfatter får ingenting igjen for en full liste over hva mer som er galt med et tre som uansett ikke stages.
- **Ingen OTel-config-injeksjon i staged opencode-modus.** `experimental.openTelemetry` skrives ikke, fordi det ville betydd å redigere enten den delte configen eller den digestbundne payloaden. OTel-miljøvariablene settes fortsatt.
- **To `Unreachable:`-grener** rundt `rec.perm()` i `payload.go` og `stage.go` er beholdt som defensiv feilretur, fordi `ParsePayloadManifest` allerede har avvist alle andre modes.
- **Kosmetisk rest i `openCodeDefaultModel`:** kjøres `config setup` med en `inherit`-pakke aktiv, merkes den innebygde modell-id-en «Nav default». Ingen M2-flyt setter en pakke før setup, så ingenting når dit i dag (`internal/provider/pakke.go`).
- **Tier-cachens 6-timers TTL er fortsatt et avgrensningstall.** Ingenting er målt. Cachen ble innsnevret framfor slettet med revisjonspinnen, og bærer nå bare ikke-payload-svaret. Verdien, den ene gjenværende stien og slettetriggeren står i [§4](#4-launch-beslutningene).

## 7. Begrunnelser som ikke sto skrevet noe sted før dette dokumentet

Valgene under var tatt uten at begrunnelsen fantes i kode, plan eller issue. De er skrevet ned her framfor hentet fra et tidligere dokument. Behandle dem som begrunnelser vi står inne for, ikke som noe som har vært gjennom en gjennomgang.

- **Pakkeroten er `~/.nav-pilot/pakker/` og ikke `os.UserCacheDir()`** fordi operativsystemet kan rydde en cache-katalog under en økt som kjører, og fordi nav-pilots egen state allerede bor på ett sted. Det argumentet veide allerede tungt da treet levde én launch. Det veier tyngre nå, når revisjonen skal ligge mellom økter og en launch peker klienten rett på den.
- **Revisjonsstien navngir kilden** (`<eier>-<repo>/<sha>`, småskrevet for repo-id-er), der det faste prefikset `nav-pilot-staged-` den erstatter var bevisst intetsigende. Prefikset fantes for å gjøre navnevakten i `CleanupStaged` mulig. Med den funksjonen borte er det ingen grunn igjen til at en kataloglisting skal skjule hvilken pakke som ligger der. `/` flates til `-`, så `a/b-c` og `a-b/c` gir samme katalognavn. Det er akseptert, siden en kollisjon i tillegg krever samme SHA *og* en `SourceRepo` i staten som matcher.

## 8. Der kildene er uenige

Koden er fasit. Tabellen holder oversikt over hvor en plan, en PR-tekst eller en tidligere kontraktversjon sier noe annet.

| Kilde | Hva kilden sier | Hva som gjelder |
| --- | --- | --- |
| M2-planen, staging-katalog | Fast katalog per pakke, klient og kontekst, «tømmes og skrives på nytt per launch» | Lukket. Koden svarte først med en unik `MkdirTemp`-katalog per launch av hensyn til samtidige økter. Med revisjonspinnen er katalogen fast per kilde og revisjon, og riktig av nøyaktig den grunnen den var feil før: den er innholdsadressert og skrives aldri om ([§3](#3-materialiseringsmodellen-og-tillitsgrensen)) |
| WP3-planen, cplt-argumentrekkefølge | `--pass-env` før `--allow-read` | Referansens rekkefølge, `--allow-read` først ([§4](#4-launch-beslutningene)) |
| #457 og WP3-planen, brukerens Copilot-instruksjoner | `CopilotEnv` gjenbrukt som den var, så brukerens `~/.copilot`-innhold blandes inn i en Tier 2-økt. Planen klassifiserte det som «launcher policy» | #458 injiserer ingenting, og valget er samlet i ett kall. Klassifiseringen er forlatt ([§4](#4-launch-beslutningene), [§5.3](#53-skal-en-pakke-kunne-be-om-brukerens-egen-kontekst-eier-nav-pilot)) |
| #457-teksten, hva en resolve-feil gjør | En akseptert regresjon for copilot-brukere med egen kilde | Koden i #458 advarer og faller tilbake til legacy-stien. #457-framstillingen var feil på to punkter ([§4](#4-launch-beslutningene)) |
| #458 og en tidligere §2.5, `--no-audit` | Launcher-policy vi ikke tar i bruk, med referansens egen kommentar som kilde | Team eSyfo svarte at flagget er bærende ved den testede cplt-baselinen ([kommentar i #437](https://github.com/navikt/copilot/issues/437#issuecomment-5437575432)). Den stagede stien sender det, og vår klassifisering er forlatt ([§2.5](#25-flagg-fra-referansen)) |
| M1-kontrakten ([#436](https://github.com/navikt/copilot/pull/436)), hvor `primaryAgents` hører hjemme | Rosteret på klientoppføringen, også for Tier 2 | eSyfos G4-svar viste at payload-rostere skiller seg per kontekst. Rosteret ligger på payloaden for Tier 2 ([§4](#4-launch-beslutningene)). M1-formen er forlatt, ikke deprekert, den hadde ingen konsumenter |
| WP3-planen, omstokking av opencode-argumenter | `_opencode_client_arguments` kuttet, «legges inn hvis noen melder fra» | #458 implementerer den ([§4](#4-launch-beslutningene)). Planen er utdatert på dette punktet |

## Se også

- [Agentpakke-kontrakten](README.agentpakke.md), hva en agentpakke er og hva nav-pilot krever av den
- [`cli/nav-pilot/schemas/agentpakke-v1.json`](../cli/nav-pilot/schemas/agentpakke-v1.json), kontrakten selv
- [#435](https://github.com/navikt/copilot/issues/435), PRD-en, med kontraktbeslutningene per revisjon
- [#437](https://github.com/navikt/copilot/issues/437), M2: krav G1-G4, C1-C4, F1, og statusdialogen med Team eSyfo
- [cli/nav-pilot/DESIGN.md](../cli/nav-pilot/DESIGN.md), internt design, sømmer og migrasjonsplan
