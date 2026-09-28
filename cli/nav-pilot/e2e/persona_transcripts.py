#!/usr/bin/env python3
"""Turn `go test ./e2e -run TestScripts -v` output into transcripts for a persona review.

Reads the test log on stdin and prints Markdown: one section per journey with
each command, its [stdout]/[stderr] and nothing else. Assertion lines, the
script's # comments and the environment header are dropped, as the "Reviewing
a transcript with the UX rubric" section of e2e/README.md asks: they tell the
reviewer what to expect.

    go test ./e2e -run 'TestScripts/alpha_' -v | python3 e2e/persona_transcripts.py [--max CHARS]
    python3 e2e/persona_transcripts.py --selftest
"""

import re
import sys

# go test -v prints "=== NAME" before a test's log only when another test
# printed last; a lone or first journey follows its "=== RUN"/"=== CONT".
NAME = re.compile(r"^=== (?:RUN|CONT|NAME)\s+TestScripts/(\S+)$")
# testscript's own checks on output and files. Everything else a script runs
# (exec, exits, pty-run, ttyin, fake-*, cp, mkdir) is something the user did
# or the setup they did it in, so it stays.
ASSERT = re.compile(r"^> (! )?(stdout|stderr|cmp|cmpenv|exists|grep|validjson|stop|skip)\b")


def transcripts(log):
    out, name, body, in_env = [], None, [], False

    def flush():
        if name and body:
            out.append((name, body[:]))

    for raw in log.splitlines():
        m = NAME.match(raw)
        if m:
            flush()
            name, body, in_env = m.group(1), [], True
            continue
        if name is None:
            continue
        if raw.startswith(("=== ", "--- ", "PASS", "FAIL", "ok ")):
            flush()
            name, body = None, []
            continue
        # The log is indented 8 spaces under the test name; the first line
        # instead carries 4 and "testscript.go:NNN: ". Output keeps its own indent.
        line = re.sub(r"^    (testscript\.go:\d+: |    )", "", raw, count=1)
        if in_env:
            in_env = line.strip() != ""
            continue
        if line.startswith("#") or ASSERT.match(line):
            continue
        if line in ("PASS", "FAIL"):
            continue
        # A terminal escape in a Markdown code block renders as nothing.
        body.append(line.replace("\x1b", "\\e"))
    flush()
    return out


def markdown(ts, max_chars=None):
    """Whole journeys only: stops before the one that would pass max_chars."""
    parts, size = [], 0
    for i, (name, body) in enumerate(ts):
        part = f"### {name}\n\n```text\n" + "\n".join(body).strip("\n") + "\n```\n"
        if max_chars and size + len(part) > max_chars:
            parts.append(f"{len(ts) - i} more journeys left out for length.\n")
            break
        parts.append(part)
        size += len(part) + 1
    return "\n".join(parts)


SAMPLE = """=== RUN   TestScripts
=== RUN   TestScripts/alpha_decide_errors
=== PAUSE TestScripts/alpha_decide_errors
=== RUN   TestScripts/alpha_help
=== PAUSE TestScripts/alpha_help
=== CONT  TestScripts/alpha_decide_errors
    testscript.go:609: WORK=$WORK
        HOME=$WORK/home
        NO_COLOR=1

        # No --options. (0.252s)
        > exits 2 nav-pilot alpha decide 'Is this a bug fix?'
        [stderr]

        Error: --options needs 2 to 26 comma-separated labels
          as in --options yes,no

        > ! stdout .
        > stderr '--options needs'
        > ttyin -stdin answer-no
        > exec nav-pilot alpha local use gemma3-4b
        [stderr]
        \x1b[?25l  Later: nav-pilot alpha local restart
        PASS

=== NAME  TestScripts/alpha_help
    testscript.go:609: WORK=$WORK
        HOME=$WORK/home

        > exec nav-pilot alpha --help
        [stdout]
        Usage: nav-pilot alpha
        > stdout 'Usage'
        PASS

--- PASS: TestScripts (0.01s)
PASS
ok  	example/e2e	1.0s
"""


def selftest():
    ts = transcripts(SAMPLE)
    assert [n for n, _ in ts] == ["alpha_decide_errors", "alpha_help"], ts
    first = "\n".join(ts[0][1])
    assert "> exits 2 nav-pilot alpha decide" in first, first
    assert "Error: --options needs 2 to 26" in first, first
    assert "\n  as in --options yes,no\n" in first, first
    assert "> ttyin -stdin answer-no" in first, first
    assert "\\e[?25l" in first and "\x1b" not in first, first
    for gone in ("HOME=", "NO_COLOR", "# No --options", "> ! stdout", "> stderr", "PASS", "testscript.go"):
        assert gone not in first, (gone, first)
    assert ts[1][1][:3] == ["> exec nav-pilot alpha --help", "[stdout]", "Usage: nav-pilot alpha"], ts[1]
    md = markdown(ts)
    assert md.startswith("### alpha_decide_errors\n\n```text\n> exits 2"), md
    assert transcripts("") == []
    cut = markdown(ts, max_chars=len(markdown(ts[:1])) + 5)
    assert cut.startswith("### alpha_decide_errors") and "alpha_help" not in cut, cut
    assert cut.endswith("1 more journeys left out for length.\n"), cut
    print("persona_transcripts: selftest ok")


if __name__ == "__main__":
    args = sys.argv[1:]
    if args == ["--selftest"]:
        selftest()
    elif not args or (args[0] == "--max" and len(args) == 2):
        sys.stdout.write(markdown(transcripts(sys.stdin.read()), int(args[1]) if args else None))
    else:
        sys.exit("usage: persona_transcripts.py [--max CHARS] < go-test.log | --selftest")
