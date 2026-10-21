---
name: aksel-spacing
description: Kompatibilitetsalias for eksisterende henvisninger til aksel-spacing. Bruk aksel-builder for nye oppgaver om Aksel-layout, spacing og responsive primitives.
license: MIT
metadata:
  domain: frontend
  tags: aksel compatibility spacing
---

# Aksel spacing compatibility alias

This skill preserves existing references to `aksel-spacing`; it has no separate layout
rules. For new tasks, select **`aksel-builder`** directly.

1. Load and follow the `aksel-builder` skill.
2. Read its `references/primitives-layout.md` for responsive layout and common recipes,
   and `references/tokens-styling.md` for spacing, CSS variables, and Tailwind.
3. Verify version-sensitive props and tokens through the Aksel MCP and installed packages.
   If the MCP is unavailable, use the builder skill's Preflight fallback.
4. For upgrades, use the builder's `references/migrations.md` and discover available
   codemods with `aksel_find_docs({ kind: "migrations" })`; don't infer codemod names.

Install this alias together with `aksel-builder`. If the builder skill cannot be loaded,
report the missing dependency instead of recreating a stale token or prop catalog here.
