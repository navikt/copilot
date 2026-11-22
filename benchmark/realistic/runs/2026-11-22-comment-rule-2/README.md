# Comment rule A/B, seeded variant (#2605, #2629)

Fixture variant for the second run. `seed.py` adds stale comments to the prepared workspace after `prepare.py`, before the agent starts. Each comment describes behaviour the task removes: four in `kotlin/status-feature`, three in `typescript/feature`. Both tasks also add exported names: Kotlin functions are public by default, and the TypeScript task adds an exported `POST` handler. The source fixtures and evaluator tests are unchanged, so `check.py` scores the task exactly as before.

## Run

The owner approved this run as a one-time exception to the `HANDOFF.md` guard (#1409). The setup matches the first run (`../2026-11-22-comment-rule/`): `prepare.py --direct-worker --model gpt-6-luna --effort medium`, Copilot CLI `1.0.115-6`, `--allow-all-tools`, with HOME, XDG and `NAV_PILOT_CONFIG` in a scratch directory and no cplt.

In both runs `GH_TOKEN` was set in the environment Copilot ran in, so the agent's tools could see it. In the first run, `prepare.py` and `git` also got it. The owner accepted that risk. The token was not written to any file. All outputs were grepped for token patterns before committing, and nothing matched.

Arm A: `output-style` from `origin/main`. Arm B: with the "Code comments" section from #2629.

| Task | Arm | Evaluator pass | Stale comments left | Doc comments on new exported names | Comment lines added | Credits |
| --- | --- | --- | --- | --- | ---: | --- |
| kotlin/status-feature | A | 3/3 | 0 of 12 | 0 | 1 | 0.71, 0.73, 0.92 |
| kotlin/status-feature | B | 3/3 | 0 of 12 | 0 | 0 | 0.63, 0.72, 0.92 |
| typescript/feature | A | 3/3 | 0 of 9 | 0 | 1 | 0.60, 0.79, 0.75 |
| typescript/feature | B | 3/3 | 0 of 9 | 0 | 0 | 0.93, 0.62, 0.69 |

All 12 runs deleted every seeded stale comment. Two A runs replaced one of them with a new comment. A blind reviewer, with no arm labels, rated both new comments "restates code". No run added a doc comment to a new exported name (`complete`, `createSak`, `POST`), including the B runs where the rule was in context.

Credits: A 4.50 and B 4.51, 9.01 in total.

Decision rule: success not lower (met), cost at most +10 % (met, +0.2 %), fewer wrong or stale comments (not met: A already left none). Result: no measurable effect.
