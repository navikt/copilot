# AI Coding Agent Engagement Survey — Nav (2026)

## Background

Survey targeting ~500 technology professionals at Nav to understand how they use AI coding tools, what functionality they rely on, and where AI agents deliver (or fail to deliver) value. GitHub Copilot is the sanctioned AI coding tool at Nav, available in multiple environments. Designed to minimize survey fatigue (~5–8 minutes, 16 questions).

### Theoretical Foundation

- **SPACE Framework** (Forsgren, Storey, Maddila, Zimmermann & Noda, 2021) — Satisfaction, Performance, Activity, Communication, Efficiency. We implement four of five dimensions directly (S, P, A, E); Communication is covered indirectly through the peer review question.
- **Six Factors from "Beyond the Commit"** (Chen et al., ICSE-SEIP 2026) — Self-sufficiency, Cognitive load, Task completion, Peer review, Long-term expertise, Ownership. All six are mapped to Likert-scale questions (Q7–Q13).

### Design Principles

- Max 16 questions, 5–8 minutes completion
- Skip logic for non-users (Q1–Q3 + Q12–Q16: 6 required + 2 optional)
- Two optional open-ended questions
- Anonymous responses
- Share results back to participants

---

## Survey Questions

### Section 1: Profile (segmentation)

**Q1.** What is your primary role?

- Frontend developer
- Backend developer
- Full-stack developer
- Platform/infrastructure
- Operations/drift
- Data engineer/scientist
- Tech lead / architect
- Other: ___

**Q2.** How many years of experience do you have as a technology professional?

- 0–2
- 3–5
- 6–10
- 11+

**Q3.** Which Copilot environments do you currently use? *(select all that apply)*

- Copilot in VS Code (completions, chat, agent mode)
- Copilot in IntelliJ / JetBrains IDEs
- Copilot on github.com (PR summaries, code review, etc.)
- Copilot CLI (terminal)
- GitHub Copilot Extensions / MCP servers
- OpenCode (open-source terminal agent)
- Other: ___
- **I don't use Copilot or any AI coding tools** → *skip to Q12*

---

### Section 2: Usage Patterns (skip-logic gated — AI tool users only)

**Q4.** How often do you use AI coding tools in your development work?

- Multiple times per day
- Daily
- A few times per week
- Weekly or less

**Q5.** What do you primarily use AI coding tools for? *(select top 3)*

- Code completions / generating code
- Explaining or understanding existing code
- Writing tests
- Debugging / fixing errors
- Refactoring
- Writing documentation / comments
- Code review assistance
- Generating boilerplate / scaffolding
- Learning new languages, frameworks, or APIs
- Agentic tasks (multi-file changes, autonomous workflows)
- Other: ___

**Q6.** In which phase of development do AI tools help you most?

- Initial prototyping / getting started
- Core implementation
- Testing and validation
- Code review and iteration
- Maintenance and bug fixes
- Equally across phases

---

### Section 3: Impact — Satisfaction + The Six Factors (5-point Likert scale)

Response scale: Strongly disagree / Disagree / Neutral / Agree / Strongly agree

**Q7. Overall satisfaction (SPACE-S):** "Overall, I am satisfied with the AI coding tools available to me at Nav."

**Q8. Self-sufficiency:** "AI coding tools help me get unblocked and make progress independently, without needing to wait for help from colleagues."

**Q9. Cognitive load:** "AI coding tools reduce mental effort on repetitive or boilerplate tasks, freeing me to focus on harder problems."

**Q10. Task completion:** "AI coding tools help me complete tasks faster than I would without them."

**Q11. Peer review:** "Code I produce with AI assistance is of sufficient quality that it does not create extra burden during code review."

**Q12. Technical expertise:** "I am concerned that relying on AI tools may reduce my own deep understanding of the code and technologies I work with."

Note: reverse-scored — captures the long-term expertise concern.

**Q13. Ownership:** "I feel confident taking full responsibility for code that was generated or significantly assisted by AI."

---

### Section 4: Barriers & Opportunities (all respondents)

**Q14.** What is the biggest barrier to getting more value from AI coding tools? *(single choice)*

- Output quality / too many errors to trust
- Security and data sensitivity concerns
- Doesn't understand our codebase / internal frameworks well enough
- Lack of training / don't know how to use it effectively
- Slows me down more than it helps
- Company policy / tool access limitations
- I prefer to code without AI assistance
- Other: ___

**Q15.** *(Optional, open-ended)* What has been your most memorable experience — positive or negative — using AI coding tools?

**Q16.** *(Optional, open-ended)* Is there one thing that would make AI coding tools significantly more useful for your daily work?

---

## Complementary Methods

1. **Semi-structured interviews** — 5–8 developers across seniority and role for qualitative depth
2. **API metrics** — Correlate Copilot usage data (acceptance rates, active users) with survey sentiment
3. **Close the loop** — Share aggregated results and actions taken back to all respondents

## References

- Chen et al., "Beyond the Commit: Developer Perspectives on Productivity with AI Coding Assistants", ICSE-SEIP 2026 (arxiv.org/abs/2602.03593) — Source of the six-factor productivity framework. Q8–Q13 directly implement their factors.
- Forsgren, Storey, Maddila, Zimmermann & Noda, "The SPACE of Developer Productivity", ACM Queue, 2021 — Framework for measuring developer productivity across five dimensions. Q7 implements the Satisfaction dimension; Activity, Performance, and Efficiency are covered by Q4–Q6 and Q10.
- GitHub/Accenture Enterprise Copilot Study, github.blog, 2024 — Enterprise benchmarking context. Their approach of combining API metrics with developer surveys inspired our "Complementary Methods" section.
- Australian Government M365 Copilot Trial, digital.gov.au, 2024 — Survey design patterns (Likert scales, task-frequency grids, pre/post structure) adapted for our coding-specific context. Note: their trial was for M365 Copilot (office productivity), not coding tools.
- Stack Overflow Developer Survey 2025, survey.stackoverflow.co/2025/ — Benchmarking context for AI tool adoption rates and usage patterns. Q4 and Q5 are inspired by their AI section question design.
