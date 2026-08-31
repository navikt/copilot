# nav-pilot: målinger og beslutninger, august 2026

Dette dokumentet er en **protokoll**, ikke en beskrivelse av gjeldende atferd. Det
registrerer hva som faktisk ble målt i august 2026, hvor usikre tallene er, og
hvilke beslutninger målingene bærer. Beskrivelsen av designet lever i
[`nav-pilot-design.md`](nav-pilot-design.md); den gjeldende modelltabellen lever i
[`modellvalg.md`](modellvalg.md). Ingen av dem skal duplisere tallene under.

Målingene kommer fra `scripts/nav-pilot-golden.sh` mot levende modeller.
Rå baseline-filer ligger i [`golden-baselines/`](golden-baselines/).

## 1. Personvern-blindsonen: modellsammenligning

### Hva som ble målt

Én påstand, fra golden-test 3: at personaen reiser de påkrevde blindsonene
**personvern** og **tilgangskontroll** for prompten `t2`, «ny tjeneste som leser
fnr fra ID-porten». Testen er definert i `scripts/nav-pilot-golden.sh` (regexene
`RE_BS1` og `RE_BS2`). Rundt 195 levende kjøringer totalt.

| Modell | Feil | Kjøringer | Rate | 95 % KI (Wilson) |
|---|---|---|---|---|
| Claude Sonnet 4.6 (sittende) | 2 | 50 | 4,0 % | 1,1 til 13,5 % |
| GPT-5.6 Sol | 1 | 50 | 2,0 % | 0,4 til 10,5 % |
| GPT-5.6 Luna | 1 | 50 | 2,0 % | 0,4 til 10,5 % |
| GPT-5.6 Terra | 5 | 45 | 11,1 % | 4,8 til 23,5 % |

Fisher eksakt mot den sittende modellen: Sol p = 1,00, Luna p = 1,00,
Terra p = 0,25.

### Hva resultatet betyr

**Ingen av forskjellene er signifikante.** Benchmarken viste ikke at noen modell
er tryggere, og den viste ikke at noen er mindre trygg. Den viste at modellene er
**umulige å skille fra hverandre** ved denne utvalgsstørrelsen. Konfidens-
intervallene overlapper alle. Det er nettopp derfor kostnad ble avgjørende: da
sikkerhet ikke skiller kandidatene, er det ingenting igjen som gjør det.

Ikke les tabellen som en rangering. Punktestimatene ser ut som en rangering, men
usikkerheten dekker hele spennet.

### Den metodiske lærdommen

Denne kostet ekte penger å lære, så den står her eksplisitt.

Ved **n = 5** er én observert feil forenlig med en sann feilrate mellom
**3,6 og 62,4 prosent** (Wilson, samme metode som tabellen over). En tidlig
n = 5-runde fikk Terra til å se utrygg ut og Luna til å se ren ut. Ved høyere n
feilet Luna også én gang, og Terras ulempe ble liggende innenfor støyen.

Konsekvenser for framtidige sammenligninger på en sjelden hendelse:

- n må ligge i titalls per modell før et punktestimat betyr noe.
- Å skille 2,5 prosent fra 10 prosent med konfidens krever rundt **n = 200 per
  modell**.
- En liten runde kan avkrefte at noe er *veldig* galt. Den kan ikke rangere
  kandidater som ligger nær hverandre, og den skal ikke brukes til det.

### Forbehold om sporbarhet

Baseline-filene under `docs/golden-baselines/` inneholder
**størrelsesmålinger**, ikke bestått/ikke-bestått per kjøring. Tallene i tabellen
over er talt opp under selve kjøringen og er ikke gjenskapbare fra de commitede
filene alene. Gjentas benchmarken, bør rådataene for assertions logges også.

## 2. Funnet som betyr mer enn modellvalget

**Den påkrevde personvern-blindsonen blir oversett på alle modeller som ble
testet: 2 til 4 prosent av gangene på de tre andre, og 11,1 prosent på Terra.**
Terra ligger høyere, men ikke signifikant høyere.

Ingen modellbytte fikser dette. Feilen ligger ikke i modellen; den ligger i
personaen. En prompt som eksplisitt sier at tjenesten leser fødselsnummer fra
ID-porten, skal reise personvern hver eneste gang, ikke i 89 til 98 prosent av
tilfellene.

Dette trenger et eget issue og en egen fiks i `agents/nav-pilot.agent.md`. Det er
ikke løst av noe i dette dokumentet.

## 3. Output-størrelse: et separat eksperiment

Dette er en annen måling med et annet formål, og den skal ikke blandes med
modellsammenligningen over.

Den alltid-på output-style-instruksjonen fra #481 ble målt før og etter, med
5 gjentak på én modell (`claude-sonnet-4.6`, klient `copilot`). Baselines:
`docs/golden-baselines/2026-08-28-before-output-style.txt` og
`docs/golden-baselines/2026-08-28-after-output-style.txt`.

Median byte per prompt:

| Prompt | Før | Etter | Endring |
|---|---|---|---|
| `t1` | 277 | 269 | trivielt ned |
| `t2` | 1063 | 1036 | innenfor støy |
| `t4` | 2902 | 3085 | innenfor støy |
| `t5` (auth-spørsmålet) | 807 | 481 | rundt 40 % ned |
| `t6` | 249 | 310 | innenfor støy |

**Resultatet er svakt.** Én prompt av fem ble materielt kortere, én trivielt, tre
lå innenfor støyen. Og støyen er stor: `t6` varierte fra 232 til 1168 byte over
fem identiske kjøringer, altså femgangeren. Med den spredningen kan en
median-til-median-sammenligning ved n = 5 ikke bære en konklusjon om at
instruksjonen virker generelt.

Det den kan si er at auth-spørsmålet ble vesentlig kortere. Det er ett
observasjonspunkt, ikke en effektstørrelse.

Merk at baseline-filene har filnavn datert 2026-08-28, mens `# date:`-feltet inni
dem sier 2026-08-30. Innholdet er autoritativt.

## 4. Beslutninger

Hver beslutning står med begrunnelsen sin og med hva den hviler på.

### 4.1 Modellpinner flyttes: Luna for lesing, Sol for resonnering

Lese- og mønsteranvendende agenter skal til Luna. Resonneringssjiktet skal til
Sol. `forfatter` blir stående på Claude Sonnet 4.6 for norsk tekst.

**Hviler på:** kostnad. Ikke på sikkerhet, som ikke skilte kandidatene
(seksjon 1). `forfatter`-unntaket hviler på norsk språkkvalitet, ikke på
benchmarken, som ikke målte norsk tekst.

Tabellen i [`modellvalg.md`](modellvalg.md) viser fortsatt de gamle pinnene og
oppdateres når byttet er merget. Den gjentas ikke her, fordi den flytter seg.

### 4.2 Terra tas ikke i bruk

Terra er **ikke bevist dårligere**: p = 0,25 er ikke et bevis på noe. Men Sol er
billigere og har bedre punktestimat. Det finnes altså ikke noe argument *for*
Terra, og det holder til å la være.

Dette er ikke det samme som å konkludere at Terra er utrygg. Den konklusjonen har
vi ikke data til.

### 4.3 nav-pilot styrer standard klientkonfigurasjon

Der det finnes en standard, setter nav-pilot den. Der nav-pilot ikke kan sette
en standard, får brukeren beskjed.

**Verifisert i koden:** `ResolvedModelNotice` i
`cli/nav-pilot/internal/provider/pakke.go` skriver én linje som navngir modellen
launchen kjører på og hvor den kommer fra, og returnerer tom streng nettopp der
launchen ikke navngir noen modell. Rekkefølgen er brukerens egen innstilling
først, deretter den aktive agentpakkas erklæring.

#### Retting: det finnes ingen felles presedensrekkefølge

Denne seksjonen sa tidligere at «brukeren kan alltid overstyre». Setningen er
trukket. Den er riktig om nav-pilots egen del av kjeden og usann om hvilken
modell brukeren faktisk ender opp på, og et dokument som lover én rekkefølge på
tvers av klientene blir trodd.

**Det nav-pilot styrer** er to ting, og bare to:

1. Om kommandolinja navngir en modell i det hele tatt, og hvilken. `resolve()`
   i `cli/nav-pilot/internal/cli/config.go` legger config-filas `model`
   (linje 257) og `--model` fra kommandolinja (linje 305) inn i det *samme*
   feltet, `ResolvedConfig.Model`, lenge før noen modellbeslutning tas.
   Launch-byggerne ser derfor bare «brukeren sa noe» eller «brukeren sa
   ingenting», aldri hvilken av de to som sa det. Sa brukeren ingenting, leser
   `BuildCopilotArgs` pakkas erklæring
   (`cli/nav-pilot/internal/provider/copilot_launch.go:82`), mens den gamle
   opencode-stien sender `--model` uansett
   (`cli/nav-pilot/internal/provider/opencode_launch.go:101`; tom verdi
   normaliseres til Nav-standarden i `ToOpenCodeModel`).
2. Hvilken `model:`-linje en materialisert agentfil bærer, gjennom
   `openCodeAgentModel` i `cli/nav-pilot/internal/artifacts/export.go` (#490).

**Det klienten styrer** er hvordan de to rangeres mot hverandre, og der er
klientene uenige med hverandre:

- copilot avgjør selv når `--model` ikke sendes, og med `inherit` på copilot
  (4.7) er det dagens tilstand.
- opencode 1.18.25, målt ved å lese binæren og bekreftet med `opencode debug`
  ([#498](https://github.com/navikt/copilot/pull/498)): i TUI-en, som er den
  nav-pilot faktisk starter, vinner agentens egen `model:` over `--model`. På
  `opencode run` vinner flagget over frontmatteren.

To ting følger. Det ubetingede `--model`-flagget på opencode-stien er allerede
virkningsløst for enhver agent som har fått en `model:`-linje, og en modell
brukeren pinner taper mot agentens erklæring i TUI-en. Det er opencodes
oppførsel, ikke noe nav-pilot innfører, men det er grunnen til at «alt brukeren
setter vinner» ikke kan stå som en påstand.

Innledningen i #498 lovet «én regel på tvers av klientene». Målingen lenger nede
i samme PR motsier den. Modellrekkefølgen i `docs/agentpakke-beslutninger.md`
(brukerens pin, så pakkas `defaultModel`, ellers ingenting) er derimot riktig,
fordi den bare sier hva nav-pilot selv gjør og stopper der.

### 4.4 Per-agent-modell, ikke én modell per sesjon

Modellvalg er en egenskap ved hva en agent er *til for*, ikke ved sesjonen den
kjører i. En review-agent og en tekstforfatter har ikke samme behov.

**Forbehold, og det er viktig:** dette er en kvalitets- og kostnadspreferanse,
ikke en sikkerhetskontroll. Den sittende modellen feiler den samme sjekken med
samme rate (seksjon 1). Ingen skal lese per-agent-pinning som en barriere mot
blindsone-feilen.

### 4.5 Hentet JSON-konfigurasjonsprofil: bygget og forkastet

En profil som klienten henter over nett ble implementert og deretter forkastet.
Koden ligger på grenen `feat/model-default-profile` for den som vil se den.

Fire grunner, de tre første strukturelle:

1. **En pin finnes for at en launch skal lese frosne byte.** En standard som må
   kunne endre seg uten brukerhandling er det stikk motsatte kravet. De to kan
   ikke bo i samme mekanisme.
2. **`pinRevision` skriver tilstand uten `Files`.** For en Tier 1-installasjon
   *er* `Files`-lista hele oppføringen. En profil uten filer har ingen plass i
   den modellen.
3. **Innholdssynk-pipelinen bærer allerede agent-frontmatter sentralt.** Å legge
   til en andre distribusjonsvei for det samme er duplisering, ikke kapabilitet.
4. **En samtykkebasert leveringsvei for endrede standarder finnes allerede.**
   `cli/nav-pilot/internal/cli/interactive.go` (linje 205 til 243) tilbyr «Sync
   now?» med Yes og No når et scope er utdatert, og
   `cli/nav-pilot/internal/artifacts/staleness.go` har
   `checkInterval = 24 * time.Hour` (linje 13). Agentenes `model:`-linjer er
   installert innhold. En endret modellpin når derfor en bruker innen et døgn,
   mot ett Yes-klikk, uten en linje ny kode.

Punkt 4 er baseline-en enhver framtidig hentemekanisme må slå. Det eneste en
slik mekanisme legger til, er å slippe klikket. Forbeholdet er
`tracksDefaultSource`: staleness-sjekken gjelder installasjoner som følger
standardkilden, ikke en installasjon som peker et annet sted.

**Om reviewgaten for innhold hver klient henter:** repoet har en CODEOWNERS. Den
ligger i rota og ikke i `.github/`, og består av én linje, `* @navikt/copilot`
([#77](https://github.com/navikt/copilot/pull/77)). Hele teamet eier alt, så
gaten er en vanlig PR med teamet som kodeeier, ikke en egen gate for innhold som
distribueres videre.

### 4.6 `transformAgent` forblir allowlist-formet

`transformAgent` i `cli/nav-pilot/internal/artifacts/export.go` bygger
frontmatter på nytt fra et fast sett felter i stedet for å slippe kildens
frontmatter gjennom.

**Verifisert eksperimentelt, mot opencode 1.18.25:** opencode avviser copilots
`tools:`-liste direkte, med `Configuration is invalid ... Expected object |
undefined`, og agentfila lastes da ikke i det hele tatt. opencode vil ha et map;
copilot sender en liste av MCP-kvalifiserte strenger.

Det avgjørende er hvem som rammes: **opencode re-materialiserer ved hver
launch**. En dårlig transform når hver eneste bruker umiddelbart, uten kanari og
uten utrulling å stanse. Det er derfor pass-through ikke er verdt bekvemmelig-
heten. Begrunnelsen står foreløpig bare her. `transformAgent` har ingen
doc-kommentar i koden i dag, så det finnes ikke noe vern mot at formen blir
«forenklet» bort igjen. Den bør inn i funksjonsdokumentasjonen, som en egen
kodeendring.

### 4.7 Copilots pakke-erklæring er `inherit`

Tier 1 copilot fikk en `defaultModel`-erklæring i #490. Verdien er
`agentpakke.InheritModel`, altså strengen `inherit`, ikke en modell-id og ikke
`auto`.

**Konsekvensen er at merge-en ikke endret atferd.** En Tier 1 copilot-launch
sendte aldri `--model` når brukeren ikke hadde pinnet noe, og `inherit` er verdien
som sier akkurat det. Argv er byte-identisk på alle veier, og de eksisterende
golden launch-vektorene er beviset.

Å velge en reell verdi er en **egen beslutning**, og den tilhører den som eier
Auto-routing-preferansen dokumentert på `OpenCodeDefaultModel` i
`cli/nav-pilot/internal/provider/provider.go`. Å pinne en id der ville reversert
den preferansen som en bivirkning.

**Spaken er én linje, og den er ikke en hentemekanisme.** Erklæringen står på
`cli/nav-pilot/internal/agentpakke/legacy.go:71`, og lesestedet #490 la til blir
reelt nådd: `BuildCopilotArgs` i
`cli/nav-pilot/internal/provider/copilot_launch.go:82` leser
`pakkeDeclaredModel("copilot")` når brukeren ikke har pinnet noe. En konkret
modell-id på den ene linja endrer altså standarden for standardklienten uten at
noe hentes over nett. Kommentaren over erklæringen skyver valget til den som
eier rutingsbeslutningen, og den beslutningen er ikke tatt.

### 4.8 `--model` driver klientmodellen, ikke skriving i klientens config

nav-pilot setter modellen klienten starter med ved å sende `--model` ved
oppstart. Den skriver ikke inn i klientens egen konfigurasjonsfil.

**Begrunnelsen:** nav-pilot rangerer over det brukeren har valgt inne i klienten
sin, og oppstartsflagget uttrykker akkurat den rangeringen uten at nav-pilot
strekker seg inn i filer utvikleren eier.

Alternativet ble bygget og forkastet: Nav-standarden skrevet inn i
`~/.config/opencode/opencode.json` som toppnivå-`model`, med fletting slik at
utviklerens øvrige nøkler overlever
([#498](https://github.com/navikt/copilot/pull/498), grenen
`feat/align-model-semantics`). Den ble gjennomgått og ikke merget; `main` har
aldri båret den. Grunnen er rekkevidden: å skrive i klientens egen konfigurasjon
endrer standarden for *alle* opencode-økter på maskinen, også de nav-pilot aldri
startet. Det er en større påstand enn «nav-pilot bestemmer hva nav-pilot starter
med», og den rekker utenfor nav-pilots eget område.

Hva som ville endret beslutningen står i
[#500](https://github.com/navikt/copilot/issues/500): et behov for noe som ikke
kan uttrykkes som et oppstartsflagg, der modeller per agent via
`agent.<navn>.model` er det nærmeste eksempelet, eller et ønske om at
Nav-standarden skal gjelde utenfor nav-pilot-startede økter. Issuet lister også
det som må avklares før noen bygger det: eierskap av nøkler, reversering,
formatering av utviklerens fil, og synlighet i brukerdokumentasjonen.

## 5. Feller i koden, notert for den som utvider

**`StalenessCache` rekonstrueres tre steder, ikke to.**
`cli/nav-pilot/internal/artifacts/staleness.go` bygger et helt nytt
`StalenessCache` på linje 133 (etter et mislykket oppslag) og på linje 141
(etter et vellykket), og `cli/nav-pilot/internal/cli/update.go` gjør det samme
på linje 130 etter en selvoppdatering. Ingen av dem leser det eksisterende
objektet og endrer ett felt; alle tre setter feltene de bryr seg om og lar
resten falle bort. Et nytt felt på structen blir derfor stille borte på det
stedet man glemmer.

## Referanser

- `scripts/nav-pilot-golden.sh`: harnesset, prompter og assertions
- [`golden-baselines/`](golden-baselines/): rå størrelsesmålinger
- [`modellvalg.md`](modellvalg.md): gjeldende modelltabell
- [`nav-pilot-design.md`](nav-pilot-design.md): designet og beslutningshistorikken
- `cli/nav-pilot/internal/agentpakke/legacy.go`: `inherit`-erklæringen med begrunnelse
- `cli/nav-pilot/internal/provider/copilot_launch.go`: lesestedet for pakkas
  erklæring
- [#500](https://github.com/navikt/copilot/issues/500): vilkårene for å skrive
  klientkonfigurasjon
- `cli/nav-pilot/internal/artifacts/export.go`: `transformAgent` og `openCodeAgentModel`
