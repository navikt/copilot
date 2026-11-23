# mcp-compliance

Checks an MCP server that sits behind `fleet-gateway` against MCP 2026-07-28 and the auth rules the Nav MCP servers share. The server can be written in any language. The tool checks a subset of the shared rules that `runMCP2026Compliance` and the auth tests check in `apps/mcp-kodeverk`, `apps/mcp-team-catalog` and `apps/fleet-gateway/echo`.

| Check | Expected |
|---|---|
| `server/discover` runs first; if it does not answer 200 the tool stops | |
| No token, invalid token | 401. No token: WARN if `WWW-Authenticate: Bearer` is missing or carries an `error` code (RFC 6750 3.1). Invalid token: WARN without `error="invalid_token"`. A missing or wrong challenge header only warns; any status other than 401 is FAIL |
| App token (`idtyp=app`), if `MCP_COMPLIANCE_APP_TOKEN` is set | 403 |
| `server/discover` | 200, `supportedVersions` holds `2026-07-28`, `capabilities`, `ttlMs`, `cacheScope`, `resultType: complete`, `_meta` server info |
| Missing `MCP-Protocol-Version`, or `Mcp-Method`, `Mcp-Name` or `_meta` protocol version that does not match the body | 400, `-32020` |
| Unsupported `MCP-Protocol-Version` | 400, `-32022` |
| Missing `method`, `id: null` | 400, `-32600` |
| `GET`, `DELETE` (skipped with `-legacy`) | 405, `Allow: POST` |
| Body over `-max-bytes` (default 1 MiB, must be greater than 0) | 413 |
| `tools/list`: hidden instructions in a description or schema description | FAIL for injection phrases, WARN for `you must`, `always`, `never`; one row per tool and fragment |
| `tools/list`: zero-width, bidi (U+202A-U+202E, U+2066-U+2069) or tag (U+E0000-U+E007F) characters | FAIL |
| `tools/list`: any of `readOnlyHint`, `destructiveHint`, `idempotentHint`, `openWorldHint` missing from `annotations` | FAIL |
| `tools/list`: description over 1024 characters | WARN |

The description checks cover each tool's `description` and every `description` in its `inputSchema`. With `-tool`, only that tool is checked. FAIL patterns (case-insensitive): `ignore (all|any|the) previous|prior (instructions)`, `system prompt`, `do not tell the user` (also `don't`) and the tags `<system>` and `<instructions>`. WARN patterns: the words `you must`, `always` and `never`, which honest text such as "you must pass an id" also uses. The tool prints only the matching fragment, redacted and capped at 200 characters, never the whole description. The scan covers `title` and `annotations.title` too. U+061C joins the FAIL set; soft hyphen, U+034F, U+180E and variation selectors give WARN. The check is best-effort: it does not normalise text, so it misses homoglyphs, full-width text and synonyms such as disregard and override. Invisible characters are reported as a code point and byte offset.

Not covered, so keep these in your own tests:

- `-32602` for a missing or invalid `_meta`
- 404 for unknown methods and for `ping`
- base64-encoded `Mcp-Name` (`=?base64?...?=`)
- that `Mcp-Session-Id` is ignored

Requests send `Accept: application/json, text/event-stream`; the tool reads a JSON or SSE response.

The tool prints a PASS/FAIL/WARN/SKIP table and exits 1 if any check fails.

Tokens are read from environment variables, never flags, so they stay out of process lists and shell history. The tool never prints them.

- `MCP_COMPLIANCE_TOKEN`: a valid user token for the server. Leave it unset for a server that runs without auth locally; the auth checks then fail, which is correct for anything that will run behind `fleet-gateway`.
- `MCP_COMPLIANCE_APP_TOKEN`: an app token the server must reject. Unset means the check is skipped.

## Install

```sh
brew install navikt/tap/mcp-compliance
```

Or with Go 1.26 or later:

```sh
go install github.com/navikt/copilot/cli/mcp-compliance@latest
```

Every change to `cli/mcp-compliance` merged to `main` is released automatically and tagged `cli/mcp-compliance/vX.Y.Z`; `@latest` picks the newest tag, and the Homebrew formula follows within an hour. Use `@vX.Y.Z` to pin a version.

Binaries for macOS and Linux (amd64, arm64) and a `SHA256SUMS` file are on each [release](https://github.com/navikt/copilot/releases?q=mcp-compliance). Verify a binary's build provenance with:

```sh
gh attestation verify mcp-compliance-linux-amd64 --repo navikt/copilot
```

## Run locally

Start your server, then:

```sh
mcp-compliance -url http://localhost:8080/mcp
```

Kotlin (Ktor or Spring) and TypeScript servers are tested the same way: the tool only speaks HTTP. Start the server with `./gradlew run` or `npm start`, and point `-url` at its MCP endpoint.

`-tool` names a tool that can be called with empty arguments. Without it the tool takes the first entry from `tools/list`. Use `-legacy` while your server still serves the old protocol (`MCP_LEGACY_PROTOCOL_UNTIL`).

## Test tokens

The auth checks need a token your server accepts and, optionally, an app token it must reject. Locally the server usually validates tokens through Texas. A fake Texas that issues test tokens for local runs and CI is tracked in [#2552](https://github.com/navikt/copilot/issues/2552). Until then, run the server against a stub introspection endpoint that accepts your test token, and set:

```sh
export MCP_COMPLIANCE_TOKEN=...       # accepted user token
export MCP_COMPLIANCE_APP_TOKEN=...   # optional, idtyp=app, must get 403
```

Never use a production token.

## Run in CI

Use the GitHub Action in this directory. It needs no Go toolchain: it downloads the release binary for the runner (Linux or macOS, amd64 or arm64), checks it against `SHA256SUMS`, runs it and fails the job on any FAIL. It starts nothing, so start your server in an earlier step.

```yaml
- uses: actions/checkout@v7
# Kotlin: ./gradlew installDist and start build/install/*/bin/*
# TypeScript: npm ci, then npm start
- run: ./start-server.sh &   # your own start step
- run: curl -fsS --retry 30 --retry-delay 1 --retry-connrefused http://127.0.0.1:8080/health
- uses: navikt/copilot/cli/mcp-compliance@cli/mcp-compliance/v0.2.0
  with:
    url: http://127.0.0.1:8080/mcp
    token: ${{ secrets.MCP_TEST_TOKEN }}   # test token, never production
```

| Input | Default | Meaning |
|---|---|---|
| `url` | required | MCP endpoint of the running server |
| `version` | empty | Release such as `v0.2.0`. Empty uses the release the Action is pinned to (`uses: navikt/copilot/cli/mcp-compliance@cli/mcp-compliance/vX.Y.Z`). `latest` takes the newest release, and is also used, with a notice, when the Action ref is a branch, a SHA or a local path |
| `token` | empty | `MCP_COMPLIANCE_TOKEN` |
| `app-token` | empty | `MCP_COMPLIANCE_APP_TOKEN`; empty skips the 403 check |
| `tool` | empty | `-tool` |
| `args` | empty | Extra flags, such as `-legacy` |
| `github-token` | `github.token` | Token for the release download |

The Action ref is a release tag, so the Action and the tool are released together. `navikt/copilot` runs it against `mcp-kodeverk` in `ci.yaml`, and the Kotlin and TypeScript templates use it in their CI.

To install the binary yourself instead:

```sh
gh release download cli/mcp-compliance/v0.2.0 --repo navikt/copilot \
  --pattern mcp-compliance-linux-amd64 --pattern SHA256SUMS
sha256sum --check --ignore-missing SHA256SUMS
```

## Reading the output

One row per check: PASS, FAIL, WARN or SKIP, then a summary line. FAIL means the server breaks a rule; fix it, the exit code is 1. WARN means worth a look, for example a 401 without an `invalid_token` challenge or a description that says "you must"; it does not fail the run. SKIP means a check had nothing to run against, such as an unset app token.

The tool's own tests use an in-process server (`main_test.go`).
