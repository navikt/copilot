# Video-systemet

Dette dokumentet forklarer arkitekturen og den praktiske arbeidsflyten for shorts-video i `navikt/copilot`.

Målet er enkelt: publiser ferdige videoobjekter til én GCS-bucket per miljø, oppdater manifestet, og vis videoene på forsiden i `my-copilot`.

## Arkitektur

| Del | Ansvar |
|---|---|
| GCS-bucket (`VIDEO_BUCKET_PUBLIC`) | Lagrer videofiler, poster, HLS-filer, teksting og publisert `video_manifest.json`. Bucketen må være offentlig lesbar. |
| `copilot-api` | Leser manifestet, validerer innhold, og returnerer offentlige asset-URL-er |
| `my-copilot` | Henter feed fra `copilot-api` og renderer videoer direkte fra GCS |
| Publiseringsscript | Laster opp objekter til bucket og skriver manifestet til slutt |

Publiseringsscriptet setter bucket-IAM `allUsers:roles/storage.objectViewer` før opplasting.

⚠️ Legg kun inn ikke-sensitive, offentlige videoassets i denne bucketen. Ikke legg inn personopplysninger, hemmeligheter eller annet skjermingsverdig innhold.

## Dataflyt

1. Videofiler ligger i bucket under `videos/<id>/...`.
2. `video_manifest.json` i bucket inneholder metadata + objektstier for hver video.
3. `copilot-api` leser manifestet og bygger URL-er mot `https://storage.googleapis.com/<bucket>/...`.
4. `my-copilot` henter feeden og sender URL-ene videre til frontend.
5. Nettleseren henter video, poster og HLS-segmenter direkte fra GCS.

Publiseringsrekkefølgen er alltid: **upload av filer -> oppdater manifest -> visning i feed**.

## Endepunkter

`copilot-api` eksponerer:

- `GET /public/v1/videos`
- `GET /public/v1/videos/{id}/play`
- `GET /public/v1/videos/{id}/captions`

## Miljø og konfigurasjon

Systemet bruker én public bucket per miljø:

- dev: `copilot-videos-public-dev`
- prod: `copilot-videos-public-prod`

Viktige variabler:

| Variabel | Bruk |
|---|---|
| `VIDEO_BUCKET_PUBLIC` | Bucket for alle videoobjekter og manifest |
| `VIDEO_MANIFEST_URL` | Manifest-URL i GCS (bruk `gs://...` for å lese direkte via GCS) |
| `VIDEO_PUBLIC_BASE_URL` | Base URL for offentlige assets (normalt `https://storage.googleapis.com/<bucket>`) |
| `VIDEO_FEED_CACHE_SECONDS` | Cache-TTL for manifest/feed |
| `VIDEO_MANIFEST_PATH` | Lokal fallback-fil når URL ikke er satt |

## Lokal utvikling

Start begge apper fra repo-roten:

```bash
cd <repo-rot>
mise dev
```

`hack/dev.sh` gjør dette for video:

- bruker `VIDEO_BUCKET_PUBLIC_DEV` når den er satt
- setter `VIDEO_BUCKET_PUBLIC`, `VIDEO_MANIFEST_URL` og `VIDEO_PUBLIC_BASE_URL`
- faller tilbake til lokal `video_manifest.local-fallback.json` når bucket-variabler mangler
- lokal fallback-fil er kun for dev/test og skal ikke oppdateres manuelt
- setter kort cache-TTL lokalt (`VIDEO_FEED_CACHE_SECONDS=60` som standard)

## Hurtigkommandoer (mise)

Fra `apps/my-copilot`:

```bash
mise run video:episode1:prepare
mise run video:episode1:publish:dev
mise run video:episode1:regen:dev
mise run video:reset-and-republish
mise run video:prod:reset-and-republish
```

## Publisere en ny video i dev

Det finnes to praktiske spor.

### Spor A: Du har kun en `.mp4`

Generer publiseringspakke:

```bash
cd apps/my-copilot
mise run video:prepare -- \
  --input ./videos/nav-pilot.s01e01.prompt.mp4 \
  --id nav-pilot-s01e01-prompt \
  --title "Nav-pilot S01E01: Prompt" \
  --category nav-pilot \
  --series "video-demoer-kost-token-optimalisering" \
  --season 1 \
  --episode 1 \
  --tags "prompting,cost"
```

Valgfritt kan du sende overlay-metadata inn ved pakking:

```bash
--overlay-file ./video-packages/nav-pilot-s01e01-prompt/overlay.json
```

Dette lager `video-packages/<id>/` med:

- `poster.jpg`
- `video.mp4`
- `hls/master.m3u8`
- `hls/segments/*.ts`
- `video-package.json`
- `publish.sh`

`publish.sh` sender nå `video-package.json` videre til `publish-video.ts` via `--package-file`, slik at metadata (inkludert `publish_metadata` og eventuell `overlay`) automatisk blir med i manifestet.

Kjør så:

```bash
cd video-packages/nav-pilot-s01e01-prompt
VIDEO_BUCKET_PUBLIC=copilot-videos-public-dev ./publish.sh
```

`publish.sh` verifiserer først at du er autentisert og har tilgang til bucketen (`gcloud storage ls`/`gsutil ls`), setter så offentlig bucket-lesetilgang (`allUsers:objectViewer`), og publiserer deretter med `gcloud storage` (fallback `gsutil`).

### Spor B: Du har allerede ferdige filer

Publiser direkte:

```bash
cd apps/my-copilot
VIDEO_BUCKET_PUBLIC=copilot-videos-public-dev mise run video:publish:dev -- \
  --id intro-cli \
  --title "Intro til Copilot CLI" \
  --category copilot \
  --duration-sec 42 \
  --poster-file /path/poster.jpg \
  --hls-file /path/master.m3u8 \
  --mp4-file /path/video.mp4 \
  --captions-file /path/captions.vtt \
  --series "video-demoer-kost-token-optimalisering" \
  --season 1 \
  --episode 1 \
  --tags "prompting,cost"
```

## Hvordan manifestet styrer feeden

Kun videoer med `is_published: true` vises i feeden. API-et validerer også ID-er og objektstier.

Eksempel på én manifest-entry:

```json
{
  "id": "intro-cli",
  "title": "Intro til Copilot CLI",
  "description": "",
  "category": "copilot",
  "published_at": "2026-06-06T09:00:00.000Z",
  "duration_sec": 42,
  "aspect_ratio": "9:16",
  "language": "nb",
  "poster_object": "videos/intro-cli/poster.jpg",
  "hls_master_object": "videos/intro-cli/master.m3u8",
  "mp4_object": "videos/intro-cli/video.mp4",
  "captions_object": "videos/intro-cli/captions.vtt",
  "is_published": true,
  "sort_order": 100,
  "metadata": {
    "series": "video-demoer-kost-token-optimalisering",
    "season": 1,
    "episode": 1,
    "tags": ["prompting", "cost"]
  }
}
```

`metadata` er valgfritt. Det gjør modellen mer utvidbar uten å bryte eksisterende klienter.

## HLS-kontrakt og trygg publisering

For å unngå segment-404 på Safari og andre spillere, skal playlisten bruke én kanonisk struktur:

- `videos/<id>/master.m3u8`
- `videos/<id>/segments/segment_###.ts`

`publish-video.ts` laster opp segmenter basert på URI-ene i `master.m3u8`, ikke bare katalogtreet lokalt.

Nyttige flagg:

- `--strict-hls true`: feiler publisering hvis URI-er i playlist ikke følger kanonisk `segments/segment_###.ts`.
- `--clean-prefix true`: aktiverer cleanup av stale objekter i `videos/<id>/` **etter** vellykket opplasting og manifest-oppdatering.
- `--clean-prefix-apply true`: utfører faktisk sletting. Uten denne kjører cleanup i dry-run.
- `--clean-prefix-max-deletes <n>`: sikkerhetsgrense for antall objekter som kan slettes i én kjøring (default `50`).

Eksempel:

```bash
VIDEO_PUBLISH_ENV=prod VIDEO_BUCKET_PUBLIC=copilot-videos-public-prod \
node --experimental-strip-types scripts/publish-video.ts \
  --id intro-cli \
  ... \
  --strict-hls true \
  --clean-prefix true \
  --clean-prefix-apply true \
  --clean-prefix-max-deletes 20
```

## Verifisering etter publisering

1. Sjekk feed:

```bash
curl "http://localhost:8080/public/v1/videos?limit=5"
```

2. Sjekk avspilling:

```bash
curl "http://localhost:8080/public/v1/videos/<id>/play"
```

3. Sjekk at et asset er offentlig tilgjengelig:

```bash
curl -I "https://storage.googleapis.com/<bucket>/videos/<id>/master.m3u8"
```

4. Åpne `http://localhost:3000` og bekreft at videoen vises i shorts-seksjonen.

## Vanlige feil

- **Tom feed:** manifestet finnes ikke, eller `VIDEO_MANIFEST_URL` peker feil.
- **503 fra API:** manifest kunne ikke leses, og det finnes ingen cachet kopi.
- **`AccessDenied` på video i nettleseren:** bucket mangler offentlig lesetilgang. Kjør publiseringsscriptet på nytt, eller sett `allUsers:roles/storage.objectViewer` manuelt.
- **Avspilling feiler:** HLS-master peker på segmenter som ikke finnes i samme bucket/prefix.
- **Duplikater i prefix (`segment_*.ts` og `segments/segment_*.ts`):** kjør republisering med `--clean-prefix true` etter at playlist-URI-er er kanoniske.
- **Valideringsfeil ved publisering:** `id` eller objektstier inneholder ugyldige tegn.
- **`gsutil` feiler med Python 3.14:** bruk `gcloud storage` (scriptet gjør dette automatisk før eventuell `gsutil`-fallback).

## Relevante filer i repoet

- `apps/copilot-api/video_handlers.go`
- `apps/copilot-api/video_manifest.go`
- `apps/my-copilot/src/lib/public-videos.ts`
- `apps/my-copilot/src/components/shorts-feed.tsx`
- `apps/my-copilot/scripts/prepare-video-package.ts`
- `apps/my-copilot/scripts/publish-video.ts`
- `hack/dev.sh`
