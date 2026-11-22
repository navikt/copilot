# Comment rule A/B, 22 November 2026 (#2605, #2629)

Arm A: `instructions/output-style.instructions.md` from `origin/main`. Arm B: the same file with the "Code comments" section from #2629. Nothing else differs.

The owner approved this run as a one-time exception to the `HANDOFF.md` guard (#1409).

Setup: `prepare.py --direct-worker --model gpt-6-luna --effort medium`, which copies every repo instruction into `.github/instructions/`. Copilot CLI `1.0.115-6` ran with `--no-auto-update --disable-builtin-mcps --max-ai-credits 30 --allow-all-tools`. HOME, XDG and `NAV_PILOT_CONFIG` pointed at a scratch directory, and auth came from `GH_TOKEN` in the process environment. cplt was not used. The evaluator is `check.py` (Docker, network disabled). Each attempt directory holds `manifest.json`, `result.json`, `candidate.patch` and the CLI usage lines (`usage.txt`).

The session transcripts for all six B runs contain the "Code comments" section. None of the A transcripts do.

| Task | Arm | Evaluator pass | Comment lines added | Credits |
| --- | --- | --- | ---: | --- |
| kotlin/status-feature | A | 3/3 | 0 | 0.67, 0.63, 0.79 |
| kotlin/status-feature | B | 3/3 | 0 | 0.99, 0.92, 0.76 |
| typescript/feature | A | 3/3 | 0 | 1.03, 0.76, 0.91 |
| typescript/feature | B | 3/3 | 0 | 0.67, 0.78, 0.73 |

Totals: A 4.79 credits (0.80 per run), B 4.85 credits (0.81 per run), 9.64 credits for both arms. Comment density is 0 in all 12 patches (source lines starting with `//`, `/*`, `*` or `#`), so the blind accuracy review had nothing to review. The B patches also add exported functions without doc comments, so the model did not follow that part of the rule.

Pre-registered decision rule: success not lower (met), cost at most +10 % (met, +1.3 %), fewer wrong or stale comments (not met: no comments in either arm). Result: no measurable effect. These two tasks and one model do not support shipping the rule.
