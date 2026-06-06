# Video-systemet

Dette dokumentet forklarer arkitekturen og den praktiske arbeidsflyten for shorts-video i `navikt/copilot`.

Målet er enkelt: publiser ferdige videoobjekter til én GCS-bucket per miljø, oppdater manifestet, og vis videoene på forsiden i `my-copilot`.

## Arkitektur

| Del | Ansvar |
|---|---|
| GCS-bucket (`VIDEO_BUCKET_PUBLIC`) | Lagrer videofiler, poster, HLS-filer, teksting og `video_manifest.json` |
| `copilot-api` | Leser manifestet, validerer innhold, og eksponerer offentlige video-endepunkter |
| `my-copilot` | Henter feed fra `copilot-api` og viser shorts på forsiden |
| Publiseringsscript | Laster opp objekter til bucket og skriver manifestet til slutt |

## Dataflyt

1. Videofiler ligger i bucket under `videos/<id>/...`.
2. `video_manifest.json` inneholder metadata + objektstier for hver video.
3. `copilot-api` leser manifestet med cache og bygger offentlige URL-er.
4. `my-copilot` henter feeden og renderer videoene.

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
| `VIDEO_MANIFEST_URL` | URL til manifest i GCS |
| `VIDEO_PUBLIC_BASE_URL` | Base-URL som brukes til å bygge offentlige objekt-URL-er |
| `VIDEO_FEED_CACHE_SECONDS` | Cache-TTL for manifest/feed |
| `VIDEO_MANIFEST_PATH` | Lokal fallback-fil når URL ikke er satt |

## Lokal utvikling

Start begge apper fra repo-roten:

```bash
cd /Users/hans/go/src/github.com/navikt/copilot
mise dev
```

`hack/dev.sh` gjør dette for video:

- bruker `VIDEO_BUCKET_PUBLIC_DEV` når den er satt
- setter `VIDEO_BUCKET_PUBLIC`, `VIDEO_PUBLIC_BASE_URL` og `VIDEO_MANIFEST_URL`
- faller tilbake til lokal `video_manifest.json` når bucket-variabler mangler
- setter kort cache-TTL lokalt (`VIDEO_FEED_CACHE_SECONDS=60` som standard)

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
  --category nav-pilot
```

Dette lager `video-packages/<id>/` med:

- `poster.jpg`
- `video.mp4`
- `hls/master.m3u8`
- `hls/segments/*.ts`
- `video-package.json`
- `publish.sh`

Kjør så:

```bash
cd video-packages/nav-pilot-s01e01-prompt
VIDEO_BUCKET_PUBLIC=copilot-videos-public-dev ./publish.sh
```

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
  --captions-file /path/captions.vtt
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
  "sort_order": 100
}
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

3. Åpne `http://localhost:3000` og bekreft at videoen vises i shorts-seksjonen.

## Vanlige feil

- **Tom feed:** manifestet finnes ikke, eller `VIDEO_MANIFEST_URL` peker feil.
- **503 fra API:** manifest kunne ikke leses, og det finnes ingen cachet kopi.
- **Avspilling feiler:** HLS-master peker på segmenter som ikke finnes i samme bucket/prefix.
- **Valideringsfeil ved publisering:** `id` eller objektstier inneholder ugyldige tegn.

## Relevante filer i repoet

- `apps/copilot-api/video_handlers.go`
- `apps/copilot-api/video_manifest.go`
- `apps/my-copilot/scripts/prepare-video-package.ts`
- `apps/my-copilot/scripts/publish-video.ts`
- `hack/dev.sh`
