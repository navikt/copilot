# AI Coding Agent Engagement Survey — Nav (2026)

## Background

Survey targeting ~500 technology professionals at Nav to understand how they use AI coding tools, what functionality they rely on, and where AI agents deliver (or fail to deliver) value. GitHub Copilot is the sanctioned AI coding tool at Nav, available in multiple environments. Designed to minimize survey fatigue (~5 minutes, 12 questions).

### Theoretical Foundation

- **SPACE Framework** (Forsgren, Storey, Maddila, Zimmermann & Noda, 2021) — Satisfaction, Performance, Activity, Communication, Efficiency. We implement four of five dimensions directly (S, P, A, E); Communication is covered indirectly through the peer review question.
- **Six Factors from "Beyond the Commit"** (Chen et al., ICSE-SEIP 2026) — Self-sufficiency, Cognitive load, Task completion, Peer review, Long-term expertise, Ownership. Five of six are mapped to Likert-scale questions (Q4–Q9). Self-sufficiency is merged into Task completion.

### Design Principles

- Max 12 questions, ~5 minutes completion
- Skip logic for non-users (Q1–Q2 + Q9–Q12: 5 required + 1 optional)
- One optional open-ended question
- Anonymous responses
- Share results back to participants

---

## Survey Questions

### Section 1: Profile (segmentation)

**Q1.** How many years of experience do you have as a technology professional?

- 0–2
- 3–5
- 6–10
- 11+

**Q2.** Which AI coding environments do you currently use? *(select all that apply)*

- Copilot in VS Code (completions, chat, agent mode)
- Copilot in IntelliJ / JetBrains IDEs
- Copilot on github.com (PR summaries, code review, etc.)
- Copilot CLI (terminal)
- GitHub Copilot Extensions / MCP servers
- Claude Code (Anthropic terminal agent)
- OpenCode (open-source terminal agent)
- Other: ___
- **I don't use any AI coding tools** → *skip to Q9*

---

### Section 2: Usage Patterns (skip-logic gated — AI tool users only)

**Q3.** In your day-to-day work, where have AI coding tools created the most value? *(select up to 3)*

- Code completions / generating code
- Explaining or understanding existing code
- Writing tests
- Debugging / fixing errors
- Refactoring
- Writing documentation / comments
- Code review assistance
- Generating boilerplate / scaffolding
- Learning new languages, frameworks, or APIs
- Delegating multi-step tasks to an autonomous agent
- Other: ___

---

### Section 3: Impact — Satisfaction + The Six Factors (5-point Likert scale)

Response scale: Strongly disagree / Disagree / Neutral / Agree / Strongly agree

**Q4. Overall satisfaction (SPACE-S):** "Overall, I am satisfied with the AI coding tools available to me at Nav."

**Q5. Cognitive load:** "AI coding tools reduce mental effort on repetitive or boilerplate tasks, freeing me to focus on harder problems."

**Q6. Task completion:** "AI coding tools help me get unblocked and complete tasks faster than I would without them."

**Q7. Peer review:** "Code I produce with AI assistance is of sufficient quality that it does not create extra burden during code review."

**Q8. Technical expertise:** "I am concerned that relying on AI tools may reduce my own deep understanding of the code and technologies I work with."

Note: reverse-scored — captures the long-term expertise concern.

**Q9. Ownership:** "I feel confident taking full responsibility for code that was generated or significantly assisted by AI."

**Q10. Legal/security friction:** "Uncertainty around data privacy (PII) or internal security rules prevents me from fully utilizing AI coding tools in my daily work."

Note: reverse-scored — high agreement signals a barrier the team can address through clearer guidelines and tooling.

---

### Section 4: Barriers & Opportunities (all respondents)

**Q11.** If you could change one thing about AI coding tools at Nav, what would it be? *(single choice)*

- Better understanding of our codebase and internal frameworks
- More training and guidance on effective use
- Clearer guidelines on security and data sensitivity
- Fewer tool access limitations
- Support for more AI tools or environments
- Nothing — I'm satisfied with the current state
- I prefer to code without AI assistance
- Other: ___

**Q12.** *(Optional, open-ended)* What has been your most memorable experience — positive or negative — using AI coding tools, and what one change would make them more useful?

---

## Complementary Methods

1. **Semi-structured interviews** — 5–8 developers, deliberately sampling across extremes: heavy agent users who have found effective workflows, non-users or skeptics, and senior developers with deep codebase knowledge. Explore themes too nuanced for the survey:
   - **Mastery & agency** — How do developers maintain a sense of mastery and professional growth when work shifts toward evaluating AI-generated code rather than writing it?
   - **Technical debt** — Does AI help reduce tech debt, or does it accelerate accumulation of harder-to-maintain code?
   - **AI fatigue** — Is the cognitive shift from writing code to reviewing and steering AI output energizing or draining?
   - **Development phase** — Where in the development lifecycle (design, implementation, testing, maintenance) do AI tools add or destroy value?
   - **Transplanting success** — What patterns, habits, and repo setups distinguish developers who get high value from agents, and how can we spread those?
2. **API metrics** — Correlate Copilot usage data (acceptance rates, active users, frequency) with survey sentiment. Usage frequency was cut from the survey since it's measurable from API data.
3. **Close the loop** — Share aggregated results and actions taken back to all respondents

## References

- Chen et al., "Beyond the Commit: Developer Perspectives on Productivity with AI Coding Assistants", ICSE-SEIP 2026 (arxiv.org/abs/2602.03593) — Source of the six-factor productivity framework. Q5–Q9 implement five of their six factors; self-sufficiency is merged into task completion (Q6). Results on these questions are directly comparable to Chen et al.'s findings.
- Forsgren, Storey, Maddila, Zimmermann & Noda, "The SPACE of Developer Productivity", ACM Queue, 2021 — Framework for measuring developer productivity across five dimensions. Q4 implements Satisfaction; Activity and Efficiency are covered by Q3 and Q6.
- GitHub/Accenture Enterprise Copilot Study, github.blog, 2024 — Enterprise benchmarking context. Their approach of combining API metrics with developer surveys inspired our "Complementary Methods" section.
- Australian Government M365 Copilot Trial, digital.gov.au, 2024 — Survey design patterns (Likert scales, task-frequency grids, pre/post structure) adapted for our coding-specific context. Note: their trial was for M365 Copilot (office productivity), not coding tools.
- Stack Overflow Developer Survey 2025, survey.stackoverflow.co/2025/ — Benchmarking context for AI tool adoption rates and usage patterns. Q3 draws on their AI section question design; adoption rates from Q2 can be compared against their global figures.
