#!/usr/bin/env python3
"""preToolUse-gate: en egendefinert ARIA-rolle i src/**/*.tsx skal spørres om.

`accessibility.agent.md:249` merker "Custom ARIA-roller" som ⚠️ Ask First.
Målt i golden-harnesset 31. august 2026 skrev agenten filen likevel, 0/5 på
både Claude Sonnet 4.6 og GPT-5.6 Luna (#517). Tre omformuleringer av en
tilsvarende regel ga null målbar effekt over ~150 kall (#484), så regelen
flyttes ut av personaen og inn i verktøylaget: her kan den ikke ignoreres.

`permissionDecision: "deny"` framfor "ask": "ask" spør interaktivt, og
oppførselen i `-p` er udokumentert. "deny" er lik i begge modus, og
`permissionDecisionReason` når fram til modellen. Blokkeringen blir dermed
selve oppfordringen om å spørre.

Kontrakten er lest ut av GitHub Copilot CLI 1.0.82 sin egen bundle:
  inn   camelCase-hendelsesnavn gir camelCase-payload (toolName/toolArgs),
        PascalCase gir VS Code-formen (tool_name/tool_input). Begge leses.
  verktøy  edit/str_replace {path, old_str, new_str}, create {path, file_text},
        str_replace_editor {command, path, ...}
  ut    både flat form (CLI-ens egen SDK-form) og hookSpecificOutput
        (Claude Code-formen). CLI-en oppgir å godta begge dialektene.

Merk: en skriving rutet gjennom `execute` og et heredoc går utenom både
write() og hooks. `ws_fingerprint` i golden-harnesset fanger den likevel, så
uu3 feiler fortsatt om modellen går rundt porten. Påstanden validerer altså
porten i stedet for å forutsette den.
"""

import json
import os
import re
import sys
from pathlib import PurePosixPath

# Ordgrense foran: `role=` skal treffe, `ariaRole=` og `data-role=` ikke.
# Etterfulgt av ", ' eller { dekker både role="listbox" og role={rolle}.
ROLE_ATTR = re.compile(r"(?<![\w.-])role\s*=\s*[\"'{]")

# Nøkkelen som bærer det nye innholdet varierer med verktøyet, og med
# hvilken dialekt CLI-en sender. Alle leses; ingen av dem finnes på et lese-
# eller søkekall, så en read kan ikke treffe her.
NEW_KEYS = ("file_text", "new_str", "new_string", "content", "newText")
OLD_KEYS = ("old_str", "old_string", "oldText")
PATH_KEYS = ("path", "file_path", "filePath")

REASON = (
    "En egendefinert ARIA-rolle er ⚠️ Ask First i accessibility.agent.md:249, "
    "og et avvik fra Aksel-mønsteret er det samme på :250. Ikke skriv endringen "
    "selv. Spør utvikleren om bekreftelse først, eller vis til Aksel-komponenten "
    "som allerede har rollen innebygd: <Select> for et statusvalg, eller "
    "<UNSAFE_Combobox> når valget skal kunne søkes i. Aksel-komponentene har "
    "tastaturnavigasjon og skjermleserstøtte fra før, så en egendefinert "
    "role=\"listbox\" må begrunnes mot dem for å være verdt det."
)


def text_of(args, keys):
    """Slår sammen alle strengverdiene under `keys`, uansett nesting."""
    out = []
    if isinstance(args, dict):
        for k, v in args.items():
            if k in keys and isinstance(v, str):
                out.append(v)
            else:
                out.append(text_of(v, keys))
    elif isinstance(args, list):
        out.extend(text_of(v, keys) for v in args)
    return "\n".join(s for s in out if s)


def paths_of(args):
    out = []
    if isinstance(args, dict):
        for k, v in args.items():
            if k in PATH_KEYS and isinstance(v, str):
                out.append(v)
            else:
                out.extend(paths_of(v))
    elif isinstance(args, list):
        for v in args:
            out.extend(paths_of(v))
    return out


def is_src_tsx(path):
    p = PurePosixPath(path.replace("\\", "/"))
    return p.suffix == ".tsx" and "src" in p.parts


def decide(payload):
    """→ grunntekst hvis kallet skal nektes, ellers None."""
    args = payload.get("toolArgs")
    if args is None:
        args = payload.get("tool_input", {})

    if not any(is_src_tsx(p) for p in paths_of(args)):
        return None

    new = text_of(args, NEW_KEYS)
    if not ROLE_ATTR.search(new):
        return None

    # En str_replace som bare flytter en rolle som alt står der, innfører
    # ingenting. Porten gjelder det å innføre rollen, ikke å røre den.
    if ROLE_ATTR.search(text_of(args, OLD_KEYS)):
        return None

    return REASON


def main():
    try:
        raw = sys.stdin.read()
        # Sett NAV_PILOT_HOOK_DEBUG=<fil> for å få hver payload logget. Uten den
        # er "porten lastet ikke" og "porten traff ikke" samme observasjon fra
        # utsiden, og det er nettopp den forskjellen som avgjør om oppsettet i
        # det hele tatt virker i `-p`. Logg utenfor arbeidsmappa: golden-
        # harnesset fingeravtrykker den, og en loggfil inni ville telt som en
        # skriving fra agenten.
        debug = os.environ.get("NAV_PILOT_HOOK_DEBUG")
        if debug:
            with open(debug, "a", encoding="utf8") as fh:
                fh.write(raw.rstrip("\n") + "\n")
        payload = json.loads(raw)
        reason = decide(payload) if isinstance(payload, dict) else None
    except Exception:
        # Fail-open. En preToolUse-hook som feiler nekter kallet (1.0.82), og
        # en port som nekter *alt* er verre enn ingen port: den ville gjort
        # uu3 grønn av feil grunn og brekt alt annet agenten gjør.
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
# Kjører skriptet som subprosess med ekte stdin, ikke bare decide(), slik at
# JSON-inn, JSON-ut og exitkoden er dekket. `mise run hooks:test`.

SELFTEST = [
    (
        "nekter role= inn i src/**/*.tsx",
        {
            "hookEventName": "preToolUse",
            "toolName": "str_replace",
            "toolArgs": {
                "path": "src/app/komponenter/StatusPanel.tsx",
                "old_str": "<span>{status}</span>",
                "new_str": '<ul role="listbox">{status}</ul>',
            },
        },
        True,
    ),
    (
        "slipper gjennom aria-label uten role=",
        {
            "hookEventName": "preToolUse",
            "toolName": "str_replace",
            "toolArgs": {
                "path": "src/app/komponenter/StatusPanel.tsx",
                "old_str": "<button onClick={onSlett}>",
                "new_str": '<button aria-label="Slett" onClick={onSlett}>',
            },
        },
        False,
    ),
    (
        "slipper gjennom en lesing",
        {
            "hookEventName": "preToolUse",
            "toolName": "view",
            "toolArgs": {"path": "src/app/komponenter/StatusPanel.tsx"},
        },
        False,
    ),
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
