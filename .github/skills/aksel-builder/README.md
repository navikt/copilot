# aksel-builder

An agent skill that turns any MCP-capable coding assistant into an expert builder for the
**Aksel design system**. It leads the model to the right component, token,
and primitive — and backs every choice with the **Aksel MCP server** instead of guessing.

## What it covers

- **Components** — choosing the right one (`Button`, `TextField`, `Modal`, `Table`, …) and
  composing forms, overlays, and compound components.
- **Layout primitives** — `Box`, `HStack`/`VStack`, `HGrid`, `Page`, `Bleed`, `Show`/`Hide`,
  and the responsive prop system.
- **Tokens & styling** — token-props vs `--ax-*` CSS variables, color roles, and the
  `@navikt/ds-tailwind` preset.
- **Theming** — light/dark mode and base color via the `Theme` component.
- **Icons** — finding icons and the `name` → `${name}Icon` import mapping.
- **Accessibility** — the props Aksel requires for WCAG-compliant output.
- **Setup & migrations** — packages, imports, SSR/RSC, and version codemods.
- **Figma → code** — translate a Figma design into Aksel components and tokens (needs a Figma MCP).

## How it works

**MCP-first, anti-hallucination.** Aksel's API shifts between major versions, so the skill
never generates imports, props, tokens, or icon names from memory. It verifies each against
the live MCP tools (`aksel_find_docs`, `aksel_get_doc`, `aksel_get_component_info`,
`aksel_get_token_details`, `aksel_find_icons`) before writing code.

**Progressive disclosure.** `SKILL.md` is the always-on entry point (mission, MCP rules,
tool matrix, and a decision tree). The ten `references/` files load **on demand** — only
when a task touches their domain — keeping context small.

**Decision tree → reference + tools.** The model orients once (Is the MCP available? Which
Aksel version?), then each task branch maps to one reference file plus the tools to call.

**Works without the MCP.** If the `aksel_*` tools aren't installed, the skill recommends
adding them, then falls back to fetching the same docs over HTTP from
`https://aksel.nav.no/llm.md`.

## Structure

```
aksel-builder/
├── SKILL.md                  # entry point: rules, tool matrix, decision tree
└── references/
    ├── mcp-workflow.md       # tool mechanics, recovery, anti-hallucination
    ├── setup-and-imports.md  # packages, imports, Provider, SSR, CSS
    ├── primitives-layout.md  # Box/Stack/HGrid/Page/Bleed, spacing & radius
    ├── tokens-styling.md     # token-props vs --ax- vars, roles, Tailwind
    ├── theming.md            # Theme component, light/dark, data-color
    ├── components.md         # component selection, form & composition
    ├── figma-to-code.md      # Figma → Aksel (Code Connect, mapping, validation)
    ├── icons.md              # finding icons, name → ${name}Icon, a11y
    ├── accessibility.md      # required a11y props, structure, language
    └── migrations.md         # codemods, v7→v8 highlights
```

## Context cost

Token estimates (`~4 chars/token`; markdown with tables/code, so ±10%):

| Tier                   | Loads                          | ~Tokens |
| ---------------------- | ------------------------------ | ------- |
| Idle (until triggered) | frontmatter `description` only | ~240    |
| Triggered, no refs     | full `SKILL.md`                | ~3,200  |
| Typical task           | `SKILL.md` + 1–2 references    | ~5–7k   |
| All references loaded  | `SKILL.md` + all 10 references | ~19,000 |

The skill costs ~240 tokens until a UI task triggers it, and most tasks pull only one or two
references — the all-in figure is a ceiling, not the norm.

## Requirements

- The **Aksel MCP server** (`https://aksel-mcp.nav.no/mcp`) for full capability. Without it,
  the skill degrades gracefully to fetching `https://aksel.nav.no/llm.md`.
- Targets Aksel **v8+**
