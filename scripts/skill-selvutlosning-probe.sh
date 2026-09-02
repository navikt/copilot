#!/usr/bin/env bash
#
# Måler om en «bruk bare når brukeren eksplisitt ber om det»-description faktisk
# hindrer at en konvertert prompt-skill fyrer av seg selv (#604).
#
# TO ARMER, samme kropp, samme fixture, samme modell — bare description skiller:
#   A (uguardet)  den beskrivelsen du skriver uten å tenke på selvutløsning
#   B (guardet)   samme, pluss «bruk bare når brukeren eksplisitt ber om ...»
#
# TO PROMPTER per arm:
#   probe      forklar/gjennomgå en eksisterende React-komponent — ber IKKE om scaffold
#   borderline endre en eksisterende Aksel-komponent — komponentarbeid, men ikke scaffold
#   positive   be eksplisitt om en ny komponent — skillen SKAL fyre
#
# Positivkontrollen er det som gjør «fyrte ikke» tolkbart: uten den er en grønn
# arm B ikke til å skille fra en skill som aldri var relevant (#583).
#
# DETEKSJON: Copilot CLI skriver «● skill(<navn>)» i transkriptet når den laster
# en skill. Ingen heuristikk, ingen regex på modellens prosa.
#
# ISOLASJON: HOME peker på en kopi av ~/.copilot med config/settings/instruksjoner,
# men UTEN skills/ og agents/. Ellers ville brukerens 32 installerte skills
# (aksel-builder blant dem) konkurrere om den samme oppgaven.
#
# Kjøres for hånd. Live modellkall, koster penger, ikke i CI.
#
#   ./scripts/skill-selvutlosning-probe.sh --model claude-sonnet-5 --repeat 5 --out /tmp/probe
#   ./scripts/skill-selvutlosning-probe.sh --model claude-sonnet-5 --only borderline --out /tmp/probe
set -uo pipefail

MODEL=""; REPEAT=5; OUT=""; ONLY="probe borderline positive"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --model)  MODEL="$2"; shift 2 ;;
    --repeat) REPEAT="$2"; shift 2 ;;
    --out)    OUT="$2"; shift 2 ;;
    --only)   ONLY="$(echo "$2" | tr ',' ' ')"; shift 2 ;;
    *) echo "ukjent flagg: $1" >&2; exit 2 ;;
  esac
done
[[ -n "$MODEL" && -n "$OUT" ]] || { echo "trenger --model og --out" >&2; exit 2; }
command -v copilot >/dev/null || { echo "ingen copilot CLI" >&2; exit 2; }

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BODY="$REPO_ROOT/prompts/aksel-component.prompt.md"
[[ -f "$BODY" ]] || { echo "fant ikke $BODY" >&2; exit 2; }

DESC_A='Scaffold en responsiv React-komponent med Aksel Design System, riktige tokens og props verifisert via Aksel MCP / aksel-builder-skillen'
DESC_B="$DESC_A. Bruk KUN når brukeren eksplisitt ber om å scaffolde eller opprette en ny komponent. Ikke bruk for å forklare, gjennomgå, feilsøke eller endre eksisterende komponentkode."

PROBE='forklar hva src/app/komponenter/StatusPanel.tsx gjør og hvilke problemer du ser i den'
BORDERLINE='legg til en lukkeknapp med Aksel-ikon og riktig tilgjengelig navn i src/app/komponenter/StatusPanel.tsx'
POSITIVE='lag en ny React-komponent Varselkort med Aksel'

mkdir -p "$OUT"
RESULTS="$OUT/results.psv"
[[ -f "$RESULTS" ]] || printf 'arm|prompt|run|model|fired|bytes\n' >"$RESULTS"

# Isolert HOME: brukerens config og globale instruksjoner, ingen skills/agents.
HOMEDIR="$OUT/home"; mkdir -p "$HOMEDIR/.copilot"
for f in config.json settings.json permissions-config.json copilot-instructions.md; do
  [[ -f "$HOME/.copilot/$f" ]] && cp "$HOME/.copilot/$f" "$HOMEDIR/.copilot/"
done
TOKEN="${COPILOT_GITHUB_TOKEN:-$(gh auth token 2>/dev/null)}"
[[ -n "$TOKEN" ]] || { echo "ingen GitHub-token (gh auth login)" >&2; exit 2; }

WS="$OUT/ws"
seed_ws() {
  local desc="$1"
  rm -rf "$WS"
  mkdir -p "$WS/.github/skills/aksel-component" "$WS/src/app/komponenter"
  cat >"$WS/src/app/komponenter/StatusPanel.tsx" <<'EOF'
import { useState } from "react";
import { TrashIcon } from "@navikt/aksel-icons";

export function StatusPanel({ status, onSlett }) {
  const [apen, setApen] = useState(false);
  return (
    <div className="p-4 mx-8">
      <div onClick={() => setApen(!apen)} style={{ outline: "none" }}>
        Vis detaljer
      </div>
      <span tabIndex={5} style={{ color: status === "feil" ? "red" : "green" }}>
        {status}
      </span>
      <button onClick={onSlett}>
        <TrashIcon />
      </button>
    </div>
  );
}
EOF
  # Kroppen er promptfila uendret fra og med linja etter frontmatteren.
  { printf -- '---\nname: aksel-component\ndescription: %s\n---\n' "$desc"
    awk 'BEGIN{n=0} /^---$/{n++; next} n>=2' "$BODY"
  } >"$WS/.github/skills/aksel-component/SKILL.md"
}

run_one() {
  local arm="$1" desc="$2" label="$3" prompt="$4" run="$5"
  local out="$OUT/$arm-$label-run$run.txt"
  seed_ws "$desc"
  echo "  → $arm/$label run ${run}…" >&2
  ( cd "$WS" && HOME="$HOMEDIR" GH_TOKEN="$TOKEN" timeout 300 \
      copilot -p "$prompt" --model "$MODEL" --allow-all-tools --no-color --log-level none ) \
      >"$out" 2>"${out%.txt}.err"
  local bytes fired
  bytes="$(wc -c <"$out" | tr -d ' ')"
  if [[ "$bytes" -lt 200 ]]; then
    fired="dead"   # tomt transkript beviser ingenting, verken pass eller fail
  elif grep -qF 'skill(aksel-component)' "$out"; then
    fired="yes"
  else
    fired="no"
  fi
  printf '%s|%s|%s|%s|%s|%s\n' "$arm" "$label" "$run" "$MODEL" "$fired" "$bytes" >>"$RESULTS"
}

for arm in A B; do
  desc="$DESC_A"; [[ "$arm" == B ]] && desc="$DESC_B"
  case " $ONLY " in *" probe "*)
    for i in $(seq 1 "$REPEAT"); do run_one "$arm" "$desc" probe "$PROBE" "$i"; done ;; esac
  case " $ONLY " in *" borderline "*)
    for i in $(seq 1 "$REPEAT"); do run_one "$arm" "$desc" borderline "$BORDERLINE" "$i"; done ;; esac
  case " $ONLY " in *" positive "*)
    for i in $(seq 1 3);        do run_one "$arm" "$desc" positive "$POSITIVE" "$i"; done ;; esac
done

echo
echo "resultat ($MODEL):"
awk -F'|' 'NR>1 {k=$1"/"$2; n[k]++; if($5=="yes") y[k]++; if($5=="dead") d[k]++}
  END {for (k in n) printf "  %-12s fyrte %d/%d%s\n", k, y[k]+0, n[k], (d[k]?" (døde: " d[k] ")":"")}' \
  "$RESULTS" | sort
echo "rå transkripter og results.psv: $OUT"
