---
title: "Lokal modell på Linux, med din egen server (eksperimentell)"
date: 2026-09-28
author: starefossen
category: nav-pilot
cli: true
excerpt: "nav-pilot kan bruke en modellserver du kjører selv, som Ollama eller llama-server. Da virker lokal modell på Linux også. Oppsettet er eksperimentelt, og vi har ingen tall fra x86_64 eller NVIDIA."
tags:
  - nav-pilot
  - local-models
  - linux
  - alpha
---

**Siste tall:** Hvilke modeller vi anbefaler nå, står på [Lokale modeller](/innsikt/lokale-modeller#modeller-per-minne).

```bash
# terminal 1
ollama serve

# terminal 2
nav-pilot alpha local setup    # finner serveren, foreslår modell og lagrer oppsettet
nav-pilot alpha local doctor   # sjekker at serveren gjør det nav-pilot trenger
```

Til nå har lokal modell i nav-pilot krevd en Mac med Apple Silicon. Nå kan nav-pilot bruke en server du kjører selv: Ollama, `llama-server` fra llama.cpp eller en annen server med OpenAI-kompatibelt API. Da virker lokal modell på Linux også.

Dette er **eksperimentelt**. Du trenger nav-pilot 2026.09.28-113621 eller nyere. Oppskriften, også for `llama-server`, står i [Kom i gang med egen server](/nav-pilot/lokal/egen-server).

`setup` starter aldri en server selv og trenger ikke `sudo`. Koden din sendes til serveren, så nav-pilot godtar bare localhost og private IP-adresser.

## Hva du får

- `nav-pilot alpha decide` stiller flervalgsspørsmål til modellen på serveren din. Serveren må støtte logprobs.
- Med opencode som klient (`nav-pilot config set client opencode`) blir modellen underagenten `local-worker`, som hovedagenten kan sende avgrensede oppgaver til. Copilot CLI har ingen slik underagent.

## Hva vi har prøvd

**Linux:** i en arm64-container på CPU med 5 GiB minne og Qwen3 1.7B. `setup` ga opp etter 10 minutter, fordi kontekstsjekken på 30 000 tokens går for sakte på CPU ([#1222](https://github.com/navikt/copilot/issues/1222)), så vi lagret oppsettet for hånd med `nav-pilot config set`. Da virket installasjonen mot Ollama, `doctor` fant riktig feil, `decide` svarte på under et halvt sekund, og en opencode-økt startet mot den lokale modellen. Med `llama-server` kom vi til `doctor`, ikke lenger.

Men en kodeøkt trenger mer enn 6 GB minne og en kontekst på minst rundt 16 000 tokens. På denne maskinen ga Ollama modellen 4 096 tokens kontekst og kuttet øktens første melding, på rundt 15 000 tokens, til rundt 2 000. Modellen jobbet i blinde. `llama-server` med 32 768 tokens ble drept av minnemangel etter en lang prompt. Hvor mye minne en kodeøkt på CPU trenger, har vi ikke målt.

**Mac:** på en M5 Max med 128 GB virket `setup`, `doctor` og en opencode-økt mot Ollama, `llama-server` og `mlx_lm.server`, og `decide` svarte uten feil i 565 kall per server.

**x86_64 og NVIDIA:** ikke prøvd. Vi har ingen tall derfra.

## Lite minne

- **Ollama kan gi modellen for lite kontekst** på maskiner med lite minne, og kan kutte lange prompter uten å si fra. `doctor` fanger det, og `setup --fix-context` lager en kopi av modellen med 64k kontekst, hvis den får plass.
- **Får ikke konteksten plass,** foreslår `doctor` en som gjør det, for eksempel 16 384 tokens, og hvordan du legger deler av modellen i vanlig minne. Se [Lite minne](/nav-pilot/lokal/egen-server#lite-minne).
- **Starter ikke `llama-server` fra llama.cpp-bygget for Ubuntu** (`libgomp.so.1: cannot open shared object file`), mangler `libgomp1`: `sudo apt install libgomp1`.

## Si fra

Vi trenger særlig deg som har Linux på x86_64, med eller uten NVIDIA-kort. Kjør `nav-pilot alpha local doctor` og lim inn resultatet i #nav-pilot, sammen med server, modell, maskin og hvor mye grafikkminne du har. Feil melder du i [navikt/copilot](https://github.com/navikt/copilot/issues).

**Kilder:**

- [Linux smoke test of nav-pilot's local endpoint path](https://github.com/navikt/mlx-workspace/blob/main/reports/2026-09-27-linux-smoke/report.md) (navikt/mlx-workspace, 28. september 2026)
- [nav-pilot's own-endpoint path on real servers](https://github.com/navikt/mlx-workspace/blob/main/reports/2026-09-28-local-endpoint-validation/report.md) (navikt/mlx-workspace, 28. september 2026)
- [Bring your own local endpoint (Ollama, llama-server)](https://github.com/navikt/copilot/pull/998) (navikt/copilot, 27. september 2026)
- [Guided local endpoint setup](https://github.com/navikt/copilot/pull/1000) (navikt/copilot, 27. september 2026)
- [The 30k context probe takes over 10 minutes on a CPU-only machine](https://github.com/navikt/copilot/issues/1222) (navikt/copilot, 28. september 2026)
- [Setup saves a short context on request, and names an OOM](https://github.com/navikt/copilot/pull/1130) (navikt/copilot, 28. september 2026)
- [apt-get -y for scripted installs, libgomp1 for llama-server](https://github.com/navikt/copilot/pull/1134) (navikt/copilot, 28. september 2026)
- [Doctor advises a context that fits on small GPUs](https://github.com/navikt/copilot/pull/1139) (navikt/copilot, 28. september 2026)
