---
title: "cplt sikkerhetsoppdatering — Bubblewrap-namespaces, proxy-forced og upstream proxy"
date: 2026-07-05
author: starefossen
category: praksis
excerpt: "En bølge med sikkerhetsherding i cplt: namespace-isolasjon på Linux, opt-in kernel-tvunget proxy og støtte for bedriftsproxy — uten å overselge hva lagene faktisk garanterer."
tags:
  - cplt
  - security
  - sandbox
  - proxy
---

[cplt](https://github.com/navikt/cplt) — sandboxen for AI-kodingsagenter — har fått en runde med sikkerhetsherding og tre nye funksjoner. Som alltid: dette er lag i et forsvar-i-dybden-oppsett, ikke sølvkuler. [SECURITY.md](https://github.com/navikt/cplt/blob/main/SECURITY.md) beskriver den ærlige trusselmodellen.

---

## Bubblewrap namespace-isolasjon (Linux)

På Linux kan cplt nå pakke agenten inn i [Bubblewrap](https://github.com/containers/bubblewrap)-namespaces (PID/IPC/UTS/cgroup/user) med en privat `/tmp` — **oppå** Landlock + seccomp-BPF, ikke som erstatning. Agenten kan ikke lenger se eller signalisere prosesser på verten, og `/tmp` er en fersk tmpfs uten exec.

Aktiveres automatisk når `bwrap` er installert, med graceful fallback til kun Landlock + seccomp hvis ikke. Eksplisitt styring med `sandbox.use_bubblewrap`:

```sh
cplt config set sandbox.use_bubblewrap true   # hard krav — feiler om bwrap mangler
cplt config set sandbox.use_bubblewrap false  # kun Landlock + seccomp
```

Viktige avgrensninger, med vilje:

- **Ikke en nettverksgrense.** Det opprettes ingen network namespace — vertsnettverket deles slik at agenten når CONNECT-proxyen på `127.0.0.1`. Nettverkskontrollen ligger fortsatt hos proxyen og Landlock-portregler.
- **Ikke full filsystem-konfidensialitet.** Hele rotfilsystemet bind-mountes read-only og er synlig i mount-tabellen; det er Landlock (deny-by-default) som nekter lesing, ikke mount-namespacet.

Kilde: [navikt/cplt#64](https://github.com/navikt/cplt/pull/64)

---

## Proxy-forced modus (opt-in)

Til nå har proxyen vært rådgivende på kernel-nivå: sandboxen tillater utgående `*:443`, og trafikken går gjennom proxyen fordi cplt injiserer `HTTPS_PROXY`. En rå socket — eller `env -u HTTPS_PROXY` — kunne dermed nå nettet utenom domenefiltreringen.

`proxy.forced` lukker den bypassen: proxyen blir obligatorisk, og kernel-egress begrenses til proxy-porten alene (ingen direkte `*:443`). Feiler proxyen ved oppstart, starter ikke agenten — fail-closed.

```sh
cplt config set proxy.forced true
```

Håndhevingen er asymmetrisk mellom plattformene:

- **macOS** pinner fullt til `localhost:<proxy_port>` — ingen restkanal.
- **Linux** dropper `*:443`-regelen, men Landlock er portbasert og kan ikke pinne til localhost, så en smal portbasert restkanal (`evil.com:<proxy_port>`) gjenstår. Dette er en kjent og bevisst begrensning, sporet oppstrøms i [navikt/cplt#114](https://github.com/navikt/cplt/issues/114).

Av som standard. Kilde: [navikt/cplt#117](https://github.com/navikt/cplt/pull/117)

---

## Upstream proxy-chaining for bedriftsnettverk

Team bak en bedriftsproxy har til nå måttet skru av cplt-proxyen. Med `proxy.upstream` (i merge-køen) videresender cplt CONNECT-tunneler gjennom bedriftsproxyen i stedet — og beholder egen domenefiltrering, logging og portsjekker. Policyen håndheves **før** videresending: et blokkert domene når aldri bedriftsproxyen.

```sh
cplt config set proxy.upstream "http://corporate-proxy.example.com:8080"
```

Basic-auth i URL-en støttes; kun `http`-skjema mot upstream.

Kilde: [navikt/cplt#125](https://github.com/navikt/cplt/pull/125)

---

## Andre forbedringer i samme bølge

- **Landlock best-effort ABI** — sandboxen bruker beste tilgjengelige Landlock-ABI på eldre kjerner i stedet for å kreve nyeste ([#119](https://github.com/navikt/cplt/pull/119))
- **Guard- og sikkerhetsfikser** — tetting av gh/git guard-bypasses og parsing-feil, pluss en runde med fikser for loopback-SSRF, trust pinning og en token-lekkasje ([#118](https://github.com/navikt/cplt/pull/118))
- **Verktøystier fra miljøvariabler** — `GOPATH` og tilsvarende respekteres for ikke-standard plasseringer (på vei, [#124](https://github.com/navikt/cplt/pull/124))

Se [cplt-siden](/cplt) for oppdatert funksjonsoversikt og interaktiv config-utforsker.
