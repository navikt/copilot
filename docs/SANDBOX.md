# Agent Sandbox Research

Research into sandboxing capabilities across GitHub Copilot surfaces, with focus on Copilot CLI.

**Last updated**: 2026-04-09

---

## Overview

| Surface | Built-in Sandbox | Isolation Type | Status |
|---------|-----------------|----------------|--------|
| **VS Code Agent Mode** | ✅ Preview | OS-level (bubblewrap / sandbox-exec) | 6 settings available |
| **Cloud Coding Agent** | ✅ GA | Ephemeral VM + firewall | Production-ready |
| **Copilot CLI** | ❌ None | Software-level permission prompts only | Feature requested ([#892](https://github.com/github/copilot-cli/issues/892)) |

---

## 1. VS Code Agent Mode Sandbox

VS Code wraps terminal execution through OS-level sandboxing ([trust & safety docs](https://code.visualstudio.com/docs/copilot/concepts/trust-and-safety), [security docs](https://code.visualstudio.com/docs/copilot/security)). All settings are marked **Preview**.

| Setting | Purpose | Details |
|---------|---------|---------|
| `chat.agent.sandbox.enabled` | Master toggle | `on` / `off` — restricts what tools like the terminal can do |
| `chat.agent.sandbox.allowedNetworkDomains` | Network whitelist | Wildcards supported (`*.example.com`). Empty = no network |
| `chat.agent.sandbox.deniedNetworkDomains` | Network blacklist | Takes precedence over allowed list |
| `chat.agent.sandbox.fileSystem.linux` | Filesystem rules (Linux) | Literal paths only (e.g., `./src/`, `~/.ssh`). Requires **bubblewrap** + **socat** |
| `chat.agent.sandbox.fileSystem.mac` | Filesystem rules (macOS) | Git-style glob patterns (e.g., `./src/**/*.ts`). Uses Apple **sandbox-exec** (Seatbelt) |
| `claudeAgent.allowDangerouslySkipPermissions` | Bypass prompts | For sandboxes with no internet access only |

### How It Works

VS Code **wraps** the terminal — it intercepts shell execution and routes commands through the OS sandbox mechanism. The agent process never runs with direct host access.

- **Linux**: [bubblewrap](https://github.com/containers/bubblewrap) creates a user namespace with restricted filesystem and network. `socat` handles socket forwarding.
- **macOS**: Apple's built-in `sandbox-exec` (Seatbelt framework) enforces path and network rules via kernel-level policy profiles.

When sandboxing is enabled, VS Code **auto-approves** terminal commands inside the sandbox, reducing prompt fatigue while maintaining safety.

### Trust Boundaries

VS Code enforces multiple trust layers beyond sandbox settings ([docs](https://code.visualstudio.com/docs/copilot/security)):

| Boundary | How It Works |
|----------|-------------|
| **Workspace Trust** | Untrusted workspaces run in restricted mode — agents and risky features disabled |
| **Extension Publisher Trust** | Users prompted to trust extensions before activation |
| **MCP Server Trust** | Each MCP server must be explicitly trusted; config changes re-trigger checks |
| **Network Domain Trust** | Explicit approval required before agents access external URLs/APIs |

Trust can be revoked at any time via built-in commands.

### Dependencies

| OS | Required packages |
|----|------------------|
| Linux | `bubblewrap`, `socat` (must be installed separately) |
| macOS | None — `sandbox-exec` is built into macOS |
| Windows | Not yet supported |

---

## 2. Cloud Coding Agent Sandbox

The cloud coding agent runs in an **ephemeral GitHub Actions VM** — the most robust sandbox ([customizing the environment](https://docs.github.com/en/enterprise-cloud@latest/copilot/customizing-copilot/customizing-the-development-environment-for-copilot-coding-agent)).

- **Isolated VM**: Fresh Ubuntu runner per session, destroyed after completion
- **Integrated firewall**: Restricts network access to approved hosts (configurable, can be disabled)
- **`copilot-setup-steps.yml`**: Defines environment (dependencies, runner type, services)
- **Org-level runner controls** ([April 2026](https://github.blog/changelog/2026-04-03-organization-runner-controls-for-copilot-cloud-agent/)): Admins can lock runner config across all repos
- **Org-level firewall settings** ([April 2026](https://github.blog/changelog/2026-04-03-organization-firewall-settings-for-copilot-cloud-agent/)): Org admins can configure firewall for all repos, custom allowlists
- **Validation tools**: CodeQL, secret scanning, code review run automatically ([configurable](https://github.blog/changelog/2026-03-18-configure-copilot-coding-agents-validation-tools))

### Firewall Details

The firewall controls **outbound** network access from the agent's VM ([docs](https://docs.github.com/en/enterprise-cloud@latest/copilot/how-tos/use-copilot-agents/coding-agent/customize-the-agent-firewall)):

- Common package registries, OS repos, and CAs are pre-allowed
- Admins can customize allowlists (e.g., internal registries)
- Blocked requests generate warnings in PR comments for transparency
- **Limitation**: Only governs processes started by the Copilot agent — not processes started by MCP servers

### Third-Party Hardening: StepSecurity Harden-Runner

[StepSecurity Harden-Runner](https://github.com/step-security/harden-runner) provides additional runtime visibility for cloud agent sessions ([blog](https://www.stepsecurity.io/blog/securing-github-copilot-in-github-actions-with-harden-runner)):

- Monitors every process execution, file access, and outbound network connection
- Maps activity back to specific workflow steps
- Detects source code tampering during build
- Complements the built-in firewall by solving the "black box" problem

---

## 3. Copilot CLI — Current State

### What Exists: Permission System (Software-Level)

The CLI has permission flags ([documented in #307](https://github.com/github/copilot-cli/issues/307), [CLI getting started](https://docs.github.com/en/copilot/how-tos/copilot-cli/cli-getting-started)), but these are **software-level** restrictions — the agent process runs with full user permissions.

| Flag | Purpose |
|------|---------|
| `--allow-all-tools` | Auto-approve all tool executions |
| `--allow-all-paths` | Allow access to any file path |
| `--allow-tool <name>` | Allow specific tools only |
| `--deny-tool <name>` | Block specific tools (takes precedence) |
| `--add-dir <path>` | Add directory to allowlist |
| `--allow-all-urls` | Allow all network requests |

Interactive confirmation prompts are the primary safety mechanism.

### What Does NOT Exist

- ❌ OS-level process isolation (no bubblewrap, no sandbox-exec)
- ❌ Network domain firewall
- ❌ Filesystem path enforcement at OS level
- ❌ Org-level policy controls
- Zero sandbox-related code in the [github/copilot-cli](https://github.com/github/copilot-cli) repository (confirmed: repo is issues/releases only — binary is closed-source)
- Zero PRs implementing sandbox functionality ([search: is:pr sandbox](https://github.com/github/copilot-cli/pulls?q=is%3Apr+sandbox))

### Known Bypass Vulnerabilities

| Issue | Title | Severity |
|-------|-------|----------|
| [#2309](https://github.com/github/copilot-cli/issues/2309) | Files deleted outside allowed dirs via `~` expansion and Python `shutil.rmtree` | `--add-dir` bypass |
| [#2173](https://github.com/github/copilot-cli/issues/2173) | Shell tool can read ANY file outside working directory | `--allow-tool 'shell'` grants full read |
| [#2392](https://github.com/github/copilot-cli/issues/2392) | `preToolUse` hooks not enforced in subagents | Hook-based restrictions trivially bypassed |

### Feature Requests

| Issue | Title | 👍 | Labels |
|-------|-------|-----|--------|
| [#892](https://github.com/github/copilot-cli/issues/892) | **Add sandbox mode** — restrict file access to working directory | 26 | `priority: medium`, `effort: large`, `needs-human` |
| [#1163](https://github.com/github/copilot-cli/issues/1163) | **Use unshare/bubblewrap by default on Linux** | 2 | `area:platform-linux` |
| [#55](https://github.com/github/copilot-cli/issues/55) | **Package copilot-cli in Docker image** | 20 | `enhancement` |
| [#1971](https://github.com/github/copilot-cli/issues/1971) | **Granular org policies for CLI tools** | 1 | `area:tools` |
| [#316](https://github.com/github/copilot-cli/issues/316) | **Epic: Permissions Improvements** (tracks 16+ sub-issues) | 7 | `area:permissions` |
| [#899](https://github.com/github/copilot-cli/issues/899) | **Allow/filter/include/exclude files and paths** | 5 | `triage` |
| [#2284](https://github.com/github/copilot-cli/issues/2284) | **Persist `/add-dir` across sessions** | 3 | `area:permissions` |

### Why Sandboxing Is Architecturally Harder for the CLI

VS Code sandbox works because VS Code **wraps** the terminal — it intercepts shell execution and routes it through bubblewrap/sandbox-exec ([VS Code security docs](https://code.visualstudio.com/docs/copilot/security)). The CLI **is** the terminal, so it cannot sandbox itself from within its own process. Sandboxing the CLI requires an external wrapper (container, VM, or OS-level namespace). This architectural gap is discussed in [#892](https://github.com/github/copilot-cli/issues/892) and the [SSW rule on secure CLI environments](https://www.ssw.com.au/rules/use-github-copilot-cli-secure-environment).

---

## 4. Workaround: Docker Sandboxes

### Option A: `docker sandbox run` (Docker Desktop 4.50+)

Docker Desktop 4.50+ (late 2025) introduced `docker sandbox run` for AI coding agents, using **microVM-based isolation** (Firecracker) ([Docker docs](https://docs.docker.com/ai/sandboxes/), [Docker blog](https://www.docker.com/blog/docker-sandboxes-run-claude-code-and-other-coding-agents-unsupervised-but-safely/), [Andrew Lock deep dive](https://andrewlock.net/running-ai-agents-safely-in-a-microvm-using-docker-sandbox/)).

#### How It Works

```
Host Machine
└── Docker Desktop
    └── Firecracker microVM (dedicated Linux kernel)
        ├── Isolated filesystem (only /workspace mounted)
        ├── Isolated network (policy-controlled)
        ├── Docker daemon (for agent tools)
        └── Copilot CLI process
```

Each sandbox gets:
- **Dedicated Linux kernel** — not shared with host (unlike regular containers)
- **Isolated filesystem** — only the project directory is mounted at `/workspace`
- **Network policy** — Open, Balanced (default), or Locked Down
- **No access to** host home directory, SSH keys, secrets, or system config

#### Dependencies

| Dependency | Version | Notes |
|------------|---------|-------|
| Docker Desktop | ≥ 4.50 | Required for `docker sandbox run` |
| `sbx` CLI | Latest | Alternative CLI for sandbox management |
| Copilot subscription | Any paid plan | For Copilot authentication |
| GitHub PAT | Fine-grained with "Copilot Requests" | For token-based auth |

#### Quick Start

```bash
# Native docker sandbox (Docker Desktop 4.50+)
docker sandbox run copilot .

# Or with a community template
docker sandbox run --template ghcr.io/henrybravo/docker-sandbox-run-copilot copilot .

# Standalone docker run (no docker sandbox required)
docker run -it --rm \
  -v $(pwd):/workspace \
  -e GITHUB_TOKEN="$GITHUB_TOKEN" \
  ghcr.io/henrybravo/docker-sandbox-run-copilot \
  copilot
```

#### Network Policies

| Policy | Behavior |
|--------|----------|
| **Open** | Minimal restrictions — full internet access |
| **Balanced** (default) | Blocks risky outbound; allows package registries, APIs |
| **Locked Down** | Almost no network — only what's needed to run/test code |

#### Community Template: [docker-sandbox-run-copilot](https://github.com/henrybravo/docker-sandbox-run-copilot) (14⭐)

Pre-built Docker image with Copilot CLI + dev tools:

| Included | Version |
|----------|---------|
| Copilot CLI | Configurable (default: latest) |
| Node.js | 22 LTS |
| GitHub CLI (`gh`) | Latest |
| Docker CLI | Latest (for Docker-in-Docker) |
| Python 3 | System |
| Go | System |
| ripgrep, jq, git, vim | Latest |

The image runs as non-root user `agent`, mounts workspace at `/workspace`, and handles GitHub token auth via environment variables or mounted volumes.

See also: [Bruno Borges' tutorial on dev.to](https://dev.to/brunoborges/running-github-copilot-cli-safely-with-docker-sandbox-2f4i) — step-by-step guide using `docker sandbox create copilot`.

#### Limitations

- Dev tools must be pre-installed in the image (your host `go`, `dotnet`, etc. are not available)
- Requires Docker Desktop (not available on all platforms/environments)
- microVM startup adds latency compared to bare-metal CLI
- macOS ARM: Firecracker microVM runs Linux — no native macOS sandbox-exec inside
- Authentication: Sandboxes don't import existing `~/.copilot/` tokens — requires device flow auth

---

### Option B: copilot_here (Cross-Platform Docker Wrapper)

[copilot_here](https://github.com/GordonBeeming/copilot_here) by Gordon Beeming is a polished, cross-platform CLI wrapper that runs Copilot CLI in Docker containers ([project site](https://gordonbeeming.com/copilot_here/), [security blog post](https://gordonbeeming.com/blog/2025-10-03/taming-the-ai-my-paranoid-guide-to-running-copilot-cli-in-a-secure-docker-sandbox)).

#### Features

| Feature | Details |
|---------|---------|
| **Isolation** | Docker container with only current directory mounted (read-only by default) |
| **Auth** | Auto-uses existing `gh` CLI credentials — no re-authentication needed |
| **Network Airlock** | Allowlist-proxy that controls which endpoints Copilot can reach |
| **Multi-stack images** | Node.js, .NET (8/9/10), Playwright, Rust, Go, Java, compound images |
| **Mount control** | `--mount`, `--mount-rw`, `--save-mount` for persistent directory access |
| **YOLO mode** | `copilot_yolo` alias to auto-approve all commands safely inside container |
| **Platform support** | Linux, macOS (Intel + Apple Silicon), Windows (WSL/PowerShell/Docker Desktop) |

#### Installation

```bash
# Homebrew (macOS/Linux)
brew tap gordonbeeming/tap && brew install --cask copilot-here

# WinGet (Windows)
winget install GordonBeeming.CopilotHere

# .NET Tool
dotnet tool install -g copilot_here

# Shell script
curl -fsSL https://github.com/GordonBeeming/copilot_here/releases/download/cli-latest/install.sh | $SHELL
```

#### Usage

```bash
copilot_here "add tests for this script"
copilot_here --mount-rw ~/logs "refactor the logging function"
copilot_here --image nodejs "build the frontend"
```

#### Key Advantage Over Docker Sandboxes

copilot_here provides a **more practical daily-driver experience** with auto-auth, multiple pre-built environment images, and fine-grained mount control. Docker Sandboxes provide **stronger isolation** (microVM vs container).

---

### Option C: Dev Container Feature

The official [Dev Container Feature](https://github.com/devcontainers/features/tree/main/src/copilot-cli) ([source: devcontainer-feature.json](https://github.com/devcontainers/features/blob/main/src/copilot-cli/devcontainer-feature.json), [install.sh](https://github.com/devcontainers/features/blob/main/src/copilot-cli/install.sh)) installs Copilot CLI into any Dev Container.

#### How It Works

Add to your `devcontainer.json`:

```json
{
  "features": {
    "ghcr.io/devcontainers/features/copilot-cli:1": {}
  }
}
```

For prerelease versions:

```json
{
  "features": {
    "ghcr.io/devcontainers/features/copilot-cli:1": {
      "version": "prerelease"
    }
  }
}
```

#### Dependencies

| Dependency | Required | Notes |
|------------|----------|-------|
| VS Code + Dev Containers extension | Yes | Or GitHub Codespaces |
| Docker Desktop / Docker Engine | Yes | For container runtime |
| Debian/Ubuntu-based container | Yes | Feature uses `apt-get` + `wget` |
| `bash` | Yes | For install script |
| Copilot subscription | Yes | For authentication |

#### What the Install Script Does

1. Downloads the Copilot CLI binary from GitHub Releases (`copilot-linux-{arch}.tar.gz`)
2. Installs to `/usr/local/bin/copilot`
3. Supports `x64` and `arm64` architectures
4. Installs dependencies: `wget`, `tar`, `ca-certificates`, `git`

#### Isolation Model

Dev Containers provide container-level isolation (Linux namespaces + cgroups), not microVM-level. The container shares the host kernel. This is less secure than Docker Sandboxes ([comparison](https://www.morphllm.com/docker-sandbox)) but more practical for daily development.

| Aspect | Docker Sandbox | Dev Container |
|--------|---------------|---------------|
| Isolation | microVM (Firecracker) | Container (namespaces) |
| Kernel | Dedicated | Shared with host |
| Setup | `docker sandbox run copilot .` | `devcontainer.json` + rebuild |
| Dev tools | Pre-baked in image | Customizable via features/Dockerfile |
| IDE integration | Terminal only | Full VS Code integration |
| Persistence | Ephemeral or persistent | Persistent (volume-backed) |
| Overhead | Higher (VM boot) | Lower (container start) |

#### Limitations

- Not a security sandbox — it's a development environment with container-level isolation
- Shares host kernel (container escape theoretically possible)
- Requires rebuilding the container when changing features
- No network policy controls (unlike Docker Sandboxes)

---

### Option D: Manual bubblewrap (Linux Only)

From [github/copilot-cli#1163](https://github.com/github/copilot-cli/issues/1163), a user-contributed workaround:

```bash
bwrap --ro-bind / / \
      --bind "$PWD" "$PWD" \
      --bind /tmp /tmp \
      --bind "$HOME/.copilot" "$HOME/.copilot" \
      --dev /dev \
      --proc /proc \
      --unshare-all \
      copilot
```

#### What This Does

- Read-only bind of entire root filesystem
- Read-write only for current directory, `/tmp`, and `~/.copilot`
- Unshares all namespaces (PID, network, mount, etc.)

#### Dependencies

| Dependency | Required | Notes |
|------------|----------|-------|
| `bubblewrap` | Yes | `apt install bubblewrap` or `dnf install bubblewrap` |
| Linux | Yes | Not available on macOS or Windows |
| Unprivileged user namespaces | Yes | Kernel config `CONFIG_USER_NS=y` (most distros) |

#### Known Issue

A commenter on [#1163](https://github.com/github/copilot-cli/issues/1163) reported `bwrap: loopback: Failed RTM_NEWADDR: Operation not permitted` on v1.0.9, requiring `--yolo` to bypass. The `--unshare-all` flag may be too restrictive for some setups.

---

### Option E: Rootless Docker / Podman (Linux)

From [georg.dev](https://georg.dev/blog/07-sandbox-your-github-copilot-cli-on-linux/) and [dev.to/lunran](https://dev.to/lunran/how-to-set-up-a-sandbox-environment-for-github-copilot-cli-on-linux-39n2):

Rootless Docker maps container root to an unprivileged host user, adding a security layer over standard Docker.

```bash
# Install rootless Docker prerequisites
sudo apt-get install -y uidmap dbus-user-session

# Build custom image with Copilot CLI
docker build -t copilot-sandbox .

# Run sandboxed
docker run --rm -it \
  -v ~/my/project:/workspace \
  -v ~/.copilot:/home/agent/.copilot \
  copilot-sandbox copilot --yolo
```

**Podman** can be used as a drop-in replacement (`podman` instead of `docker`), often with better rootless support out-of-the-box.

**Key limitation**: Neither rootless Docker nor Podman isolates the network by default — Copilot retains internet access. Use `--network=none` for full network isolation.

---

## 5. Third-Party Sandbox Ecosystem

### Copilot CLI-Specific Tools

These tools are purpose-built to run `copilot` inside an isolated environment:

| Tool | Type | Copilot-Specific | Source |
|------|------|-----------------|--------|
| **Docker Sandboxes** | microVM (Firecracker) | ✅ Native `copilot` agent type | [Docker docs](https://docs.docker.com/ai/sandboxes/agents/copilot/) |
| **copilot_here** | Docker container wrapper | ✅ Built for Copilot CLI | [GitHub](https://github.com/GordonBeeming/copilot_here) |
| **docker-sandbox-run-copilot** | Docker template | ✅ Copilot image + entrypoint | [GitHub](https://github.com/henrybravo/docker-sandbox-run-copilot) |
| **GeekTrainer's gist** | Docker sandbox config | ✅ Copilot CLI setup | [Gist](https://gist.github.com/GeekTrainer/949f8d198ef9a2b9f90253f17ec8ce1c) |
| **Dev Container Feature** | Dev Container | ✅ Installs Copilot CLI | [GitHub](https://github.com/devcontainers/features/tree/main/src/copilot-cli) |

### Process-Level Sandbox Tools (Copilot-Compatible)

These tools provide OS-level process isolation for any CLI agent, including `copilot`. They don't require Docker and run natively on the host:

| Tool | Isolation | Copilot Support | Platform | License | Source |
|------|-----------|----------------|----------|---------|--------|
| **Vectimus** | Cedar policy engine — intercepts tool calls via hooks, <10ms eval | ✅ Explicit `PreToolUse` hook | macOS, Linux, Win | Apache 2.0 | [GitHub](https://github.com/vectimus/vectimus) |
| **Yu** | sandbox-exec + credential proxy + env sanitization + APFS snapshots | ✅ `yu . -- copilot` | macOS (Apple Silicon) | MIT | [GitHub](https://github.com/qingant/yu) |
| **Hazmat** | Dedicated macOS user + Seatbelt + pf firewall + DNS blocklist + snapshots | ⚠️ Via `hazmat exec copilot` | macOS only | MIT | [GitHub](https://github.com/dredozubov/hazmat) |
| **Zerobox** | OpenAI Codex sandbox crates — bubblewrap (Linux) / sandbox-exec (macOS) | ⚠️ Wraps any process | macOS, Linux | Apache 2.0 | [GitHub](https://github.com/afshinm/zerobox) |

#### Vectimus — Cedar Policy Enforcement

[Vectimus](https://github.com/vectimus/vectimus) is unique in that it doesn't sandbox at the OS level — it intercepts every tool call at the **hook layer** and evaluates it against [AWS Cedar](https://www.cedarpolicy.com/) policies before execution. It explicitly supports Copilot CLI via a `PreToolUse` command hook:

```json
// .github/hooks/vectimus.json
{
  "hooks": {
    "PreToolUse": [
      {"type": "command", "command": "vectimus hook --source copilot"}
    ]
  }
}
```

- **11 incident-driven policy packs** covering all [OWASP Agentic Top 10](https://owasp.org/www-project-agentic-ai-threats/) categories
- Each policy links to a **real attack** it prevents (e.g., Clinejection npm backdoor, Terraform destroy incident)
- **Ed25519-signed audit receipts** — cryptographic proof of every allow/deny decision
- **Observe mode** — trial run without blocking to see what would be caught
- **MCP server governance** — blocks all MCP servers by default, approve via allowlist
- **Sentinel pipeline** — 3-agent system that scans for new threats daily, drafts policies, replays in sandbox, opens PRs
- Install: `pipx install vectimus && vectimus init`

#### Yu — Credential Isolation

[Yu](https://github.com/qingant/yu) solves a different problem: the agent can use credentials (git push, API calls) **without ever seeing them**. Four isolation layers:

1. **Filesystem**: macOS `sandbox-exec` hides everything except the project directory — `~/.ssh`, `~/.aws` don't exist inside the sandbox
2. **Environment**: Default-deny whitelist — secrets (`KEY`, `TOKEN`, `SECRET`) replaced with dummy values
3. **API proxy**: Localhost reverse proxy swaps dummy tokens for real keys in HTTP headers
4. **Command proxy**: `git`, `ssh`, `gh`, `aws` intercepted by shims — real commands run outside sandbox with full credentials

```bash
yu . -- copilot          # Copilot CLI in credential-isolated sandbox
yu . -- claude           # Claude Code
yu .                     # Auto-detect agent
```

- **APFS snapshots**: Automatic copy-on-write snapshots before risky operations, instant rollback via `yu rollback`
- **No permission prompts**: The sandbox is the security — agents run with full autonomy
- Apple Silicon macOS only (Linux planned)

#### Hazmat — macOS OS-Level Containment

[Hazmat](https://github.com/dredozubov/hazmat) implements **7 concurrent security layers** to make `--dangerously-skip-permissions` safe:

1. **User isolation** — Dedicated `agent` macOS user account (no access to main user's home)
2. **Kernel sandbox** — Per-session Seatbelt SBPL policy (filesystem default-deny)
3. **Credential denial** — Kernel-enforced blocks on `~/.ssh`, `~/.aws`, `~/.gnupg`
4. **Network firewall** — macOS `pf` blocks SMTP, IRC, FTP, Tor, VPN, exfil protocols
5. **DNS blocklist** — Known tunnel/paste/C2 services (ngrok, pastebin, webhook.site) → localhost
6. **Supply chain hardening** — `ignore-scripts=true` in npmrc (blocks `postinstall` attacks)
7. **Automatic snapshots** — Kopia backups, `hazmat restore` for rollback

- **TLA+ formally verified** — Setup/rollback logic proven across 26,905 states, found 3 real bugs
- Mitigates **16+ Claude Code CVEs** ([documented audit](https://github.com/dredozubov/hazmat/blob/master/docs/cve-audit.md))
- Install: `brew install dredozubov/tap/hazmat && hazmat init`
- ⚠️ Known gap: HTTPS exfiltration not blocked (can `curl` any domain on 443)

#### Zerobox — Codex Sandbox Crates

[Zerobox](https://github.com/afshinm/zerobox) is a single Rust binary using **upstream OpenAI Codex sandboxing crates** (`codex-sandboxing`, `codex-linux-sandbox`). Deno-like permission model:

```bash
zerobox --deny-read=/etc --deny-net=evil.com -- copilot
zerobox --allow-write=./project --allow-net=api.github.com -- copilot
```

- **Credential injection via MITM proxy** — sandboxed process sees dummy keys, proxy swaps for real ones
- **Snapshot/restore** — Record and auto-restore filesystem changes
- **~10ms overhead**, ~7-8MB memory footprint, no Docker required
- macOS (sandbox-exec) + Linux (bubblewrap + seccomp)
- 427 ⭐ — most popular of the process-level tools

### Generic Container-Based Sandbox Tools

| Tool | Type | How It Works | Source |
|------|------|-------------|--------|
| **sandock** | Python CLI wrapper | Runs any program in a Docker container with Deno-like permission model | [PyPI](https://pypi.org/project/sandock/) |
| **bubblewrap** | Linux namespace sandbox | OS-level process isolation via user namespaces | [GitHub](https://github.com/containers/bubblewrap) |
| **Rootless Docker** | Container runtime | Non-privileged Docker daemon, maps root→unprivileged user | [Docker docs](https://docs.docker.com/engine/security/rootless/) |
| **Podman** | Container runtime | Rootless by default, drop-in Docker replacement | [podman.io](https://podman.io/) |

### Cloud Agent Sandbox Platforms

These are SDK-based platforms for building agent infrastructure. They **do not natively support Copilot CLI** but could theoretically run it inside their VMs if you install the binary manually. They're primarily designed for custom agent systems using their own SDKs:

| Platform | Isolation | SDK | Primary Use Case | Copilot CLI Support | Source |
|----------|-----------|-----|-----------------|-------------------|--------|
| **E2B** | Firecracker microVM | Python, JS | Cloud code execution for AI agents | ❌ Not native (install manually) | [e2b.dev](https://e2b.dev/) |
| **Daytona** | Secure containers | Python | Stateful agent code execution | ❌ Not native | [daytona.io](https://www.daytona.io/) |
| **Runloop** | Micro-VM (Devboxes) | Python, REST | Agent evaluation + production sandboxes | ❌ Not native | [runloop.ai](https://runloop.ai/) |
| **Modal** | Firecracker | Python | ML/compute workloads | ❌ Not native | [modal.com](https://modal.com/) |
| **VibeKit** | Via E2B/Daytona/Modal | TypeScript | OSS SDK for orchestrating coding agents in sandboxes | ❌ Orchestration layer, not CLI wrapper | [GitHub](https://github.com/superagent-ai/vibekit) |

> **Key distinction**: The cloud platforms above are for _building your own_ sandboxed agent systems. For running the `copilot` binary safely, use the Copilot-specific or generic CLI tools in the tables above.

### Cloud Agent Hardening (for Copilot Cloud Coding Agent)

These complement the cloud coding agent's built-in firewall:

| Tool | What It Does | Source |
|------|-------------|--------|
| **StepSecurity Harden-Runner** | EDR-like runtime visibility for GitHub Actions — monitors processes, files, network in agent sessions | [GitHub](https://github.com/step-security/harden-runner), [blog](https://www.stepsecurity.io/blog/securing-github-copilot-in-github-actions-with-harden-runner) |

---

## 6. Comparison Matrix

| Capability | VS Code Agent | Cloud Agent | CLI (bare) | Docker Sandbox | copilot_here | Yu | Zerobox | Vectimus |
|-----------|---------------|-------------|------------|---------------|-------------|-----|---------|----------|
| Process isolation | ✅ sandbox-exec/bwrap | ✅ Ephemeral VM | ❌ | ✅ microVM | ⚠️ Container | ✅ sandbox-exec | ✅ bwrap/sandbox-exec | ❌ Hook-layer only |
| Network firewall | ✅ Domain lists | ✅ Integrated | ❌ | ✅ Policy-based | ✅ Airlock proxy | ❌ (planned) | ✅ Domain allow/deny | ❌ |
| Filesystem restrictions | ✅ Path rules | ✅ VM boundary | ⚠️ Software-only | ✅ Mount-based | ✅ Mount control | ✅ Invisible paths | ✅ Deno-like perms | ⚠️ Policy-based |
| Credential isolation | N/A | ✅ VM boundary | ❌ | ✅ No host access | ⚠️ Mount control | ✅ Proxy + dummy keys | ✅ MITM proxy | ❌ |
| Snapshot/rollback | ❌ | N/A | ❌ | ❌ | ❌ | ✅ APFS clones | ✅ Record + restore | ❌ |
| Policy/audit trail | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ Cedar + Ed25519 |
| Org-level controls | ❌ | ✅ Runner + firewall | ❌ | ❌ | ❌ | ❌ | ❌ | ⚠️ Daemon mode |
| Host kernel shared | N/A | ❌ | N/A | ❌ (dedicated) | ✅ | N/A (native) | N/A (native) | N/A |
| Auth experience | Seamless | Automatic | Seamless | Device flow | Auto (gh CLI) | Seamless | Seamless | Seamless |
| Startup overhead | Negligible | ~30s | None | ~5-10s | ~5s | Negligible | ~10ms | ~10ms |
| Docker required | No | No | No | Yes (Desktop) | Yes | No | No | No |
| Platform | Linux, macOS | Linux (Actions) | All | Docker Desktop | Linux, macOS, Win | macOS (Apple Silicon) | macOS, Linux | macOS, Linux, Win |
| Maturity | Preview (official) | GA (official) | GA (official) | GA (Docker) | Mature (community) | New (community) | New (427⭐) | New (28⭐) |

---

## 7. Recommendation for Nav

Most Nav developers use **macOS (Apple Silicon)** with **Docker Desktop** and **VS Code**. The recommendations below are ordered by maturity and practical value.

### Tier 1: Ready to Use Today

1. **VS Code built-in sandbox** — Zero-install. Toggle `chat.agent.sandbox.enabled` in settings. Uses macOS `sandbox-exec` natively. Covers agent mode sessions. _Preview but official._

2. **`docker sandbox run copilot .`** — One command, strongest isolation (Firecracker microVM). Requires Docker Desktop 4.50+. [Official Docker docs](https://docs.docker.com/ai/sandboxes/agents/copilot/). _Best for YOLO/unattended sessions._

3. **[copilot_here](https://github.com/GordonBeeming/copilot_here)** — Daily-driver CLI wrapper. Auto-auth via `gh`, network airlock, multi-stack images. `brew install --cask copilot-here`. _Best DX for teams that want sandboxing without friction._

### Tier 2: Worth Evaluating

4. **[Yu](https://github.com/qingant/yu)** — **Credential isolation** without containers. The agent can `git push` without ever seeing your SSH key. Uses macOS `sandbox-exec` + credential proxy + APFS snapshots. Particularly interesting for Nav because:
   - Most Nav developers are on Apple Silicon Macs ✅
   - No Docker dependency — native macOS process, negligible overhead ✅
   - Solves credential exposure (SSH keys, GitHub tokens, API keys) at the OS level ✅
   - Auto-rollback via APFS snapshots ✅
   - `yu . -- copilot` to sandbox Copilot CLI ✅
   - ⚠️ Very new project, macOS-only, no Linux support yet

5. **[Zerobox](https://github.com/afshinm/zerobox)** — Process-level sandbox using OpenAI Codex crates. Deno-like permission flags (`--deny-read`, `--deny-net`). Cross-platform (macOS + Linux). 427⭐, most popular of the new tools. No Docker required.

### Tier 3: Watch

6. **[Vectimus](https://github.com/vectimus/vectimus)** — Policy engine (not sandbox). Interesting for org-wide governance — intercepts tool calls and evaluates against Cedar policies. Could complement any sandbox tool. 28⭐, single maintainer.

7. **Official CLI sandbox** — [#892](https://github.com/github/copilot-cli/issues/892) is labeled `priority: medium, effort: large`. Both Gemini CLI and Claude Code already ship built-in sandboxing. Competitive pressure is increasing, but no timeline from GitHub.

### Proposed Testing Plan

Test these 4 tools on a real Nav project (e.g., a simple Kotlin/Ktor or Next.js app) and evaluate:

| Test | What to Verify |
|------|---------------|
| **Setup** | Install time, prerequisites, auth flow |
| **DX** | Startup speed, tool availability, prompt experience |
| **Filesystem** | Can the agent read `~/.ssh`? `~/.aws`? Files outside project? |
| **Network** | Can the agent `curl` arbitrary hosts? Exfiltrate data? |
| **Credentials** | Are env vars visible? Can the agent see real tokens? |
| **Git** | Does `git push` work? With whose credentials? |
| **Rollback** | Can you undo all changes from a session? |
| **Dev tools** | Does the agent have access to `go`, `node`, `pnpm`, etc.? |

```bash
# Quick test commands for each tool:

# 1. Docker Sandbox
docker sandbox run copilot .

# 2. copilot_here
brew tap gordonbeeming/tap && brew install --cask copilot-here
copilot_here "list files in home directory"

# 3. Yu
sudo curl -fsSL https://github.com/qingant/yu/releases/latest/download/yu-darwin-arm64 \
  -o /usr/local/bin/yu && sudo chmod +x /usr/local/bin/yu
yu . -- copilot

# 4. Zerobox
curl -fsSL https://raw.githubusercontent.com/afshinm/zerobox/main/install.sh | sh
zerobox --deny-read=$HOME/.ssh --deny-read=$HOME/.aws -- copilot
```

---

## References

### Official GitHub / Microsoft

- [github/copilot-cli](https://github.com/github/copilot-cli) — Public repo (issue tracker, releases)
- [Epic: Permissions Improvements (#316)](https://github.com/github/copilot-cli/issues/316) — Tracks 16+ permission-related issues
- [Add sandbox mode (#892)](https://github.com/github/copilot-cli/issues/892) — Primary sandbox feature request
- [VS Code: Trust and safety](https://code.visualstudio.com/docs/copilot/concepts/trust-and-safety) — Agent trust model, approval levels, checkpoints
- [VS Code: Security](https://code.visualstudio.com/docs/copilot/security) — Trust boundaries, MCP server trust, sandbox details
- [VS Code: Copilot CLI sessions](https://code.visualstudio.com/docs/copilot/agents/copilot-cli) — Worktree/workspace isolation modes
- [Safeguarding VS Code against prompt injections](https://github.blog/security/vulnerability-research/safeguarding-vs-code-against-prompt-injections/) — GitHub Security blog
- [Copilot cloud agent firewall docs](https://docs.github.com/en/enterprise-cloud@latest/copilot/how-tos/use-copilot-agents/coding-agent/customize-the-agent-firewall) — Firewall configuration
- [Org firewall settings (April 2026)](https://github.blog/changelog/2026-04-03-organization-firewall-settings-for-copilot-cloud-agent/)
- [Org runner controls (April 2026)](https://github.blog/changelog/2026-04-03-organization-runner-controls-for-copilot-cloud-agent/)
- [Copilot Trust Center](https://copilot.github.trust.page/) — Compliance (SOC 1/2/3, ISO 27001, ISO 42001)
- [Demystifying Copilot Security Controls](https://techcommunity.microsoft.com/blog/azuredevcommunityblog/demystifying-github-copilot-security-controls-easing-concerns-for-organizational/4468193) — Microsoft Tech Community

### Docker Sandboxes

- [Docker Sandboxes documentation](https://docs.docker.com/ai/sandboxes/)
- [Docker Copilot agent page](https://docs.docker.com/ai/sandboxes/agents/copilot/) — Official Docker docs for Copilot in sandboxes
- [Docker blog: Run coding agents unsupervised but safely](https://www.docker.com/blog/docker-sandboxes-run-claude-code-and-other-coding-agents-unsupervised-but-safely/)
- [Docker blog: Run agents in YOLO mode, safely](https://www.docker.com/blog/docker-sandboxes-run-agents-in-yolo-mode-safely/)
- [Running GitHub Copilot CLI Safely with Docker Sandbox](https://dev.to/brunoborges/running-github-copilot-cli-safely-with-docker-sandbox-2f4i) — Bruno Borges, dev.to
- [Running AI agents safely in a microVM](https://andrewlock.net/running-ai-agents-safely-in-a-microvm-using-docker-sandbox/) — Andrew Lock
- [docker-sandbox-run-copilot](https://github.com/henrybravo/docker-sandbox-run-copilot) — Community Docker template
- [GeekTrainer's Docker Sandbox gist for Copilot CLI](https://gist.github.com/GeekTrainer/949f8d198ef9a2b9f90253f17ec8ce1c)

### Community Sandbox Projects

- [copilot_here](https://github.com/GordonBeeming/copilot_here) — Cross-platform secure Docker wrapper with network airlock
  - [Project site](https://gordonbeeming.com/copilot_here/)
  - [Paranoid guide to secure Copilot CLI](https://gordonbeeming.com/blog/2025-10-03/taming-the-ai-my-paranoid-guide-to-running-copilot-cli-in-a-secure-docker-sandbox)
  - [Q1 2026 updates (Podman, Go, package managers)](https://gordonbeeming.com/blog/2026-03-04/copilot_here-q1-2026-updates-package-managers-golang-podman-and-more)
- [Sandbox Copilot CLI on Linux](https://georg.dev/blog/07-sandbox-your-github-copilot-cli-on-linux/) — Rootless Docker guide, georg.dev
- [Set up sandbox for Copilot CLI on Linux](https://dev.to/lunran/how-to-set-up-a-sandbox-environment-for-github-copilot-cli-on-linux-39n2) — dev.to/lunran
- [Dev Container Feature: copilot-cli](https://github.com/devcontainers/features/tree/main/src/copilot-cli) — Official Dev Container feature
- [SSW Rule: Use GitHub Copilot CLI secure environment](https://www.ssw.com.au/rules/use-github-copilot-cli-secure-environment) — SSW best practice rule
- [sandock](https://pypi.org/project/sandock/) — Generic CLI sandboxing tool (PyPI)
- [Workspace vs Worktree Isolation in Copilot CLI](https://www.kenmuse.com/blog/workspace-vs-worktree-isolation-in-copilot-cli/) — Ken Muse

### Process-Level Sandbox Tools

- [Vectimus](https://github.com/vectimus/vectimus) — Cedar policy enforcement for AI coding agents (Apache 2.0)
  - [Sentinel threat pipeline](https://github.com/vectimus/sentinel) — Automated policy generation from real incidents
  - [OWASP Agentic AI Threats Top 10](https://owasp.org/www-project-agentic-ai-threats/) — Framework mapped by Vectimus policies
  - [AWS Cedar policy language](https://www.cedarpolicy.com/) — Policy engine used by Vectimus
- [Yu](https://github.com/qingant/yu) — Credential isolation sandbox for AI coding agents (MIT)
  - [Blog: Your AI coding agent is running naked](https://blog.dreambubble.ai/en/posts/your-ai-coding-agent-is-running-naked-on-your-laptop) — Motivation and architecture
  - [Environment as a Service paper](https://blog.dreambubble.ai/en/posts/environment-as-a-service-agent-as-the-interface) — Theoretical foundation (Section 7.4)
- [Hazmat](https://github.com/dredozubov/hazmat) — macOS OS-level containment for AI agents (MIT)
  - [Blog: How I Made --dangerously-skip-permissions Safe](https://codeofchange.io/how-i-made-dangerously-skip-permissions-safe-in-claude-code/) — Design rationale
  - [CVE audit](https://github.com/dredozubov/hazmat/blob/master/docs/cve-audit.md) — 16+ Claude Code CVEs mitigated
  - [Supply chain hardening brief](https://github.com/dredozubov/hazmat/blob/master/docs/brief-supply-chain-hardening.md) — axios attack case study
- [Zerobox](https://github.com/afshinm/zerobox) — Cross-platform process sandbox using OpenAI Codex crates (Apache 2.0)
  - Uses upstream [codex-sandboxing](https://github.com/openai/codex) crates from OpenAI

### Cloud Agent Sandbox Platforms (Generic, Not Copilot-Specific)

- [E2B](https://e2b.dev/) — Cloud sandbox microVMs for AI agents
- [Daytona](https://www.daytona.io/) — Stateful agent code execution infrastructure
- [Runloop](https://runloop.ai/) — Micro-VM agent sandboxes with evaluation
- [Modal](https://modal.com/) — Cloud compute for ML/agent workloads
- [VibeKit](https://github.com/superagent-ai/vibekit) — OSS SDK for orchestrating agents in sandboxes
- [AI Code Sandbox Benchmark 2026](https://www.superagent.sh/blog/ai-code-sandbox-benchmark-2026) — Modal vs E2B vs Daytona comparison

### Security Research

- [Agentic DevOps Safe Mode: A Practical Framework](https://arinco.com.au/blog/agentic-devops-safe-mode-a-practical-framework-for-secure-github-copilot-agents/) — Defense-in-depth model for Copilot agents
- [StepSecurity: Securing Copilot in GitHub Actions](https://www.stepsecurity.io/blog/securing-github-copilot-in-github-actions-with-harden-runner) — Runtime visibility with Harden-Runner
- [The Race to Ship AI Tools Left Security Behind: Sandbox Escape](https://cymulate.com/blog/the-race-to-ship-ai-tools-left-security-behind-part-1-sandbox-escape/) — Cymulate security research
- [NVIDIA: Practical Security for Sandboxing Agentic Workflows](https://developer.nvidia.com/blog/practical-security-guidance-for-sandboxing-agentic-workflows-and-managing-execution-risk/)
- [GitHub Copilot CLI Safety: Complete Security Guide](https://www.datcrazy.co/blog/github-copilot-cli-safety-complete-security-guide--2025-12)

### Competitor Reference

- [Claude Code containerization docs](https://code.claude.com/docs/en/devcontainer)
- [Anthropic sandbox-runtime](https://github.com/anthropic-experimental/sandbox-runtime) — Referenced in #892
