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

Language-neutral, applies to Norwegian and English alike. Norwegian spelling and compound-word minimums live in `norwegian-text.instructions.md`; the full Norwegian language wash (klarspråk, anglicisms, fagtermer, Norwegian AI words) lives in the `klarsprak` skill.

**Puffery. Cut, or replace with something concrete:** groundbreaking / banebrytende, revolutionary / revolusjonerende, innovative / innovativ, robust, holistic / holistisk, seamless / sømløs, comprehensive / helhetlig, "plays a crucial role" / "spiller en avgjørende rolle", "represents a significant step forward" / "representerer et betydelig skritt fremover", "underscores the need for" / "understreker behovet for", "took the world by storm" / "har tatt verden med storm", "digital transformation" / "digital transformasjon", "enables" / "muliggjør", "facilitates" / "tilrettelegger for", "streamline the process" / "effektivisere prosessen", "put the user at the centre" / "sette brukeren i sentrum".

**Opening and closing phrases. Cut:** "it is worth noting" / "det er verdt å merke seg", "it is important to point out" / "det er viktig å påpeke", "in today's world" / "i dagens verden", "in a world where" / "i en verden der", "in an era where" / "i en tid der", "let us explore" / "la oss utforske", "let us dive into" / "la oss dykke ned i", "in summary" / "oppsummert kan man si at", "in short" / "kort sagt", "in conclusion" / "avslutningsvis", "there are several aspects to this" / "det finnes flere aspekter ved dette", "it should be mentioned that" / "det bør nevnes at", "remember that" / "husk at", "the results speak for themselves" / "resultatene taler for seg selv".

**Rhetorical patterns. Rewrite:**

- "Not only X, but also Y" / "Ikke bare X, men også Y". Split into two sentences, or keep the one that matters.
- "It is not about X, it is about Y" / "Det handler ikke om X, men om Y". Say only Y.
- "In an era where ..." framing paired with a closing perspective. Cut both ends.
- Tricolon, three nouns or clauses in series. Once is fine, repeated is a tell.
- False informality: casual opening that switches to polished formal prose. Hold one tone.
- Justification paragraphs that explain why something matters without adding information, including the "that is why X is so important" close.

**Structural tells:**

- Transition words as paragraph openers: "Videre", "Dessuten", "I tillegg", "I lys av dette", "Når det gjelder", "Furthermore", "Moreover", "Additionally".
- A summary sentence at the end of a section that repeats what the section just said.
- Identical grammatical structure in every bullet of a list.
- Forced balance between options ("begge har sine fordeler", "both have their merits"). Pick one and say why.
- Restating a point in other words right after making it, or defining what the audience already knows.
- The template arc: hook, context, hero, result, big picture, conclusion. Break it up and start with the news.

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
