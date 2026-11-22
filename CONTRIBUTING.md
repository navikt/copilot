# Bidra

Konvensjoner og kommandoer står i [AGENTS.md](AGENTS.md). Her er det som ikke står der.

## CI og merge-kø

Alle pull requests og oppføringer i merge-køen kjører [`ci.yaml`](.github/workflows/ci.yaml). Den gir én sjekk, `ci-ok`, som er den eneste påkrevde sjekken i rulesettet for `main`.

- Jobben `changes` finner hvilke komponenter endringen berører, og bare deres sjekker kjøres.
- gitleaks skanner hele historikken på hver kjøring.
- `ci-ok` feiler hvis en kjørt sjekk feiler eller avbrytes. Hoppede jobber teller som OK.

Komponent-workflowene (`copilot-api.yaml`, `my-copilot.yaml` osv.) deployer PR-preview til dev og `main` til prod. De er ikke påkrevde og kjører ikke i merge-køen.

## Hemmeligheter

Vil du fange lekkasjer før commit, installer gitleaks (`brew install gitleaks`) og kjør:

```bash
gitleaks git --pre-commit --staged
```

Falske positiver legges i [.gitleaks.toml](.gitleaks.toml).

## Lokalt oppsett

Du trenger [mise](https://mise.jdx.dev) og [fnox](https://fnox.jdx.dev):

```bash
mise install
lefthook install
```

Hemmeligheter ligger i macOS Keychain via fnox, ikke i `.env`-filer. Hver app har en `fnox.toml`:

| Tjeneste | Apper | Hemmeligheter |
|---|---|---|
| `copilot-portal` | my-copilot | ingen, alt går gjennom copilot-api |
| `copilot-api` | copilot-api | GITHUB_APP_ID, GITHUB_APP_PRIVATE_KEY, GITHUB_APP_INSTALLATION_ID, GITHUB_APP_ORG_INSTALLATION_ID, GITHUB_BILLING_TOKEN |
| `copilot-jobs` | copilot-adoption, copilot-metrics, mcp-onboarding | GITHUB_APP_ID, GITHUB_APP_PRIVATE_KEY, GITHUB_APP_INSTALLATION_ID, SLACK_WEBHOOK_URL |

```bash
cd apps/copilot-api
fnox set GITHUB_APP_ID
```

Annen backend (1Password, GCP Secret Manager) settes i en gitignored `fnox.local.toml`, se [fnox providers](https://fnox.jdx.dev/providers/). Ikke-hemmelig konfig ligger under `[env]` i hver apps `.mise.toml`.

## MCP-policy i navikt

Policyen er satt på organisasjonsnivå og gjelder alle med Copilot-sete:

- MCP servers in Copilot: Enabled
- MCP Registry URL: `https://mcp-registry.nav.no`
- Policy: Registry only

Policyen matcher på server-ID. En lokal server må ha en oppføring i registeret med nøyaktig samme ID, se [GitHub docs](https://docs.github.com/en/copilot/reference/mcp-allowlist-enforcement).
