# mcp-compliance

Checks an MCP server that sits behind `fleet-gateway` against MCP 2026-07-28 and the auth rules the Nav MCP servers share. The server can be written in any language. The tool checks a subset of the shared rules that `runMCP2026Compliance` and the auth tests check in `apps/mcp-kodeverk`, `apps/mcp-team-catalog` and `apps/fleet-gateway/echo`.

| Check | Expected |
|---|---|
| `server/discover` runs first; if it does not answer 200 the tool stops | |
| No token, invalid token | 401. A 401 without a `WWW-Authenticate` / `invalid_token` challenge gives WARN, not FAIL |
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

Requests send only `Accept: application/json`; the tool does not parse SSE.

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

Releases are tagged `cli/mcp-compliance/vX.Y.Z`, and `@latest` picks the newest tag. Before the first tag it resolves a pseudo-version of `main`. Use `@vX.Y.Z` to pin a version.

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

Use the release binary, so the job needs no Go toolchain. Pin the version.

```yaml
- name: Install mcp-compliance
  env:
    GH_TOKEN: ${{ github.token }}
    VERSION: v0.1.0
  run: |
    gh release download "cli/mcp-compliance/${VERSION}" --repo navikt/copilot \
      --pattern mcp-compliance-linux-amd64 --pattern SHA256SUMS
    sha256sum --check --ignore-missing SHA256SUMS
    install -m 0755 mcp-compliance-linux-amd64 /usr/local/bin/mcp-compliance

# Kotlin: ./gradlew run &   TypeScript: npm ci && npm start &
- run: ./start-server.sh &   # your own start step
- run: mcp-compliance -url http://localhost:8080/mcp
  env:
    MCP_COMPLIANCE_TOKEN: ${{ secrets.MCP_TEST_TOKEN }}
```

Wait for the server to answer before the last step, for example with `curl --retry 10 --retry-connrefused`.

UNVERIFIED: the commands above have not been run end to end against a real Kotlin or TypeScript server. The tool is tested against an in-process server only (`main_test.go`).
