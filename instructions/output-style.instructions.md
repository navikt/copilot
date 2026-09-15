---
applyTo: "**"
---

# Output Style

How output is written: chat answers, commit messages, PR descriptions, documentation and code comments. Never applies inside code blocks, program output or quoted error strings.

## Length

- No unrequested files. Do not add a README, summary document or test scaffold that was not asked for.
- No speculative scaffolding. Build what the task needs now, not what it might need later.
- Size the answer to the question. A one-line question gets a one-line answer.
- Lead with the answer, reasoning after. Never restate the question before answering it.

## Density

Drop these:

- Filler: "bare", "egentlig", "faktisk", "selvfølgelig", "simpelthen", "just", "really", "basically", "actually"
- Politeness: "gjerne", "med glede", "sure", "certainly", "happy to", "great question"
- Hedging: "kanskje", "muligens", "det kan hende", "maybe", "perhaps", "might"

Sentence fragments are fine. Prefer short synonyms: "fix", not "implement a solution for".

- One idea per sentence. If a sentence has to be read twice, split it.
- Short paragraphs, two to four sentences.
- Plain word over fancy word: "bruke" not "benytte", "use" not "utilise", "hjelpe" not "fasilitere", "start" not "commence".
- Cut adverbs. Use a stronger verb or the measured number instead: "runs quickly" becomes "is fast" or "runs in 40 ms"; "forbedrer betydelig" becomes the difference you measured.
- Active voice, and name the actor: "the compiler validates queries", not "queries are validated". Passive only when the actor is unknown or does not matter.
- Cut a sentence that only restates the one before it, and a closing paragraph that summarises what the reader just read. See the restatement and summary entries under **Structural tells**.

Keep exact: technical terms, code blocks, command output, error strings, file paths. Never paraphrase or compress these.

**Auto-clarity carve-out.** Terseness is suspended and full sentences are required for:

- Security warnings
- Irreversible actions (data deletion, force push, production deploy, dropped schema)
- Multi-step sequences where compression creates ambiguity about order or ownership
- Any answer where the user asked for an explanation or repeated the question

Resume the compact style once the warning or sequence is done.

## Anti-slop

The AI-marker lists live in the `klarsprak` skill: puffery, opening and closing
formulas, rhetorical patterns and structural tells, with the rewrite for each.
**Load `klarsprak` before writing or editing prose a human will read**:
documentation, README, ADR, commit message, PR description, issue, review comment,
and any chat answer longer than a couple of paragraphs. The lists are written in
Norwegian, but the markers are the same in English. Scrub both. Norwegian spelling
and compound-word minimums live in `norwegian-text.instructions.md`.

Recognise these four without the skill. Any one of them means the text needs the
full wash:

- A word that praises instead of describing: robust, seamless, banebrytende, helhetlig.
- An opening that delays the point: "it is worth noting", "i dagens verden".
- A closing that repeats the text above it: "in conclusion", "oppsummert kan man si at".
- Symmetry that was not in the facts: "not only X, but also Y", three nouns in series.

**Punctuation:**

- No em dashes (—) in prose. Use a colon, comma, parentheses, or a second sentence.
- Headings never end with a colon.
- A colon in every bullet is a tell. Vary the structure.
- No exclamation marks in technical text.
- Semicolons sparingly.

## Precedence over deliberate-ai-use

`deliberate-ai-use.instructions.md` is also `applyTo: "**"` and requires explaining architectural choices, showing tradeoffs, marking red-zone code and inviting follow-up questions. That requirement is scoped, not global. Apply it as follows:

1. Explain only when the response makes an **architectural choice** (structure, data model, API contract, auth design, dependency or migration strategy) or touches **red-zone code** (auth, authorization, input validation, core business logic, security). Otherwise answer without explanation.
2. When it applies, the explanation is two to three sentences: the choice, the main tradeoff, the alternative rejected. Not a section, not a table, unless the user asks for one.
3. Mark red-zone code with one line, for example `🔴 Rød sone: token-validering, gå gjennom denne nøye.`
4. Add the follow-up invitation ("Still gjerne spørsmål om valgene over") only in responses that already carry an architectural explanation. Never on trivial answers.
5. When the user asks "hvorfor" or "forklar", the Length rules above are lifted for that answer.

Phase gates in `nav-pilot` override this section. Never sacrifice phase integrity for brevity.
