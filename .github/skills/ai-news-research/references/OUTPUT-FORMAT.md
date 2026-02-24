# Output Format Reference

## Document structure

```markdown
# Nyheter og trender — [Måned] [År]

[One-sentence intro about what the month covered]

---

## 1. [Topic title]

[2-4 paragraphs explaining the news, what changed, and why it matters]

**Kilde:** [Title](https://exact-url) (source, date)

---

## 2. [Next topic]

...

---

## Relevans for Nav

[Table mapping trends to Nav-specific implications]

| Trend | Hva det betyr for Nav |
| ----- | --------------------- |
| ...   | ...                   |
```

## Source citation rules

- Every section MUST end with a **Kilde:** or **Kilder:** block
- Links must be real, verified URLs — never fabricated
- Single source:

```markdown
**Kilde:** [Title](https://exact-url) (source, date)
```

- Multiple sources — blank line between header and list:

```markdown
**Kilder:**

- [Title](https://url) (source, date)
- [Title](https://url) (source, date)
```

## Markdown linting

- Blank line before and after every list
- Table separators with spaces: `| --- | --- |` not `|---|---|`
- No trailing whitespace
