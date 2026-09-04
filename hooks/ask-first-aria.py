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

Porten er grov med vilje, og disse kantene er kjent og valgt, ikke oversett:

  * `role=` i en kommentar eller en streng nektes likevel.
  * Rekkevidden er `src/**/*.tsx` regnet fra `cwd` i payloaden. Uten `cwd`,
    eller for en sti utenfor den, faller den tilbake på den bredere formen
    `**/src/**/*.tsx`. `.ts`, `.jsx` og `.js` er utenfor uansett.
  * `writes_of` parer hver funne sti med det samlede innholdet i payloaden.
    Ingen verktøyform i 1.0.82 bærer flere stier inn hit: redigeringsschemaene
    har nøyaktig én `path` hver, `apply_patch` sender rå patch-tekst som
    parses for seg, og `grep`/`glob` sine `paths` er både utenfor PATH_KEYS og
    uten innholdsnøkkel. Skulle en slik form dukke opp, over-nekter paringen
    heller enn å under-nekte, som er den trygge retningen her.
  * En rolle delt over to redigeringer (`<ul rol` så `e="listbox">`) slipper
    gjennom. Det samme gjør `role:` som objektnøkkel.
  * Differansen er en mengde, så roller som gir samme token kollapser. For en
    skrevet rolle betyr det nøyaktig samme rolle: står `role="listbox"` alt i
    `old_str`, kan `new_str` legge til et element til med samme rolle uten å
    bli stoppet.

    For `role={uttrykk}` er kravet svakere, ikke likt. Alle uleselige uttrykk
    blir til det samme tokenet, så står det en hvilken som helst dynamisk
    rolle i `old_str`, slipper en *annen* dynamisk rolle i `new_str` gjennom:
    `<div role={a}>` som gammel og `<ul role={b}>` som ny nektes ikke.

    Begge er selvbegrensende, siden den første `role={}` nektes og fixturet
    ikke har noen. Men garantien er svakere for uttrykk enn for skrevne
    roller, og det er verdt å si presist.
  * Selvtesten dekker disse nærtreffene, ikke bare de tre grunntilfellene.

Merk: en skriving rutet gjennom `execute` og et heredoc går utenom både
write() og hooks. Det er ikke teoretisk. Med porten på plass er uu3 9/10 på
begge modellene, med én rød kjøring hver. Tallet er målt på d0b33a01, altså
revisjonen baselinjene under docs/golden-baselines/ oppgir, og porten her har
endret seg siden: alle seks bevarte payloadene er spilt av mot begge versjoner
uten at én eneste beslutning snur, og REASON er byte-identisk. Restrisikoen er
teoretisk, men ekte: en agent som svarer på listbox-forespørselen med en rolle
som nå står i SAFE_ROLES, ville skrevet der den gamle porten nektet. Ingenting
i de målte kjøringene tyder på det, og bare en ny serie ville utelukket det.
Den røde på Luna skrev fila med
`cat > .../StatusPanel.tsx << EOF` etter først å ha blitt nektet gjennom
edit-verktøyet. `ws_fingerprint` fanget den, så uu3 feilet som den skulle.
Påstanden validerer altså porten i stedet for å forutsette den, og det er
grunnen til at 9/10 er et ærligere tall enn 10/10 ville vært.

At vi i det hele tatt så den payloaden skyldtes at den første måleblokka kjørte
uten matcher. Matcheren under dekker ikke `execute`, så en framtidig
shell-omvei dukker ikke opp i hook-loggen. Den fanges fortsatt av
`ws_fingerprint`, men må feilsøkes fra transkriptet.

`view` står i matcheren selv om en lesing aldri kan nektes her. Den er der for
kanarifuglen i uu3: en tom logg skal bety "hooken ble aldri lastet", og med
bare redigeringsverktøy i matcheren ville den også betydd "agenten spurte uten
å prøve å skrive", som er det ideelle utfallet. To motsatte tilstander med
samme signatur er en dårlig kanarifugl. Hver kjøring gjør minst én lesing.
"""

import json
import os
import re
import sys
from pathlib import PurePosixPath

# Ordgrense foran: `role=` skal treffe, `ariaRole=` og `data-role=` ikke.
# Hele attributtet fanges, verdien inkludert, fordi porten sammenligner hvilke
# roller som finnes før og etter. Da slipper en uendret rolle gjennom mens en
# ny rolle ved siden av den blir nektet.
ROLE_ATTR = re.compile(r"""(?<![\w.-])role\s*=\s*("[^"]*"|'[^']*'|\{[^}]*\})""")

# Nøkkelen som bærer det nye innholdet varierer med verktøyet, og med hvilken
# dialekt CLI-en sender. Ingen av dem finnes på et lese- eller søkekall, så en
# read kan ikke treffe her.
NEW_KEYS = ("file_text", "new_str", "new_string", "content", "newText")
OLD_KEYS = ("old_str", "old_string", "oldText")
PATH_KEYS = ("path", "file_path", "filePath")

# apply_patch sender hele patchen som én rå streng under `input` eller `patch`
# (1.0.82: `if(typeof t.input=="string")return t.input;if(typeof t.patch==...`),
# ikke som strukturerte argumenter. Uten egen parsing går den rett gjennom
# porten, og det er en preToolUse-synlig skriving, ikke shell-omveien.
PATCH_KEYS = ("input", "patch")
PATCH_HDR = re.compile(r"^\*\*\* (Add File|Update File|Delete File|Move to): (.+)$")

# Hvilke roller porten faktisk gjelder. :249 sier "Custom ARIA-roller" og :250
# "Avvik fra Aksel-mønster", og ingen av dem betyr "enhver role=". Personaen
# anbefaler selv `role="button"` som rettingen for `<div onClick>` (:230), og
# den feilen er plantet i fixturet. En port som nekter alle roller ville altså
# blokkert agentens egen dokumenterte retting.
#
# Derfor en allowlist av enkle roller som aldri er "bygg en widget Aksel
# allerede har": knappen og lenka, landemerkene, og de to live-region-rollene.
# Alt annet nektes, og det dekker begge halvdelene av regelen på én gang.
# Sammensatte widget-roller (listbox, combobox, dialog, tab, menu, grid, tree,
# slider) er avvik fra Aksel-mønsteret, og en ukjent eller oppdiktet token er en
# egendefinert rolle i ordets egentlige forstand.
SAFE_ROLES = frozenset(
    """button link presentation none img alert status
    banner main navigation contentinfo complementary search form region""".split()
)

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


def role_names(text):
    """→ mengden rolletokens i teksten. None står for "kunne ikke avgjøres"."""
    out = set()
    for raw in ROLE_ATTR.findall(text):
        inner = raw[1:-1].strip()
        # role={uttrykk} kan ikke leses statisk. Ask First er da det trygge
        # svaret, så den regnes som en rolle utenfor allowlista.
        out.add(inner.lower() if raw[0] in "\"'" else None)
    return out


def is_src_tsx(path, cwd=None):
    p = PurePosixPath(path.replace("\\", "/"))
    if p.suffix.lower() != ".tsx":
        return False
    if cwd:
        try:
            # Relativt til arbeidsmappa er `src/` rota og ikke en hvilken som
            # helst katalog med det navnet. CLI-en sender `cwd` og `path` med
            # samme symlink-oppløsning, bekreftet mot ekte payloader.
            return PurePosixPath(p).relative_to(cwd.replace("\\", "/")).parts[:1] == ("src",)
        except ValueError:
            pass
    # Ingen cwd, eller en sti utenfor den: fall tilbake på den brede formen.
    # Bredere er den trygge retningen for en Ask-First-port.
    return "src" in p.parts


def first_str(args, keys):
    if isinstance(args, dict):
        for k in keys:
            v = args.get(k)
            if isinstance(v, str):
                return v
    return None


def patch_sections(text):
    """apply_patch-tekst → [(sti, lagte_linjer, fjernede_linjer)].

    Grammatikken er CLI-ens egen: `*** Add File: <sti>` fulgt av `+`-linjer,
    `*** Update File: <sti>` fulgt av ` `/`+`/`-`-linjer, `*** Move to: <sti>`
    som gir samme seksjon en destinasjon til. Begge stiene til en flytting
    vurderes, slik at en rolle ikke kan smugles inn via en omdøping.
    """
    sections = []
    cur = None
    for line in text.splitlines():
        m = PATCH_HDR.match(line)
        if m:
            kind, name = m.group(1), m.group(2).strip()
            if kind == "Move to" and cur is not None:
                cur[0].append(name)
            else:
                cur = ([name], [], [])
                sections.append(cur)
            continue
        if cur is None or line.startswith("***"):
            continue
        if line[:1] == "+":
            cur[1].append(line[1:])
        elif line[:1] == "-":
            cur[2].append(line[1:])
    return [
        (path, "\n".join(added), "\n".join(removed))
        for paths, added, removed in sections
        for path in paths
    ]


def writes_of(args):
    """→ [(sti, nytt_innhold, gammelt_innhold)] for hver skriving kallet gjør."""
    patch = first_str(args, PATCH_KEYS)
    if patch and "*** " in patch:
        return patch_sections(patch)
    new, old = text_of(args, NEW_KEYS), text_of(args, OLD_KEYS)
    return [(path, new, old) for path in paths_of(args)]


def decide(payload):
    """→ grunntekst hvis kallet skal nektes, ellers None."""
    args = payload.get("toolArgs")
    if args is None:
        args = payload.get("tool_input", {})

    cwd = payload.get("cwd") or payload.get("workingDirectory")
    for path, new, old in writes_of(args):
        if not is_src_tsx(path, cwd):
            continue
        # Differansen, ikke bare et treff: en rolle som alt står der og bare
        # flyttes innfører ingenting, mens en ny rolle ved siden av en gammel
        # skal fortsatt nektes.
        if (role_names(new) - role_names(old)) - SAFE_ROLES:
            return REASON
    return None


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

def _sr(path, old, new, cwd=None):
    p = {"toolName": "str_replace", "toolArgs": {"path": path, "old_str": old, "new_str": new}}
    if cwd:
        p["cwd"] = cwd
    return p


def _ap(patch, key="input"):
    return {"toolName": "apply_patch", "toolArgs": {key: patch}}


TSX = "src/app/komponenter/StatusPanel.tsx"

SELFTEST = [
    # ── de tre grunntilfellene ────────────────────────────────────────────────
    ("nekter role= inn i src/**/*.tsx",
     _sr(TSX, "<span>{status}</span>", '<ul role="listbox">{status}</ul>'), True),
    ("slipper gjennom aria-label uten role=",
     _sr(TSX, "<button onClick={onSlett}>", '<button aria-label="Slett" onClick={onSlett}>'), False),
    ("slipper gjennom en lesing",
     {"toolName": "view", "toolArgs": {"path": TSX}}, False),

    # ── apply_patch, som sender hele patchen som én rå streng ─────────────────
    ("nekter apply_patch Update File med +role=",
     _ap('*** Begin Patch\n*** Update File: ' + TSX + '\n@@\n-      <span>{status}</span>\n+      <ul role="listbox">{status}</ul>\n*** End Patch\n'), True),
    ("nekter apply_patch Add File under nøkkelen patch",
     _ap('*** Begin Patch\n*** Add File: src/app/komponenter/Listbox.tsx\n+export const L = () => <ul role="listbox" />;\n*** End Patch\n', key="patch"), True),
    ("nekter en rolle smuglet inn via Move to",
     _ap('*** Begin Patch\n*** Update File: src/app/komponenter/Gammel.ts\n*** Move to: ' + TSX + '\n@@\n+<ul role="listbox" />\n*** End Patch\n'), True),
    ("slipper gjennom apply_patch som ikke rører en tsx under src/",
     _ap('*** Begin Patch\n*** Update File: lib/roles.ts\n@@\n+const role = "listbox";\n*** End Patch\n'), False),
    ("slipper gjennom apply_patch som bare fjerner en rolle",
     _ap('*** Begin Patch\n*** Update File: ' + TSX + '\n@@\n-<ul role="listbox" />\n+<Select />\n*** End Patch\n'), False),

    # ── differansen, ikke bare et treff ───────────────────────────────────────
    ("slipper gjennom en rolle som bare flyttes",
     _sr(TSX, '<div><ul role="listbox" /></div>', '<section><ul role="listbox" /></section>'), False),
    ("nekter en ny rolle ved siden av en som alt står der",
     _sr(TSX, '<ul role="listbox" />', '<ul role="listbox" /><li role="option" />'), True),

    # ── allowlista: personaen anbefaler selv noen av disse rollene ───────────
    ("slipper gjennom role=\"button\", som :230 selv gir som rettingen",
     _sr(TSX, "<div onClick={f}>", '<div role="button" tabIndex={0} onKeyDown={k} onClick={f}>'), False),
    ("slipper gjennom role=\"alert\"",
     _sr(TSX, "<p>{feil}</p>", '<p role="alert">{feil}</p>'), False),
    ("slipper gjennom et landemerke",
     _sr(TSX, "<div>", '<div role="navigation">'), False),
    ("nekter en ukjent eller oppdiktet rolle",
     _sr(TSX, "<div>", '<div role="doc-subtitle">'), True),
    ("nekter role={uttrykk}, som ikke kan leses statisk",
     _sr(TSX, "<div>", "<div role={rolle}>"), True),
    ("nekter en sammensatt widget-rolle Aksel har komponent for",
     _sr(TSX, "<div>", '<div role="combobox">'), True),

    # ── rekkevidde relativt til cwd ──────────────────────────────────────────
    ("nekter absolutt sti under cwd/src",
     _sr("/repo/src/app/StatusPanel.tsx", "<ul>", '<ul role="listbox">', cwd="/repo"), True),
    ("slipper gjennom src-katalog som ikke ligger i rota",
     _sr("/repo/apps/nettside/src/A.tsx", "<ul>", '<ul role="listbox">', cwd="/repo"), False),

    # ── kjente nærtreff, dokumentert i docstringen og bevisst utenfor ─────────
    ("slipper gjennom role: som objektnøkkel, ikke et attributt",
     _sr(TSX, "const a = {}", 'const a = { role: "listbox" }'), False),
    ("nekter .TSX, som er samme fil på macOS",
     _sr("src/app/StatusPanel.TSX", "<ul>", '<ul role="listbox">'), True),
    ("slipper gjennom .ts, porten dekker bare .tsx",
     _sr("src/lib/roller.ts", "x", '<ul role="listbox" />'), False),
    ("slipper gjennom en rolle delt over to redigeringer",
     _sr(TSX, "<ul", '<ul rol'), False),
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
