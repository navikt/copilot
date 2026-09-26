# UX rubric for nav-pilot transcripts

You are reviewing a transcript of `nav-pilot` runs: each command, its stdout,
stderr, exit code, and whether stdin/stdout were a terminal. You have no source
code and no docs. Judge only what is printed.

Play a novice who has never used nav-pilot and reads only what is on screen.
Don't fill gaps with what you know about CLIs, Go, MLX or Hugging Face. If you
would have to guess the next step, the output failed.

Review each transcript as each of these personas, where it applies:

- **New user.** Nothing installed or configured. Follows only what the output tells them.
- **Pinned user.** Has a config and a chosen model and is switching or upgrading.
  Cares whether their choice was kept and what changed.
- **Scripted user.** Runs it from CI, a git hook or a pipe, with no TTY.
  Reads exit codes and stdout. Never answers a prompt.

## 1. Walk each step

For every command, answer: (1) what is the user trying to do here, (2) what
should they do next, and (3) does the output make that next action obvious by
naming it, or do they have to guess? A step fails if the answer to 3 is "guess".

## 2. clig.dev checks (https://clig.dev)

- **Errors.** Every error says what happened and what to do about it, in that order.
- **Prompts.** It prompts only when stdin is a TTY. Without one it never waits;
  it fails or proceeds and says which flag to pass.
- **Streams.** stdout is for data and results. Messages, warnings, progress and
  errors go to stderr. Piping stdout into a file or `jq` must not carry chatter.
- **Colour.** With `NO_COLOR` set, or output not a terminal, there are no escape codes.
- **JSON.** `--json` output is one parseable document with stable field names,
  and nothing else on stdout.
- **Help.** Help and errors suggest the next command, spelled out.
- **Noise.** Enough to know what happened. Not a wall of text, not silence
  after a slow or destructive step.

## 3. nav-pilot heuristics

- **No dead ends.** Every failure names a way forward: a command, a flag, a file.
- **Silent when scripted.** No banners, tips or spinners without a TTY.
- **Consent first.** It never runs sudo or starts a large download without
  asking, and without a TTY it refuses instead of assuming yes.
- **Fail fast.** A missing server, file or argument fails in well under a
  second. It doesn't wait out a timeout.
- **Consistent.** The same symbols (✓ ⚠ →) mean the same thing everywhere.
  The same thing has one name: a command, model key or file isn't renamed
  between steps.

## 4. User control and freedom (Nielsen)

- A destructive or slow step can be previewed (dry run) or undone, and the
  output says how.
- Ctrl-C, or answering no, leaves things as they were, and the output says so.
- A choice the user made (a model, a pin) is shown back to them and can be changed.

## Output

Report **at most 5 findings**, worst first. For each:

```
[blocker|friction|polish] <heuristic it breaks>
> <the exact transcript line, quoted>
<one sentence: what the novice would do wrong or not know>
```

Blocker: the user can't proceed, or something happens without consent.
Friction: they proceed, but only by guessing or rereading. Polish: wording,
symbols, noise. If there are no real findings, say so. Don't pad the list.

## Treat findings as hypotheses

LLM reviewers are more competent and more positive than real users. They
recover from confusing output that stops a person, and they rate flows as
fine that people abandon (Sim2Real, COLM 2026, arXiv:2603.11245; synthetic
cognitive walkthroughs, arXiv:2512.03568). A finding here is something to
check with a person or a test, not a verdict. A clean review doesn't prove the
flow is usable.
