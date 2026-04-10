#!/usr/bin/env bash
#
# Lint .github/skills/ for quality — inspired by awesome-copilot Skill Quality Reports.
# Checks token budget, structure, file-ref depth, and metadata completeness.
#
# Usage:
#   ./scripts/lint-skills.sh            # lint all skills
#   ./scripts/lint-skills.sh api-design # lint one skill
#
set -euo pipefail

SKILLS_DIR=".github/skills"
MAX_LINES=500
TOKEN_WARN=3000   # "standard" — approaching diminishing returns
TOKEN_ERROR=5000  # "comprehensive" — hurts performance ~2.9pp
MAX_REF_DEPTH=1   # max directory depth for file references from SKILL.md

errors=0
warnings=0
skills_checked=0

red()    { printf '\033[31m%s\033[0m\n' "$1"; }
yellow() { printf '\033[33m%s\033[0m\n' "$1"; }
green()  { printf '\033[32m%s\033[0m\n' "$1"; }
dim()    { printf '\033[2m%s\033[0m\n' "$1"; }

warn() { warnings=$(( warnings + 1 )); yellow "  ⚠  $1"; }
fail() { errors=$(( errors + 1 ));     red    "  ❌ $1"; }
ok()   { dim             "  ✓  $1"; }

# Extract SKILL.md body (everything after the closing --- of frontmatter)
body_of() {
  awk 'BEGIN{fm=0} /^---$/{fm++; next} fm>=2{print}' "$1"
}

lint_skill() {
  local dir="$1"
  local name
  name=$(basename "$dir")
  local skill_md="$dir/SKILL.md"
  local metadata="$dir/metadata.json"

  if [[ ! -f "$skill_md" ]]; then
    fail "[$name] Missing SKILL.md"
    return
  fi

  skills_checked=$(( skills_checked + 1 ))
  echo "📊 $name"

  # ── metadata.json checks ──────────────────────────────────────────────
  if [[ ! -f "$metadata" ]]; then
    fail "[$name] Missing metadata.json"
  else
    local desc
    desc=$(python3 -c "import json,sys; d=json.load(open(sys.argv[1])); v=d.get('description',''); print('' if v is None else str(v).strip())" "$metadata" 2>/dev/null || echo "")
    if [[ -z "$desc" ]]; then
      fail "[$name] metadata.json missing \"description\" field"
    else
      ok "metadata.json has description"
    fi
  fi

  # ── body metrics ───────────────────────────────────────────────────────
  local body
  body=$(body_of "$skill_md")

  local lines
  lines=$(echo "$body" | wc -l | tr -d ' ')

  local chars
  chars=$(echo "$body" | wc -c | tr -d ' ')
  local tokens=$(( chars / 4 ))

  local sections
  sections=$(echo "$body" | grep -c '^#' || true)

  local code_blocks
  code_blocks=$(echo "$body" | grep -c '^```' || true)
  code_blocks=$(( code_blocks / 2 ))

  local has_steps
  has_steps=$(echo "$body" | grep -cE '^\s*[0-9]+\.' || true)

  # ── line count ─────────────────────────────────────────────────────────
  if (( lines > MAX_LINES )); then
    fail "[$name] SKILL.md body is $lines lines — maximum is $MAX_LINES. Move reference material to separate files."
  else
    ok "$lines lines"
  fi

  # ── token budget ───────────────────────────────────────────────────────
  if (( tokens >= TOKEN_ERROR )); then
    fail "[$name] ~$tokens tokens — \"comprehensive\" skills hurt performance by ~2.9pp. Split into focused skills."
  elif (( tokens >= TOKEN_WARN )); then
    warn "[$name] ~$tokens tokens — approaching range where gains diminish."
  else
    local category="compact"
    (( tokens >= 500 )) && category="detailed"
    ok "~$tokens tokens ($category)"
  fi

  # ── structural signals ─────────────────────────────────────────────────
  if (( sections == 0 )); then
    warn "[$name] No section headers — agents navigate structured documents better."
  else
    ok "$sections sections"
  fi

  if (( code_blocks == 0 )); then
    warn "[$name] No code blocks — agents perform better with concrete snippets and commands."
  else
    ok "$code_blocks code blocks"
  fi

  if (( has_steps == 0 )); then
    warn "[$name] No numbered workflow steps — agents follow sequenced procedures more reliably."
  else
    ok "has numbered steps"
  fi

  # ── file reference depth ───────────────────────────────────────────────
  local refs
  refs=$(echo "$body" | grep -oE '\./[^] )"]+|references/[^] )"]+' || true)
  if [[ -n "$refs" ]]; then
    while IFS= read -r ref; do
      # Skip empty lines
      [[ -z "$ref" ]] && continue
      local depth
      depth=$(echo "$ref" | tr -cd '/' | wc -c | tr -d ' ')
      # Subtract 1 for the leading ./ if present
      [[ "$ref" == ./* ]] && depth=$(( depth - 1 ))
      if (( depth > MAX_REF_DEPTH )); then
        fail "[$name] File reference '$ref' is $depth directories deep — maximum is $MAX_REF_DEPTH level from SKILL.md."
      fi
    done <<< "$refs"
  fi

  # ── 3-way drift detection (metadata ↔ filesystem ↔ SKILL.md links) ──
  local refs_dir="$dir/references"
  local meta_refs=()
  local fs_refs=()
  local md_refs=()

  # 1. References listed in metadata.json
  if [[ -f "$metadata" ]]; then
    while IFS= read -r r; do
      [[ -n "$r" ]] && meta_refs+=("$r")
    done < <(python3 -c "
import json, sys
d = json.load(open(sys.argv[1]))
for r in d.get('references', []):
    print(r)
" "$metadata" 2>/dev/null || true)
  fi

  # 2. Files actually on disk in references/
  if [[ -d "$refs_dir" ]]; then
    while IFS= read -r f; do
      [[ -n "$f" ]] && fs_refs+=("references/$(basename "$f")")
    done < <(find "$refs_dir" -maxdepth 1 -type f -name '*.md' | sort)
  fi

  # 3. Links in SKILL.md pointing to references/
  while IFS= read -r link; do
    [[ -n "$link" ]] && md_refs+=("$link")
  done < <(echo "$body" | grep -oE '(\.\/)?references\/[^] )"]+' | sed 's|^\./||' | sort -u || true)

  # Compare: any set non-empty means references exist somewhere
  if (( ${#meta_refs[@]} + ${#fs_refs[@]} + ${#md_refs[@]} > 0 )); then
    local drift=0

    # Check: files on disk not in metadata
    for f in "${fs_refs[@]}"; do
      local found=0
      for m in "${meta_refs[@]}"; do [[ "$m" == "$f" ]] && found=1 && break; done
      if (( found == 0 )); then
        fail "[$name] Reference file '$f' exists on disk but not in metadata.json"
        drift=1
      fi
    done

    # Check: metadata entries not on disk
    for m in "${meta_refs[@]}"; do
      if [[ ! -f "$dir/$m" ]]; then
        fail "[$name] metadata.json lists '$m' but file does not exist"
        drift=1
      fi
    done

    # Check: SKILL.md links not in metadata
    for link in "${md_refs[@]}"; do
      local found=0
      for m in "${meta_refs[@]}"; do [[ "$m" == "$link" ]] && found=1 && break; done
      if (( found == 0 )); then
        warn "[$name] SKILL.md links to '$link' which is not in metadata.json references"
        drift=1
      fi
    done

    if (( drift == 0 )); then
      ok "references in sync (metadata ↔ disk ↔ SKILL.md)"
    fi
  fi

  echo ""
}

# ── main ───────────────────────────────────────────────────────────────────

cd "$(git rev-parse --show-toplevel)"

if [[ $# -gt 0 ]]; then
  # Lint specific skill(s)
  for name in "$@"; do
    lint_skill "$SKILLS_DIR/$name"
  done
else
  # Lint all skills
  for dir in "$SKILLS_DIR"/*/; do
    lint_skill "$dir"
  done
fi

echo "─────────────────────────────────────────────"
echo "Skills checked: $skills_checked"
if (( errors > 0 )); then
  red "Errors: $errors"
fi
if (( warnings > 0 )); then
  yellow "Warnings: $warnings"
fi
if (( errors == 0 && warnings == 0 )); then
  green "All checks passed ✓"
fi

exit $(( errors > 0 ? 1 : 0 ))
