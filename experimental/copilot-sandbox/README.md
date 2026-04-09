# copilot-sandbox

macOS Seatbelt sandbox wrapper for GitHub Copilot CLI. Runs Copilot inside Apple's kernel-level sandbox (`sandbox-exec`) so the agent can work on your project but cannot access your secrets or exfiltrate data.

> **macOS only** — uses Apple's Seatbelt framework (the same mechanism App Store apps run under).

## What it does

| Capability | Status |
|---|---|
| Read/write project directory | ✅ Allowed |
| Read `~/.copilot` (auth tokens) | ✅ Allowed |
| Read `~/.gitconfig` | ✅ Allowed |
| Access macOS Keychain | ✅ Allowed |
| Read `~/.ssh`, `~/.gnupg`, `~/.aws` | 🔒 Kernel-blocked |
| Read `~/.kube`, `~/.docker`, `~/.nais` | 🔒 Kernel-blocked |
| Read `~/.netrc`, `~/.npmrc` | 🔒 Kernel-blocked |
| Direct outbound network | 🔒 Kernel-blocked |
| Network via localhost proxy | ✅ Allowed (with `--with-proxy`) |
| Child process inheritance | ✅ All restrictions apply to subprocesses |

## Quick start

```bash
cd experimental/copilot-sandbox
mise install           # Install Rust toolchain
mise run build:release

# Run copilot in sandbox (no network — dry run / offline mode)
./target/release/copilot-sandbox -- --version

# Run with network proxy (required for Copilot API access)
./target/release/copilot-sandbox --with-proxy -- -p "fix the tests"

# Create config file with defaults
./target/release/copilot-sandbox --init-config
```

## Usage

```
copilot-sandbox [OPTIONS] [-- <COPILOT_ARGS>...]

Options:
  -d, --project-dir <DIR>        Project directory (default: git repo root)
      --with-proxy               Enable localhost CONNECT proxy
      --no-proxy                 Disable proxy (overrides config file)
      --proxy-port <PORT>        Proxy port (default: 18080)
      --blocked-domains <FILE>   Domain blocklist file
      --allow-read <PATH>        Additional read-allowed paths
      --allow-write <PATH>       Additional read+write-allowed paths
      --deny-path <PATH>         Additional denied paths
      --no-validate              Skip profile validation
      --init-config              Create default config file and exit
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

**Security model**: deny-by-default. The sandbox profile starts with `(deny default)`, imports macOS `bsd.sb` for system library access, then explicitly allows only what Copilot needs. Sensitive paths are denied AFTER broader allows, which correctly overrides them in Seatbelt's rule evaluation.

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

## References

- [Apple sandbox-exec(1)](https://keith.github.io/xcode-man-pages/sandbox-exec.1.html)
- [Chromium Seatbelt V2 Design](https://chromium.googlesource.com/chromium/src/sandbox/+show/refs/heads/main/mac/seatbelt_sandbox_design.md)
- [michaelneale/agent-seatbelt-sandbox](https://github.com/michaelneale/agent-seatbelt-sandbox)
- [HackTricks: macOS Sandbox](https://book.hacktricks.wiki/en/macos-hardening/macos-security-and-privilege-escalation/macos-security-protections/macos-sandbox/index.html)
