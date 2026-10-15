---
applyTo: ".github/workflows/**,hooks/**,scripts/**,agents/**,skills/**,instructions/**,prompts/**,docs/**"
---

# Workflows, hooks, and customizations instructions

## GitHub Actions workflows (`.github/workflows/**`)

- **Pin actions by SHA**: Pin every third-party action to a full 40-character commit SHA with a `# ratchet:<action>@<version>` comment.
- **Least-privilege permissions**: Declare `permissions: contents: read` at the workflow level and grant write permissions (`contents: write`, `pull-requests: write`, `issues: write`, `id-token: write`) only on the specific jobs that need them.
- **No untrusted interpolation in `run:`**: Never interpolate `${{ github.event.* }}`, branch names, PR titles/bodies, or step outputs containing external text directly inside `run:` scripts. Pass values through `env:` variables and quote them in shell.
- **Safe `$GITHUB_OUTPUT` writes**: When writing multiline values to `$GITHUB_OUTPUT`, generate a random heredoc delimiter with `EOF_$(openssl rand -hex 8)`.
- **Supply-chain checks**: Verify SHA-256 checksums (`sha256sum -c -`) before extracting downloaded release archives in CI, and pin global `npm install -g` packages to exact versions with `--ignore-scripts`.

## Pre-tool hooks (`hooks/**` and `.github/hooks/**`)

- **Mirror invariant**: Every change to `hooks/*.py` must be copied byte-for-byte to `.github/hooks/*.py`. Run `mise run hooks:test` (`python3 hooks/<name>.py --selftest` and `diff -q`) before committing.
- **Bypass resistance**: When editing `ask-first-aria.py`, `klarsprak-gate.py`, or `gh-poll-gate.py`, normalize file paths (`posixpath.normpath`), account for global CLI flags (`git -C`, `gh -R` / `--repo`), and add regression cases to `SELFTESTS`.

## Customizations and documentation (`agents/**`, `skills/**`, `instructions/**`, `prompts/**`, `docs/**`)

- **Language split**: Write body text in `agents/`, `skills/`, `instructions/`, and `prompts/` in English. Keep YAML frontmatter `description` and `metadata.json` fields in Norwegian. Run `mise run skills:lint` and `mise run generate` after changes.
- **Norwegian prose (`docs/**`)**: Write clear Norwegian (`klarspråk`) without AI filler, em dashes, or unadapted English calques. Keep commands, flags, file paths, and code identifiers exact.
