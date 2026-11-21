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

The description checks cover each tool's `description` and every `description` in its `inputSchema`. With `-tool`, only that tool is checked. FAIL patterns (case-insensitive): `ignore (all|any|the) previous|prior (instructions)`, `system prompt`, `do not tell the user` (also `don't`) and the tags `<system>` and `<instructions>`. WARN patterns: the words `you must`, `always` and `never`, which honest text such as "you must pass an id" also uses. The tool prints only the matching fragment, redacted and capped at 200 characters, never the whole description. Invisible characters are reported as a code point and byte offset.

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

## Run locally

Start your server, then:

```sh
go run github.com/navikt/copilot/cli/mcp-compliance@main -url http://localhost:8080/mcp
```

`-tool` names a tool that can be called with empty arguments. Without it the tool takes the first entry from `tools/list`. Use `-legacy` while your server still serves the old protocol (`MCP_LEGACY_PROTOCOL_UNTIL`).

## Run in CI

```yaml
- uses: actions/setup-go@v6
  with:
    go-version: "1.26"
- run: ./start-server.sh &   # your own start step
- run: go run github.com/navikt/copilot/cli/mcp-compliance@main -url http://localhost:8080/mcp
  env:
    MCP_COMPLIANCE_TOKEN: ${{ secrets.MCP_TEST_TOKEN }}
```

UNVERIFIED: both commands above have not been run end to end against a real server. The tool is tested against an in-process server only (`main_test.go`). Pin a commit SHA instead of `@main` if you want the checks to stay fixed.
