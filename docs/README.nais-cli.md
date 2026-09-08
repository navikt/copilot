# 🔐 nais-CLI og cplt-sandkassen

Kort svar: `nais` **kan** fungere inne i en cplt-sandkasse, men du bør ikke skru det på.
Kjør bootstrap utenfor sandkassen først, og start agenten etterpå. Begrunnelsen er ikke at
det er umulig — den er at `nais` er laget for å skrive ekte secrets til stdout, og stdout er
modellens kontekstvindu.

## Hva som skjer ut av boksen

| Kommando | I en agent-sesjon (`cplt`) |
|---|---|
| `nais --version`, `nais --help` | Fungerer |
| `nais device status` | `unable to connect to naisdevice; make sure naisdevice is running` |
| `nais status`, `nais app …`, `nais secret …` | `ERROR: Please run nais login --nais to (re-)authenticate.` |
| `nais login` (uten flagg), `nais kubeconfig`, `nais postgres …` | Feiler — de går via `gcloud`, og `~/.config/gcloud` er blokkert |

Feilmeldingen fra `nais device status` er misvisende: naisdevice *kjører*. Sandkassen nekter
bare tilkoblingen til agentens unix-socket.

> **Merk:** `cplt exec` kjører med en strammere profil enn en ekte agent-sesjon — den har ikke
> tilgang til nøkkelringen. Ikke bruk `cplt exec` til å konkludere om hva agenten din kan.

## Hvorfor

To uavhengige sperrer:

1. **Legitimasjonen ligger utenfor det agenten ser.** `nais` lagrer tokenet kryptert i
   `~/Library/Application Support/nais/nais-credentials.json.enc` (macOS) eller
   `~/.config/nais/…` (Linux). cplt gir agenten bare et smalt sett kataloger i hjemmeområdet,
   og denne er ikke blant dem. Dekrypteringsnøkkelen ligger i nøkkelringen
   (`cli.nais.io` / `nais-user`), og den *er* tilgjengelig i en agent-sesjon — så det er fila,
   ikke nøkkelen, som mangler.

2. **naisdevice-socketen er stengt.** `nais device …` snakker gRPC over
   `~/Library/Application Support/naisdevice/agent.sock`. Sandkassen nekter `connect()` til
   sockets den ikke har fått eksplisitt.

I tillegg er `~/.config/gcloud` blokkert på linje med `~/.aws`, `~/.azure`, `~/.kube` og
`~/.ssh`. Alt i `nais` som går via `gcloud` er stengt uansett hva du ellers åpner.

## Anbefalt: kjør oppsettet før du starter agenten

```sh
nais device status          # skal si "Connected"
nais login
./hentEnv.sh                # eller: nais app env <app> -e dev-gcp -t <team> -o json > .env
cplt                        # start agenten etterpå
```

`.env` ligger da i prosjektkatalogen, som agenten har full tilgang til. Docker-compose og
appen leser den som vanlig. Agenten trenger aldri `nais`.

## Ikke gi agenten tilgang til secrets

Det er fullt mulig å få `nais` til å virke i sandkassen. To grants holder for alt som går mot
Nais-APIet:

```sh
cplt \
  --allow-read   "$HOME/Library/Application Support/nais/nais-credentials.json.enc" \
  --allow-socket "$HOME/Library/Application Support/naisdevice/agent.sock"
```

**Ikke gjør dette.** Nettopp fordi det virker, er valget ditt og ikke sandkassens.
`nais secret get --with-values` og `nais app env` finnes for å skrive ekte, produksjonsnære
secrets til stdout. I en agent-sesjon *er* stdout modellens kontekstvindu: det havner i
transkripsjonen, det blir liggende, og det sendes med på hvert eneste påfølgende modellkall.
En hemmelighet som har vært innom et kontekstvindu må regnes som lekket.

Med disse to grantene har agenten et generelt verktøy for å lese alle teamets secrets. Det er
akkurat den evnen blokkeringen av `~/.config/gcloud` og `~/.aws` finnes for å holde unna.

Trenger du bare VPN-status for feilsøking, gi kun socketen:

```sh
cplt --allow-socket "$HOME/Library/Application Support/naisdevice/agent.sock"
```

Det gir `nais device status` og ingenting annet — ingen innlogging, og `nais app env` feiler
fortsatt.

## Småting

- `nais login --nais` inne i sandkassen krever i tillegg *skrivetilgang* til
  legitimasjonskatalogen og en callback-lytter på `localhost:8865`. Logg heller inn utenfor.
  Med bare lesetilgang vil en token-fornyelse etter hvert feile.
- Sett `DO_NOT_TRACK=1` hvis du vil unngå at `nais` sender telemetri til
  `collector-internet.nav.cloud.nais.io` ved hver kjøring.
- Under `--preset strict` er utgående trafikk allowlist-styrt, og `auth.nais.io` /
  `console.nav.cloud.nais.io` står ikke på lista.
- naisdevice-agenten lytter også på en localhost-TCP-port, men den er tilfeldig ved hver
  omstart, så `--allow-port` er ikke en farbar vei.
