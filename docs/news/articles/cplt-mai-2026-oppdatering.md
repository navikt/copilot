---
title: "cplt oppdatering — proxy som standard, Linux-sandbox og credential-beskyttelse"
date: 2026-05-06
category: praksis
excerpt: "Fire endringer som gjør cplt tryggere og mer portabel: proxy som standard, granulær logging, Linux-støtte og credential-beskyttelse."
tags:
  - cplt
  - security
  - sandbox
  - proxy
---

## Hva er cplt?

[cplt](https://github.com/navikt/cplt) sandboxer AI-kodingsagenter med OS-primitiver — macOS Seatbelt og Linux Landlock + seccomp-BPF. All tilgang til filer, nettverk og prosesser styres av kjernen, ikke av agenten selv.

Nøkkelfunksjoner:

- **Filsystem-sandbox** — agenten ser bare prosjektmappa og eksplisitt tillatte stier
- **Nettverksproxy** — all utgående trafikk filtreres, telemetri og analytics blokkeres
- **Credential-beskyttelse** — SSH-nøkler, `.env`-filer og registry-tokens er utilgjengelige
- **Multi-agent** — støtter Copilot CLI, OpenCode og generiske shell-agenter
- **Konfigurerbar** — én TOML-fil per prosjekt, alt kan overstyres per kjøring

```bash
brew install navikt/tap/cplt
cplt -- -p "fix the tests"
```

Se [cplt-dokumentasjonen](/cplt) for interaktiv config-utforsker, nettverksdiagram og fullstendig funksjonsoversikt.

---

## Hva er nytt (mai 2026)

Fire endringer denne uka gjør cplt tryggere og mer portabel.

---

## Proxy aktivert som standard

CONNECT-proxyen er nå på som standard. Tidligere måtte du eksplisitt slå den på med `--proxy`. Proxyen binder til en ephemeral port (OS velger ledig port), så det er ingen konflikter med andre tjenester.

Hva dette betyr i praksis:

- All utgående trafikk fra sandboxed agents går gjennom proxyen
- Blokkerte domener (telemetri, analytics) stoppes automatisk
- Private IP-adresser blokkeres med mindre du eksplisitt tillater dem

Slå av med `--no-proxy` eller permanent:

```bash
cplt config set proxy.enabled false

# Eller: bruk fast port i stedet for ephemeral
cplt config set proxy.port 8888
```

**Kilde:** [feat: enable proxy by default with ephemeral port](https://github.com/navikt/cplt/commit/1686f7a) (cplt, 2026-04-30)

---

## Proxy log-nivå erstatter global quiet-flag

Proxy-logging til stderr er nå styrt av et eget `log_level`-felt, uavhengig av `sandbox.quiet`. Standard er `none` — ingen proxy-output til terminalen.

Tilgjengelige nivåer:

| Nivå      | Hva som logges                       |
| --------- | ------------------------------------ |
| `none`    | Ingenting til stderr (standard)      |
| `error`   | DNS-feil, connect-feil               |
| `blocked` | Feil + blokkerte forbindelser        |
| `all`     | Alt inkludert vellykkede tilkoblinger |

Audit-loggen (`proxy.log_file`) skriver alltid alt, uavhengig av nivå.

```bash
cplt config set proxy.log_level blocked
cplt config set proxy.log_file ~/.config/cplt/proxy.log
```

**Kilde:** [feat(proxy): add log_level config](https://github.com/navikt/cplt/commit/f4f56ea) (cplt, 2026-05-05)

---

## Plattformagnostisk sandbox-API (Linux-støtte)

cplt har fått et felles sandbox-API som abstraherer bort plattformforskjeller. Samme config gir kernel-enforced sandbox på både macOS (Seatbelt) og Linux (Landlock LSM + seccomp-BPF).

Nye public API-funksjoner:

- `SandboxConfig` — deklarativ policy uavhengig av OS
- `prepare()` → `PreparedSandbox` — genererer plattformspesifikk policy
- `exec_sandboxed()` — kjører kommando med kernel-enforced sandbox
- `preflight()` — sjekker om OS-primitiver er tilgjengelige

Linux-enforcement krever kernel 5.13+ (Landlock v1). GitHub Actions Ubuntu runners støtter dette. Seccomp-BPF legger til syscall-filtrering som ekstra lag.

CI kjører nå integrasjonstester på begge plattformer: 37 macOS + 24 Linux kernel-tester.

**Kilde:** [refactor: introduce platform-agnostic sandbox API (#19)](https://github.com/navikt/cplt/commit/0e116aa) (cplt, 2026-05-05)

---

## Credential-filer blokkert som standard

Credential-filer for package managers er nå blokkert inne i sandboxen, selv om parent-katalogen er tillatt for lesing:

- `~/.m2/settings.xml` og `settings-security.xml` (Maven)
- `~/.gradle/gradle.properties` (Gradle)
- `~/.cargo/credentials` og `credentials.toml` (Cargo)

En ondsinnet agent som har lesetilgang til `~/.m2/` (for å resolve dependencies) kan ikke lenger lese registry-passord eller tokens fra disse filene.

Override for utviklere som trenger private registries:

```bash
cplt config set allow.read '["~/.m2/settings.xml"]'
```

Eller per kjøring: `cplt --allow-read ~/.m2/settings.xml`

**Kilde:** [feat: deny registry credential files inside allowed tool dirs](https://github.com/navikt/cplt/commit/6da3b36) (cplt, 2026-05-05)


