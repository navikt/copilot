# Spike progress

- Branch verified: `spike/native-copilot-plugins`; initial worktree clean.
- Added `scripts/testdata/native-plugin-spike/`: diagnostic payload,
  scratch A/B marketplace generator, stdlib tests, manual app/VS Code matrix
  and source-grounded candidate dependency audit.
- Integrated `native-plugin:check` in `.mise.toml` and `hack/check.sh`; added
  `.github/workflows/native-plugin-spike.yaml` for the same stdlib check on
  scoped PR/main-push paths and merge groups, without dependency installation.
- Official Agent Plugins 1.0 schema and client references reviewed over HTTP.
- Process inspection attempted from scratch: `ps -axo pid,command` was denied
  (`operation not permitted`). Stopped; no alternate inspection attempted.
- Benchmark status therefore unknown: tests, generator execution and builds
  deliberately not run. Python code has only been reviewed, not executed.
- Coordinator reports successful, user-approved worktree mise trust. Trust is
  no longer a blocker; benchmark status still prevents local suite execution.
- Current edits remain for coordinator review/commit/draft-PR handling. This
  worker has not committed or pushed; CI results have not been observed.
- No GUI launch, client install, nav-pilot execution or HOME/config writes
  performed by this worker. No canonical artifacts or manifests regenerated.
- Human client observations remain blocked on an interactive test run; local
  marketplace support in the app is explicitly unverified.
- This spike is not a production distribution release or runtime guarantee.
