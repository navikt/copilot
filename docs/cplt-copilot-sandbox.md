# Copilots lokale sandkasse inni cplt på Linux

Start Copilot med `cplt` fra prosjektet, eller med `nav-pilot copilot` hvis du bruker nav-pilot. Ikke aktiver Copilots `/sandbox` inni cplt. cplt håndhever allerede sandkassen rundt hele agentprosessen og barna dens. nav-pilot er ikke nødvendig for å starte cplt.

```sh
cplt
# Alternativt, med nav-pilot:
nav-pilot copilot
```

## Installert bwrap betyr ikke at en ny sandkasse kan starte

[GitHubs Linux-krav](https://docs.github.com/en/copilot/how-tos/cloud-and-local-sandboxes/using-local-sandboxing#prerequisites) er `bwrap` >=0.5.0 og `slirp4netns` på PATH. Dette er nødvendige verktøy, ikke en garanti for at prosessen får opprette namespaces.

cplt bruker Landlock og seccomp-BPF på Linux, med bubblewrap som et ekstra isolasjonslag når det brukes. [seccomp-filteret](https://github.com/navikt/cplt/blob/22656614c9224d82d2baade3b599f561239052f9/src/sandbox_landlock.rs#L2299-L2371) avviser blant annet `unshare`, `setns` og `mount` med `EPERM`. Filteret gjelder også uten det ytre bubblewrap-laget. Med bubblewrap påfører cplt [filteret inne i de ferdig opprettede namespaces](https://github.com/navikt/cplt/blob/22656614c9224d82d2baade3b599f561239052f9/src/sandbox_bubblewrap.rs#L1714-L1720), før agenten starter.

Dermed kan bwrap være installert og virke i vanlig terminal, men likevel feile inni cplt med `No permissions to create a new namespace`. En melding fra Copilot om manglende bubblewrap er ikke alene bevis for at pakken mangler eller er for gammel. Ikke installer på nytt eller endre kernelinnstillinger ut fra den meldingen alene. Ikke slå av seccomp eller namespace-begrensninger for å få den indre sandkassen til å starte.

## Hva dagens cplt gjør ved oppstart

cplt støtter ikke denne indre Copilot-sandkassen. For Copilot setter [oppstartskoden](https://github.com/navikt/cplt/blob/22656614c9224d82d2baade3b599f561239052f9/src/sandbox_exec.rs#L276-L283) `COPILOT_CLI_SANDBOX_SUPPORT_OVERRIDE=unsupported` i barnets miljø. Det skal få Copilot til å hoppe over støtteproben og la sin egen sandkasse være av for økten. cplt endrer ikke den lagrede `sandbox.enabled`-innstillingen.

Copilot kan da si at kommandoer kjører «unsandboxed». Det beskriver Copilots eget isolasjonslag, ikke cplt-laget rundt prosessen. cplt er fortsatt sikkerhetsgrensen. Du får ikke Copilots ekstra, smalere isolasjon av enkeltkommandoer inni den.

Oppstartsmekanismen ble [verifisert med Copilot CLI 1.0.83 i cplt PR #520](https://github.com/navikt/cplt/pull/520), merget 15. september 2026. Det er historisk verifikasjon, ikke en ny test av alle nyere klientversjoner. Dagens kode har fortsatt mekanismen. Et eksplisitt `--pass-env COPILOT_CLI_SANDBOX_SUPPORT_OVERRIDE` hindrer cplt i å sette verdien, men gir ikke prosessen tillatelse til å opprette namespaces. Ikke bruk det som løsning på denne feilen.

[GitHub dokumenterer](https://docs.github.com/en/copilot/how-tos/cloud-and-local-sandboxes/using-local-sandboxing#using-local-sandboxing-in-copilot-cli) at `/sandbox` åpner konfigurasjonen, og at `/sandbox enable` aktiverer sandkassen også for fremtidige økter. En administrert policy kan kreve Copilots lokale sandkasse. Ikke omgå et slikt krav med flagg, miljøvariabler eller konfigurasjonsendringer. Hvis policyen krever den indre sandkassen, må kompatibiliteten avklares med policyansvarlig og GitHub før denne startflyten kan brukes.

## Hva som er verifisert, og hva som fortsatt er åpent

Kildegjennomgangen 13. november 2026 brukte cplt-revisjon `22656614c9224d82d2baade3b599f561239052f9`.

| Evidens | Hva den viser |
| --- | --- |
| seccomp-filteret og oppstartskoden over | cplt avviser de nødvendige operasjonene og unngår Copilots indre sandkasse ved vanlig oppstart. |
| [Eksisterende Linux-test `bwrap_seccomp_still_blocks_unshare`](https://github.com/navikt/cplt/blob/22656614c9224d82d2baade3b599f561239052f9/tests/integration_linux.rs#L3582-L3601) | Testen krever at `unshare` fortsatt avvises inni det ytre bubblewrap-laget. |
| [Eksisterende unit-test](https://github.com/navikt/cplt/blob/22656614c9224d82d2baade3b599f561239052f9/tests/unit_tests.rs#L7547-L7577) og [E2E-tester](https://github.com/navikt/cplt/blob/22656614c9224d82d2baade3b599f561239052f9/tests/e2e.rs#L1305-L1331) | Override er avgrenset til Copilot; E2E-testene kontrollerer verdien i barnets miljø og eksplisitt pass-through. |
| [Rapporten i #2176](https://github.com/navikt/copilot/issues/2176) | Brukeren rapporterer bwrap 0.11.1 og namespace-avslag bare inni cplt. Dette er forenlig med policyen, men brukerens eksakte forløp er ikke reprodusert. |

Linux-testene er lest, ikke kjørt som del av denne oppfølgingen. Verifikasjonsmiljøet var macOS uten et klargjort, isolert Linux-testmiljø. Det er ikke kjørt bwrap på hosten, brukt privilegerte containere eller endret kernelinnstillinger.

Hvis vanlig oppstart med dagens cplt fortsatt gir bubblewrap-meldingen, trengs Linux-distribusjon og kernelversjon, `cplt --version`, `copilot --version`, startkommando uten tokens og noen sladdede linjer rundt feilen. Oppgi også om `--pass-env COPILOT_CLI_SANDBOX_SUPPORT_OVERRIDE` brukes, og om klienten viser et krav fra administrert policy. Dette skiller eldre oppstartskode, eksplisitt pass-through og klient-/policyendringer uten å kreve full doctor-output.

Andre permissions-feil må undersøkes ut fra den konkrete operasjonen som feiler. Denne forklaringen bekrefter ikke at de er løst.
