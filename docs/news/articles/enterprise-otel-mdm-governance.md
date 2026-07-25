---
title: "Sentralstyrt OpenTelemetry og MDM-distribusjon for Copilot"
date: 2026-07-08
category: copilot
excerpt: "Enterprise-administratorer kan nå styre OpenTelemetry-eksport og Copilot-innstillinger via MDM, server-managed settings eller konfigurasjonsfiler — for VS Code og CLI."
url: "https://github.blog/changelog/2026-07-08-enterprise-managed-opentelemetry-export-for-vs-code-and-cli"
tags:
  - enterprise-controls
  - observability
  - mdm
  - governance
---

## Sentralstyrt OpenTelemetry-eksport

Organisasjoner kan nå bestemme hvor GitHub Copilot sender OpenTelemetry-data (OTel) — slik at telemetri flyter til en godkjent collector uten at hver utvikler trenger å sette `OTEL_*`-miljøvariabler manuelt.

Konfigurasjonen leveres gjennom en `telemetry`-blokk i enterprise-managed settings og gjelder for både Copilot Chat-utvidelsen i VS Code og agentvertsprosessen som driver Copilot CLI.

Administratorer kan styre:

- OTLP-endepunkt og transportprotokoll (`otlp-http` eller `otlp-grpc`)
- OTel-tjenestenavn og ressursattributter
- Eksportør-headere (f.eks. autentiseringstoken for collectoren)
- Om prompt-, respons- og verktøyinnhold skal fanges — og om utviklere kan endre dette

En sentralstyrt verdi «vinner» alltid over miljøvariabler og brukerinnstillinger.

## MDM-distribusjon av Copilot-innstillinger

Enterprise-administratorer kan nå distribuere sentralstyrte Copilot-innstillinger direkte til enheter gjennom tre kanaler:

- **Native MDM** — Microsoft Intune, Jamf eller Group Policy (Windows Registry / macOS managed preferences)
- **Server-managed** — innstillinger løst fra den innloggede GitHub-kontoen
- **Filbasert** — `managed-settings.json` deployet med Chef, Puppet, Ansible eller lignende

Innstillingene gjelder konsistent på tvers av VS Code og Copilot CLI, uavhengig av hvordan utvikleren logger inn.

**Kilder:**

- [Enterprise-managed OpenTelemetry export for VS Code and CLI](https://github.blog/changelog/2026-07-08-enterprise-managed-opentelemetry-export-for-vs-code-and-cli) (GitHub Changelog, 8. juli 2026)
- [Deploy managed Copilot settings via MDM in VS Code and CLI](https://github.blog/changelog/2026-07-08-deploy-managed-copilot-settings-via-mdm-in-vs-code-and-cli) (GitHub Changelog, 8. juli 2026)
