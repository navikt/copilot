---
name: workstation-security
description: Sikkerhetssjekk for macOS-utviklermaskiner — brannmur, SSH, Git, hemmeligheter, nettverk og Nav-plattformverktøy
license: MIT
compatibility: macOS developer workstation
metadata:
  domain: auth
  tags: security macos developer-tools audit hardening
---

# Workstation Security Audit

Run these checks with `run_in_terminal` or `bash`. Report every finding with a severity (CRITICAL / HIGH / MEDIUM / INFO / PASS) and a concrete fix. Summarize results in a table at the end.

## 1. macOS System Security

```bash
/usr/libexec/ApplicationFirewall/socketfilterfw --getglobalstate 2>/dev/null
/usr/libexec/ApplicationFirewall/socketfilterfw --getstealthmode 2>/dev/null
fdesetup status 2>/dev/null
csrutil status 2>/dev/null
spctl --status 2>/dev/null
```

| Check | Expected | If failing | Severity |
|-------|----------|------------|----------|
| Firewall | enabled | System Settings → Network → Firewall | HIGH |
| Stealth mode | on | `sudo /usr/libexec/ApplicationFirewall/socketfilterfw --setstealthmode on` | MEDIUM |
| FileVault | On | System Settings → Privacy & Security → FileVault | CRITICAL |
| SIP | enabled | Boot Recovery → `csrutil enable` | CRITICAL |
| Gatekeeper | assessments enabled | System Settings → Privacy & Security → Allow apps from App Store and identified developers | HIGH |

### Additional OS Hardening

1. Automatic security updates — must be enabled:
   ```bash
   softwareupdate --schedule 2>/dev/null
   defaults read /Library/Preferences/com.apple.SoftwareUpdate AutomaticallyInstallMacOSUpdates 2>/dev/null
   defaults read /Library/Preferences/com.apple.SoftwareUpdate CriticalUpdateInstall 2>/dev/null
   defaults read /Library/Preferences/com.apple.SoftwareUpdate AutomaticDownload 2>/dev/null
   ```
   `CriticalUpdateInstall` and `AutomaticDownload` should return `1`. If not → **MEDIUM**. Fix: System Settings → General → Software Update → Automatic Updates.

2. Screen lock — must require password immediately:
   ```bash
   sysadminctl -screenLock status 2>/dev/null
   ```
   If not immediate → **MEDIUM**. Fix: System Settings → Lock Screen → Require password immediately.

3. Remote Login (SSH server) — should be off unless needed (requires admin):
   ```bash
   sudo systemsetup -getremotelogin 2>/dev/null || echo "Requires admin — check System Settings → General → Sharing → Remote Login"
   ```
   If "On" → **MEDIUM**. Fix: System Settings → General → Sharing → disable Remote Login.

4. Login items and LaunchAgents — review for unexpected persistence:
   ```bash
   ls ~/Library/LaunchAgents/ 2>/dev/null
   ls /Library/LaunchAgents/ 2>/dev/null
   ```
   Flag unknown or suspicious entries → **MEDIUM**.

## 2. Nav Platform Security

These checks are specific to Nav developer machines connected to the NAIS platform.

> **Note:** Only check the tools listed below. Missing optional developer tools (Copilot CLI, nav-pilot, etc.) are not security findings and should not be reported.

1. **naisdevice** — must be installed and healthy:
   ```bash
   ls /Applications/naisdevice.app 2>/dev/null && echo "INSTALLED" || echo "NOT INSTALLED"
   ```
   Not installed → **HIGH**. Fix: `brew install --cask nais/tap/naisdevice`.

2. **Kolide agent** — must be enrolled and running:
   ```bash
   ps aux | grep -i kolide | grep -v grep | head -3
   ls /private/var/kolide-k2/ 2>/dev/null
   ```
   Not running → **HIGH**. Fix: Enroll at https://auth.kolide.com/device/registrations/new. Resolve any Kolide issues flagged in Slack.

3. **GitHub CLI** — check auth state:
   ```bash
   gh auth status 2>/dev/null
   ```
   Not logged in or expired → **MEDIUM**. Fix: `gh auth login`.

4. **Security scanning tools** — should be installed:
   ```bash
   which trivy gitleaks zizmor 2>/dev/null
   ```
   Missing tools → **INFO**. Fix: `brew install trivy gitleaks zizmor`.

5. **gcloud authentication** — check for active credentials:
   ```bash
   gcloud auth list 2>/dev/null | head -5
   ```
   Review active accounts — ensure only your Nav identity is active.

## 3. SSH Configuration

1. Check `~/.ssh/` directory permissions — must be `700`:
   ```bash
   stat -f "%Sp %p" ~/.ssh 2>/dev/null
   ```
2. Check private key permissions — must be `600`:
   ```bash
   find ~/.ssh -type f -name "id_*" ! -name "*.pub" -exec stat -f "%Sp %p %N" {} \;
   ```
3. Check SSH key algorithm strength — weak keys are **HIGH**:
   ```bash
   for key in ~/.ssh/id_*; do
     [ -f "$key" ] && [[ "$key" != *.pub ]] && ssh-keygen -l -f "$key" 2>/dev/null
   done
   ```
   RSA < 3072 bits → **HIGH**. DSA → **CRITICAL** (deprecated). Ed25519 or ECDSA → **PASS**.
4. SSH private keys should be encrypted — unencrypted keys are **HIGH**:
   ```bash
   for key in ~/.ssh/id_*; do
     if [ -f "$key" ] && [[ "$key" != *.pub ]]; then
       ssh-keygen -y -P "" -f "$key" &>/dev/null && echo "Unencrypted key: $key"
     fi
   done
   ```
   Any reported key is not encrypted. Fix: set a passphrase on the key(s), manage them in your password manager, or use a tool like [Secretive](https://github.com/maxgoedjen/secretive).
5. Check for `ForwardAgent yes` — **HIGH** if enabled for untrusted hosts:
   ```bash
   grep -n "ForwardAgent" ~/.ssh/config 2>/dev/null
   ```
   Fix: remove `ForwardAgent yes`; use `ssh -A <host>` only when needed.
6. Check for `StrictHostKeyChecking no` — **HIGH** if set globally:
   ```bash
   grep -n "StrictHostKeyChecking" ~/.ssh/config 2>/dev/null
   ```

## 4. Git Configuration

1. Credential helper — `osxkeychain` or `manager` is secure; `store` is **HIGH** (plaintext); `cache` is **MEDIUM**:
   ```bash
   git config --global credential.helper
   ```
2. Plaintext credentials — must not exist (**CRITICAL**):
   ```bash
   ls -la ~/.git-credentials ~/.netrc 2>/dev/null
   ```
3. Commit signing — recommended (**INFO** if missing):
   ```bash
   git config --global commit.gpgsign
   ```
4. TLS verification — must not be `false` (**CRITICAL**):
   ```bash
   git config --global http.sslVerify
   ```
5. Pre-commit hooks — check for secret scanners (gitleaks, detect-secrets):
   ```bash
   git config --global core.hooksPath
   ```

## 5. Credential Files

1. Sensitive files must be `600` (owner-only). Check each that exists:
   ```bash
   for f in ~/.npmrc ~/.yarnrc.yml ~/.kube/config ~/.docker/config.json \
            ~/.pulumi/credentials.json ~/.terraform.d/credentials.tfrc.json \
            ~/.config/gh/hosts.yml ~/.aws/credentials ~/.azure/accessTokens.json \
            ~/.netrc; do
     [ -f "$f" ] && stat -f "%Sp  %N" "$f"
   done
   ```
   Any file with group/other read → **HIGH**. Fix: `chmod 600 <file>`.

2. Scan for plaintext tokens (**CRITICAL** if found):
   ```bash
   grep -l "authToken=ghp_\|authToken=npm_\|authToken=glpat-\|_password=" ~/.npmrc 2>/dev/null
   grep -l "npmAuthToken:" ~/.yarnrc.yml 2>/dev/null
   grep -l "password" ~/.pypirc ~/.netrc 2>/dev/null
   ```
   Fix: remove hardcoded tokens; use environment variables or credential helpers.

3. Cloud provider credentials — JSON files in `~/.config/gcloud/`, `~/.aws/`, `~/.azure/` should be `600`:
   ```bash
   find ~/.config/gcloud ~/.aws ~/.azure -name "*.json" -o -name "credentials" 2>/dev/null | \
     xargs -I{} stat -f "%Sp  %N" {} 2>/dev/null
   ```

## 6. Shell Configuration

1. History files must be `600`:
   ```bash
   stat -f "%Sp %p" ~/.zsh_history ~/.bash_history 2>/dev/null
   ```
2. History privacy — sensitive commands should be excludable (**INFO** if not set):
   - zsh: `grep HIST_IGNORE_SPACE ~/.zshrc`
   - bash: `grep HISTCONTROL ~/.bashrc`
3. Secrets in shell profiles — scan for hardcoded API keys, tokens, passwords (**HIGH** if found):
   ```bash
   grep -nE '^\s*export\s+\w*(API_KEY|SECRET|_TOKEN|PASSWORD|AWS_SECRET|GITHUB_TOKEN|NPM_TOKEN|PRIVATE_KEY)\s*=' \
     ~/.zshrc ~/.zprofile ~/.zshenv ~/.bashrc ~/.bash_profile ~/.profile 2>/dev/null
   ```
4. Remote code execution patterns — `curl | bash` in profiles (**MEDIUM**):
   ```bash
   grep -nE 'curl\s.*\|\s*(ba)?sh|wget\s.*\|\s*(ba)?sh' \
     ~/.zshrc ~/.zprofile ~/.bashrc ~/.bash_profile 2>/dev/null
   ```
   Note: `eval "$(brew shellenv)"` and `eval "$(mise activate)"` are standard and safe.

## 7. Network Exposure

1. Services listening on all interfaces (0.0.0.0) — flag anything unexpected (**MEDIUM**):
   ```bash
   lsof -i -P -n 2>/dev/null | grep LISTEN | grep -v '127.0.0.1\|::1' | awk '{print $1, $9}' | sort -u
   ```
   Known safe: rapportd (Apple Handoff), Tailscale/IPNExtension (VPN), ControlCenter (AirPlay — disable if unused).
   Dev servers (node, python) on 0.0.0.0 should bind to 127.0.0.1 instead.

2. Firewall exceptions — review for stale entries:
   ```bash
   /usr/libexec/ApplicationFirewall/socketfilterfw --listapps 2>/dev/null | head -1
   ```
   Flag if >30 exceptions (**INFO**). Remove stale entries in System Settings → Firewall → Options.

## 8. Package Managers & Developer Tools

1. **npm** — check TLS and script settings:
   ```bash
   npm config get strict-ssl 2>/dev/null
   npm config get ignore-scripts 2>/dev/null
   ```
   `strict-ssl=false` → **HIGH** (TLS disabled). `ignore-scripts` absent → **INFO**.

2. **pip** — check for TLS bypass:
   ```bash
   python3 -m pip config list 2>/dev/null | grep -E 'trusted-host|index-url'
   ```
   `trusted-host` set → **HIGH** (TLS bypassed). Custom `index-url` not pointing to pypi.org → **MEDIUM**.

3. **Go** — check checksum verification:
   ```bash
   go env GONOSUMCHECK GONOSUMDB GOFLAGS 2>/dev/null
   ```
   Non-empty GONOSUMCHECK → **MEDIUM** (checksum verification bypassed).

4. **Docker** — check for plaintext credentials:
   ```bash
   cat ~/.docker/config.json 2>/dev/null | python3 -c "import sys,json; d=json.load(sys.stdin); print('credHelpers:', d.get('credHelpers',{})); print('credsStore:', d.get('credsStore','')); print('auths:', list(d.get('auths',{}).keys()))" 2>/dev/null
   ```
   Plaintext `auth` or `password` in `auths` → **HIGH**. Using `credHelpers` or `credsStore` → **PASS**.

5. **Homebrew** — list third-party taps for awareness:
   ```bash
   brew tap 2>/dev/null | grep -v '^homebrew/'
   ```
   Review for unexpected taps → **INFO**.

6. **VS Code** — list extensions for review:
   ```bash
   code --list-extensions 2>/dev/null | wc -l
   ```
   Review for extensions from unknown publishers → **INFO**.

## 9. Outdated Software

Outdated tools can contain known vulnerabilities. Check each package manager for pending updates.

1. **Homebrew formulae** — check for outdated packages:
   ```bash
   brew outdated 2>/dev/null
   ```
   Security-critical tools outdated (trivy, gitleaks, zizmor, git) → **MEDIUM**. Others → **INFO**. Fix: `brew upgrade`.

2. **Homebrew casks** — check for outdated applications:
   ```bash
   brew outdated --cask 2>/dev/null
   ```
   Outdated browsers or naisdevice → **MEDIUM**. Others → **INFO**. Fix: `brew upgrade --cask`.

3. **npm global packages**:
   ```bash
   npm outdated -g 2>/dev/null
   ```
   Outdated → **INFO**. Fix: `npm update -g`.

4. **pip packages**:
   ```bash
   pip3 list --outdated 2>/dev/null
   ```
   Outdated → **INFO**. Fix: `pip3 install --upgrade <package>`.

5. **mise/asdf runtimes**:
   ```bash
   mise outdated 2>/dev/null
   ```
   Outdated → **INFO**. Fix: `mise upgrade`.

6. **macOS system updates**:
   ```bash
   softwareupdate -l 2>/dev/null
   ```
   Pending security updates → **MEDIUM**. Other updates → **INFO**. Fix: `softwareupdate -ia`.

## Report Format

Summarize all findings in a table:

```
| Severity | Category    | Finding                              | Remediation           |
|----------|-------------|--------------------------------------|-----------------------|
| CRITICAL | Credentials | Plaintext token in ~/.npmrc          | Remove token, use env |
| HIGH     | SSH         | ForwardAgent enabled for 'myhost'    | Remove from config    |
| PASS     | FileVault   | Disk encryption enabled              |                       |
```

End with an overall verdict: **CRITICAL** / **HIGH** / **MEDIUM** / **GOOD** based on the worst finding, and a count summary (e.g., "0 critical, 1 high, 2 medium, 18 passed").

## Related

| Resource | Use For |
|----------|---------|
| `@security-champion` | Trusselmodellering, compliance, Navs sikkerhetsarkitektur |
| `@security-review` | Sikkerhetssjekk av kodeendringer før commit/push |
| `@auth-agent` | JWT-validering, TokenX, ID-porten, Maskinporten |
| `@nais-agent` | Nais-manifest, accessPolicy, hemmeligheter |
| [sikkerhet.nav.no](https://sikkerhet.nav.no) | Navs Golden Path og autoritative sikkerhetsretningslinjer |
