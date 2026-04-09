//! Unit tests for sandbox profile generation, domain blocking, and IP checks.
//!
//! These tests verify core logic without invoking sandbox-exec,
//! so they run on any platform (Linux CI, macOS, etc.).

/// Mirrors main::is_unsafe_root for unit testing.
fn is_unsafe_root(path: &std::path::Path, home: &std::path::Path) -> bool {
    let p = path.to_string_lossy();
    p == "/" || p == "/Users" || p == "/tmp" || p == "/private/tmp" || path == home
}

/// Mirrors proxy::is_blocked_in_content for unit testing.
fn is_blocked_in_content(hostname: &str, contents: &str) -> bool {
    let host = hostname.to_lowercase();
    for line in contents.lines() {
        let pattern = line.trim().to_lowercase();
        if pattern.is_empty() || pattern.starts_with('#') {
            continue;
        }
        if host == pattern || host.ends_with(&format!(".{pattern}")) {
            return true;
        }
    }
    false
}

/// Mirrors proxy::is_private_target for unit testing.
fn is_private_target(host: &str) -> bool {
    if let Ok(ip) = host.parse::<std::net::IpAddr>() {
        return match ip {
            std::net::IpAddr::V4(v4) => {
                v4.is_loopback() || v4.is_private() || v4.is_link_local() || v4.is_unspecified()
            }
            std::net::IpAddr::V6(v6) => v6.is_loopback() || v6.is_unspecified(),
        };
    }
    host == "localhost"
        || host.ends_with(".localhost")
        || host.ends_with(".local")
        || host == "0.0.0.0"
        || host == "[::]"
}

// ============================================================
// Unsafe root detection
// ============================================================

#[test]
fn rejects_filesystem_root() {
    let home = std::path::Path::new("/Users/testuser");
    assert!(is_unsafe_root(std::path::Path::new("/"), home));
}

#[test]
fn rejects_users_dir() {
    let home = std::path::Path::new("/Users/testuser");
    assert!(is_unsafe_root(std::path::Path::new("/Users"), home));
}

#[test]
fn rejects_tmp() {
    let home = std::path::Path::new("/Users/testuser");
    assert!(is_unsafe_root(std::path::Path::new("/tmp"), home));
}

#[test]
fn rejects_private_tmp() {
    let home = std::path::Path::new("/Users/testuser");
    assert!(is_unsafe_root(std::path::Path::new("/private/tmp"), home));
}

#[test]
fn rejects_home_dir() {
    let home = std::path::Path::new("/Users/testuser");
    assert!(is_unsafe_root(home, home));
}

#[test]
fn allows_project_subdir() {
    let home = std::path::Path::new("/Users/testuser");
    assert!(!is_unsafe_root(
        std::path::Path::new("/Users/testuser/projects/my-app"),
        home
    ));
}

#[test]
fn allows_deep_project_path() {
    let home = std::path::Path::new("/Users/testuser");
    assert!(!is_unsafe_root(
        std::path::Path::new("/Users/testuser/go/src/github.com/org/repo"),
        home
    ));
}

// ============================================================
// Domain blocking
// ============================================================

#[test]
fn blocks_exact_domain_match() {
    let blocklist = "evil.com\npastebin.com\n";
    assert!(is_blocked_in_content("evil.com", blocklist));
    assert!(is_blocked_in_content("pastebin.com", blocklist));
}

#[test]
fn blocks_subdomain_match() {
    let blocklist = "evil.com\n";
    assert!(is_blocked_in_content("sub.evil.com", blocklist));
    assert!(is_blocked_in_content("deep.sub.evil.com", blocklist));
}

#[test]
fn does_not_block_partial_match() {
    let blocklist = "evil.com\n";
    assert!(!is_blocked_in_content("notevil.com", blocklist));
    assert!(!is_blocked_in_content("evil.com.safe.org", blocklist));
}

#[test]
fn allows_unlisted_domain() {
    let blocklist = "evil.com\n";
    assert!(!is_blocked_in_content("good.com", blocklist));
    assert!(!is_blocked_in_content("api.github.com", blocklist));
}

#[test]
fn ignores_comments_and_empty_lines() {
    let blocklist = "# This is a comment\n\nevil.com\n  # Another comment\n";
    assert!(is_blocked_in_content("evil.com", blocklist));
    assert!(!is_blocked_in_content("good.com", blocklist));
}

#[test]
fn case_insensitive_blocking() {
    let blocklist = "Evil.COM\n";
    assert!(is_blocked_in_content("evil.com", blocklist));
    assert!(is_blocked_in_content("EVIL.COM", blocklist));
    assert!(is_blocked_in_content("Evil.Com", blocklist));
}

#[test]
fn empty_blocklist_blocks_nothing() {
    assert!(!is_blocked_in_content("evil.com", ""));
    assert!(!is_blocked_in_content("anything.org", "# only comments\n"));
}

// ============================================================
// Private IP / localhost detection
// ============================================================

#[test]
fn detects_ipv4_loopback() {
    assert!(is_private_target("127.0.0.1"));
    assert!(is_private_target("127.0.0.2"));
}

#[test]
fn detects_ipv4_private_ranges() {
    assert!(is_private_target("10.0.0.1"));
    assert!(is_private_target("172.16.0.1"));
    assert!(is_private_target("192.168.1.1"));
}

#[test]
fn detects_ipv4_link_local() {
    assert!(is_private_target("169.254.1.1"));
}

#[test]
fn detects_ipv4_unspecified() {
    assert!(is_private_target("0.0.0.0"));
}

#[test]
fn detects_ipv6_loopback() {
    assert!(is_private_target("::1"));
}

#[test]
fn allows_public_ipv4() {
    assert!(!is_private_target("8.8.8.8"));
    assert!(!is_private_target("140.82.121.3"));
}

#[test]
fn detects_localhost_hostname() {
    assert!(is_private_target("localhost"));
    assert!(is_private_target("sub.localhost"));
}

#[test]
fn detects_dot_local_hostname() {
    assert!(is_private_target("myhost.local"));
}

#[test]
fn allows_normal_hostnames() {
    assert!(!is_private_target("api.github.com"));
    assert!(!is_private_target("registry.npmjs.org"));
}

// ============================================================
// Profile content verification (string-based)
// ============================================================

fn gen_test_profile(project: &str, home: &str, proxy_port: Option<u16>) -> String {
    use std::fmt::Write;
    let mut profile = String::new();

    writeln!(profile, "(version 1)").unwrap();
    writeln!(profile, "(deny default)").unwrap();
    writeln!(
        profile,
        "(import \"/System/Library/Sandbox/Profiles/bsd.sb\")"
    )
    .unwrap();
    writeln!(profile, "(allow process-exec)").unwrap();
    writeln!(profile, "(allow process-fork)").unwrap();
    writeln!(profile, "(allow file-read* (subpath \"{project}\"))").unwrap();
    writeln!(profile, "(allow file-write* (subpath \"{project}\"))").unwrap();
    writeln!(profile, "(allow file-read* (subpath \"{home}/.copilot\"))").unwrap();
    writeln!(
        profile,
        "(allow file-read* (literal \"{home}/.gitconfig\"))"
    )
    .unwrap();

    for dotfile in &[
        ".ssh",
        ".gnupg",
        ".aws",
        ".azure",
        ".kube",
        ".docker",
        ".nais",
        ".password-store",
    ] {
        writeln!(profile, "(deny file-read* (subpath \"{home}/{dotfile}\"))").unwrap();
        writeln!(profile, "(deny file-write* (subpath \"{home}/{dotfile}\"))").unwrap();
    }

    writeln!(profile, "(deny network*)").unwrap();
    if let Some(port) = proxy_port {
        writeln!(
            profile,
            "(allow network-outbound (remote ip \"localhost:{port}\"))"
        )
        .unwrap();
    }

    profile
}

#[test]
fn profile_contains_deny_default() {
    let p = gen_test_profile("/projects/app", "/Users/test", None);
    assert!(p.contains("(deny default)"));
}

#[test]
fn profile_grants_project_access() {
    let p = gen_test_profile("/projects/app", "/Users/test", None);
    assert!(p.contains("(allow file-read* (subpath \"/projects/app\"))"));
    assert!(p.contains("(allow file-write* (subpath \"/projects/app\"))"));
}

#[test]
fn profile_grants_copilot_config_access() {
    let p = gen_test_profile("/projects/app", "/Users/test", None);
    assert!(p.contains("(allow file-read* (subpath \"/Users/test/.copilot\"))"));
}

#[test]
fn profile_denies_sensitive_dirs() {
    let p = gen_test_profile("/projects/app", "/Users/test", None);
    for dir in &[
        ".ssh",
        ".gnupg",
        ".aws",
        ".azure",
        ".kube",
        ".docker",
        ".nais",
        ".password-store",
    ] {
        assert!(
            p.contains(&format!(
                "(deny file-read* (subpath \"/Users/test/{dir}\"))"
            )),
            "should deny read to {dir}"
        );
        assert!(
            p.contains(&format!(
                "(deny file-write* (subpath \"/Users/test/{dir}\"))"
            )),
            "should deny write to {dir}"
        );
    }
}

#[test]
fn profile_blocks_all_network_without_proxy() {
    let p = gen_test_profile("/projects/app", "/Users/test", None);
    assert!(p.contains("(deny network*)"));
    assert!(!p.contains("network-outbound (remote ip"));
}

#[test]
fn profile_allows_only_proxy_port() {
    let p = gen_test_profile("/projects/app", "/Users/test", Some(18080));
    assert!(p.contains("(deny network*)"));
    assert!(p.contains("(allow network-outbound (remote ip \"localhost:18080\"))"));
}

#[test]
fn profile_deny_rules_come_after_allow_rules() {
    let p = gen_test_profile("/projects/app", "/Users/test", None);
    let allow_pos = p
        .find("(allow file-read* (subpath \"/projects/app\"))")
        .unwrap();
    let deny_pos = p
        .find("(deny file-read* (subpath \"/Users/test/.ssh\"))")
        .unwrap();
    assert!(
        deny_pos > allow_pos,
        "deny rules must come after allow rules for correct Seatbelt evaluation"
    );
}
