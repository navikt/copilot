# UX rubric calibration log

Human labels for each sample review, so later rounds can show whether the
rubric is getting better at finding what people care about. Kept out of
[UX_RUBRIC.md](UX_RUBRIC.md) on purpose: the reviewer gets that file, and
past labels would bias it toward repeating them. Never give it this one.

**Round 1, 2026-09-26** (sample in #979). The user's coordinator labelled the
review and the user approved the labels. #979 lists 15 findings plus the two
artefacts. The legend row groups three of them.

| Finding | Label | Outcome |
| --- | --- | --- |
| Bare `alpha local use` exits 0 | Real | Exits 2, output on stderr |
| Non-TTY `init` prints the plan to stdout before refusing | Real | Refuses first, stdout empty |
| `✓ Downloaded.` doesn't say whether anything was downloaded | Real | "Already on disk. Nothing to download." |
| `--eval` JSON error doesn't show the line format | Real | Names the line and the format |
| No legend for `*`; `local_model` and "dispatch" undefined in help | Polish, accepted | Legend and help text added |
| Raw `proxyconnect 127.0.0.1:9` error | Not real | Test-setup artefact (e2e proxy) |
| "8 GB RAM" next to 25 GB of weights | Not real | Test-setup artefact (fixture) |
| `Set it up:` twice in status | Not yet assessed | |
| Blank lines around errors | Not yet assessed | |
| `Later:` vs `Load it:` for the same state | Not yet assessed | |
| TLS-proxy warning names no workaround | Not yet assessed | |
| `init` doesn't mention `use <key>` for picking a smaller model | Not yet assessed | |
| Bare `alpha` prints usage to stderr while `--help` uses stdout | Not real | Correct per clig.dev: usage shown for an error goes to stderr, help that was asked for goes to stdout |
| `alpha local --help` prints the same page as `alpha --help` | Polish, accepted | Not scheduled |
| `use` prints its ⚠ warning to stdout with the ✓ result | Real | Fixed: the warnings go to stderr |
