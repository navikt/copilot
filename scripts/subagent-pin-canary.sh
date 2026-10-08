#!/usr/bin/env bash
# Canary: does Copilot CLI still run a subagent on its own `model:` pin?
#
# A parent agent on PARENT_MODEL delegates a one-line task to a probe agent
# pinned to CHILD_MODEL. The debug log names the model of every turn
# ("turn tool surface resolved {"model":...}"); the child's pin must be among
# them. Method from docs/golden-baselines/2026-10-08-subagent-arv (PR #1477).
#
# Isolated COPILOT_HOME in a temp dir: never reads or writes ~/.copilot.
# Cost: about 0.1 AI credit per run.
#
# Exit: 0 pin honoured, 1 pin not honoured, 2 could not run (auth, network,
# client change). Token: COPILOT_GITHUB_TOKEN, else GH_TOKEN, else `gh auth token`.
set -uo pipefail

PARENT_MODEL=${PARENT_MODEL:-gpt-6-luna}
CHILD_MODEL=${CHILD_MODEL:-gpt-5.6-luna}

token=${COPILOT_GITHUB_TOKEN:-${GH_TOKEN:-$(gh auth token 2>/dev/null)}}
[ -n "$token" ] || { echo "canary: no token (set COPILOT_GITHUB_TOKEN or log in with gh)" >&2; exit 2; }
to=$(command -v timeout || command -v gtimeout) || { echo "canary: no timeout command" >&2; exit 2; }
command -v copilot >/dev/null || { echo "canary: copilot not on PATH" >&2; exit 2; }

S=$(mktemp -d)
trap 'rm -rf "$S"' EXIT
mkdir -p "$S/home/agents" "$S/work"
cat > "$S/home/agents/probe-parent.agent.md" <<'EOF'
---
name: probe-parent
description: Probe parent. Delegates to probe-child.
---
When asked, start the subagent probe-child with the task "Reply with the single word OK." Do not specify a model. Then reply DONE.
EOF
cat > "$S/home/agents/probe-child.agent.md" <<EOF
---
name: probe-child
description: Probe child that replies OK.
model: $CHILD_MODEL
---
Reply with the single word OK. Use no tools.
EOF

cd "$S/work" || exit 2
COPILOT_HOME=$S/home COPILOT_GITHUB_TOKEN=$token "$to" 400 copilot \
  --agent probe-parent --model "$PARENT_MODEL" \
  -p "start the subagent probe-child" --allow-all-tools \
  --log-level debug --log-dir "$S/logs" > "$S/out" 2>&1
rc=$?

echo "copilot $(COPILOT_HOME=$S/home copilot --version 2>/dev/null | head -1)"
grep -h 'AI Credits' "$S/out"
models=$(grep -rhoE 'turn tool surface resolved \{"model":"[^"]*"' "$S/logs" 2>/dev/null |
  sed 's/.*"model":"//;s/"$//' | sort | uniq -c)
echo "turns per model:"
echo "${models:-  (none)}"
grep -rh 'did not commit a new selection' "$S/logs" 2>/dev/null | sed 's/^.*\[WARNING\]/[WARNING]/'

if [ "$rc" = 124 ]; then
  echo "canary: copilot timed out after 400 s; probe incomplete" >&2
  exit 2
fi
if [ -z "$models" ]; then
  echo "canary: no turns in the debug log; could not run" >&2
  tail -20 "$S/out" >&2
  exit 2
fi
if ! grep -qE "^ *[0-9]+ ${CHILD_MODEL//./\\.}\$" <<<"$models"; then
  echo "canary: FAIL, probe-child pinned to $CHILD_MODEL did not run on it" >&2
  exit 1
fi
echo "canary: OK, probe-child ran on its pin $CHILD_MODEL"
