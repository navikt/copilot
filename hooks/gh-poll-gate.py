#!/usr/bin/env python3
"""preToolUse-gate: å vente på GitHub Actions ved å spørre gjentatte ganger nektes.

`gh` kan blokkere til et kjør er ferdig. `gh run watch <id> --exit-status` og
`gh pr checks <nr> --watch --fail-fast` returnerer først når svaret finnes, og
exitkoden bærer resultatet. En løkke som sover og spør på nytt gjør det samme
arbeidet dyrere: hver runde legger et verktøysvar i konteksten, og gjør agenten
ett kall per runde, også en modellrunde.

Porten finnes fordi mønsteret er lett å skrive og vanskelig å se. Den som
skriver løkka får riktig svar til slutt, så ingenting føles galt.

## Hva som nektes

Tre former, alle målt mot ekte kommandoer og ikke gjettet:

  1. En løkke (`for`/`while`/`until`) som inneholder både `sleep` og et
     spørrende `gh`-kall.
  2. `sleep N` etterfulgt av et spørrende `gh`-kall i samme kommando, med `;`,
     `&&` eller linjeskift imellom. Den formen er én runde av samme løkke,
     skrevet ut for hånd.
  3. `gh ... --watch` med `| head`, `| tail` eller liknende avkorting foran en
     løkke. Da er watch-en der, men resultatet kastes og løkka gjør jobben.

Et *spørrende* kall er `gh run list`, `gh run view`, `gh pr view`, `gh pr
checks` uten `--watch`, `gh api .../actions/runs`. Et blokkerende kall er
`gh run watch` og `gh pr checks --watch`. Porten teller bare de spørrende.

## Hva som slipper gjennom

Et enkelt statusoppslag. `gh run list` én gang er hvordan man finner ID-en å
watche, og å nekte den ville gjort porten til et hinder framfor en rettelse.
Porten krever gjentakelse: løkke, eller sleep rett foran.

Å vente på noe `gh` ikke kan watche — en deploy-URL som svarer, en kø hos en
tredjepart — er legitimt. `POLL_OK=1` foran kommandoen slipper gjennom.
Prefikset er ankret til kommandostart, som i `slop-gate.py`, så det ikke kan
gjemmes inne i en lengre kjede.

## Kjente kanter, valgt og ikke oversett

  * Porten leser tekst, ikke shell-grammatikk. En løkke som henter status via
    et skript porten ikke ser inn i, slipper gjennom. Det er greit: porten skal
    fange den vanlige formen, ikke være et bevis.
  * `sleep` alene nektes ikke. Å sove før en enkelt sjekk er ikke polling.
  * En `gh run watch` inne i en løkke slipper gjennom. Løkka er da sannsynligvis
    over flere kjør, som er riktig bruk.

Matcheren er `bash|shell|execute`, samme som `klarsprak-gate.py`, av samme
grunn: `bash` er det målte navnet i Copilot CLI 1.0.83, `shell` og `execute`
koster ingenting og gjør porten robust mot en omdøping.
"""

import json
import os
import re
import sys

# Et kall som spør om tilstand og returnerer med en gang.
ASKS = re.compile(
    r"\bgh\s+(?:"
    r"run\s+(?:list|view)|"
    r"pr\s+(?:view|status|checks)|"
    r"api\b[^\n|;&]*\bactions/runs|"
    r"workflow\s+view"
    r")\b"
)

# Et kall som blokkerer til svaret finnes. Har kommandoen en av disse og ingen
# spørrende gjentakelse, er den allerede riktig.
BLOCKS = re.compile(r"\bgh\s+(?:run\s+watch|pr\s+checks[^\n;&|]*--watch)\b")

LOOP = re.compile(r"(^|[;&|]\s*|\n\s*)(for|while|until)\s")
SLEEP = re.compile(r"\bsleep\s+[\d.]+")

# sleep og et spørrende kall i samme kjede, i den rekkefølgen: én runde av
# løkka, skrevet ut for hånd.
SLEEP_THEN_ASK = re.compile(
    r"\bsleep\s+[\d.]+\s*(?:;|&&|\n)\s*[^\n]{0,120}?\bgh\s+(?:run|pr|api|workflow)\b"
)

POLL_OK = re.compile(r"^\s*POLL_OK=1\s")

REASON = (
    "Dette venter på GitHub Actions ved å spørre om igjen. `gh` kan blokkere i "
    "stedet, og da koster ventingen ett verktøysvar totalt framfor ett per runde:\n\n"
    "  gh run watch <run-id> --exit-status --compact >/dev/null; echo $?\n"
    "  gh pr checks <nr> --watch --fail-fast\n\n"
    "Begge returnerer først når svaret finnes, og exitkoden bærer resultatet, så "
    "utdata kan kastes. Trenger du run-id-en: `gh run list --commit $(git rev-parse HEAD) "
    "--json databaseId,name,status`, én gang.\n\n"
    "Venter du på noe `gh` ikke kan watche — en deploy som svarer, en kø hos en "
    "tredjepart — er polling riktig. Sett `POLL_OK=1` foran kommandoen."
)


def command_text(node, out):
    """Samler kommandotekst rekursivt. Samme form som klarsprak-gate.py."""
    if isinstance(node, dict):
        for key, value in node.items():
            if key in ("command", "commandLine", "cmd", "script") and isinstance(value, str):
                out.append(value)
            else:
                command_text(value, out)
    elif isinstance(node, list):
        for item in node:
            command_text(item, out)
    return out


def decide(payload):
    tool = str(payload.get("toolName") or payload.get("tool_name") or "")
    if not re.search(r"bash|shell|execute", tool, re.I):
        return None

    args = payload.get("toolArgs") or payload.get("tool_input") or {}
    for command in command_text(args, []):
        if not command.strip() or POLL_OK.match(command):
            continue
        if not ASKS.search(command):
            continue
        # En blokkerende form er allerede svaret, med mindre den står sammen med
        # en spørrende løkke — da er watch-en pynt og løkka gjør jobben.
        looped = bool(LOOP.search(command) and SLEEP.search(command))
        handrolled = bool(SLEEP_THEN_ASK.search(command))
        if not (looped or handrolled):
            continue
        if BLOCKS.search(command) and not looped:
            continue
        return REASON
    return None


def main():
    try:
        raw = sys.stdin.read()
        debug = os.environ.get("NAV_PILOT_HOOK_DEBUG")
        if debug:
            with open(debug, "a", encoding="utf8") as fh:
                fh.write(raw.rstrip("\n") + "\n")
        payload = json.loads(raw)
        reason = decide(payload) if isinstance(payload, dict) else None
    except Exception:
        # Fail-open, som de to andre portene: en preToolUse-hook som feiler
        # nekter kallet, og en port som nekter alt er verre enn ingen port.
        reason = None

    if reason:
        json.dump(
            {
                "permissionDecision": "deny",
                "permissionDecisionReason": reason,
                "hookSpecificOutput": {
                    "hookEventName": "preToolUse",
                    "permissionDecision": "deny",
                    "permissionDecisionReason": reason,
                },
            },
            sys.stdout,
        )
    sys.exit(0)


# ─── Selvtest ────────────────────────────────────────────────────────────────
# Kjører skriptet som subprosess med ekte stdin. `mise run hooks:test`.

def _sh(command, tool="shell"):
    return {"toolName": tool, "toolArgs": {"command": command}}


SELFTEST = [
    # ── nektes: løkka ───────────────────────────────────────────────────────
    ("for-løkke som sover og lister kjør",
     _sh('for i in $(seq 1 60); do gh run list --json status; sleep 30; done'), True),
    ("while-løkke over pr view",
     _sh('while true; do gh pr view 1 --json state; sleep 20; done'), True),
    ("until-løkke over pr checks uten --watch",
     _sh('until gh pr checks 1 | grep -q pass; do sleep 15; done'), True),
    ("løkke over actions-API-et",
     _sh('for i in 1 2 3; do gh api repos/o/r/actions/runs; sleep 10; done'), True),

    # ── nektes: én runde skrevet ut for hånd ────────────────────────────────
    ("sleep foran et statusoppslag",
     _sh('sleep 25; gh run list --commit abc --json databaseId'), True),
    ("sleep med && foran pr view",
     _sh('sleep 30 && gh pr view 688 --json mergeStateStatus'), True),

    # ── nektes: watch som pynt foran ei løkke ───────────────────────────────
    ("watch avkortet, løkka gjør jobben",
     _sh('gh run watch 1 --compact | head -1; for i in 1 2; do gh run list; sleep 5; done'), True),

    # ── slipper gjennom: den riktige formen ─────────────────────────────────
    ("gh run watch alene",
     _sh('gh run watch 34095675784 --exit-status --compact >/dev/null; echo $?'), False),
    ("pr checks --watch",
     _sh('gh pr checks 688 --watch --fail-fast'), False),
    ("pr checks --watch med tail",
     _sh('gh pr checks 688 --watch --required 2>&1 | tail -10'), False),

    # ── slipper gjennom: ett oppslag er ikke polling ────────────────────────
    ("ett run list for å finne id-en",
     _sh('gh run list --commit $(git rev-parse HEAD) --json databaseId,name,status'), False),
    ("ett pr view",
     _sh('gh pr view 688 --json mergeStateStatus --jq .mergeStateStatus'), False),
    ("sleep uten gh etterpå",
     _sh('sleep 5; make build'), False),
    ("løkke uten sleep",
     _sh('for n in 1 2 3; do gh pr view $n --json state; done'), False),
    ("løkke over noe annet enn gh",
     _sh('for i in 1 2; do curl -s https://example.com; sleep 5; done'), False),

    # ── escape hatch ────────────────────────────────────────────────────────
    ("POLL_OK=1 slipper gjennom",
     _sh('POLL_OK=1 for i in 1 2; do gh run list; sleep 10; done'), False),
    ("POLL_OK må stå først",
     _sh('echo POLL_OK=1 && for i in 1 2; do gh run list; sleep 10; done'), True),

    # ── ikke vår sak ────────────────────────────────────────────────────────
    ("annet verktøy enn shell",
     {"toolName": "str_replace_editor", "toolArgs": {"command": "for i in 1 2; do gh run list; sleep 5; done"}}, False),
    ("tom kommando",
     _sh(''), False),
]


def selftest():
    import subprocess

    failed = 0
    for name, payload, want_deny in SELFTEST:
        p = subprocess.run(
            [sys.executable, __file__],
            input=json.dumps(payload),
            capture_output=True,
            text=True,
        )
        got_deny = False
        if p.stdout.strip():
            got_deny = json.loads(p.stdout).get("permissionDecision") == "deny"
        ok = p.returncode == 0 and got_deny == want_deny
        print(f"{'✅' if ok else '❌'} {name}")
        if not ok:
            failed += 1
            print(f"   exit={p.returncode} deny={got_deny} want={want_deny}")
            print(f"   stdout={p.stdout!r} stderr={p.stderr!r}")
    print(f"\n{len(SELFTEST) - failed}/{len(SELFTEST)} ok")
    return 1 if failed else 0


if __name__ == "__main__":
    if "--selftest" in sys.argv:
        sys.exit(selftest())
    main()
