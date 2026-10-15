---
applyTo: "cli/nav-pilot/**"
---

# nav-pilot CLI instructions

## Hostile repository and filesystem boundary

- **Symlink rejection**: Reject symlinks when hashing, copying, staging, or exporting artifacts (`source.DirHash`, `source.CopyDir`, `source.CopyFile`, `artifacts.ExportOpenCode`, `artifacts.SyncOpenCodeArtifacts`). Always verify target paths with `source.CheckSymlink` before writing or deleting files in user or repository scopes.
- **Untrusted repository declarations**: `.nav-pilot/agentpakke.lock.json` in a target repository is untrusted input. Reject absolute or local filesystem paths in `decl.Source` (`loadRepoDeclaration` and `source.ComposeIfDeclared`); only `navikt/<repo>` sources are permitted from repo declarations.
- **OpenCode config and MCP policy (`internal/provider/opencode_mcp.go`, `staged_launch.go`)**:
  - Never expand arbitrary host `os.Environ()` variables via `{env:VAR}` from `opencode.json(c)` into `OPENCODE_CONFIG_CONTENT` or `NAV_PILOT_MCP_BLOCKED`. Only expand variables permitted by `isAllowedOpenCodeEnvVar`.
  - Bound ancestor directory walks in `openCodeConfigDocs` to the git repository root or `$HOME`.
  - Validate `{file:relPath}` references in `withOpenCode2UserConfig` so resolved paths stay within the staged payload directory and pass `source.CheckSymlink`.
  - Reject local MCP server commands that use path separators, unsafe runner flags (`-c`, `--eval`, `--with`, `--from`, multiple `-p`), or `environment` overrides.
  - Ensure `hooks-bridge.js` and `hooks-bridge-v2.js` block unlisted MCP servers and reject `list_mcp_resources` / `list_mcp_resource_templates` calls that omit `args.server` when any server is blocked.

## Testing and home directory isolation

- **Mandatory test isolation**: Every test package in `cli/nav-pilot` must have a `TestMain` calling `testhome.Run` (`TestEveryTestPackageCallsRun` enforces this). Tests that invoke CLI commands or auth paths must use `isolatedConfig(t)` or `isolatedRun(t)` and `keyring.MockInit()` so the developer's real `~/.copilot/`, `~/.nav-pilot/`, `$HOME`, and macOS Keychain are never modified.
- **Concurrency and race safety**: Avoid mutating package-level globals across goroutines (`source.FetchTimeout`, `provider.MCPClient`, `provider.MCPHostsOff`). Run `go test -race ./...` when modifying concurrent staging, launch, or background refresh paths.
