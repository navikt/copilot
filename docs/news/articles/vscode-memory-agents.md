---
title: "VS Code agents får memory og Copilot Memory"
date: 2026-06-10
category: copilot
excerpt: "VS Code dokumenterer local memory tool og Copilot Memory med user, repository og session scopes, plus auto-expiry og verifikasjon av fakta."
url: "https://code.visualstudio.com/docs/agents/memory"
tags:
  - vscode
  - memory
  - copilot-memory
  - agents
  - privacy
---

VS Code dokumenterte 10. juni hvordan agents bruker memory for å huske kontekst mellom samtaler. Memory tool er i preview og lagrer lokalt på maskinen din.

Dokumentasjonen deler memory i tre nivåer: user, repository og session. User memory er for personlige preferanser, repository memory for prosjektspesifikke regler, og session memory for midlertidig arbeidskontekst.

Copilot Memory er separat og GitHub-hosted. Den er repository-scoped, deles mellom Copilot-surfaces og blir verifisert mot kodebasen før bruk. Memories utløper automatisk etter 28 dager.

For team betyr dette mindre repetisjon og bedre gjenbruk av repo-kunnskap, men også behov for tydelig styring av hva som får bli lagret.
