//! User configuration loaded from `~/.config/copilot-sandbox/config.toml`.
//!
//! The config file is optional — copilot-sandbox works without it.
//! CLI flags always override config values for scalar fields.
//! For list fields (allow/deny paths), CLI and config values are merged (union).
//!
//! Override config location with `COPILOT_SANDBOX_CONFIG` env var.

use serde::Deserialize;
use std::path::PathBuf;

/// Default config directory relative to $HOME.
const CONFIG_DIR: &str = ".config/copilot-sandbox";
const CONFIG_FILE: &str = "config.toml";

// Characters that would break SBPL profile string interpolation.
const SBPL_UNSAFE_CHARS: &[char] = &['"', ')', '(', ';', '\\', '\n', '\r', '\0'];

/// Top-level config file structure.
#[derive(Debug, Default, Deserialize)]
#[serde(default)]
pub struct Config {
    pub proxy: ProxyConfig,
    pub allow: AllowConfig,
    pub deny: DenyConfig,
    pub sandbox: SandboxConfig,
}

#[derive(Debug, Default, Deserialize)]
#[serde(default)]
pub struct ProxyConfig {
    /// Enable the CONNECT proxy (default: false).
    pub enabled: Option<bool>,
    /// Proxy listen port (default: 18080).
    pub port: Option<u16>,
    /// Path to blocked domains file.
    pub blocked_domains: Option<String>,
}

#[derive(Debug, Default, Deserialize)]
#[serde(default)]
pub struct AllowConfig {
    /// Additional paths to allow reading.
    pub read: Vec<String>,
    /// Additional paths to allow writing.
    pub write: Vec<String>,
}

#[derive(Debug, Default, Deserialize)]
#[serde(default)]
pub struct DenyConfig {
    /// Additional paths to explicitly deny.
    pub paths: Vec<String>,
}

#[derive(Debug, Default, Deserialize)]
#[serde(default)]
pub struct SandboxConfig {
    /// Run sandbox-exec validation test on startup (default: true).
    pub validate: Option<bool>,
}

/// Resolved configuration after merging config file + CLI flags.
/// All paths are expanded and canonicalized.
#[derive(Debug)]
pub struct Resolved {
    pub with_proxy: bool,
    pub proxy_port: u16,
    pub blocked_domains: Option<PathBuf>,
    pub allow_read: Vec<PathBuf>,
    pub allow_write: Vec<PathBuf>,
    pub deny_paths: Vec<PathBuf>,
    pub no_validate: bool,
}

impl Config {
    /// Load config from `~/.config/copilot-sandbox/config.toml` (or COPILOT_SANDBOX_CONFIG).
    /// Returns `Config::default()` if the file doesn't exist.
    /// Returns an error if the file exists but is malformed or unreadable.
    pub fn load() -> Result<Self, String> {
        let Some(path) = config_path() else {
            return Ok(Config::default());
        };

        if !path.exists() {
            return Ok(Config::default());
        }

        let contents = std::fs::read_to_string(&path)
            .map_err(|e| format!("Cannot read config file {}: {e}", path.display()))?;

        let config: Config = toml::from_str(&contents)
            .map_err(|e| format!("Invalid TOML in {}: {e}", path.display()))?;

        eprintln!("\x1b[0;34m[sandbox]\x1b[0m Config:   {}", path.display());

        Ok(config)
    }

    /// Merge config file values with CLI flags.
    ///
    /// Precedence rules:
    /// - Booleans: explicit CLI flag > config > default
    /// - Scalars: CLI (if Some) > config > hardcoded default
    /// - Lists: union of config + CLI (both contribute)
    ///
    /// Returns an error if a deny path from config cannot be resolved
    /// (security-critical: silently dropping deny rules is dangerous).
    #[allow(clippy::too_many_arguments)]
    pub fn merge(
        &self,
        cli_with_proxy: bool,
        cli_no_proxy: bool,
        cli_proxy_port: Option<u16>,
        cli_blocked_domains: Option<PathBuf>,
        cli_allow_read: Vec<PathBuf>,
        cli_allow_write: Vec<PathBuf>,
        cli_deny_paths: Vec<PathBuf>,
        cli_no_validate: bool,
    ) -> Result<Resolved, String> {
        // Proxy: --no-proxy always wins, then --with-proxy, then config, then false (default off).
        // The proxy is a passive logging tool — Copilot CLI doesn't use it (Node.js ignores
        // http_proxy env vars). It's useful for logging traffic from tools like `gh` or `curl`.
        let with_proxy = if cli_no_proxy {
            false
        } else if cli_with_proxy {
            true
        } else {
            self.proxy.enabled.unwrap_or(false)
        };

        // Port: CLI (if provided) > config > 18080
        let proxy_port = cli_proxy_port.or(self.proxy.port).unwrap_or(18080);

        // Blocked domains: CLI > config > exe_dir fallback (handled later in main)
        let blocked_domains = cli_blocked_domains
            .or_else(|| self.proxy.blocked_domains.as_ref().map(|s| expand_tilde(s)));

        // Allow-read: merge config + CLI
        let config_dir = config_path().and_then(|p| p.parent().map(|d| d.to_path_buf()));
        let mut allow_read: Vec<PathBuf> = Vec::new();
        for s in &self.allow.read {
            match resolve_config_path(s, config_dir.as_ref()) {
                Ok(p) => allow_read.push(p),
                Err(e) => {
                    eprintln!("\x1b[0;33m[sandbox]\x1b[0m Warning: allow.read path {s:?}: {e}");
                }
            }
        }
        allow_read.extend(cli_allow_read);

        // Allow-write: merge config + CLI
        let mut allow_write: Vec<PathBuf> = Vec::new();
        for s in &self.allow.write {
            match resolve_config_path(s, config_dir.as_ref()) {
                Ok(p) => allow_write.push(p),
                Err(e) => {
                    eprintln!("\x1b[0;33m[sandbox]\x1b[0m Warning: allow.write path {s:?}: {e}");
                }
            }
        }
        allow_write.extend(cli_allow_write);

        // Deny-paths: merge config + CLI
        // SECURITY: config deny paths MUST resolve — a silently dropped deny is dangerous
        let mut deny_paths: Vec<PathBuf> = Vec::new();
        for s in &self.deny.paths {
            match resolve_config_path(s, config_dir.as_ref()) {
                Ok(p) => deny_paths.push(p),
                Err(e) => {
                    return Err(format!(
                        "deny.paths entry {s:?} cannot be resolved: {e}\n\
                         Fix the path in your config or remove it. \
                         Silently dropping deny rules is a security risk."
                    ));
                }
            }
        }
        deny_paths.extend(cli_deny_paths);

        // Validate: --no-validate wins, then config, then true (validate by default)
        let no_validate = if cli_no_validate {
            true
        } else {
            !self.sandbox.validate.unwrap_or(true)
        };

        // Validate all paths for SBPL injection characters
        for p in allow_read
            .iter()
            .chain(allow_write.iter())
            .chain(deny_paths.iter())
        {
            validate_sbpl_path(p)?;
        }

        Ok(Resolved {
            with_proxy,
            proxy_port,
            blocked_domains,
            allow_read,
            allow_write,
            deny_paths,
            no_validate,
        })
    }
}

impl Resolved {
    /// Log the effective merged configuration.
    pub fn log_effective(&self) {
        let blue = "\x1b[0;34m";
        let nc = "\x1b[0m";

        if self.with_proxy {
            eprintln!(
                "{blue}[sandbox]{nc} Proxy:    localhost:{}",
                self.proxy_port
            );
        }
        if !self.allow_read.is_empty() {
            let paths: Vec<String> = self
                .allow_read
                .iter()
                .map(|p| p.display().to_string())
                .collect();
            eprintln!("{blue}[sandbox]{nc} Allow-R:  {}", paths.join(", "));
        }
        if !self.allow_write.is_empty() {
            let paths: Vec<String> = self
                .allow_write
                .iter()
                .map(|p| p.display().to_string())
                .collect();
            eprintln!("{blue}[sandbox]{nc} Allow-RW: {}", paths.join(", "));
        }
        if !self.deny_paths.is_empty() {
            let paths: Vec<String> = self
                .deny_paths
                .iter()
                .map(|p| p.display().to_string())
                .collect();
            eprintln!("{blue}[sandbox]{nc} Deny:     {}", paths.join(", "));
        }
    }
}

/// Return the config file path.
/// Checks `COPILOT_SANDBOX_CONFIG` env var first, then `~/.config/copilot-sandbox/config.toml`.
pub fn config_path() -> Option<PathBuf> {
    if let Ok(custom) = std::env::var("COPILOT_SANDBOX_CONFIG") {
        return Some(expand_tilde(&custom));
    }
    std::env::var("HOME")
        .ok()
        .map(|h| PathBuf::from(h).join(CONFIG_DIR).join(CONFIG_FILE))
}

/// Generate a default config file with comments explaining each option.
pub fn default_config_contents() -> String {
    r#"# copilot-sandbox configuration
#
# This file configures default behavior for copilot-sandbox.
# CLI flags always override these settings.
# Location: ~/.config/copilot-sandbox/config.toml
# Override: COPILOT_SANDBOX_CONFIG=/path/to/config.toml

# ─── Proxy ───────────────────────────────────────────────────
# Optional CONNECT proxy that logs outbound HTTPS connections.
# Disabled by default — Copilot CLI connects directly to its APIs
# and does not use http_proxy/https_proxy env vars.
# Enable with --with-proxy for connection visibility and domain blocking.
# The proxy is a passive logging tool, not a security boundary.
[proxy]
# enabled = false
# port = 18080
# blocked_domains = "~/.config/copilot-sandbox/blocked.txt"

# ─── Allowed paths ──────────────────────────────────────────
# Additional paths the sandboxed process may access.
# These are merged with any --allow-read / --allow-write CLI flags.
# Tilde (~/) is expanded to $HOME.
# Relative paths are resolved from this config file's directory.
[allow]
# read = [
#     "~/some/reference/docs",
# ]
# write = []

# ─── Denied paths ───────────────────────────────────────────
# Additional paths to explicitly block (overrides allows).
# Merged with any --deny-path CLI flags.
# WARNING: paths that cannot be resolved will cause a startup error
# (silently dropping deny rules is a security risk).
[deny]
# paths = [
#     "~/.config/gcloud",
#     "~/.config/op",
# ]

# ─── Sandbox behavior ───────────────────────────────────────
[sandbox]
# Run sandbox-exec validation test on every launch (default: true).
# Disable to save ~200ms startup if you trust your config.
# validate = true
"#
    .to_string()
}

/// Expand leading `~/` to `$HOME/`. Only this form is supported.
pub fn expand_tilde(path: &str) -> PathBuf {
    if let Some(rest) = path.strip_prefix("~/") {
        if let Ok(home) = std::env::var("HOME") {
            return PathBuf::from(home).join(rest);
        }
    } else if path == "~"
        && let Ok(home) = std::env::var("HOME")
    {
        return PathBuf::from(home);
    }
    PathBuf::from(path)
}

/// Expand tilde, resolve relative paths against config dir, and canonicalize.
fn resolve_config_path(path: &str, config_dir: Option<&PathBuf>) -> Result<PathBuf, String> {
    let expanded = expand_tilde(path);

    // If relative and we know the config dir, resolve from there
    let full = if expanded.is_relative() {
        if let Some(dir) = config_dir {
            dir.join(&expanded)
        } else {
            expanded
        }
    } else {
        expanded
    };

    std::fs::canonicalize(&full).map_err(|e| format!("path does not exist or is inaccessible: {e}"))
}

/// Validate that a path doesn't contain characters that could break SBPL string interpolation.
fn validate_sbpl_path(path: &std::path::Path) -> Result<(), String> {
    let s = path.to_string_lossy();
    for c in SBPL_UNSAFE_CHARS {
        if s.contains(*c) {
            return Err(format!(
                "Path contains unsafe character '{c}' for sandbox profile: {s}\n\
                 This could be used for SBPL injection. Remove or rename the path."
            ));
        }
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn default_config_is_valid_toml() {
        let config: Config = toml::from_str("").unwrap();
        assert!(config.proxy.enabled.is_none());
        assert!(config.proxy.port.is_none());
        assert!(config.allow.read.is_empty());
        assert!(config.deny.paths.is_empty());
        assert!(config.sandbox.validate.is_none());
    }

    #[test]
    fn parses_full_config() {
        let toml_str = r#"
[proxy]
enabled = true
port = 9090
blocked_domains = "~/my-blocklist.txt"

[allow]
read = ["/opt/homebrew/share"]
write = ["/tmp/sandbox-out"]

[deny]
paths = ["~/.config/gcloud"]

[sandbox]
validate = false
"#;
        let config: Config = toml::from_str(toml_str).unwrap();
        assert_eq!(config.proxy.enabled, Some(true));
        assert_eq!(config.proxy.port, Some(9090));
        assert_eq!(
            config.proxy.blocked_domains,
            Some("~/my-blocklist.txt".to_string())
        );
        assert_eq!(config.allow.read, vec!["/opt/homebrew/share"]);
        assert_eq!(config.allow.write, vec!["/tmp/sandbox-out"]);
        assert_eq!(config.deny.paths, vec!["~/.config/gcloud"]);
        assert_eq!(config.sandbox.validate, Some(false));
    }

    #[test]
    fn partial_config_uses_defaults() {
        let toml_str = "[proxy]\nenabled = true\n";
        let config: Config = toml::from_str(toml_str).unwrap();
        assert_eq!(config.proxy.enabled, Some(true));
        assert!(config.proxy.port.is_none());
        assert!(config.allow.read.is_empty());
    }

    #[test]
    fn expand_tilde_replaces_home() {
        let expanded = expand_tilde("~/some/path");
        let home = std::env::var("HOME").unwrap();
        assert_eq!(expanded, PathBuf::from(format!("{home}/some/path")));
    }

    #[test]
    fn expand_tilde_bare() {
        let expanded = expand_tilde("~");
        let home = std::env::var("HOME").unwrap();
        assert_eq!(expanded, PathBuf::from(home));
    }

    #[test]
    fn expand_tilde_no_tilde() {
        let expanded = expand_tilde("/absolute/path");
        assert_eq!(expanded, PathBuf::from("/absolute/path"));
    }

    #[test]
    fn expand_tilde_not_at_start() {
        // Only leading ~/ is expanded; mid-path ~ is left alone
        let expanded = expand_tilde("some/~/path");
        assert_eq!(expanded, PathBuf::from("some/~/path"));
    }

    #[test]
    fn cli_proxy_flag_overrides_config() {
        let config: Config = toml::from_str("[proxy]\nenabled = false\n").unwrap();
        let resolved = config
            .merge(true, false, None, None, vec![], vec![], vec![], false)
            .unwrap();
        assert!(resolved.with_proxy);
    }

    #[test]
    fn no_proxy_flag_overrides_config_enabled() {
        let config: Config = toml::from_str("[proxy]\nenabled = true\n").unwrap();
        let resolved = config
            .merge(false, true, None, None, vec![], vec![], vec![], false)
            .unwrap();
        assert!(!resolved.with_proxy);
    }

    #[test]
    fn config_proxy_used_when_no_cli_flag() {
        let config: Config = toml::from_str("[proxy]\nenabled = true\n").unwrap();
        let resolved = config
            .merge(false, false, None, None, vec![], vec![], vec![], false)
            .unwrap();
        assert!(resolved.with_proxy);
    }

    #[test]
    fn cli_port_overrides_config() {
        let config: Config = toml::from_str("[proxy]\nport = 9090\n").unwrap();
        let resolved = config
            .merge(
                false,
                false,
                Some(12345),
                None,
                vec![],
                vec![],
                vec![],
                false,
            )
            .unwrap();
        assert_eq!(resolved.proxy_port, 12345);
    }

    #[test]
    fn config_port_used_when_cli_none() {
        let config: Config = toml::from_str("[proxy]\nport = 9090\n").unwrap();
        let resolved = config
            .merge(false, false, None, None, vec![], vec![], vec![], false)
            .unwrap();
        assert_eq!(resolved.proxy_port, 9090);
    }

    #[test]
    fn default_port_when_neither_set() {
        let config = Config::default();
        let resolved = config
            .merge(false, false, None, None, vec![], vec![], vec![], false)
            .unwrap();
        assert_eq!(resolved.proxy_port, 18080);
    }

    #[test]
    fn cli_no_validate_overrides_config() {
        let config: Config = toml::from_str("[sandbox]\nvalidate = true\n").unwrap();
        let resolved = config
            .merge(false, false, None, None, vec![], vec![], vec![], true)
            .unwrap();
        assert!(resolved.no_validate);
    }

    #[test]
    fn config_validate_false_sets_no_validate() {
        let config: Config = toml::from_str("[sandbox]\nvalidate = false\n").unwrap();
        let resolved = config
            .merge(false, false, None, None, vec![], vec![], vec![], false)
            .unwrap();
        assert!(resolved.no_validate);
    }

    #[test]
    fn deny_paths_merged_from_config_and_cli() {
        // Use /tmp which always exists and can be canonicalized
        let config: Config = toml::from_str("[deny]\npaths = [\"/tmp\"]\n").unwrap();
        let cli_deny = vec![PathBuf::from("/var")];
        let resolved = config
            .merge(false, false, None, None, vec![], vec![], cli_deny, false)
            .unwrap();
        assert!(
            resolved
                .deny_paths
                .iter()
                .any(|p| p.to_string_lossy().contains("tmp"))
        );
        assert!(resolved.deny_paths.contains(&PathBuf::from("/var")));
    }

    #[test]
    fn deny_path_config_error_on_nonexistent() {
        let config: Config =
            toml::from_str("[deny]\npaths = [\"/nonexistent/path/xyz\"]\n").unwrap();
        let result = config.merge(false, false, None, None, vec![], vec![], vec![], false);
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("cannot be resolved"));
    }

    #[test]
    fn default_config_contents_is_valid_toml() {
        let contents = default_config_contents();
        let config: Config = toml::from_str(&contents).unwrap();
        assert!(config.proxy.enabled.is_none());
    }

    #[test]
    fn proxy_disabled_by_default_when_no_config_or_flags() {
        let config = Config::default();
        let resolved = config
            .merge(false, false, None, None, vec![], vec![], vec![], false)
            .unwrap();
        assert!(
            !resolved.with_proxy,
            "Proxy should be disabled by default — it's a passive logging tool, not required for Copilot"
        );
    }

    #[test]
    fn sbpl_injection_rejected() {
        let path = PathBuf::from("/tmp/evil\")(allow file-read* (subpath \"/");
        let result = validate_sbpl_path(&path);
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("unsafe character"));
    }

    #[test]
    fn normal_paths_pass_sbpl_validation() {
        let path = PathBuf::from("/Users/hans/projects/my-app");
        assert!(validate_sbpl_path(&path).is_ok());
    }
}
