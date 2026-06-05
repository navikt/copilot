# Video-systemet

Dette dokumentet beskriver hvordan shorts-videoene i `navikt/copilot` er satt opp, og hvordan du kommer i gang lokalt og i dev.

## Oversikt

Video-løsningen består av fire deler:

1. `copilot-api` eksponerer en offentlig feed og avspillingsmetadata.
2. Video-manifestet ligger i GCS og leses ved runtime.
3. `my-copilot` viser en vertikal shorts-feed på forsiden.
4. `video:publish`-taskene laster opp filer og oppdaterer manifestet i GCS.

## Flyt

1. Du laster opp poster, HLS-master, eventuelle segmenter og teksting til dev-bucketen.
2. Du publiserer manifestet med `video:publish:dev`.
3. `copilot-api` leser manifestet fra GCS og svarer på:
   - `GET /public/v1/videos`
   - `GET /public/v1/videos/{id}/play`
   - `GET /public/v1/videos/{id}/captions`
4. `my-copilot` henter feeden og renderer videoene på forsiden.

## Kom i gang lokalt

Start appene:

```bash
cd /Users/hans/go/src/github.com/navikt/copilot
mise dev
```

Eller separat:

```bash
cd apps/copilot-api && mise dev
cd apps/my-copilot && mise dev
```

## Konfigurasjon

Viktige variabler:

| Variabel | Bruk |
|---|---|
| `VIDEO_BUCKET_PUBLIC` | Public bucket for poster, HLS og manifest |
| `VIDEO_BUCKET_RAW` | Raw bucket for originalfiler |
| `VIDEO_MANIFEST_URL` | Runtime-URL til manifestet |
| `VIDEO_PUBLIC_BASE_URL` | Base-URL for offentlige video-objekter |
| `VIDEO_FEED_CACHE_SECONDS` | TTL for feed og manifest-cache |
| `VIDEO_MANIFEST_PATH` | Lokal fil for tester og lokal fallback |

## Publiser en video i dev

```bash
cd apps/my-copilot
VIDEO_BUCKET_PUBLIC=<dev-bucket> mise run video:publish:dev -- \
  --id intro-cli \
  --title "Intro til Copilot CLI" \
  --category copilot \
  --duration-sec 42 \
  --poster-file /path/poster.jpg \
  --hls-file /path/master.m3u8 \
  --captions-file /path/captions.vtt
```

Merk:

- `video:publish:dev` oppdaterer manifestet i dev-bucketen.
- Hvis HLS-masteren peker på segmenter, må segmentene være lastet opp til samme prefix i bucketen.
- `gsutil`/GCP-tilgang må være satt opp lokalt.

## Verifiser e2e

1. Publiser en testvideo i dev.
2. Sjekk feeden:

```bash
curl http://localhost:8080/public/v1/videos?limit=5
```

3. Sjekk play-URL:

```bash
curl http://localhost:8080/public/v1/videos/<id>/play
```

4. Åpne `http://localhost:3000` og se at videoen vises i shorts-seksjonen.

## Feilsøking

- **Tom feed**: Sjekk at manifestet finnes i GCS og at `VIDEO_MANIFEST_URL` peker riktig.
- **503 ved refresh**: `copilot-api` server stale manifest hvis den har en cached kopi, ellers feiler requesten.
- **Video spiller ikke**: Kontroller at HLS-masteren peker på eksisterende segmenter i samme bucket-prefix.
- **Feil object path**: Filnavn må bare bruke trygge tegn; publish-scriptet validerer dette.

## Kilder i repoet

- `apps/copilot-api/video_handlers.go`
- `apps/copilot-api/video_manifest.go`
- `apps/my-copilot/scripts/publish-video.ts`
- `apps/my-copilot/.mise.toml`
- `apps/copilot-api/README.md`
- `docs/video-demoer-kost-token-optimalisering.md`
