# copilot-sandbox

macOS Seatbelt sandbox wrapper for GitHub Copilot CLI. Runs Copilot inside Apple's kernel-level sandbox (`sandbox-exec`) so the agent can work on your project but cannot access your secrets.

> **macOS only** — uses Apple's Seatbelt framework (the same mechanism App Store apps run under).

## Philosophy

In a world of vibe-coded AI tools, this project chooses a different path. We don't do magic. We don't do clever. We do honest, auditable security that you can read, understand, and verify in minutes.

The sandbox is ~1500 lines of Rust that generates a Seatbelt profile and optionally runs a CONNECT proxy. No frameworks, no runtime dependencies, no telemetry. Every security boundary is kernel-enforced and tested. Every design decision is documented with the threat it mitigates and the prior art it builds on.

**Our priorities, in order:**

1. **Correct** — every claim is tested, every edge case has a CVE or research reference
2. **Transparent** — read [SECURITY.md](SECURITY.md), it hides nothing
3. **Simple** — single static binary, zero config required, sane defaults
4. **Useful** — get out of the way and let Copilot do its job, safely

We'd rather ship something small that actually works than something impressive that doesn't.

## What it does

**Primary control: filesystem isolation.** The sandbox blocks access to credentials and secrets at the kernel level. All restrictions apply to Copilot and every process it spawns.

| Resource | Status | Notes |
|---|---|---|
| Read/write project directory | ✅ Allowed | |
| Read `~/.copilot` (auth, native modules) | ✅ Allowed | Includes `file-map-executable` for `keytar.node`, `pty.node` |
| Read `~/.config/gh` (GitHub CLI auth) | ✅ Allowed (read-only) | Copilot spawns `gh auth token` — see [Security trade-offs](#security-trade-offs) |
| Read `~/.gitconfig` | ✅ Allowed (read-only) | |
| Access macOS Keychain | ✅ Allowed | Copilot uses `keytar.node` for token storage |
| Outbound network (TCP) | ✅ Allowed | Copilot needs `api.business.githubcopilot.com` etc. |
| Read `~/.ssh`, `~/.gnupg`, `~/.aws` | 🔒 Kernel-blocked | |
| Read `~/.kube`, `~/.docker`, `~/.nais` | 🔒 Kernel-blocked | |
| Read `~/.config/gcloud`, `~/.config/op` | 🔒 Kernel-blocked | |
| Read `~/.netrc`, `~/.npmrc`, `~/.vault-token` | 🔒 Kernel-blocked | |
| Execute binaries from `/tmp` | 🔒 Kernel-blocked | Prevents write-then-exec attacks |
| Child process inheritance | ✅ All restrictions apply to subprocesses | |

For the full security model, threat analysis, and test strategy, see **[SECURITY.md](SECURITY.md)**.

## Install

### Pre-compiled binary (recommended)

Download the latest release for your Mac:

```bash
# Apple Silicon (M1/M2/M3/M4)
curl -fsSL https://github.com/navikt/copilot/releases/latest/download/copilot-sandbox-aarch64-apple-darwin.tar.gz | tar xz
sudo mv copilot-sandbox /usr/local/bin/

# Intel Mac
curl -fsSL https://github.com/navikt/copilot/releases/latest/download/copilot-sandbox-x86_64-apple-darwin.tar.gz | tar xz
sudo mv copilot-sandbox /usr/local/bin/
```

Every release binary has [build provenance attestation](https://docs.github.com/en/actions/security-for-github-actions/using-artifact-attestations/using-artifact-attestations-to-establish-provenance-for-builds) — verify it with:

```bash
gh attestation verify copilot-sandbox -o navikt
```

### Build from source

```bash
cd experimental/copilot-sandbox
mise install           # Install Rust toolchain
mise run build:release
```

## Quick start

```bash
# Run Copilot in sandbox (credentials protected, network allowed)
copilot-sandbox -- -p "fix the tests"

# Verify the sandbox works
copilot-sandbox -- --version

# Enable proxy for connection logging
copilot-sandbox --with-proxy -- -p "fix the tests"

# Create config file with defaults
copilot-sandbox --init-config
```

## Usage

```
copilot-sandbox [OPTIONS] [-- <COPILOT_ARGS>...]
```

Everything after `--` is passed directly to the `copilot` command.

### File access

Copilot can only read and write to the project directory. Everything else (SSH keys, cloud credentials, etc.) is blocked by the kernel.

| Flag | What it does |
|---|---|
| `-d, --project-dir <DIR>` | Which directory Copilot can work in. Defaults to the current git repo root. |
| `--allow-read <PATH>` | Let Copilot read files outside the project (e.g. shared libraries, docs). Can be repeated. |
| `--allow-write <PATH>` | Let Copilot read AND write outside the project. Use carefully. Can be repeated. |
| `--deny-path <PATH>` | Block a path that would otherwise be allowed. Deny always wins. Can be repeated. |

### Proxy (optional)

The proxy is **disabled by default**. Copilot CLI connects directly to its APIs (Node.js does not natively respect `http_proxy`/`https_proxy` env vars). The proxy is useful for:

- **Connection logging** — see what domains tools like `gh` and `curl` connect to
- **Domain blocking** — block known exfiltration infrastructure (paste sites, webhook services, etc.)

| Flag | What it does |
|---|---|
| `--with-proxy` | Start a localhost CONNECT proxy that logs connections. |
| `--no-proxy` | Disable the proxy, even if your config file enables it. |
| `--proxy-port <PORT>` | Which port the proxy listens on (default: 18080). |
| `--blocked-domains <FILE>` | A text file with domains to block, one per line (e.g. `pastebin.com`). Re-read on every request. |

> **Why doesn't the proxy intercept Copilot traffic?** Copilot CLI is a Node.js application. Node.js does not natively use `http_proxy`/`https_proxy` env vars. Setting these vars actually *breaks* Copilot's auth flow with `api.business.githubcopilot.com`. Go-based tools like `gh` do respect proxy env vars and will be logged.

### Debugging

| Flag | What it does |
|---|---|
| `--print-profile` | Print the generated sandbox profile (SBPL) and exit. |
| `--show-denials` | Stream macOS sandbox denial logs in real time. |
| `--no-validate` | Skip the startup check that verifies sandbox restrictions are active. |
| `--init-config` | Create a starter config file at `~/.config/copilot-sandbox/config.toml` and exit. |

### Examples

```bash
# Most common: run Copilot in sandbox
copilot-sandbox -- -p "fix the tests"

# With connection logging
copilot-sandbox --with-proxy -- -p "fix the tests"

# Let Copilot read a shared library directory
copilot-sandbox --allow-read ~/shared-libs -- -p "use shared-libs"

# Block a path you don't want Copilot to see
copilot-sandbox --deny-path ~/.config/gh -- -p "refactor auth"

# Block paste sites (with proxy enabled)
copilot-sandbox --with-proxy --blocked-domains ./blocked-domains.txt -- -p "refactor"

# Inspect the generated sandbox profile
copilot-sandbox --print-profile

# Debug: see what the sandbox blocks in real time
copilot-sandbox --show-denials -- -p "fix the tests"
```

## Configuration file

Save your preferred defaults to `~/.config/copilot-sandbox/config.toml` so you don't need to pass flags every time.

**Create the default config:**

```bash
copilot-sandbox --init-config
```

This creates a commented template at `~/.config/copilot-sandbox/config.toml`:

```toml
[proxy]
# enabled = false           # Set to true for connection logging
# port = 18080
# blocked_domains = "~/.config/copilot-sandbox/blocked-domains.txt"

[sandbox]
# validate = true

[allow]
# read = ["~/some/path"]
# write = ["~/another/path"]

[deny]
# paths = ["~/extra/secret"]
```

**Precedence** (highest to lowest):

1. CLI flags (`--with-proxy`, `--proxy-port`, etc.)
2. Config file (`~/.config/copilot-sandbox/config.toml`)
3. Built-in defaults

CLI flags always override the config file. Use `--no-proxy` to disable a proxy that's enabled in config.

**Environment variable override:**

Set `COPILOT_SANDBOX_CONFIG` to use a config file at a custom location:

```bash
COPILOT_SANDBOX_CONFIG=/path/to/custom.toml copilot-sandbox -- --version
```

**Path expansion:** Paths in config support `~/` expansion, resolved relative to the config file directory.

## Architecture

```
┌──────────────────────────────────┐
│  copilot-sandbox (Rust binary)   │
│  ┌───────────┐  ┌─────────────┐ │
│  │ Profile    │  │ CONNECT     │ │
│  │ Generator  │  │ Proxy       │ │
│  │ (SBPL)     │  │ (optional)  │ │
│  └─────┬─────┘  └─────────────┘ │
│        │                         │
│        ▼                         │
│  sandbox-exec (Apple kernel)     │
│        │                         │
│        ▼                         │
│  copilot (sandboxed)             │
│  ├── All child processes         │
│  ├── Cannot read ~/.ssh          │
│  ├── Network allowed (TCP)       │
│  └── Filesystem = primary ctrl   │
└──────────────────────────────────┘
```

**Security model**: deny-by-default filesystem with kernel enforcement. Network is allowed because Copilot needs to reach its API endpoints. See [SECURITY.md](SECURITY.md) for the full threat model, defense layers, and honest gaps.

## Security trade-offs

### `~/.config/gh` is readable

Copilot spawns `gh auth token` to authenticate. This reads `~/.config/gh/hosts.yml` which contains a GitHub OAuth token. We allow this because:

- **Required for auth**: Without `gh` auth, Copilot falls back to Keychain only. Many users rely on `gh` CLI for auth.
- **Read-only**: The sandbox cannot modify the token file.
- **Same-destination token**: The token is a GitHub token that Copilot already sends to GitHub's API. An attacker would need to exfiltrate it to a *different* server.
- **Risk**: With outbound TCP allowed, a compromised Copilot could exfiltrate this token. Use `--deny-path ~/.config/gh` if this concerns you (Copilot will use Keychain auth instead).

### Outbound network is allowed

SBPL (Seatbelt Profile Language) does not support wildcard port filtering (e.g., "any host on port 443"). Copilot connects to multiple CDN-backed endpoints with changing IPs (`api.business.githubcopilot.com`, `api.githubcopilot.com`, `proxy.business.githubcopilot.com`). We cannot enumerate these IPs. Therefore:

- **All outbound TCP is allowed** — this is a pragmatic choice, not a security ideal
- **Filesystem isolation is the primary control** — credentials are kernel-blocked regardless of network
- **The proxy cannot intercept Copilot traffic** — Node.js ignores `http_proxy` env vars, and setting them breaks auth

See [SECURITY.md](SECURITY.md) for the full threat model and honest gaps.

## Domain blocking

When the proxy is enabled (`--with-proxy`), it can block domains commonly used for data exfiltration. A default blocklist is included based on real attack infrastructure observed in 2025–2026 supply chain incidents:

```bash
# Enable proxy with domain blocking
copilot-sandbox --with-proxy --blocked-domains blocked-domains.txt -- -p "fix tests"

# Or set it permanently in config
copilot-sandbox --init-config
# Then edit ~/.config/copilot-sandbox/config.toml:
#   [proxy]
#   enabled = true
#   blocked_domains = "~/.config/copilot-sandbox/blocked-domains.txt"
```

The blocklist covers webhook capture services, paste sites, file sharing, tunneling services, and IP recon endpoints. See [`blocked-domains.txt`](blocked-domains.txt) for the full list with sources.

> **Note:** The proxy only captures traffic from tools that respect `http_proxy` (like `gh`, `curl`). Copilot CLI's own API traffic bypasses the proxy. Domain blocking is a defense-in-depth measure, not a primary control.

## Copilot CLI network endpoints

Copilot CLI 1.0.21 connects directly to these endpoints (empirically verified):

| Endpoint | Purpose |
|---|---|
| `api.github.com` | GitHub API (user info, token validation) |
| `api.githubcopilot.com` | Copilot API |
| `api.business.githubcopilot.com` | Copilot Business API (enterprise users) |
| `proxy.business.githubcopilot.com` | Copilot Business proxy |

## Limitations

- **macOS only** — uses `sandbox-exec` (deprecated but functional, used by Chromium and VS Code)
- **No TLS inspection** — the proxy sees domain names (via CONNECT) but not request bodies
- **No network filtering** — SBPL doesn't support domain-based or port-based filtering for outbound TCP
- **Keychain access required** — Copilot stores auth tokens in macOS Keychain
- **Proxy doesn't intercept Copilot** — Node.js ignores `http_proxy`; the proxy is for tools like `gh`
- **`sandbox-exec` is deprecated** — Apple has not removed it but may in future macOS versions

For known attack vectors, out-of-scope threats, and prior art, see [SECURITY.md](SECURITY.md).

## References

- [SECURITY.md](SECURITY.md) — Full security model, threat analysis, test strategy, and prior art
- [Apple sandbox-exec(1)](https://keith.github.io/xcode-man-pages/sandbox-exec.1.html)
- [Chromium Seatbelt V2 Design](https://chromium.googlesource.com/chromium/src/sandbox/+show/refs/heads/main/mac/seatbelt_sandbox_design.md)
- [OWASP SSRF Prevention Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html)
- [michaelneale/agent-seatbelt-sandbox](https://github.com/michaelneale/agent-seatbelt-sandbox)
