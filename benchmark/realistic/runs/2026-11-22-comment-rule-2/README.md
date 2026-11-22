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

All 12 runs deleted every seeded stale comment. Two A runs replaced one of them with a new comment. A blind reviewer, with no arm labels, rated both new comments "restates code". No run added a doc comment to a new exported name (`complete`, `createSak`, `POST`), including the B runs where the rule was in the workspace instructions. Transcripts were not checked for this run.

Credits: A 4.50 and B 4.51, 9.01 in total.

Decision rule: success not lower (met), cost at most +10 % (met, +0.2 %), fewer wrong or stale comments (not met: A already left none). Result: no measurable effect.

## Treatment and replay

`treatment/` holds the exact arm files and their hashes in `SHA256SUMS`:

- `output-style.A.instructions.md`: the file at `c90737c4` on `main`. `main` did not change this file before either run.
- `output-style.B.instructions.md`: A plus the #2629 section (commit `cb075818`).
- `B.patch`: the diff from A to B.

`manifest.json` lists the B hash for every attempt, including A attempts: `prepare.py` ran from the #2629 branch and hashed the files then, and only afterwards did the runner overwrite the file with the A version. `treatment/exposure.json` records, per attempt, the session ID, the arm, the hash of the output-style file in the workspace, and whether the rule text appeared in the transcript. The transcripts themselves are not kept.

Replay one attempt:

```sh
repo=$(git rev-parse --show-toplevel)
run=2026-11-22-comment-rule   # or 2026-11-22-comment-rule-2
task=kotlin/status-feature    # or typescript/feature
out=$(mktemp -d)/attempt      # prepare.py creates it; must be absolute
scratch=$(mktemp -d)          # isolated HOME
cd "$repo"
# The evidence is not in c90737c4: copy it out first.
cp -R "benchmark/realistic/runs/$run/treatment" "$scratch/treatment"
cp benchmark/realistic/runs/2026-11-22-comment-rule-2/seed.py "$scratch/seed.py"
git checkout c90737c4
git apply "$scratch/treatment/B.patch"   # B arm only
python3 -B benchmark/realistic/prepare.py "$task" "$out" --model gpt-6-luna --effort medium --direct-worker
# A arm, if prepared from a B checkout:
# cp "$scratch/treatment/output-style.A.instructions.md" "$out/workspace/.github/instructions/output-style.instructions.md"
python3 -B "$scratch/seed.py" "$task" "$out/workspace"   # second run only
cd "$out/workspace" && git init -q && git add -A && git -c commit.gpgsign=false commit -qm base
env GH_TOKEN="$(gh auth token)" HOME="$scratch" XDG_CONFIG_HOME="$scratch/.config" XDG_CACHE_HOME="$scratch/.cache" \
  XDG_DATA_HOME="$scratch/.local/share" XDG_STATE_HOME="$scratch/.local/state" NAV_PILOT_CONFIG="$scratch/np.toml" \
  copilot --no-auto-update --model gpt-6-luna --reasoning-effort medium --disable-builtin-mcps \
  --max-ai-credits 30 --allow-all-tools -p "$(cat "$out/evaluator/fixture/prompt.md")"
git add -A && git diff --cached HEAD -- . ':!node_modules' ':!build' ':!.gradle' ':!.next' > "$out/candidate.patch"
cd "$repo"
python3 -B benchmark/realistic/check.py "benchmark/realistic/$task" --candidate "$out/workspace" --output "$out/result.json"
```

The token comes from `gh auth token` inside the command substitution. It is never printed or written to a file, and only the `copilot` process and its children get it (in the first run, `prepare.py` and `git` got it as well).
