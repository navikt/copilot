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
- **Accessibility** — labels, semantics, focus, and checks for the complete user flow.
- **Setup & migrations** — packages, imports, SSR/RSC, and version codemods.
- **Figma → code** — translate a Figma design into Aksel components and tokens (needs a Figma MCP).

## How it works

**MCP-first, anti-hallucination.** Aksel's API shifts between major versions, so the skill
never generates imports, props, tokens, or icon names from memory. It verifies each against
MCP tools (`aksel_find_docs`, `aksel_get_doc`, `aksel_get_component_info`,
`aksel_get_token_details`, `aksel_find_icons`) and installed packages before writing code.
Docs/props are fetched live and cached; token, icon, and migration catalogs are bundled
with the MCP server. They are not automatically aligned with the consuming app's version.

**Progressive disclosure.** `SKILL.md` is loaded when the skill is invoked (mission, MCP rules,
tool matrix, and a decision tree). The ten `references/` files load **on demand** — only
when a task touches their domain — keeping context small.

**Decision tree → reference + tools.** The model orients once (Is the MCP available? Which
Aksel version?), then each task branch maps to one reference file plus the tools to call.

**Works without the MCP.** If the `aksel_*` tools aren't installed, the skill recommends
adding them, then falls back to fetching documentation over HTTP from
`https://aksel.nav.no/llm.md`. These pages do not replace structured token/icon catalogs;
check installed package exports and types when the prose is insufficient.

## Installation and entry points

Install the builder in the consuming repository with `nav-pilot`:

```sh
nav-pilot install aksel-builder --type skill --source navikt/copilot --repo
```

`aksel-builder` replaces the retired `aksel-spacing` skill. `nav-pilot sync --apply` removes
an unchanged `aksel-spacing` copy; install the builder in its place.

Installing an individual artifact does not install its companions. Install the builder
alongside any of these optional entry points:

| Artifact                              | Source                                                         | Role                                                  |
| ------------------------------------- | -------------------------------------------------------------- | ----------------------------------------------------- |
| `aksel` agent (`aksel-agent` in chat) | [Agent](../../agents/aksel.agent.md)                           | Routes Aksel tasks to this skill                      |
| `aksel-component` prompt              | [Prompt](../../prompts/aksel-component.prompt.md)              | Scaffolds a component using this workflow             |
| `nextjs-aksel` instruction            | [Instruction](../../instructions/nextjs-aksel.instructions.md) | Keeps Next.js guidance and Aksel integration together |

Use `--type agent`, `--type prompt`, or `--type instruction` when installing those artifact
names with the same source and scope flags. New layout tasks should select `aksel-builder`
directly. Configure Aksel MCP through the client as described in `SKILL.md` → Preflight.
Figma work also needs a connected Figma MCP server.

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

Size estimates measured from file character counts on 2026-10-21, divided by four.
Actual token counts depend on the model, language, and Markdown; these are not tokenizer
measurements. Tool responses and conversation history are excluded.

| Tier                   | Loads                          | ~Tokens   |
| ---------------------- | ------------------------------ | --------- |
| Idle (until triggered) | frontmatter `description` only | ~80       |
| Triggered, no refs     | full `SKILL.md`                | ~3,550    |
| One or two references  | `SKILL.md` + 1–2 references    | ~4.5–8.5k |
| All references loaded  | `SKILL.md` + all 10 references | ~20,700   |

Hosts may also include skill names and other metadata before invocation. Load only the
references needed for the task; the all-in figure is not the normal cost.

## Requirements

- The **Aksel MCP server** (`https://aksel-mcp.nav.no/mcp`) for full capability. Without it,
  the skill degrades gracefully to fetching `https://aksel.nav.no/llm.md`.
- Targets Aksel **v8+**
