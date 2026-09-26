# e2e

Tests that build the real `nav-pilot` binary once and run it. Nothing is
stubbed inside the binary.

- `golden_path_test.go`: Go tests against real git repositories.
- `script_test.go` + `testdata/script/*.txtar`: user journeys as
  [testscript](https://pkg.go.dev/github.com/rogpeppe/go-internal/testscript)
  files. Each one is a transcript you can read: the commands a user types and
  what stdout, stderr and the exit code must say.

```sh
go test ./e2e                                   # everything
go test ./e2e -run TestScripts/alpha_decide     # journeys whose name starts with alpha_decide
go test ./e2e -run TestScripts -v               # print every transcript
```

## Adding a journey

Copy the closest `testdata/script/*.txtar` and edit it. Every script runs in
its own `$WORK` with:

- `HOME=$WORK/home`, `NAV_PILOT_CONFIG=$HOME/config.toml` and
  `HF_HOME=$HOME/.cache/huggingface`. Files in the archive land under
  `$WORK`, so `-- home/.nav-pilot/local-models.json --` is the cached model
  manifest the binary reads without touching the network.
- `NO_COLOR=1`, telemetry off, and `HTTP(S)_PROXY` pointing at a closed port,
  so any request to a real service fails at once.
- `nav-pilot` on `PATH`. Write `exec nav-pilot ...`. A bare command name is an
  error, so it's always clear what runs as a real process.

Commands beyond the
[built-ins](https://pkg.go.dev/github.com/rogpeppe/go-internal/testscript#hdr-The_Script_Language):

| Command | What it does |
| --- | --- |
| `exits [-within DUR] CODE PROG ARGS...` | Runs PROG and asserts its exact exit code (`! exec` only knows non-zero). `-within` also fails a run slower than DUR. |
| `validjson FILE` | FILE, `stdout` or `stderr` holds exactly one JSON value. |
| `fake-mlx [MODEL]` | Starts a fake mlx-lm server in its own process and records it as the running local server. Exports `FAKE_MLX_URL`. |

The `[pty]` condition is true where a pseudo-terminal can be opened. Put
`[!pty] skip '...'` at the start of any script that uses `ttyin`. Keep TTY
scripts to detection and the first prompt: don't drive a full TUI.

Assert on stdout and stderr separately, and assert the empty one too
(`! stdout .`). Most of the value is in checking which stream a message goes to.

## Updating expected output

When a script compares output with `cmp stdout want` and a `-- want --`
section, `-update` rewrites that section from what the binary printed:

```sh
go test ./e2e -run TestScripts/<name> -update
git diff testdata/script
```

`-update` records whatever the binary does now, regressions included. Read the
diff before you commit it. `stdout`/`stderr` regex assertions aren't
rewritten; edit those by hand.

## Reviewing a transcript with the UX rubric

[UX_RUBRIC.md](UX_RUBRIC.md) is a prompt for an LLM reviewer. The reviewer
sees transcripts only, never source, so it judges what a user would see.

1. Print the transcripts: `go test ./e2e -run TestScripts/<name> -v`.
2. Remove the assertion lines (`> stdout ...`, `> stderr ...`) and the
   script's `#` comments. They tell the reviewer what to expect. Keep each
   command, its `[stdout]`/`[stderr]`, its exit code, and which of stdin,
   stdout and stderr were a TTY.
3. Give the reviewer `UX_RUBRIC.md` and the transcripts, and nothing else.

Treat what comes back as hypotheses to check with a person or a new journey,
not as a verdict. Don't commit the review as expected output. Put it in the
PR description.
