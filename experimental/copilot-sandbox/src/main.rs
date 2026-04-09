use clap::Parser;
use copilot_sandbox::{config, proxy, sandbox};
use std::path::PathBuf;
use std::process::ExitCode;

/// Run GitHub Copilot CLI inside a macOS sandbox.
///
/// Copilot can read and write your project files, but cannot access your
/// SSH keys, cloud credentials, or other secrets. All network traffic is
/// blocked unless you enable the built-in proxy with --with-proxy.
///
/// The sandbox is enforced by the macOS kernel — Copilot (and any process
/// it spawns) cannot bypass it.
///
/// Defaults can be saved to ~/.config/copilot-sandbox/config.toml
/// so you don't need to pass flags every time. Run --init-config to
/// create a starter config.
#[derive(Parser)]
#[command(name = "copilot-sandbox", version, about, after_help = "\
EXAMPLES:
  copilot-sandbox --with-proxy -- -p \"fix the tests\"
    Run Copilot with internet access (through the sandbox proxy)

  copilot-sandbox -- --version
    Verify the sandbox works (no network needed)

  copilot-sandbox --allow-read ~/shared-libs -- -p \"use shared-libs\"
    Let Copilot read files outside the project directory

  copilot-sandbox --deny-path ~/.config/gh -- -p \"refactor auth\"
    Block access to a path that is normally allowed
")]
struct Cli {
    /// Which directory Copilot can read and write to.
    /// Defaults to the current git repository root, or the working directory
    /// if you're not inside a git repo.
    #[arg(long, short = 'd', value_name = "DIR")]
    project_dir: Option<PathBuf>,

    /// Let Copilot access the internet through a local proxy.
    /// Without this flag, ALL network access is blocked and Copilot
    /// runs fully offline. You need this for most real tasks since
    /// Copilot must reach the GitHub API.
    #[arg(long)]
    with_proxy: bool,

    /// Force the proxy off, even if your config file enables it.
    /// Useful for testing or fully offline work.
    #[arg(long, conflicts_with = "with_proxy")]
    no_proxy: bool,

    /// Port for the local proxy to listen on [default: 18080].
    /// Only relevant when --with-proxy is enabled.
    #[arg(long, value_name = "PORT")]
    proxy_port: Option<u16>,

    /// File with domains to block (one per line, e.g. pastebin.com).
    /// The proxy will refuse CONNECT requests to these domains.
    /// The file is re-read on every request, so you can edit it live.
    #[arg(long, value_name = "FILE")]
    blocked_domains: Option<PathBuf>,

    /// Let Copilot read files outside the project directory.
    /// Use this when Copilot needs to reference shared libraries,
    /// monorepo siblings, or documentation stored elsewhere.
    /// Can be specified multiple times.
    #[arg(long = "allow-read", value_name = "PATH")]
    allow_read: Vec<PathBuf>,

    /// Let Copilot read AND write files outside the project directory.
    /// Use carefully — this gives Copilot full access to modify these paths.
    /// Can be specified multiple times.
    #[arg(long = "allow-write", value_name = "PATH")]
    allow_write: Vec<PathBuf>,

    /// Block access to a specific path, even if it would normally be allowed.
    /// Deny rules always win over allow rules. Use this to protect sensitive
    /// files inside otherwise-allowed directories.
    /// Can be specified multiple times.
    #[arg(long = "deny-path", value_name = "PATH")]
    deny_paths: Vec<PathBuf>,

    /// Skip the startup check that verifies the sandbox is working.
    /// The check runs a quick test command inside the sandbox to confirm
    /// that file and network restrictions are active.
    #[arg(long)]
    no_validate: bool,

    /// Create a starter config file at ~/.config/copilot-sandbox/config.toml.
    /// The config lets you save your preferred defaults so you don't need
    /// to pass flags every time. Will not overwrite an existing file.
    #[arg(long)]
    init_config: bool,

    /// Everything after -- is passed directly to the copilot command.
    /// Example: copilot-sandbox --with-proxy -- -p "fix the tests"
    #[arg(last = true)]
    copilot_args: Vec<String>,
}

const GREEN: &str = "\x1b[0;32m";
const YELLOW: &str = "\x1b[0;33m";
const RED: &str = "\x1b[0;31m";
const BLUE: &str = "\x1b[0;34m";
const NC: &str = "\x1b[0m";

fn info(msg: &str) {
    eprintln!("{BLUE}[sandbox]{NC} {msg}");
}

fn ok(msg: &str) {
    eprintln!("{GREEN}[sandbox]{NC} {msg}");
}

fn warn(msg: &str) {
    eprintln!("{YELLOW}[sandbox]{NC} {msg}");
}

fn error(msg: &str) {
    eprintln!("{RED}[sandbox]{NC} {msg}");
}

fn detect_project_root() -> Option<PathBuf> {
    let output = std::process::Command::new("git")
        .args(["rev-parse", "--show-toplevel"])
        .output()
        .ok()?;
    if output.status.success() {
        let path = String::from_utf8(output.stdout).ok()?;
        Some(PathBuf::from(path.trim()))
    } else {
        None
    }
}

// Use library's is_unsafe_root
use copilot_sandbox::is_unsafe_root;

fn main() -> ExitCode {
    let cli = Cli::parse();

    // Handle --init-config
    if cli.init_config {
        return init_config();
    }

    // macOS only
    if std::env::consts::OS != "macos" {
        error("copilot-sandbox requires macOS (uses sandbox-exec)");
        return ExitCode::FAILURE;
    }

    // Load config file and merge with CLI flags
    // Canonicalize CLI paths for consistency with config path handling
    let cli_allow_read: Vec<PathBuf> = cli
        .allow_read
        .iter()
        .filter_map(|p| match std::fs::canonicalize(p) {
            Ok(c) => Some(c),
            Err(e) => {
                warn(&format!("--allow-read path {:?}: {e}", p));
                None
            }
        })
        .collect();
    let cli_allow_write: Vec<PathBuf> = cli
        .allow_write
        .iter()
        .filter_map(|p| match std::fs::canonicalize(p) {
            Ok(c) => Some(c),
            Err(e) => {
                warn(&format!("--allow-write path {:?}: {e}", p));
                None
            }
        })
        .collect();
    let cli_deny_paths: Vec<PathBuf> = cli
        .deny_paths
        .iter()
        .map(|p| {
            std::fs::canonicalize(p).map_err(|e| {
                format!(
                    "--deny-path {:?} cannot be resolved: {e}\n\
                     Silently dropping deny rules is a security risk.",
                    p
                )
            })
        })
        .collect::<Result<Vec<_>, _>>()
        .unwrap_or_else(|e| {
            error(&e);
            std::process::exit(1);
        });

    let cfg = match config::Config::load() {
        Ok(c) => c,
        Err(e) => {
            error(&e);
            return ExitCode::FAILURE;
        }
    };
    let resolved = match cfg.merge(
        cli.with_proxy,
        cli.no_proxy,
        cli.proxy_port,
        cli.blocked_domains.clone(),
        cli_allow_read,
        cli_allow_write,
        cli_deny_paths,
        cli.no_validate,
    ) {
        Ok(r) => r,
        Err(e) => {
            error(&e);
            return ExitCode::FAILURE;
        }
    };

    // Resolve home directory
    let home_dir = match std::env::var("HOME") {
        Ok(h) => match std::fs::canonicalize(&h) {
            Ok(p) => p,
            Err(e) => {
                error(&format!("Cannot resolve $HOME ({h}): {e}"));
                return ExitCode::FAILURE;
            }
        },
        Err(_) => {
            error("$HOME not set");
            return ExitCode::FAILURE;
        }
    };

    // Resolve project directory
    let project_dir = match &cli.project_dir {
        Some(p) => match std::fs::canonicalize(p) {
            Ok(p) => p,
            Err(e) => {
                error(&format!("Cannot resolve project dir: {e}"));
                return ExitCode::FAILURE;
            }
        },
        None => {
            if let Some(root) = detect_project_root() {
                match std::fs::canonicalize(&root) {
                    Ok(p) => p,
                    Err(_) => root,
                }
            } else {
                warn("No git repo detected, using cwd");
                match std::env::current_dir().and_then(std::fs::canonicalize) {
                    Ok(p) => p,
                    Err(e) => {
                        error(&format!("Cannot resolve cwd: {e}"));
                        return ExitCode::FAILURE;
                    }
                }
            }
        }
    };

    // Safety check: reject overly broad project roots
    if is_unsafe_root(&project_dir, &home_dir) {
        error(&format!(
            "Refusing to sandbox '{}' — too broad. Use a specific project directory.",
            project_dir.display()
        ));
        return ExitCode::FAILURE;
    }

    // Check copilot is installed
    if std::process::Command::new("which")
        .arg("copilot")
        .output()
        .map(|o| !o.status.success())
        .unwrap_or(true)
    {
        error("GitHub Copilot CLI not found in PATH");
        return ExitCode::FAILURE;
    }

    info(&format!("Project:  {}", project_dir.display()));
    info(&format!("Home:     {}", home_dir.display()));
    resolved.log_effective();

    // Validate all paths that will be interpolated into SBPL profile
    if let Err(e) = sandbox::validate_sbpl_path(&project_dir) {
        error(&format!("Project dir: {e}"));
        return ExitCode::FAILURE;
    }
    if let Err(e) = sandbox::validate_sbpl_path(&home_dir) {
        error(&format!("Home dir: {e}"));
        return ExitCode::FAILURE;
    }

    // Generate sandbox profile
    let profile = sandbox::generate_profile(
        &project_dir,
        &home_dir,
        &resolved.allow_read,
        &resolved.allow_write,
        &resolved.deny_paths,
        if resolved.with_proxy {
            Some(resolved.proxy_port)
        } else {
            None
        },
    );

    // Write profile to temp file with unique name (prevents symlink attacks)
    let profile_path = std::env::temp_dir().join(format!(
        "copilot-sandbox-{}-{}.sb",
        std::process::id(),
        std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap_or_default()
            .as_nanos()
    ));

    // O_CREAT|O_EXCL: atomic create, fails if exists (prevents symlink following)
    {
        use std::io::Write as _;
        use std::os::unix::fs::OpenOptionsExt;
        let mut file = match std::fs::OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(&profile_path)
        {
            Ok(f) => f,
            Err(e) => {
                error(&format!("Cannot create sandbox profile: {e}"));
                return ExitCode::FAILURE;
            }
        };
        if let Err(e) = file.write_all(profile.as_bytes()) {
            error(&format!("Cannot write sandbox profile: {e}"));
            let _ = std::fs::remove_file(&profile_path);
            return ExitCode::FAILURE;
        }
    }

    // Validate profile with a quick test
    if !resolved.no_validate {
        match sandbox::validate(&profile_path, &project_dir, &home_dir) {
            Ok(()) => ok("Sandbox profile validated ✓"),
            Err(e) => {
                error(&format!("Sandbox validation failed: {e}"));
                return ExitCode::FAILURE;
            }
        }
    }

    // Start proxy if requested
    let mut proxy_handle = None;
    let mut proxy_env = Vec::new();

    if resolved.with_proxy {
        let blocked_file = resolved.blocked_domains.unwrap_or_else(|| {
            let exe_dir = std::env::current_exe()
                .ok()
                .and_then(|p| p.parent().map(|p| p.to_path_buf()));
            exe_dir
                .map(|d| d.join("blocked.txt"))
                .unwrap_or_else(|| PathBuf::from("blocked.txt"))
        });

        info(&format!(
            "Starting proxy on localhost:{} ...",
            resolved.proxy_port
        ));

        match proxy::start(resolved.proxy_port, blocked_file) {
            Ok(handle) => {
                ok(&format!(
                    "Proxy running on localhost:{} (thread)",
                    resolved.proxy_port
                ));
                proxy_handle = Some(handle);
                let proxy_url = format!("http://127.0.0.1:{}", resolved.proxy_port);
                proxy_env = vec![
                    ("http_proxy".to_string(), proxy_url.clone()),
                    ("https_proxy".to_string(), proxy_url.clone()),
                    ("HTTP_PROXY".to_string(), proxy_url.clone()),
                    ("HTTPS_PROXY".to_string(), proxy_url.clone()),
                    (
                        "no_proxy".to_string(),
                        "localhost,127.0.0.1,::1".to_string(),
                    ),
                    (
                        "NO_PROXY".to_string(),
                        "localhost,127.0.0.1,::1".to_string(),
                    ),
                ];
            }
            Err(e) => {
                error(&format!("Failed to start proxy: {e}"));
                return ExitCode::FAILURE;
            }
        }
    }

    info("Protected: ~/.ssh, ~/.gnupg, ~/.aws, ~/.azure, ~/.kube, ~/.docker, ~/.netrc");
    if resolved.with_proxy {
        info("Network:   All traffic through localhost proxy (logged + filterable)");
    } else {
        info("Network:   All direct outbound blocked (no proxy — Copilot API will fail!)");
        warn("Use --with-proxy if Copilot needs internet access");
    }

    eprintln!();
    ok("Starting Copilot in sandbox...");
    eprintln!("{YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━{NC}");
    eprintln!();

    // Run copilot inside sandbox
    let exit_code = sandbox::exec(
        &profile_path,
        &project_dir,
        &home_dir,
        &cli.copilot_args,
        &proxy_env,
    );

    // Cleanup
    let _ = std::fs::remove_file(&profile_path);
    if let Some(handle) = proxy_handle {
        handle.shutdown();
    }

    ExitCode::from(exit_code)
}

fn init_config() -> ExitCode {
    let Some(path) = config::config_path() else {
        error("Cannot determine config path ($HOME not set)");
        return ExitCode::FAILURE;
    };

    if path.exists() {
        error(&format!(
            "Config file already exists: {}\nEdit it directly, or remove it first to regenerate.",
            path.display()
        ));
        return ExitCode::FAILURE;
    }

    // Create parent directory
    if let Some(parent) = path.parent() {
        if let Err(e) = std::fs::create_dir_all(parent) {
            error(&format!("Cannot create config directory: {e}"));
            return ExitCode::FAILURE;
        }
    }

    match std::fs::write(&path, config::default_config_contents()) {
        Ok(()) => {
            ok(&format!("Config file created: {}", path.display()));
            info("Edit it to customize sandbox defaults.");
            ExitCode::SUCCESS
        }
        Err(e) => {
            error(&format!("Cannot write config file: {e}"));
            ExitCode::FAILURE
        }
    }
}
