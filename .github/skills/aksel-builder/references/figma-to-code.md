# Figma → Aksel code

Translate a Figma design into **Aksel-first** React code with high visual fidelity. The goal
is **not** to copy Figma's raw HTML output — it's to use Figma MCP as the design _source_, then
implement with real Aksel components, primitives, and tokens (per the rest of this skill).

Use when the user shares a Figma URL (`figma.com/design/:fileKey/...?node-id=1-2`) or asks to
"implement this design" / "build this from Figma". Don't use it for design critique without an
implementation request, or for Aksel work with no Figma reference.

## Prerequisite: the Figma MCP

This workflow needs the **Figma MCP server** — verify `get_design_context` is an available
tool. **If it's missing, stop**: tell the user this depends on a Figma MCP server, ask them to
configure one, and don't try to proceed from a screenshot alone. You also need a Figma URL in
the form `https://figma.com/design/:fileKey/:fileName?node-id=1-2`.

## Workflow (in order)

### 1. Fetch design context

Call `get_design_context` with `disableCodeConnect: false` (always — Code Connect gives the
highest-confidence component choices) and screenshots enabled.

- **Unmapped components are normal**, not an error. If Figma prompts to _create_ Code Connect
  mappings, silently skip it (authoring `.figma.ts` is an Aksel-team job, not the user's) and
  re-call `get_design_context` for the best available context.
- **Too large / truncated?** Use `get_metadata(fileKey, nodeId)` to map child nodes, pick the
  relevant child frame, and re-run `get_design_context` on that narrower node — then tell the
  user you narrowed scope. `get_metadata` is for navigation only, not a replacement.

### 2. Keep the screenshot

`get_design_context` **returns a screenshot inline** (there is no separate screenshot tool;
the only control is `excludeScreenshot` — don't set it). Treat that screenshot as the visual
source of truth for the verification step; save/keep it as `figma-visual-reference` if your
client lets you.

### 3. Assets

Use the asset URLs the `get_design_context` payload returns **directly** — don't invent
placeholders or add new icon packages. (The official Figma MCP returns remote
`figma.com/api/mcp/asset/...` URLs that expire in ~7 days; some variants serve `localhost`
URLs instead — either way, use what the payload gives.) For Aksel icons specifically, resolve
them through `aksel_find_icons` → `${name}Icon` ([icons.md](icons.md)).

### 4. Write Aksel code

`get_design_context` is **mixed-confidence** input — weight each part:

| Source               | Confidence | Use it for                                                                                                    |
| -------------------- | ---------- | ------------------------------------------------------------------------------------------------------------- |
| `CodeConnectSnippet` | Highest    | Preserve the **component choice** (a real DS-team mapping). Keep composition close; fix imports/props/wiring. |
| Screenshot           | High       | Layout hierarchy, spacing rhythm, visual intent.                                                              |
| Raw HTML / Tailwind  | Lower      | Structural hints only — adapt into Aksel; **never ship raw Figma HTML**.                                      |

> **A `CodeConnectSnippet` is not copy-pasteable as-is.** Its props are Figma's variant
> _property_ names, which differ from the real React API — e.g. the snippet
> `<Tag color="Info" variant="Outline">` must become `<Tag data-color="info" variant="outline">`
> (lowercase, and color moves to `data-color`). Keep the **component** the snippet chose, but
> **confirm and remap every prop** with `aksel_get_component_info`.

**Translate fallback HTML/Tailwind into Aksel** (don't guess the tree from CSS when the MCP
can confirm the intended abstraction):

- `flex-row` → `HStack`, `flex-col` → `VStack` ([primitives-layout.md](primitives-layout.md)).
- gap/padding values → token-**props** (`gap="space-8"`, `paddingInline="space-8"`), not raw
  Tailwind classes or `style` ([tokens-styling.md](tokens-styling.md)).
- `data-name` hints (`Button`, `Card`, `Row`, …) → look up the real component with
  `aksel_find_docs` + `aksel_get_component_info` before inventing markup
  ([components.md](components.md)).
- Map Figma color/size variables to real Aksel tokens via `aksel_get_token_details` — never
  hardcode Figma's hex/px.

**Props over `style`.** Figma emits raw CSS (font, color, padding, align…). Map each to the
Aksel component's matching prop/token (`padding="space-12"`, `data-color`, …), not a `style`
override.

### 5. Validate

**Phase 1 — code quality (don't skip).** Type errors usually render a blank page, making
visual checks useless. Before rendering: detect the project's scripts (`typecheck`, `lint`,
`build`; eslint/biome/prettier), type-check against the **app** tsconfig (not a composite root
`tsconfig.json` that only has `references`), lint/format the changed files, and fix **all**
errors before continuing.

**Phase 2 — visual parity (opt-in).** Confirm with the user before starting a dev server or
installing tooling. Prefer tooling the project already has (Playwright/Cypress/Puppeteer) or
native screen capture; **ask permission before installing anything**. Render, compare against
`figma-visual-reference` at a matching viewport, and fix the largest mismatches in a short
loop. If declined or no tooling is available, ask the user to share a screenshot. Don't claim
fidelity from reading code alone.

Watch: section widths/stretch, spacing rhythm, radius/border/shadow, type hierarchy &
wrapping, overflow/scroll, active/inactive states, token-exact colors, responsive behavior.

#### Checklist

- [ ] Type check passes (correct app tsconfig); lint/format pass on changed files.
- [ ] Majority of JSX is Aksel components; no raw Figma HTML or Tailwind left.
- [ ] No magic numbers — tokens for spacing/color/radius; props over `style` where a prop exists.
- [ ] Assets render (no broken placeholders).
- [ ] (if visual validation ran) layout, typography, and assets match the screenshot.
- [ ] Accessibility per [accessibility.md](accessibility.md).

## Common issues

- **Truncated output** → `get_metadata`, then fetch child nodes individually.
- **Mostly fallback HTML, few Code Connect mappings** → normal; translate via the rules above.
- **Code Connect returns a component** → preserve it unless the repo documents a different
  import path; it's often copy-pasteable after fixing imports/props/wiring.
- **Figma token ≠ Aksel token** → prefer the Aksel token for consistency; adjust spacing/size
  minimally to keep fidelity.

## Communication

Be concise and implementation-oriented. Note where the source was high-confidence (Code
Connect) vs inferred from fallback HTML. Ask clarifying questions only when the ambiguity would
change the shipped UI. Don't turn it into a design critique unless asked.
