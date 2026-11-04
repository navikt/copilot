# Build nav-design-beta

The installable payload is `plugins/nav-design-beta/`, listed in
`.github/plugin/marketplace.json`. Merging makes this opt-in beta available to
users of the `nav-pilot` marketplace. See the package README for installation
and permissions. Bump both versions together when publishing an update.

Python 3.10+ standard library and this checkout are the only build prerequisites:

```sh
python3 -B scripts/testdata/design-prototype-beta/export.py
python3 -B scripts/testdata/design-prototype-beta/export.py --check
python3 -B scripts/testdata/design-prototype-beta/test_export.py
```

Equivalent repo tasks: `mise run native-plugin:generate` and
`mise run native-plugin:check` (also checks the diagnostic fixture). No broad
catalog generation is needed: root `agents/`, `skills/`, and instructions stay
unchanged. Run local tests only when no benchmark is running.

`package/` holds the small beta agent, native manifests, and user README as plain
source files. The generator copies those, the canonical `SKILL.md` and full
references/resources byte-for-byte, and root `LICENSE`. Source-only root skill
`README.md` and `metadata.json` are excluded. It does not adapt
the original Aksel agent or rewrite skill prose. The beta agent explicitly
prohibits acting on upstream install/config snippets; a live reviewer must assess
whether the client follows that boundary. The package README is the beta entry point.

The fixed output can be regenerated only when its files still match the previous
`.generated.json` hashes and contain no unknown files. Modified/unknown files or
removed source paths stop generation before writing; preserve and review them
manually. The inventory is an accidental-overwrite check, not a security boundary.
`--check` compares every output byte, including inventory, without writing.
An optional positional scratch destination must be outside this repository and
nonexistent (an identical existing output is a no-op). Nothing is ever deleted.
Use trusted parents with no concurrent changes; I/O failures may leave partial
output. No client, config, authentication, or network access occurs during build.

The focused tests check source/resource byte identity, reference links, exact
allowlist, setup prohibition, manifest fields, drift, and overwrite refusal. They
are not a full schema compiler or evidence of live client compatibility.
