---
name: deliberate-ai-use
description: "Bevisst AI-bruk i Nav: grønn og rød sone, tre-forsøks-regelen, erfaringsnivå og generer-så-forstå, med forskningen bak. Bruk når utvikleren spør om AI bør gjøre en oppgave, vil lære, eller når en oppgave skal plasseres i grønn eller rød sone."
license: "MIT"
---

# Deliberate AI Use

The always-on rules (explain choices, mark red-zone code, ask before delegating fully) are in `deliberate-ai-use.instructions.md`. This skill holds the rest of the framework: why it exists, how to classify a task, and how a developer works with generated code.

## Why

Research shows that *how* you use AI matters more than *whether* you use it. Developers who delegate blindly score 35–39 % on comprehension. Those who actively ask questions after code generation score 86 %, higher than those who code without AI at all (67 %).

The framework makes sure AI tools strengthen developers' skills instead of weakening them.

**Sources:**

- [Anthropic: How AI assistance impacts coding skills](https://www.anthropic.com/research/AI-assistance-coding-skills) (2026)
- [INNOQ: AI Coding Patterns Through Cognitive Load Theory](https://www.innoq.com/en/blog/2026/03/ai-cognitive-lens-cognitive-load-theory/) (2026)
- [METR: AI experienced OS dev study](https://metr.org/blog/2025-07-10-early-2025-ai-experienced-os-dev-study/) (2025)
- [Stray et al.: Developer Productivity With and Without GitHub Copilot](https://arxiv.org/abs/2509.20353) (HICSS-59, 2026), a study of Nav IT
- [MIT/Microsoft: The Effects of Generative AI on High-Skilled Work](https://economics.mit.edu/sites/default/files/inline-files/draft_copilot_experiments.pdf) (2025)
- Nav developer survey 2026: 59 % are worried about losing skills

## Green and red zone

Classify a task before using AI on it.

### 🟢 Green zone: suited to AI

Tasks where AI adds the most value without weakening understanding:

- Boilerplate and repetitive code (Nais manifest, CRUD endpoints, Dockerfile)
- Technology the developer already knows well
- Configuration and infrastructure
- Refactoring with a known goal (rename, extract, move)
- Test data and fixtures

### 🔴 Red zone: code by hand first

Tasks where coding by hand builds critical skill:

- **Debugging**: troubleshooting is the strongest learning mechanism
- **New concepts**: technology the developer has not used before (learn first, generate afterwards)
- **Core logic**: business rules, calculations, state machines
- **Security-critical code**: authentication, authorization, input validation
- **Architecture decisions**: system design, data models, API contracts

**Three-attempt rule:** try to solve the problem yourself, with at least three attempts (approaches), before asking AI for help. Each attempt builds understanding that makes you better able to judge the AI's suggestions.

**Experience level:** junior developers should keep more in the red zone. Research shows they gain the most productivity from AI, but are also the most vulnerable to losing skills. Experienced developers can have a wider green zone for technology they already know well.

## Generate-then-understand

When AI generates code, do not just accept it. Use the generate-then-understand pattern:

1. **Generate**: let the AI write the code
2. **Understand**: ask *why* the code is written the way it is
3. **Verify**: check that you can explain every part yourself
4. **Adapt**: make deliberate changes, not just copy-paste

### Good follow-up questions

Offer these to the developer as written, in Norwegian:

```text
Hvorfor valgte du denne tilnærmingen fremfor alternativene?
Hva kan gå galt med denne koden?
Hvilke edge cases dekker den ikke?
Forklar tradeoffene i denne designbeslutningen
```
