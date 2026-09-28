---
title: "Lokal modell på Linux, med din egen server (eksperimentell)"
date: 2026-09-28
author: starefossen
category: nav-pilot
cli: true
excerpt: "nav-pilot kan nå bruke en modellserver du kjører selv, som Ollama eller llama-server. Dermed virker lokal modell på Linux også. Oppsettet er eksperimentelt: vi har prøvd det mot Ollama, llama-server og mlx_lm.server på én Mac, og på Linux bare i en arm64-container."
tags:
  - nav-pilot
  - local-models
  - linux
  - alpha
---

Til nå har lokal modell i nav-pilot krevd en Mac med Apple Silicon. Nå kan nav-pilot i stedet bruke en server du kjører selv: Ollama, `llama-server` fra llama.cpp eller en annen server med OpenAI-kompatibelt API. Da kan du bruke lokal modell på Linux også.

Dette er **eksperimentelt**. Vi har ikke målt det på en vanlig Linux-maskin ennå. Vi skriver om det nå fordi vi trenger noen som prøver.

Slik ser det ut med `llama-server`:

```bash
# terminal 1: serveren blir stående så lenge den kjører
llama-server --jinja -c 65536 --port 8080 -hf unsloth/Qwen3.6-35B-A3B-GGUF:UD-Q4_K_XL

# terminal 2
nav-pilot alpha local setup    # finner serveren, foreslår modell og lagrer oppsettet
nav-pilot alpha local doctor   # sjekker at serveren gjør det nav-pilot trenger
```

`setup` starter aldri en server selv og trenger ikke `sudo`. Mangler modellen i Ollama, spør den før den henter noe. Du trenger nav-pilot 2026.09.28-013345 eller nyere. Den versjonen retter to feil som ville villedet nye Ollama-brukere. Hele oppskriften, også for Ollama, står i [Kom i gang med egen server](/nav-pilot/lokal/egen-server).

## Hva som virker

| Steg      | Kommando                            | Hva den gjør                                                                                                                 |
| --------- | ----------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| Koble til | `nav-pilot alpha local setup`       | Finner servere som kjører på maskinen, foreslår modellen som ligger nærmest vår egen, og lagrer oppsettet hvis du svarer ja. |
| Sjekke    | `nav-pilot alpha local doctor`      | Sjekker verktøykall, logprobs, kontekst og tid til første token. Feiler en sjekk, sier den hva du skal endre.                |
| Spørre    | `nav-pilot alpha decide`            | Flervalgsspørsmål til modellen på serveren din. Krever logprobs.                                                             |
| Kode      | `nav-pilot` med opencode som klient | Modellen på serveren din blir underagenten `local-worker`, som hovedagenten kan sende avgrensede oppgaver til.               |

Utsending krever opencode som klient (`nav-pilot config set client opencode`). Copilot CLI har ingen slik underagent.

Koden din sendes til serveren, så nav-pilot godtar bare localhost og private IP-adresser.

## Hva vi har prøvd

| Hvor                          | Hva vi prøvde                                                                                | Resultat                                                                                                                  |
| ----------------------------- | -------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| Mac, M5 Max med 128 GB        | `setup`, `doctor`, `decide` og én opencode-økt mot `mlx_lm.server`, Ollama og `llama-server` | Virket på alle tre, ingen feil i 565 kall per server. I Ollama-økten påsto modellen at testen var kjørt uten å kjøre den. |
| Linux, arm64-container på CPU | Installasjon, `setup`, `doctor`, `decide` og én opencode-økt med en liten modell             | Funksjonstest, ingen målinger                                                                                             |
| Linux på x86_64               | –                                                                                            | Ikke prøvd                                                                                                                |
| NVIDIA-GPU                    | –                                                                                            | Ikke prøvd                                                                                                                |

På Macen ga `decide` gjennom `mlx_lm.server` nøyaktig de samme svarene som modellen nav-pilot setter opp selv. Med GGUF-modellen på Ollama og `llama-server` var treffsikkerheten like god innenfor feilmarginen. På Ollama og `llama-server` tok første svar på en lang prompt omtrent dobbelt så lang tid: 17–19 sekunder mot 9–10 ved 30 000 tokens.

## Hva du bør vite

- **Ingen målinger på Linux eller NVIDIA.** Tallene over er fra én Mac. Vi vet ikke hvor rask modellen er på en Linux-laptop eller et vanlig grafikkort.
- **Lite grafikkminne gir lite plass til kontekst.** Modellen og konteksten må få plass sammen. Har du et kort med 8 GB eller mindre, start `llama-server` med `--n-cpu-moe 999`, så ekspertlagene ligger i vanlig minne. Får serveren ikke plass til konteksten nav-pilot trenger, feiler kontekstsjekken i `doctor`.
- **Ollama kan gi modellen for lite kontekst.** På maskiner med under 24 GB grafikkminne gir Ollama modellen 4 096 tokens, mens første melding i en Copilot-økt er på rundt 22 000 tokens. `doctor` fanger dette, og `setup --fix-context` lager en kopi av modellen med 64k kontekst.
- **nav-pilot regner modellen på serveren din som ikke målt.** Hovedagenten får den generelle instruksen om utsending, og nav-pilot stopper ingen redigeringer.

Metode og alle tallene: [valideringen mot ekte servere](https://github.com/navikt/mlx-workspace/blob/main/reports/2026-09-28-local-endpoint-validation/report.md) og [røyktesten på Linux](https://github.com/navikt/mlx-workspace/blob/main/reports/2026-09-27-linux-smoke/report.md).

## Si fra

Vi trenger særlig deg som har Linux på x86_64, med eller uten NVIDIA-kort.

- Kjør `nav-pilot alpha local doctor` og lim inn det den skriver ut i #nav-pilot, både når det virker og når det feiler.
- Si hvilken server, modell og maskin du brukte, og hvor mye grafikkminne du har.
- Feil kan du også melde som et issue i [navikt/copilot](https://github.com/navikt/copilot/issues).

«Virket ikke hos meg» er like nyttig som det motsatte.

**Kilder:**

- [Bring your own local endpoint (Ollama, llama-server)](https://github.com/navikt/copilot/pull/998) (navikt/copilot, 27. september 2026)
- [Guided local endpoint setup](https://github.com/navikt/copilot/pull/1000) (navikt/copilot, 27. september 2026)
- [Setup finds an empty Ollama, doctor accepts name:latest](https://github.com/navikt/copilot/pull/1100) (navikt/copilot, 28. september 2026)
- [nav-pilot's own-endpoint path on real servers](https://github.com/navikt/mlx-workspace/blob/main/reports/2026-09-28-local-endpoint-validation/report.md) (navikt/mlx-workspace, 28. september 2026)
- [Linux smoke test of nav-pilot's local endpoint path](https://github.com/navikt/mlx-workspace/blob/main/reports/2026-09-27-linux-smoke/report.md) (navikt/mlx-workspace)
