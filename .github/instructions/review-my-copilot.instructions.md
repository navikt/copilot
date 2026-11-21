---
applyTo: "apps/my-copilot/**/*.tsx,apps/my-copilot/**/*.ts,apps/my-copilot/**/*.md,apps/my-copilot/**/*.mdx"
excludeAgent: "cloud-agent"
---

# Review: my-copilot UI and text

Review-only guidance.

- **Aksel**: Prefer Aksel components and spacing tokens over custom markup and Tailwind `p-*`/`m-*`. Exception: `/cplt` stays English with its own dark design.
- **Accessible names**: Links, buttons, tables and landmarks that repeat on a page need unique accessible names (`aria-label`, visually hidden text). Flag "Les mer" repeated without context. Charts and diagrams need a text alternative.
- **Headings**: Heading levels follow the document outline with no skipped level. One `h1` per page.
- **Contrast**: Text and meaningful graphics at least 4.5:1 (3:1 for large text and UI components). Flag colour as the only signal.
- **Language**: Norwegian bokmål follows `apps/my-copilot/ORDBOK.md`. Use "header", not "hode". Use "KI", not "AI", except proper names and vendor terms.
- **Links and anchors**: Flag moved or removed routes without a permanent redirect, and `#anchor` links whose target id no longer exists.
- **Readable by people**: Plain language, short sentences, no filler or AI tells. Flag examples that return no results or name the wrong owner.
- **Zones**: Zone 1 routes must not render user, billing or internal data.
