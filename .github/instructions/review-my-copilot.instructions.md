---
applyTo: "apps/my-copilot/**/*.tsx,apps/my-copilot/**/*.ts,apps/my-copilot/**/*.md,apps/my-copilot/**/*.mdx"
excludeAgent: "cloud-agent"
---

# Review: my-copilot UI and text

Review-only guidance.

- **Accessible names**: Links, buttons, tables and landmarks that repeat on a page need unique accessible names (`aria-label`, visually hidden text). Flag "Les mer" repeated without context. Charts and diagrams need a text alternative.
- **Headings**: Heading levels follow the document outline with no skipped level. One `h1` per page.
- **Contrast**: Text at least 4.5:1 (3:1 for large text). Meaningful graphics and UI components at least 3:1 (WCAG 1.4.11). Flag colour as the only signal.
- **Language**: Use "header", not "hode" (ORDBOK, #2561). Use "KI", not "AI", except proper names and vendor terms. Write "AI Credits", not "AI-kreditter" (ORDBOK).
- **Links and anchors**: Flag moved or removed routes without a permanent redirect, and `#anchor` links whose target id no longer exists.
- **Readable by people**: Plain language, short sentences, no filler or AI tells. Flag examples that return no results or name the wrong owner.
