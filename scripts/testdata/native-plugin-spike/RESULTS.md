# Evidence and remaining gates

## Established by source inspection

- Agent Plugins 1.0 provides the proposed skill and Copilot namespace layout.
- The app supports separate marketplace-registration and plugin-install links;
  these open confirmation forms, not silent installations.
- Microsoft distributes its upgrade agent through an app marketplace, using a
  legacy manifest. This is not proof of Agent Plugins 1.0 parity.
- The candidate dependency audit identifies model, tool and indirect-agent
  dependencies that prevent copying the full harness unchanged.

## Automated checks

`mise run native-plugin:check` checks the diagnostic fixture's structure,
resource paths, A/B generation, overwrite refusal and deep-link encoding.
It does not run clients or perform full JSON Schema validation.

Local suites were not run: process inspection was denied, so benchmark status
could not be established. CI executes the offline checks; its result must be
read from the draft PR. No canonical artifacts require regeneration.

## Adversarial review

Astra found that conditional rule bodies could conceal globally loaded rules.
The bodies now emit markers unconditionally; only `applyTo` selects them.
Regression assertions preserve this diagnostic property.

## Not established

All manual matrix cells remain untested. In particular: installed client
versions, app local-marketplace support, payload SHA pinning, rule injection,
update/recovery behavior, coexistence and nontechnical task completion.

The app trial requires a disposable OS user or VM; no alternate app profile
mechanism has been established. A human must select the representative task
and perform the usability trial. Issues #2132, #2133 and #2134 remain open.

No GUI client was launched or installed and no real-user client configuration
was changed. This fixture is not a production package or sandbox guarantee.
