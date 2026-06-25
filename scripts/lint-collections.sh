#!/usr/bin/env bash
#
# Verify that the COLLECTIONS data in the docs page matches the actual
# manifest.json files in .github/collections/. Catches stale counts and
# missing/extra items.
#
# Usage:
#   ./scripts/lint-collections.sh          # check all collections
#   ./scripts/lint-collections.sh -q       # quiet mode (only errors)
#
set -euo pipefail

COLLECTIONS_DIR="collections"
DOCS_PAGE="apps/my-copilot/src/app/nav-pilot/docs/page.tsx"

QUIET=false
errors=0
warnings=0

red()    { printf '\033[31m%s\033[0m\n' "$1"; }
yellow() { printf '\033[33m%s\033[0m\n' "$1"; }
green()  { printf '\033[32m%s\033[0m\n' "$1"; }
dim()    { printf '\033[2m%s\033[0m\n' "$1"; }

warn() { warnings=$(( warnings + 1 )); yellow "  ⚠  $1"; }
fail() { errors=$(( errors + 1 ));     red    "  ❌ $1"; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    -q|--quiet) QUIET=true; shift ;;
    *) shift ;;
  esac
done

if [[ ! -f "$DOCS_PAGE" ]]; then
  red "Docs page not found: $DOCS_PAGE"
  exit 1
fi

# Extract collection data from manifest.json files
for manifest in "$COLLECTIONS_DIR"/*/manifest.json; do
  collection=$(basename "$(dirname "$manifest")")
  [[ "$QUIET" == false ]] && echo "📦 $collection"

  # Read manifest values with jq-like parsing via python
  manifest_data=$(python3 -c "
import json, sys
with open('$manifest') as f:
    m = json.load(f)

agents = sorted(m.get('agents', []))
skills = sorted(m.get('skills', []))
instructions = sorted(m.get('instructions', []))
prompts = sorted(m.get('prompts', []))

print(f'AGENTS_COUNT={len(agents)}')
print(f'SKILLS_COUNT={len(skills)}')
print(f'INSTRUCTIONS_COUNT={len(instructions)}')
print(f'PROMPTS_COUNT={len(prompts)}')
print(f'AGENTS_LIST={chr(44).join(agents)}')
print(f'SKILLS_LIST={chr(44).join(skills)}')
print(f'INSTRUCTIONS_LIST={chr(44).join(instructions)}')
print(f'PROMPTS_LIST={chr(44).join(prompts)}')
")

  eval "$manifest_data"

  # Check if collection exists in docs page
  if ! grep -q "name: \"$collection\"" "$DOCS_PAGE"; then
    fail "$collection: not found in docs page"
    continue
  fi

  # Extract the COLLECTIONS entry for this collection from the TSX file
  # We parse the agents/skills counts and detail strings
  docs_data=$(python3 -c "
import re, sys

with open('$DOCS_PAGE') as f:
    content = f.read()

# Find the block for this collection
pattern = r'name:\s*\"$collection\".*?(?=name:\s*\"|^\];)'
# Simpler: find lines between name: \"$collection\" and the next name: or ];
lines = content.split('\n')
in_block = False
block = []
brace_depth = 0
for line in lines:
    if 'name: \"$collection\"' in line:
        in_block = True
        brace_depth = 1
        block.append(line)
        continue
    if in_block:
        brace_depth += line.count('{') - line.count('}')
        block.append(line)
        if brace_depth <= 0:
            break

block_text = '\n'.join(block)

# Extract counts
agents_match = re.search(r'agents:\s*(\d+)', block_text)
skills_match = re.search(r'skills:\s*(\d+)', block_text)

# Extract detail strings (comma-separated items in quotes)
def extract_detail(name):
    # Match: name: \"items\" or name:\\n\"items\"
    pat = rf'{name}:\s*\n?\s*\"([^\"]+)\"'
    m = re.search(pat, block_text)
    if m:
        items = [x.strip() for x in m.group(1).split(',')]
        return sorted(items)
    return []

agents_detail = extract_detail('agents')
skills_detail = extract_detail('skills')
instructions_detail = extract_detail('instructions')
prompts_detail = extract_detail('prompts')

print(f'DOCS_AGENTS_COUNT={agents_match.group(1) if agents_match else -1}')
print(f'DOCS_SKILLS_COUNT={skills_match.group(1) if skills_match else -1}')
print(f'DOCS_AGENTS_LIST={chr(44).join(agents_detail)}')
print(f'DOCS_SKILLS_LIST={chr(44).join(skills_detail)}')
print(f'DOCS_INSTRUCTIONS_LIST={chr(44).join(instructions_detail)}')
print(f'DOCS_PROMPTS_LIST={chr(44).join(prompts_detail)}')
")

  eval "$docs_data"

  # Compare counts
  if [[ "$DOCS_AGENTS_COUNT" != "$AGENTS_COUNT" ]]; then
    fail "$collection: agents count mismatch — manifest=$AGENTS_COUNT, docs=$DOCS_AGENTS_COUNT"
  fi
  if [[ "$DOCS_SKILLS_COUNT" != "$SKILLS_COUNT" ]]; then
    fail "$collection: skills count mismatch — manifest=$SKILLS_COUNT, docs=$DOCS_SKILLS_COUNT"
  fi

  # Compare item lists
  if [[ "$DOCS_AGENTS_LIST" != "$AGENTS_LIST" ]]; then
    fail "$collection: agents list mismatch"
    [[ "$QUIET" == false ]] && dim "    manifest: $AGENTS_LIST"
    [[ "$QUIET" == false ]] && dim "    docs:     $DOCS_AGENTS_LIST"
  fi
  if [[ "$DOCS_SKILLS_LIST" != "$SKILLS_LIST" ]]; then
    fail "$collection: skills list mismatch"
    [[ "$QUIET" == false ]] && dim "    manifest: $SKILLS_LIST"
    [[ "$QUIET" == false ]] && dim "    docs:     $DOCS_SKILLS_LIST"
  fi
  if [[ "$DOCS_INSTRUCTIONS_LIST" != "$INSTRUCTIONS_LIST" ]]; then
    fail "$collection: instructions list mismatch"
    [[ "$QUIET" == false ]] && dim "    manifest: $INSTRUCTIONS_LIST"
    [[ "$QUIET" == false ]] && dim "    docs:     $DOCS_INSTRUCTIONS_LIST"
  fi
  if [[ "$DOCS_PROMPTS_LIST" != "$PROMPTS_LIST" ]]; then
    fail "$collection: prompts list mismatch"
    [[ "$QUIET" == false ]] && dim "    manifest: $PROMPTS_LIST"
    [[ "$QUIET" == false ]] && dim "    docs:     $DOCS_PROMPTS_LIST"
  fi

  [[ "$QUIET" == false ]] && [[ "$errors" -eq 0 ]] && green "  ✓ $collection ok"
done

# Summary
echo ""
if [[ "$errors" -gt 0 ]]; then
  red "❌ $errors error(s), $warnings warning(s)"
  echo ""
  echo "Fix: update COLLECTIONS in $DOCS_PAGE to match collections/*/manifest.json"
  exit 1
else
  green "✅ All collections match ($warnings warning(s))"
fi
