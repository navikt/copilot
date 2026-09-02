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

### Ikke-underlegenhet, regnet i etterkant

Seksjon 1 sier at ingen forskjell er signifikant. Det er et fravær av bevis for en
forskjell, og det er ikke det samme som bevis for at kandidatene er gode nok. Det
spørsmålet stilles med en ikke-underlegenhetstest, og det er den [#584](https://github.com/navikt/copilot/issues/584)
setter først i køen fordi den ikke koster et eneste kall: dataene finnes allerede.

**Marginen er satt i etterkant, og det må stå her.** Overskriften sier «regnet i
etterkant», og δ ble valgt etter at tallene forelå. Det betyr at δ, ikke dataene,
avgjør den ene omstridte armen: Terra stryker ved δ = 0,10 og består ved
δ ≥ 0,175. En protokoll som fastsetter δ, ensidig nivå og minste n *før* neste
måling er oppgaven i #584, og til den finnes skal denne testen leses som en
etterhåndsanalyse.

**Spørsmålet:** er kandidatens passrate på test 3 dårligere enn den sittende
modellens med mer enn en margin vi på forhånd sier vi kan leve med? Marginen er
satt til **δ = 0,10**, altså ti prosentpoeng. Kandidaten er ikke-underlegen hvis
den **nedre** grensen for differansen `kandidat − referanse` ligger over **−δ**.

**Metode:** Newcombe hybrid score-intervall for differansen mellom to andeler,
bygget på Wilson-intervallene for hver arm, altså samme metode som tabellen over,
men ikke samme konstant: tabellen i seksjon 1 er tosidig 95 prosent
(`z = 1,96`), denne er ensidig 95 prosent (`z = 1,6449`). Snur du seksjon 1
bokstavelig, får du 86,5 til 98,9 prosent for referansen, ikke 88,6 til 98,7. Ensidig, fordi hypotesen bare handler om én
retning: vi bryr oss om at kandidaten er *dårligere*, ikke om at den er bedre.
Regnestykket ligger i [`scripts/ikke-underlegenhet.py`](../scripts/ikke-underlegenhet.py)
og kjøres uten nettverk.

Inndata er tabellen over, snudd fra feil til bestått:

| Arm | Bestått | Kjøringer | Passrate | Wilson, ensidig 95 % |
|---|---|---|---|---|
| Claude Sonnet 4.6 (referanse) | 48 | 50 | 96,0 % | 88,6 til 98,7 % |
| GPT-5.6 Sol | 49 | 50 | 98,0 % | 91,5 til 99,6 % |
| GPT-5.6 Luna | 49 | 50 | 98,0 % | 91,5 til 99,6 % |
| GPT-5.6 Terra | 40 | 45 | 88,9 % | 78,9 til 94,5 % |

Resultatet:

| Kandidat | Differanse | Newcombe-intervall | Nedre grense mot −δ | Ikke-underlegen? |
|---|---|---|---|---|
| GPT-5.6 Sol | +2,0 pp | −5,0 til +9,6 pp | −5,0 > −10,0 | **ja** |
| GPT-5.6 Luna | +2,0 pp | −5,0 til +9,6 pp | −5,0 > −10,0 | **ja** |
| GPT-5.6 Terra | −7,1 pp | −17,5 til +2,2 pp | −17,5 < −10,0 | **nei** |

Sol og Luna er altså ikke-underlegne den sittende modellen på test 3 ved
δ = 0,10. Terra er det ikke: intervallet strekker seg til 17,5 prosentpoeng
dårligere, og det er mer enn marginen tillater.

**Tre presiseringer, fordi denne testen er lett å lese feil.**

Tallene −5,0, −5,0 og −17,5 er **negative nedre grenser**, ikke gevinster. En
tidligere muntlig gjengivelse av dette resultatet oppga dem uten fortegn, som
«Sol +5,0, Luna +5,0, Terra +17,5». Det er samme tall, men motsatt fortegn, og med
fortegnet borte gir Terra-raden ingen mening: en kandidat som ligger 17,5
prosentpoeng *over* referansen kan ikke stryke på en ikke-underlegenhetstest.
Fortegnet er hele testen.

At Terra ikke består, er **ikke** det samme som at Terra er underlegen. Testen
har ett utfall til, og det er Terras: *ubesluttet*. Intervallet dekker både −10 og
0. (Å bruke den øvre grensen slik er strengt tatt en tosidig 90 prosent-uttalelse,
siden bare den nedre grensen bærer den ensidige garantien. Konklusjonen står
uansett: ved tosidig 95 prosent er intervallet −19,8 til +4,2, som dekker begge.) Dataene utelukker verken at Terra er like god eller at den er ti prosentpoeng
dårligere. Det svarer til Fisher p = 0,25 i tabellen over og motsier ikke
seksjon 4.2: Terra er fortsatt ikke bevist dårligere, den er nå også ikke bevist
god nok. Det er 45 kjøringer som er for få, ikke Terra som er avslørt.

Testen gjelder **én påstand**, blindsone-assertionen i test 3, og ikke suiten som
helhet. Se avsnittet under: test 3 er den eneste påstanden i suiten som har n nok
til at et slikt regnestykke betyr noe.

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

Gjennomgangen av hele conformance-suiten i
[#583](https://github.com/navikt/copilot/issues/583) satte to tall på det samme,
og begge hører hjemme i et beslutningsdokument.

**Styrken ved n = 5 per arm til å oppdage 0,95 mot 0,75 etterlevelse er 0,01.**
Det er ikke et svakt eksperiment, det er ikke noe eksperiment. Nitti-ni av hundre
ganger ville en reell forskjell av den størrelsen gå upåaktet hen. Vi har likevel
argumentert pinningsbeslutninger fra 4 av 5 mot 5 av 5. For 80 prosent styrke på
den samme forskjellen trengs rundt 50 kjøringer per arm. For nav-pilot er det 350
kall per modell: `scripts/nav-pilot-golden.sh` bruker sju levende kall per
gjennomkjøring. (#583 oppgir spennet «140 til 350». Den nedre enden er ikke
utledet noe sted og svarer ikke til noen agent i harnesset — accessibility bruker
fire kall, code-review to — så den bør ikke siteres videre uten et regnestykke.)

Tallet er dessuten en nedre grense på Fishers eksakte test: normaltilnærmingen
gir 48,8 per arm, mens Fisher krysser 80 prosent styrke først et sted mellom
n = 50 (0,748) og n = 60 (0,845).

**Suiten har aldri skilt to modeller på noen påstand, unntatt gjennom
artefakter.** Hver gang den så ut til å gjøre det, ble forskjellen sporet tilbake
til noe annet enn modellen: cr2 4/5 mot 5/5 var byggeartefakter i
fingeravtrykket ([#578](https://github.com/navikt/copilot/pull/578)), cr3-hellingen
i [#554](https://github.com/navikt/copilot/pull/554) var regnet på et regex som
måler norsk ordforråd og ikke forklaring, og uu3-forskjellen etter hooken måler
hooken. Tre av seks nav-pilot-påstander kunne ikke feile mot noen modell vi hadde
observert; [#590](https://github.com/navikt/copilot/pull/590) rettet test 1, 2 og
6, som var de tre.

Test 3 er unntaket, og det er derfor ikke-underlegenhetstesten over kunne regnes i
det hele tatt: n = 50 per arm er den eneste tilstrekkelig kraftige sammenligningen
i repoet. Den skal ikke generaliseres til de øvrige påstandene, og et grønt
suite-resultat er fortsatt en røyktest på at personaen framkaller den grove formen
av oppførselen, ikke et belegg for at modell X er like god som modell Y for Nav.

### Forbehold om sporbarhet

Baseline-filene under `docs/golden-baselines/` inneholder
**størrelsesmålinger**, ikke bestått/ikke-bestått per kjøring. Tallene i tabellen
over er talt opp under selve kjøringen og er ikke gjenskapbare fra de commitede
filene alene. Gjentas benchmarken, bør rådataene for assertions logges også.

Dette gjelder ikke-underlegenhetstesten over med. Passtallene 48/50, 49/50, 49/50
og 40/45 er de samme tallene som feiltabellen, snudd. Regnestykket er
reproduserbart fra tallene, og skriptet ligger i repoet; **tallene er ikke
reproduserbare fra de commitede filene**. Fra og med
[#590](https://github.com/navikt/copilot/pull/590) skriver `--save-baseline` også
en `*-results.psv` med rader per kjøring og per påstand. Skal denne testen kjøres
mot nye data, er det den fila som må ligge ved siden av baselinen; uten den er
neste analyse like uetterrettelig som denne.

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

#### Retting 31. august 2026: Sol-prisen var en kampanjepris

Kostnadsargumentet over hviler på Sol til $2.00 input og $10.00 output. Det er
kampanjepris, 50 prosent av standardpris, og den varer ut 3. september 2026.
Det står i fotnoten `gpt-56-sol-promo` i [GitHubs pristabell](https://docs.github.com/en/copilot/reference/copilot-billing/models-and-pricing).
Fotnoten oppgir ikke standardprisen selv. Doblet kampanjepris gir $4.00 og
$20.00, og det tallet er utregnet fra «50 % off», ikke lest av en publisert
tabell. `scripts/sync-model-pricing.mjs` kaster fotnotene når den genererer
`model-pricing.ts`, som er grunnen til at ingen av dokumentene våre visste dette
([#503](https://github.com/navikt/copilot/issues/503)).

Blandet 10:1 mellom input og output. **Forholdet 10:1 er et anslag, ikke noe vi
har målt**, samme forbehold som i `modellvalg.md`:

| Modell | Blandet $ per million tokens (10:1) |
|---|---|
| GPT-5.6 Sol, kampanje t.o.m. 3. sep | 2,73 |
| GPT-5.6 Terra | 2,91 |
| Claude Sonnet 4.6 | 4,09 |
| GPT-5.6 Sol, antatt standardpris fra 4. sep | 5,45 |
| Claude Opus 4.6 | 6,82 |

**Pinnen står.** `@security-champion` og `@nav-pilot-opus` flytter fra Opus 4.6,
og Sol til antatt standardpris er fortsatt 20 prosent billigere enn den.
Gevinsten er 20 prosent, ikke de 60 prosentene kampanjeprisen ga, og fra
4. september er Sol dyrere enn både Sonnet 4.6 og Terra. Sol er heller ikke det
billigste Powerful-alternativet til standardpris: GPT-5.3-Codex ligger på 2,86
blandet.

Luna er ikke berørt. OpenAIs modellside for `gpt-5.6-luna` oppgir $0.20 og
$1.20 som listepris uten kampanjeformuleringer, og GitHubs pristabell har ingen
fotnote på raden. Luna-pinningene har derfor ingen utløpsdato.

### 4.2 Terra tas ikke i bruk

Terra er **ikke bevist dårligere**: p = 0,25 er ikke et bevis på noe. Men Sol er
billigere og har bedre punktestimat. Det finnes altså ikke noe argument *for*
Terra, og det holder til å la være.

Dette er ikke det samme som å konkludere at Terra er utrygg. Den konklusjonen har
vi ikke data til.

**Retting 31. august 2026:** «Sol er billigere» gjaldt kampanjeprisen. Fra
4. september er Terra billigere enn Sol ved 10:1 (2,91 mot 5,45), så
kostnadsbeinet i argumentet over faller bort. Beslutningen blir stående som den
er: den ble tatt på det grunnlaget som stod her, og punktestimatene i seksjon 1
er ikke en rangering. Om prisen alene nå taler for Terra, er det en ny
beslutning som må tas for seg, ikke en omskriving av denne.

**Tillegg:** ikke-underlegenhetstesten i seksjon 1 gir beslutningen et bein til.
Terra er den eneste kandidaten som ikke består den ved δ = 0,10. Det er fortsatt
ikke et bevis på at Terra er dårligere — utfallet er *ubesluttet*, ikke
*underlegen* — men det er heller ikke et belegg for at den er god nok, og et
argument *for* Terra må i så fall komme fra en måling som finnes.

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
- `scripts/ikke-underlegenhet.py`: ikke-underlegenhetstesten for test 3
- [#583](https://github.com/navikt/copilot/issues/583): hva conformance-suiten
  faktisk måler, styrke og ikke-separasjon
- [#584](https://github.com/navikt/copilot/issues/584): arbeidskøen som følger av
  #583, der ikke-underlegenhet stod først
- [#590](https://github.com/navikt/copilot/pull/590): `results.psv` ved siden av
  hver baseline
