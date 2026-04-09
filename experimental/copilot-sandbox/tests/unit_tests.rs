//! Unit tests for sandbox profile generation, domain blocking, and IP checks.
//!
//! These tests verify core logic without invoking sandbox-exec,
//! so they run on any platform (Linux CI, macOS, etc.).

use cplt::is_unsafe_root;
use cplt::proxy::{is_blocked_in_content, is_private_hostname, is_private_ip};
use cplt::sandbox::{generate_profile, validate_sbpl_path};

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
fn rejects_var() {
    let home = std::path::Path::new("/Users/testuser");
    assert!(is_unsafe_root(std::path::Path::new("/var"), home));
}

#[test]
fn rejects_private_var() {
    let home = std::path::Path::new("/Users/testuser");
    assert!(is_unsafe_root(std::path::Path::new("/private/var"), home));
}

#[test]
fn rejects_applications() {
    let home = std::path::Path::new("/Users/testuser");
    assert!(is_unsafe_root(std::path::Path::new("/Applications"), home));
}

#[test]
fn rejects_system() {
    let home = std::path::Path::new("/Users/testuser");
    assert!(is_unsafe_root(std::path::Path::new("/System"), home));
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
// Domain blocking (using real proxy::is_blocked_in_content)
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
// Private IP / localhost detection (using real proxy functions)
// ============================================================

#[test]
fn detects_ipv4_loopback() {
    let ip: std::net::IpAddr = "127.0.0.1".parse().unwrap();
    assert!(is_private_ip(&ip));
    let ip2: std::net::IpAddr = "127.0.0.2".parse().unwrap();
    assert!(is_private_ip(&ip2));
}

#[test]
fn detects_ipv4_private_ranges() {
    for addr in &["10.0.0.1", "172.16.0.1", "192.168.1.1"] {
        let ip: std::net::IpAddr = addr.parse().unwrap();
        assert!(is_private_ip(&ip), "should detect {addr} as private");
    }
}

#[test]
fn detects_ipv4_link_local() {
    let ip: std::net::IpAddr = "169.254.1.1".parse().unwrap();
    assert!(is_private_ip(&ip));
}

#[test]
fn detects_ipv4_unspecified() {
    let ip: std::net::IpAddr = "0.0.0.0".parse().unwrap();
    assert!(is_private_ip(&ip));
}

#[test]
fn detects_ipv6_loopback() {
    let ip: std::net::IpAddr = "::1".parse().unwrap();
    assert!(is_private_ip(&ip));
}

#[test]
fn allows_public_ipv4() {
    for addr in &["8.8.8.8", "140.82.121.3"] {
        let ip: std::net::IpAddr = addr.parse().unwrap();
        assert!(!is_private_ip(&ip), "should allow public {addr}");
    }
}

#[test]
fn detects_localhost_hostname() {
    assert!(is_private_hostname("localhost"));
    assert!(is_private_hostname("sub.localhost"));
}

#[test]
fn detects_dot_local_hostname() {
    assert!(is_private_hostname("myhost.local"));
}

#[test]
fn allows_normal_hostnames() {
    assert!(!is_private_hostname("api.github.com"));
    assert!(!is_private_hostname("registry.npmjs.org"));
}

// ============================================================
// New: CGNAT, ULA, IPv4-mapped v6
// ============================================================

#[test]
fn detects_cgnat_range() {
    let ip: std::net::IpAddr = "100.64.0.1".parse().unwrap();
    assert!(is_private_ip(&ip), "CGNAT (100.64/10) should be private");
    let ip2: std::net::IpAddr = "100.127.255.254".parse().unwrap();
    assert!(is_private_ip(&ip2));
}

#[test]
fn detects_benchmarking_range() {
    let ip: std::net::IpAddr = "198.18.0.1".parse().unwrap();
    assert!(
        is_private_ip(&ip),
        "Benchmarking (198.18/15) should be private"
    );
}

#[test]
fn detects_reserved_v4() {
    let ip: std::net::IpAddr = "240.0.0.1".parse().unwrap();
    assert!(is_private_ip(&ip), "Reserved (240/4) should be private");
}

#[test]
fn detects_ipv6_ula() {
    let ip: std::net::IpAddr = "fd12:3456:789a::1".parse().unwrap();
    assert!(is_private_ip(&ip), "ULA (fc00::/7) should be private");
}

#[test]
fn detects_ipv6_link_local() {
    let ip: std::net::IpAddr = "fe80::1".parse().unwrap();
    assert!(
        is_private_ip(&ip),
        "Link-local v6 (fe80::/10) should be private"
    );
}

// ============================================================
// SBPL path validation
// ============================================================

#[test]
fn sbpl_path_rejects_newline() {
    let path = std::path::Path::new("/tmp/evil\n(allow file-read* (subpath \"/\"))");
    assert!(validate_sbpl_path(path).is_err());
}

#[test]
fn sbpl_path_rejects_null_byte() {
    let path = std::path::Path::new("/tmp/evil\0rest");
    assert!(validate_sbpl_path(path).is_err());
}

#[test]
fn sbpl_path_rejects_quotes() {
    let path = std::path::Path::new("/tmp/evil\"path");
    assert!(validate_sbpl_path(path).is_err());
}

#[test]
fn sbpl_path_rejects_parens() {
    let path = std::path::Path::new("/tmp/evil(path)");
    assert!(validate_sbpl_path(path).is_err());
}

#[test]
fn sbpl_path_allows_normal_path() {
    let path = std::path::Path::new("/Users/test/projects/my-app");
    assert!(validate_sbpl_path(path).is_ok());
}

// ============================================================
// Profile content verification (using real generate_profile)
// ============================================================

#[test]
fn profile_contains_deny_default() {
    let p = generate_profile(
        std::path::Path::new("/projects/app"),
        std::path::Path::new("/Users/test"),
        &[],
        &[],
        &[],
        None,
    );
    assert!(p.contains("(deny default)"));
}

#[test]
fn profile_allows_tty_ioctl() {
    let p = generate_profile(
        std::path::Path::new("/projects/app"),
        std::path::Path::new("/Users/test"),
        &[],
        &[],
        &[],
        None,
    );
    assert!(
        p.contains("(allow file-ioctl)"),
        "Profile must allow file-ioctl for terminal raw mode"
    );
}

#[test]
fn profile_grants_project_access() {
    let p = generate_profile(
        std::path::Path::new("/projects/app"),
        std::path::Path::new("/Users/test"),
        &[],
        &[],
        &[],
        None,
    );
    assert!(p.contains("(allow file-read* (subpath \"/projects/app\"))"));
    assert!(p.contains("(allow file-write* (subpath \"/projects/app\"))"));
}

#[test]
fn profile_grants_copilot_config_access() {
    let p = generate_profile(
        std::path::Path::new("/projects/app"),
        std::path::Path::new("/Users/test"),
        &[],
        &[],
        &[],
        None,
    );
    assert!(p.contains("(allow file-read* (subpath \"/Users/test/.copilot\"))"));
}

#[test]
fn profile_denies_sensitive_dirs() {
    let p = generate_profile(
        std::path::Path::new("/projects/app"),
        std::path::Path::new("/Users/test"),
        &[],
        &[],
        &[],
        None,
    );
    for dir in &[
        ".ssh",
        ".gnupg",
        ".aws",
        ".azure",
        ".kube",
        ".docker",
        ".nais",
        ".password-store",
        ".config/gcloud",
        ".config/op",
        ".terraform.d",
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
fn profile_denies_sensitive_files() {
    let p = generate_profile(
        std::path::Path::new("/projects/app"),
        std::path::Path::new("/Users/test"),
        &[],
        &[],
        &[],
        None,
    );
    for file in &[
        ".netrc",
        ".npmrc",
        ".pypirc",
        ".gem/credentials",
        ".vault-token",
    ] {
        assert!(
            p.contains(&format!(
                "(deny file-read* (literal \"/Users/test/{file}\"))"
            )),
            "should deny read to {file}"
        );
    }
}

#[test]
fn profile_allows_outbound_tcp() {
    let p = generate_profile(
        std::path::Path::new("/projects/app"),
        std::path::Path::new("/Users/test"),
        &[],
        &[],
        &[],
        None,
    );
    assert!(
        p.contains("(allow network-outbound (remote tcp))"),
        "Profile must allow outbound TCP for Copilot API endpoints"
    );
    assert!(
        p.contains("(allow network-outbound (literal \"/private/var/run/mDNSResponder\"))"),
        "Profile must allow DNS resolution"
    );
}

#[test]
fn profile_deny_rules_come_after_allow_rules() {
    let p = generate_profile(
        std::path::Path::new("/projects/app"),
        std::path::Path::new("/Users/test"),
        &[],
        &[],
        &[],
        None,
    );
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

#[test]
fn profile_denies_exec_from_tmp() {
    let p = generate_profile(
        std::path::Path::new("/projects/app"),
        std::path::Path::new("/Users/test"),
        &[],
        &[],
        &[],
        None,
    );
    assert!(
        p.contains("(deny process-exec (subpath \"/private/tmp\"))"),
        "should deny exec from /private/tmp"
    );
    assert!(
        p.contains("(deny process-exec (subpath \"/private/var/folders\"))"),
        "should deny exec from /private/var/folders"
    );
}

#[test]
fn profile_allows_gh_config_read_only() {
    let p = generate_profile(
        std::path::Path::new("/projects/app"),
        std::path::Path::new("/Users/test"),
        &[],
        &[],
        &[],
        None,
    );
    assert!(
        p.contains("(allow file-read* (subpath \"/Users/test/.config/gh\"))"),
        "should allow read to .config/gh for GitHub CLI auth"
    );
    assert!(
        !p.contains("(allow file-write* (subpath \"/Users/test/.config/gh\"))"),
        "should NOT allow write to .config/gh"
    );
}

#[test]
fn profile_allows_file_map_executable_for_copilot() {
    let p = generate_profile(
        std::path::Path::new("/projects/app"),
        std::path::Path::new("/Users/test"),
        &[],
        &[],
        &[],
        None,
    );
    assert!(
        p.contains("(allow file-map-executable (subpath \"/Users/test/.copilot\"))"),
        "should allow file-map-executable for native Node.js addons (keytar.node, pty.node)"
    );
}
