# Comment rule A/B, 22 November 2026 (#2605, #2629)

Arm A: `instructions/output-style.instructions.md` from `origin/main`. Arm B: the same file with the "Code comments" section from #2629. Nothing else differs.

The owner approved this run as a one-time exception to the `HANDOFF.md` guard (#1409).

Setup: `prepare.py --direct-worker --model gpt-6-luna --effort medium`, which copies every repo instruction into `.github/instructions/`. Copilot CLI `1.0.115-6` ran with `--no-auto-update --disable-builtin-mcps --max-ai-credits 30 --allow-all-tools`. HOME, XDG and `NAV_PILOT_CONFIG` pointed at a scratch directory, and auth came from `GH_TOKEN` in the process environment. cplt was not used. The evaluator is `check.py` (Docker, network disabled). Each attempt directory holds `manifest.json`, `result.json`, `candidate.patch` and the CLI usage lines (`usage.txt`).

Before the transcripts were deleted, 6 of the 12 session directories contained the "Code comments" heading. That matches the number of B sessions, but the matches were not tied to session IDs, so `treatment/exposure.json` leaves transcript exposure unverified.

| Task | Arm | Evaluator pass | Comment lines added | Credits |
| --- | --- | --- | ---: | --- |
| kotlin/status-feature | A | 3/3 | 0 | 0.67, 0.63, 0.79 |
| kotlin/status-feature | B | 3/3 | 0 | 0.99, 0.92, 0.76 |
| typescript/feature | A | 3/3 | 0 | 1.03, 0.76, 0.91 |
| typescript/feature | B | 3/3 | 0 | 0.67, 0.78, 0.73 |

Totals: A 4.79 credits (0.80 per run), B 4.85 credits (0.81 per run), 9.64 credits for both arms. Comment density is 0 in all 12 patches (source lines starting with `//`, `/*`, `*` or `#`), so the blind accuracy review had nothing to review. The B patches also add exported functions without doc comments, so the model did not follow that part of the rule.

Pre-registered decision rule: success not lower (met), cost at most +10 % (met, +1.3 %), fewer wrong or stale comments (not met: no comments in either arm). Result: no measurable effect. These two tasks and one model do not support shipping the rule.

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
