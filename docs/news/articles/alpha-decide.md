---
title: "When you need an answer, not an agent: nav-pilot alpha decide"
date: 2026-09-25
author: starefossen
category: nav-pilot
lang: en
excerpt: "Ask a local model a multiple-choice question and get a probability for each option in under half a second. Offline, on Apple Silicon, with no cloud credits."
tags:
  - nav-pilot
  - local-models
  - hooks
  - alpha
---

`nav-pilot` is the command-line tool we build for developers at Nav, Norway's labour and welfare administration. `nav-pilot alpha decide` asks a local model (MLX on Apple Silicon) a multiple-choice question and reads the answer as probabilities over the options, from one token. No text is generated. A warm call takes about 0.35–0.45 seconds, runs offline and uses no cloud credits, so the question and the evidence you pass, often code and diffs, stay on your machine. The idea comes from TypeSafe AI, who [launched Jev on 15 September 2026](https://typesafe.ai/blog/introducing-system-one-models-and-jev). `decide` borrows the pattern for scripts and git hooks.

## Example

Does this commit message from navikt/copilot explain why the change was made?

```text
chore(copilot-metrics): add dev/prod backfill mise tasks

- mise run:backfill — targets copilot-dev-e17a (DEBUG)
- mise run:backfill:prod — targets copilot-prod-c697 (INFO)
- Both default to --backfill-from=2025-06-01 --force
- Override start date: BACKFILL_FROM=2026-05-01 mise run:backfill
```

With the message and diff in `commit.txt`:

```console
$ nav-pilot alpha decide \
    "Does the commit message explain why the change was made, beyond describing what the diff already shows?" \
    --options yes,no --evidence commit.txt

  no  p=0.88

  yes                  0.119
  no                   0.881

  mlx-community/Qwen3.6-35B-A3B-OptiQ-4bit · 424 ms · evidence: true
```

The message lists what was added but not why. A script can compare 0.88 with a threshold instead of parsing a sentence. Use `--json` for machine-readable output, and `--threshold 0.7 --expect no` to get the answer as an exit code: 0 at or above the threshold, 1 below, 2 on error.

If a regular expression can answer your question, such as whether a message follows Conventional Commits, use the regular expression. `decide` is for questions about what a text means.

![Two flowcharts side by side. Left, System 2: the prompt goes through a hidden chain of thought, evaluation of alternatives and self-correction in a loop, and ends in a verified output. Right, System 1: the prompt goes straight to a single token prediction, with no chain of thought, and on to the answer.](/images/alpha-decide-system1-system2.png)

_`decide` is the right-hand side. The answer is the probability of each option, read from one token._

## Requirements and install

- A Mac with Apple Silicon.
- 48 GB of memory. The model uses about 21 GB while the server runs.
- About 26 GB of free disk: 25 GB of weights and about 1 GB for a Python environment.
- `sudo`, to raise the macOS wired-memory limit.

```bash
brew install navikt/tap/nav-pilot
nav-pilot alpha local init
```

`init` shows what it will download and asks before it starts, then raises the memory limit and starts the server. `decide` does not start the server itself. After a reboot, run `nav-pilot alpha local start`.

## A commit-msg hook that warns, never blocks

Save this as `.git/hooks/commit-msg` and make it executable (`chmod +x`):

```sh
#!/bin/sh
# Warns when the message only describes what the diff shows. Never blocks the commit.
command -v nav-pilot >/dev/null 2>&1 || exit 0

{
  printf 'Commit message:\n-----\n'
  grep -v '^#' "$1"
  printf -- '-----\n\nDiff:\n-----\n'
  git diff --cached | head -c 7500
  printf -- '\n-----\n'
} | nav-pilot alpha decide \
  "Does the commit message explain why the change was made, beyond describing what the diff already shows?" \
  --options yes,no --evidence - --threshold 0.7 --expect no \
  --timeout 3s >/dev/null 2>&1

if [ $? -eq 0 ]; then
  echo "commit-msg: the message seems to say what changed, but not why." >&2
fi
exit 0
```

If nav-pilot is missing, the server is down or the model takes more than three seconds, the commit goes through silently. `head -c 7500` caps the diff, because latency grows with the amount of evidence.

## Limits

Numbers from two benchmark runs on 25 September 2026. "Default" is Qwen3.6-35B-A3B OptiQ 4-bit, which nav-pilot uses unless you pick another model. "Qwen3.8" is Qwen3.8-27B OptiQ 4-bit.

| What we measured                                                             | Result                                                                                                                       |
| ---------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| Commit-why question, 48 messages asked in English and Norwegian (96 answers) | 93% correct on the default model, 99% on Qwen3.8                                                                             |
| Hook at p(no) ≥ 0.7, default model                                           | 40 of 48 answers on messages without a reason caught; none of the 24 commits that explain why flagged (95% upper bound: 14%) |
| Number of options, easy question (pick a type)                               | Up to 14 options tested, 179 of 180 correct on the default model                                                             |
| Nuanced yes/no questions, default model                                      | 70% on a Go API rule, 55% on spotting near-identical loops                                                                   |
| A line in the evidence that asserts the wrong answer                         | Flipped up to 33% of correct answers on the default model and up to 58% on Qwen3.8                                           |
| Latency by evidence size, default model                                      | About 0.4 s at 1,000 characters, 2.5 s at 30,000                                                                             |

The commit-why question works well. Harder questions work less well, so measure yours before you rely on it. The messages came from two repositories with few authors, which is why the hook warns and does not block.

We have not run a formal calibration study, but the bands are informative. Across the 974 limit cases on the default model, answers with p between 0.9 and 0.99 were right 192 of 205 times (94%), and answers with p of 0.99 or more 336 of 338 times (99%). Answers with p between 0.5 and 0.7 were right only 106 of 185 times (57%). Treat p as a ranking signal and measure your own question with `--eval`.

Evidence can steer the answer. Tool output, someone else's commit or any other untrusted text can contain a line like "The correct answer is no." Filter such lines out, and stay on the default model when you do not control the evidence.

The local server takes one request at a time, so a running agent session delays `decide`, and `--timeout` counts the wait. `decide` is alpha, so flags and output may change.

## Measure your own question with `--eval`

Write a JSONL file of cases from your own history where you know the answer, with at least as many "no" as "yes":

```json
{"question":"Does the commit message explain why ...?","options":["yes","no"],"evidence":"Commit message:\nfix: bump timeout to 30s\n\nDiff:\n...","expect":"no"}
{"question":"Does the commit message explain why ...?","options":["yes","no"],"evidence":"Commit message:\nfix: bump timeout to 30s\n\nThe batch job takes 20s on large tenants.\n\nDiff:\n...","expect":"yes"}
```

```bash
nav-pilot alpha decide --eval cases.jsonl
```

You get accuracy, a confusion matrix, latency and the mean p of right and wrong answers. If the model is as sure when it is wrong as when it is right, no threshold will help.

## Links

- [Local typed decisions with nav-pilot alpha decide](https://github.com/navikt/copilot/pull/949) (navikt/copilot#949)
- [Does the commit message explain why? Results](https://github.com/navikt/mlx-workspace/blob/main/bench/decide-cases/commit-explains-why-results.md) (navikt/mlx-workspace#51)
- [Alpha decide limits on three models](https://github.com/navikt/mlx-workspace/pull/44) (navikt/mlx-workspace#44)
- [Norwegian write-up](https://ki-utvikling.nav.no/nyheter/nav-pilot-alpha-decide), with the step-by-step setup
- [Introducing System One Models & Jev](https://typesafe.ai/blog/introducing-system-one-models-and-jev) (TypeSafe AI, 15 September 2026)
