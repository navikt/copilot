# copilot-sandbox

macOS Seatbelt sandbox wrapper for GitHub Copilot CLI. Runs Copilot inside Apple's kernel-level sandbox (`sandbox-exec`) so the agent can work on your project but cannot access your secrets or exfiltrate data.

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

| Capability | Status |
|---|---|
| Read/write project directory | ✅ Allowed |
| Read `~/.copilot` (auth tokens) | ✅ Allowed |
| Read `~/.gitconfig` | ✅ Allowed |
| Access macOS Keychain | ✅ Allowed |
| Read `~/.ssh`, `~/.gnupg`, `~/.aws` | 🔒 Kernel-blocked |
| Read `~/.kube`, `~/.docker`, `~/.nais` | 🔒 Kernel-blocked |
| Read `~/.config/gcloud`, `~/.config/gh` | 🔒 Kernel-blocked |
| Read `~/.netrc`, `~/.npmrc`, `~/.vault-token` | 🔒 Kernel-blocked |
| Direct outbound network | 🔒 Kernel-blocked |
| Network via localhost proxy | ✅ Allowed (with `--with-proxy`) |
| Execute binaries from `/tmp` | 🔒 Kernel-blocked |
| Child process inheritance | ✅ All restrictions apply to subprocesses |

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
# Run copilot in sandbox (no network — dry run / offline mode)
copilot-sandbox -- --version

# Run with network proxy (required for Copilot API access)
copilot-sandbox --with-proxy -- -p "fix the tests"

# Create config file with defaults
copilot-sandbox --init-config
```

## Usage

```
copilot-sandbox [OPTIONS] [-- <COPILOT_ARGS>...]
```

Everything after `--` is passed directly to the `copilot` command.

### Network access

By default, Copilot runs with **no internet access at all**. This is the safest mode, but Copilot needs the GitHub API for most real tasks.

| Flag | What it does |
|---|---|
| `--with-proxy` | Let Copilot access the internet through a local proxy. All traffic is logged and you can block specific domains. |
| `--no-proxy` | Force the proxy off, even if your config file enables it. |
| `--proxy-port <PORT>` | Which port the proxy listens on (default: 18080). |
| `--blocked-domains <FILE>` | A text file with domains to block, one per line (e.g. `pastebin.com`). Re-read on every request, so you can edit it live. |

### File access

Copilot can only read and write to the project directory. Everything else (SSH keys, cloud credentials, etc.) is blocked by the kernel.

| Flag | What it does |
|---|---|
| `-d, --project-dir <DIR>` | Which directory Copilot can work in. Defaults to the current git repo root. |
| `--allow-read <PATH>` | Let Copilot read files outside the project (e.g. shared libraries, docs). Can be repeated. |
| `--allow-write <PATH>` | Let Copilot read AND write outside the project. Use carefully. Can be repeated. |
| `--deny-path <PATH>` | Block a path that would otherwise be allowed. Deny always wins. Can be repeated. |

### Other options

| Flag | What it does |
|---|---|
| `--no-validate` | Skip the startup check that verifies sandbox restrictions are active. |
| `--init-config` | Create a starter config file at `~/.config/copilot-sandbox/config.toml` and exit. |

### Examples

```bash
# Most common: run Copilot with internet access
copilot-sandbox --with-proxy -- -p "fix the tests"

# Fully offline (verify the sandbox works)
copilot-sandbox -- --version

# Let Copilot read a shared library directory
copilot-sandbox --with-proxy --allow-read ~/shared-libs -- -p "use shared-libs"

# Block a path you don't want Copilot to see
copilot-sandbox --with-proxy --deny-path ~/.config/gh -- -p "refactor auth"

# Block paste sites to prevent data exfiltration
copilot-sandbox --with-proxy --blocked-domains ./blocked.txt -- -p "refactor"
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
enabled = true
port = 18080
# blocked_domains = "/path/to/blocked.txt"

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
│  └─────┬─────┘  └──────┬──────┘ │
│        │               │        │
│        ▼               │        │
│  sandbox-exec ─────────┘        │
│  (Apple kernel)                  │
│        │                         │
│        ▼                         │
│  copilot (sandboxed)             │
│  ├── All child processes         │
│  ├── Cannot read ~/.ssh          │
│  ├── Cannot reach internet       │
│  └── Can only reach proxy        │
└──────────────────────────────────┘
```

**Security model**: deny-by-default with kernel enforcement. See [SECURITY.md](SECURITY.md) for the full threat model, defense layers, and how they are tested.

## Domain blocking

Edit `blocked.txt` while the proxy runs — changes take effect immediately:

```
# Block paste sites
pastebin.com
transfer.sh

# Block known exfiltration targets
requestbin.com
```

## Limitations

- **macOS only** — uses `sandbox-exec` (deprecated but functional, used by Chromium and VS Code)
- **No TLS inspection** — the proxy sees domain names (via CONNECT) but not request bodies
- **Keychain access required** — Copilot stores auth tokens in macOS Keychain
- **`sandbox-exec` is deprecated** — Apple has not removed it but may in future macOS versions

For known attack vectors, out-of-scope threats, and prior art, see [SECURITY.md](SECURITY.md).

## References

- [SECURITY.md](SECURITY.md) — Full security model, threat analysis, test strategy, and prior art
- [Apple sandbox-exec(1)](https://keith.github.io/xcode-man-pages/sandbox-exec.1.html)
- [Chromium Seatbelt V2 Design](https://chromium.googlesource.com/chromium/src/sandbox/+show/refs/heads/main/mac/seatbelt_sandbox_design.md)
- [OWASP SSRF Prevention Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html)
- [michaelneale/agent-seatbelt-sandbox](https://github.com/michaelneale/agent-seatbelt-sandbox)
