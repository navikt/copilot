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

Til nå har lokal modell i nav-pilot krevd en Mac med Apple Silicon. Nå kan nav-pilot bruke en server du kjører selv: Ollama, `llama-server` fra llama.cpp eller en annen server med OpenAI-kompatibelt API. Da virker lokal modell på Linux også.

Dette er **eksperimentelt**. Vi har ikke prøvd det på en vanlig Linux-maskin ennå, bare i en container. Vi skriver om det nå fordi vi trenger noen som prøver.

Slik ser det ut med `llama-server`:

```bash
# terminal 1: serveren blir stående så lenge den kjører
llama-server --jinja -c 65536 --port 8080 -hf unsloth/Qwen3.6-35B-A3B-GGUF:UD-Q4_K_XL

# terminal 2
nav-pilot alpha local setup    # finner serveren, foreslår modell og lagrer oppsettet
nav-pilot alpha local doctor   # sjekker at serveren gjør det nav-pilot trenger
```

`setup` starter aldri en server selv og trenger ikke `sudo`. Mangler modellen i Ollama, spør den før den henter noe. Du trenger nav-pilot 2026.09.28-113621 eller nyere. Den har rettelsene fra røyktesten på Linux. Hele oppskriften, også for Ollama, står i [Kom i gang med egen server](/nav-pilot/lokal/egen-server).

## Installere på Linux

Velg Linux på [nav-pilot-siden](/nav-pilot), så får du én kommando som installerer nav-pilot og cplt. Vil du heller bruke apt-arkivet (Debian og Ubuntu), ligger det rett under. Feiler nedlastingen av nøkkelen, stopper apt-blokken og sier hva du kan gjøre i stedet. Før fortsatte den med en tom nøkkel og endte i «Unable to locate package nav-pilot».

Bruker du llama.cpp-bygget for Ubuntu på arm64, trenger du `libgomp1` (`sudo apt install libgomp1`). Uten det stopper `llama-server` med `libgomp.so.1: cannot open shared object file`.

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

| Hvor                                   | Hva vi prøvde                                                                                | Resultat                                                                                                                                                                                 |
| -------------------------------------- | -------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Mac, M5 Max med 128 GB                 | `setup`, `doctor`, `decide` og én opencode-økt mot `mlx_lm.server`, Ollama og `llama-server` | Virket på alle tre, ingen feil i 565 kall per server. I Ollama-økten påsto modellen at testen var kjørt uten å kjøre den.                                                                |
| Linux, arm64-container på CPU med 6 GB | Installasjon og `setup` mot Ollama og `llama-server` med en liten modell (Qwen3 1.7B)        | Installasjonen virket. Røyktesten fant fem feil i installasjonen og `setup`; alle er rettet eller står i oppskriften. `doctor`, `decide` og en opencode-økt er ikke kjørt på Linux ennå. |
| Linux på x86_64                        | –                                                                                            | Ikke prøvd                                                                                                                                                                               |
| NVIDIA-GPU                             | –                                                                                            | Ikke prøvd                                                                                                                                                                               |

På Macen ga `decide` gjennom `mlx_lm.server` nøyaktig de samme svarene som modellen nav-pilot setter opp selv. Med GGUF-modellen på Ollama og `llama-server` var treffsikkerheten like god innenfor feilmarginen. På Ollama og `llama-server` tok første svar på en lang prompt omtrent dobbelt så lang tid: 17–19 sekunder mot 9–10 ved 30 000 tokens.

I containeren fikk verken Ollama eller `llama-server` plass til de 30 000 tokenene `doctor` tester med. Det er ventet med 6 GB, og det er grunnen til at `setup` nå kan lagre oppsettet likevel (se under).

## Hva du bør vite

- **Ingen målinger på Linux eller NVIDIA.** Tallene over er fra én Mac. Vi vet ikke hvor rask modellen er på en Linux-laptop eller et vanlig grafikkort.
- **Lite grafikkminne gir lite plass til kontekst.** Modellen og konteksten må få plass sammen. Får serveren ikke plass til de 30 000 tokenene `doctor` tester med, feiler kontekstsjekken. Melder serveren at den mangler minne, eller slutter den å svare, foreslår `doctor` en kontekst som får plass, for eksempel 16 384, og hvordan du legger deler av modellen i vanlig minne (`--n-cpu-moe 999` for `llama-server`). Med den mindre konteksten feiler kontekstsjekken fortsatt, men er den den eneste som feiler, spør `setup` om du vil lagre likevel. Korte prompter virker da, men en Copilot- eller opencode-økt får ikke plass. Se [Lite minne](/nav-pilot/lokal/egen-server#lite-minne).
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
- [Stop the apt snippet when the keyring download fails](https://github.com/navikt/copilot/pull/1122) (navikt/copilot, 28. september 2026)
- [Setup saves a short context on request, and names an OOM](https://github.com/navikt/copilot/pull/1130) (navikt/copilot, 28. september 2026)
- [apt-get -y for scripted installs, libgomp1 for llama-server](https://github.com/navikt/copilot/pull/1134) (navikt/copilot, 28. september 2026)
- [Doctor advises a context that fits on small GPUs](https://github.com/navikt/copilot/pull/1139) (navikt/copilot, 28. september 2026)
- [One install command per OS on the landing and cplt pages](https://github.com/navikt/copilot/pull/1146) (navikt/copilot, 28. september 2026)
- [nav-pilot's own-endpoint path on real servers](https://github.com/navikt/mlx-workspace/blob/main/reports/2026-09-28-local-endpoint-validation/report.md) (navikt/mlx-workspace, 28. september 2026)
- [Linux smoke test of nav-pilot's local endpoint path](https://github.com/navikt/mlx-workspace/blob/main/reports/2026-09-27-linux-smoke/report.md) (navikt/mlx-workspace, 27. september 2026)
