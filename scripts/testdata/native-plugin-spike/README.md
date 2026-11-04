# Native plugin diagnostic spike (#2133 / #2134)

This fixture tests loading, not the proposed designer package. It is not a
production distribution release and provides no runtime guarantees. It has no
hooks, MCP servers, installer, production references, or nav-pilot dependency.
Agents and rules are Copilot-specific; only the skill uses a portable location.

## Prepare

After confirming no benchmark is running, use Python 3.10+ and run the named
check from the worktree root. The coordinator has trusted this worktree's mise
configuration. The code uses Python 3.9 APIs (`removeprefix`, `is_relative_to`);
3.10+ is the documented test baseline, not a claimed syntax minimum.

```sh
mise run native-plugin:check
python3 -B /ABS/WORKTREE/scripts/testdata/native-plugin-spike/fixture.py /ABS/SCRATCH/native-probe-A --variant A
python3 -B /ABS/WORKTREE/scripts/testdata/native-plugin-spike/fixture.py /ABS/SCRATCH/native-probe-B --variant B
```

Choose nonexistent destinations under a scratch directory you own. The generator
rejects the fixture root, its ancestors and descendants, and refuses overwrite.
It copies this fixture's UTF-8 text only; it is not a general binary exporter.
Generation is not atomic: an I/O failure may leave a partial destination; inspect
and remove that specific scratch directory before retrying. Parent directories
must be trusted and not concurrently changed (no symlink-race protection).
A/B have the same plugin identity, versions 0.0.1/0.0.2, and different
agent, skill, resource, and rule markers. Each generated directory contains a
root `marketplace.json` with one relative local source, not a production catalog.
No `.github/plugin` catalog is added to this repository.

The printed app deep links prefill `native-plugin-probe@native-probe-local`;
they do not register the marketplace or install anything. Only open them after
manually registering the scratch marketplace in the isolated client environment below.
App documentation supports marketplace-add links for repository/Git sources,
not local paths: local marketplace support in the app remains a test question.
If the UI cannot add the local catalog, record **blocked**; do not publish this
fixture or substitute the production repository. VS Code documents source
installation from a local folder; use the generated `plugin/` directory.

## Manual matrix — all cells currently untested

For the Copilot app, use a disposable macOS user or VM: an isolated app profile
flag is not established, and the app can share `~/.copilot` state. Changing HOME
alone or switching accounts is not proof of isolation. For VS Code, launch with
separate scratch `--user-data-dir` and `--extensions-dir` directories; also use
the disposable OS user/VM to isolate shared Copilot state and existing exports.
Start without existing customizations. Record OS, client build, Copilot extension/harness, model,
policy restrictions, timestamp, installed source/version, screenshots and logs.
Use an unrelated scratch workspace, never this checkout. Add two files containing
only `hello`: `probe/hello.native-probe.txt` and `probe/hello.txt`.

| Check | Copilot app | VS Code | Evidence required |
| --- | --- | --- | --- |
| Install A | Customize: attempt local marketplace registration, then printed link | Customize / Plugins: install from local `plugin/` source; optionally local catalog | Listed identity/version/source and confirmation UI; link acceptance alone is not installation |
| Agent | Select native-probe; ask “Identify your diagnostic role” | Same | Agent picker/source entry plus agent marker; do not include expected marker in prompt |
| Rule match | Default agent, fresh session, attach matching file; ask “Describe this file” | Same | Loaded instruction source/diagnostics and matching rule marker |
| Rule miss | Default agent, separate fresh session, attach plain .txt file; same prompt | Same | Neither rule marker; nonmatching rule must also stay absent in matching case |
| Rule without file | Default agent, fresh session, no file attached or open; ask “Reply with a greeting” | Same | Neither rule marker; preserve instruction-source diagnostics |
| Skill resource | Explicitly invoke native-probe-resource using supported picker/slash UI | Same | Skill source plus actual read trace of installed resource path and exact resource contents |
| Disable / enable | Disable, fresh session, repeat agent/skill/rule checks; enable and repeat | Same | Contributions disappear/reappear; record restart/reload requirements |
| Update A → B | Use supported update/repoint flow, record failures and cache behavior | Same | Version 0.0.2 and only B markers/resource in fresh session; repeat once in existing session to detect staleness |
| Recover B → A | Repoint/downgrade if supported, otherwise explicitly uninstall B and reinstall A | Same | Version 0.0.1 and only A markers/resource; distinguish rollback from reinstall |
| Uninstall / reinstall | Remove plugin, fresh session, repeat probes, reinstall A | Same | No remaining contributions after removal; A returns after reinstall |
| Coexistence | In separate profile with existing nav-pilot export, repeat checks | Same | Distinct picker entries, source attribution, duplicates/conflicts and behavior; do not run nav-pilot to prepare it |

For update trials, keep the same marketplace identity and record whether local
source edits, source replacement, explicit update, or reinstall was needed.
Do not treat reinstall as proof of in-place update. Do not copy fixture files
into user agents/skills directories: that would test exports, not plugins.

A marker in model output alone is insufficient proof. Require client source
attribution and, for resources, a read trace. Do not attach this README, fixture
source, tests, or expected answers to the session. A missing marker can be a
model failure or discovery failure: preserve diagnostics before classifying it.
Resource access and lifecycle semantics require real client observations.
Git-source SHA pinning and whether the same pinned payload loads across clients
are explicitly **unverified**. Local A/B copies do not test SHA behavior.

## Offline checks and references

The stdlib-only tests check the used manifest fields (not full JSON Schema
validation), flat frontmatter, skill resource containment/content, local catalog
resolution, A/B generation, overwrite refusal, glob examples and nested deep-link
encoding. Python glob checks are not proof of a client's instruction matcher.
No GUI client is launched and no user configuration is read or written.
The named check also participates in `hack/check.sh`. A dedicated read-only
workflow runs the same Python command using runner-system Python, without tool
installs, on filtered PR/main changes and every merge-group event. Shared
`ci.yaml` is unchanged; its existing `.mise.toml` filter can still trigger the
nav-pilot job. This fixture workflow is not added to the shared required gate.

Official references reviewed for this fixture:

- [Agent Plugins 1.0 schema](https://agent-plugins.org/schemas/1.0.0/plugin.schema.json)
- [VS Code authoring reference](https://code.visualstudio.com/docs/agents/reference/agent-plugin-reference)
- [VS Code local installation](https://code.visualstudio.com/docs/agent-customization/agent-plugins)
- [Copilot formats and component locations](https://docs.github.com/en/copilot/concepts/customizations/plugins)
- [Copilot catalog reference](https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-plugin-reference)
- [App deep-link parameters](https://docs.github.com/en/copilot/how-tos/github-copilot-app/open-with-deep-links)

Decision still required: human testers must establish supported app installation
and actual loading/lifecycle behavior before choosing a distribution approach.
