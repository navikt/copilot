# Candidate dependency audit

No user-validated designer task or package scope exists yet. These sources
support a candidate for Norwegian prose editing, not a general design package.
No canonical files have been copied into the diagnostic plugin.

| Source | Finding | Packaging consequence |
| --- | --- | --- |
| `agents/forfatter.agent.md:3-12,19-37` | Norwegian prose editor; pins Claude Sonnet 5.5 and read/edit/grep/glob/todo/GitHub tools; redirects development work to `@nav-pilot` | Validate model availability and tool names per client; adapt the indirect agent dependency rather than shipping the whole harness |
| `agents/forfatter.agent.md:91-111` | Requires klarsprak for language/terminology and output-style for always-applicable style | Agent alone is not the complete candidate |
| `agents/forfatter.agent.md:403-410` | Accepts delegated prose work from nav-pilot; uses repository ORDBOK.md when present | Preserve scoped editing and optional glossary behavior without bundling nav-pilot |
| `skills/klarsprak/SKILL.md:2-19` | Norwegian prose review workflow preserving meaning and technical content | Candidate skill for an agreed prose-editing task; not evidence of broader designer usability |
| `instructions/norwegian-text.instructions.md:3,8-10,33-37` | Applies to Markdown; references output-style, klarsprak and forfatter | Candidate rule dependency; verify matching/nonmatching/no-file cases per client |

Candidate set: **forfatter + klarsprak + output-style + norwegian-text**.
Resolve model/tool mapping, indirect delegation and rule scope only after a human
chooses the task and acceptance criteria. The diagnostic fixture tests mechanics
independently; it does not validate this candidate's usefulness.
