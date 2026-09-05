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
#   ./scripts/skill-selvutlosning-probe.sh --subject nais-manifest --model gpt-5.6-luna --only borderline,positive --out /tmp/probe2
#
# TO SUBJEKTER, fordi de sju promptene ikke er samme form:
#   aksel-component  stillasgenerator — borderline er «endre en eksisterende komponent»
#   nais-manifest    manifestredigerer — borderline er FEILSØKING av en deploy som
#                    allerede finnes, der en skill som fyrer kan skrive i manifestet
#                    i stedet for å svare på spørsmålet. Derfor logges også `wrote`:
#                    endret kjøringen fixture-manifestet på disk?
set -uo pipefail

MODEL=""; REPEAT=5; POSREPEAT=3; OUT=""; ONLY="probe borderline positive"; SUBJECT="aksel-component"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --model)  MODEL="$2"; shift 2 ;;
    --repeat) REPEAT="$2"; shift 2 ;;
    --pos-repeat) POSREPEAT="$2"; shift 2 ;;
    --out)    OUT="$2"; shift 2 ;;
    --only)   ONLY="$(echo "$2" | tr ',' ' ')"; shift 2 ;;
    --subject) SUBJECT="$2"; shift 2 ;;
    *) echo "ukjent flagg: $1" >&2; exit 2 ;;
  esac
done
[[ -n "$MODEL" && -n "$OUT" ]] || { echo "trenger --model og --out" >&2; exit 2; }
command -v copilot >/dev/null || { echo "ingen copilot CLI" >&2; exit 2; }

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BODY="$REPO_ROOT/prompts/$SUBJECT.prompt.md"
[[ -f "$BODY" ]] || { echo "fant ikke $BODY" >&2; exit 2; }

case "$SUBJECT" in
aksel-component)
  DESC_A='Scaffold en responsiv React-komponent med Aksel Design System, riktige tokens og props verifisert via Aksel MCP / aksel-builder-skillen'
  GUARD='Bruk KUN når brukeren eksplisitt ber om å scaffolde eller opprette en ny komponent. Ikke bruk for å forklare, gjennomgå, feilsøke eller endre eksisterende komponentkode.'
  PROBE='forklar hva src/app/komponenter/StatusPanel.tsx gjør og hvilke problemer du ser i den'
  BORDERLINE='legg til en lukkeknapp med Aksel-ikon og riktig tilgjengelig navn i src/app/komponenter/StatusPanel.tsx'
  POSITIVE='lag en ny React-komponent Varselkort med Aksel'
  ;;
nais-manifest)
  DESC_A='Generer et produksjonsklart Nais-applikasjonsmanifest for Kubernetes-deployment'
  # Guarden må adressere feilsøking, ikke bare scaffolding: misfyringen her utløses
  # av et SPØRSMÅL om et manifest som finnes, ikke av en forespørsel om et nytt.
  GUARD='Bruk KUN når brukeren eksplisitt ber om å opprette eller generere et nytt Nais-manifest. Ikke bruk for å forklare, gjennomgå eller feilsøke et eksisterende manifest eller en deploy som feiler — svar da på spørsmålet uten å endre manifestet.'
  PROBE='hva betyr replicas-blokka i .nais/app.yaml?'
  BORDERLINE='deployen av dp-soknad feiler — poden starter og blir drept igjen og igjen kort tid etter oppstart. hvorfor skjer det?'
  POSITIVE='lag et nais-manifest for en ny app dp-vedtak i team-dagpenger'
  ;;
*) echo "ukjent --subject: $SUBJECT" >&2; exit 2 ;;
esac
DESC_B="$DESC_A. $GUARD"

mkdir -p "$OUT"
RESULTS="$OUT/results.psv"
[[ -f "$RESULTS" ]] || printf 'subject|arm|prompt|run|model|fired|bytes|wrote\n' >"$RESULTS"

# Isolert HOME: brukerens config og globale instruksjoner, ingen skills/agents.
HOMEDIR="$OUT/home"; mkdir -p "$HOMEDIR/.copilot"
for f in config.json settings.json permissions-config.json copilot-instructions.md; do
  [[ -f "$HOME/.copilot/$f" ]] && cp "$HOME/.copilot/$f" "$HOMEDIR/.copilot/"
done
# gh-guarden i cplt er på som standard (standard-presetet, cplt#335) og blokkerer
# både `gh auth token` (block_auth_token, default true) og `gh auth login`. Guarden
# skriver forklaringen på stderr, så den må vises i stedet for å kastes: uten den
# ser en blokkering ut som «ikke innlogget», og rådet blir feil.
TOKEN="${COPILOT_GITHUB_TOKEN:-${GH_TOKEN:-${GITHUB_TOKEN:-}}}"
if [[ -z "$TOKEN" ]]; then
  # $OUT finnes allerede (mkdir over), så feilteksten trenger ingen mktemp som
  # kan feile og etterlate et tomt filnavn i redirecten.
  GH_ERR="$OUT/gh-auth-token.err"
  TOKEN="$(gh auth token 2>"$GH_ERR")" || true
  if [[ -z "$TOKEN" ]]; then
    echo "ingen GitHub-token." >&2
    if [[ -s "$GH_ERR" ]]; then
      echo "gh svarte:" >&2
      sed 's/^/  /' "$GH_ERR" >&2
    fi
    echo "Sett COPILOT_GITHUB_TOKEN, GH_TOKEN eller GITHUB_TOKEN." >&2
    echo "Inne i en cplt-sandkasse er både 'gh auth token' og 'gh auth login' blokkert av gh-guarden; logg inn utenfor sandkassen, eller sett gh_guard.inject_token = true slik at cplt injiserer GH_TOKEN." >&2
    exit 2
  fi
  rm -f "$GH_ERR"
fi

WS="$OUT/ws"
FIXTURE=""   # fila en feilutløsning kan komme til å skrive i
seed_ws() {
  local desc="$1"
  rm -rf "$WS"
  mkdir -p "$WS/.github/skills/$SUBJECT"
  if [[ "$SUBJECT" == aksel-component ]]; then
    FIXTURE="$WS/src/app/komponenter/StatusPanel.tsx"
    mkdir -p "$WS/src/app/komponenter"
    cat >"$FIXTURE" <<'EOF'
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
  else
    # Et manifest som ALLEREDE finnes, med en plausibel årsak til restart-loop:
    # readiness peker på en sti appen ikke har, og minnegrensa er urealistisk lav.
    FIXTURE="$WS/.nais/app.yaml"
    mkdir -p "$WS/.nais"
    cat >"$FIXTURE" <<'EOF'
apiVersion: nais.io/v1alpha1
kind: Application
metadata:
  name: dp-soknad
  namespace: team-dagpenger
  labels:
    team: team-dagpenger
spec:
  image: {{image}}
  port: 8080
  liveness:
    path: /internal/health
    initialDelay: 1
    timeout: 1
  readiness:
    path: /isready
    initialDelay: 1
    timeout: 1
  resources:
    requests:
      cpu: 50m
      memory: 256Mi
    limits:
      memory: 64Mi
  replicas:
    min: 2
    max: 4
    cpuThresholdPercentage: 80
EOF
  fi
  # Kroppen er promptfila uendret fra og med linja etter frontmatteren.
  { printf -- '---\nname: %s\ndescription: %s\n---\n' "$SUBJECT" "$desc"
    awk 'BEGIN{n=0} /^---$/{n++; next} n>=2' "$BODY"
  } >"$WS/.github/skills/$SUBJECT/SKILL.md"
}

run_one() {
  local arm="$1" desc="$2" label="$3" prompt="$4" run="$5"
  local out="$OUT/$arm-$label-run$run.txt"
  seed_ws "$desc"
  local before after wrote
  before="$(shasum "$FIXTURE" | cut -d' ' -f1)"
  echo "  → $arm/$label run ${run}…" >&2
  ( cd "$WS" && HOME="$HOMEDIR" GH_TOKEN="$TOKEN" timeout 300 \
      copilot -p "$prompt" --model "$MODEL" --allow-all-tools --no-color --log-level none ) \
      >"$out" 2>"${out%.txt}.err"
  local bytes fired
  after="$(shasum "$FIXTURE" 2>/dev/null | cut -d' ' -f1)"
  wrote=no; [[ "$before" != "$after" ]] && wrote=yes
  bytes="$(wc -c <"$out" | tr -d ' ')"
  if [[ "$bytes" -lt 200 ]]; then
    fired="dead"   # tomt transkript beviser ingenting, verken pass eller fail
  elif grep -qF "skill($SUBJECT)" "$out"; then
    fired="yes"
  else
    fired="no"
  fi
  printf '%s|%s|%s|%s|%s|%s|%s|%s\n' "$SUBJECT" "$arm" "$label" "$run" "$MODEL" "$fired" "$bytes" "$wrote" >>"$RESULTS"
}

for arm in A B; do
  desc="$DESC_A"; [[ "$arm" == B ]] && desc="$DESC_B"
  case " $ONLY " in *" probe "*)
    for i in $(seq 1 "$REPEAT"); do run_one "$arm" "$desc" probe "$PROBE" "$i"; done ;; esac
  case " $ONLY " in *" borderline "*)
    for i in $(seq 1 "$REPEAT"); do run_one "$arm" "$desc" borderline "$BORDERLINE" "$i"; done ;; esac
  case " $ONLY " in *" positive "*)
    for i in $(seq 1 "$POSREPEAT"); do run_one "$arm" "$desc" positive "$POSITIVE" "$i"; done ;; esac
done

echo
echo "resultat ($MODEL):"
awk -F'|' 'NR>1 {k=$1"/"$2"/"$3; n[k]++; if($6=="yes") y[k]++; if($6=="dead") d[k]++; if($8=="yes") w[k]++}
  END {for (k in n) printf "  %-30s fyrte %d/%d, skrev i fixture %d/%d%s\n", k, y[k]+0, n[k], w[k]+0, n[k], (d[k]?" (døde: " d[k] ")":"")}' \
  "$RESULTS" | sort
echo "rå transkripter og results.psv: $OUT"
