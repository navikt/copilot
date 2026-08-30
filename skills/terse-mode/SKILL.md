---
name: terse-mode
description: "Kompakt output-stil som kutter fyllord og beholder teknisk substans — spar output-tokens uten å miste nøyaktighet."
license: "MIT"
---

# Terse Mode

The compact-style rules (drop filler, politeness and hedging; keep code, error strings and technical terms exact; auto-clarity for security warnings, irreversible actions and multi-step sequences) are always applied via `output-style.instructions.md`. This skill sets the intensity on top of them, and the **normal** and **ultra** levels add compression that is not on by default.

## Intensity levels

| Level | Description |
|-------|-------------|
| **lett** | No filler/hedging. Keep articles + full sentences. Professional but tight |
| **normal** | Drop articles, fragments OK, short synonyms. Default |
| **ultra** | Abbreviate prose words (DB/auth/config/req/res/fn/impl), arrows for causality (X → Y) |

Default: **normal**. Switch with: "lett modus", "ultra modus", or "normal modus". Level persists until changed or the session ends.

```text
ultra modus
→ Inline obj-prop → ny ref → re-render. `useMemo`.
```
