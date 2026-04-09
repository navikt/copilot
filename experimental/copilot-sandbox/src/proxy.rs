use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::PathBuf;
use std::sync::Arc;
use std::time::Duration;

const MAX_CONNECTIONS: usize = 64;
const READ_TIMEOUT: Duration = Duration::from_secs(60);
const CONNECT_TIMEOUT: Duration = Duration::from_secs(10);

const GREEN: &str = "\x1b[0;32m";
const RED: &str = "\x1b[0;31m";
const YELLOW: &str = "\x1b[0;33m";
const NC: &str = "\x1b[0m";

pub struct ProxyHandle {
    shutdown_flag: Arc<std::sync::atomic::AtomicBool>,
}

impl ProxyHandle {
    pub fn shutdown(&self) {
        self.shutdown_flag
            .store(true, std::sync::atomic::Ordering::SeqCst);
        // Connect to the listener to unblock accept()
        let _ =
            TcpStream::connect_timeout(&"127.0.0.1:0".parse().unwrap(), Duration::from_millis(100));
    }
}

/// Start the proxy on a background thread. Returns a handle for shutdown.
pub fn start(port: u16, blocked_file: PathBuf) -> Result<ProxyHandle, String> {
    let addr = format!("127.0.0.1:{port}");
    let listener = TcpListener::bind(&addr).map_err(|e| format!("Cannot bind to {addr}: {e}"))?;

    // Non-blocking accept with short timeout for clean shutdown
    listener
        .set_nonblocking(false)
        .map_err(|e| format!("set_nonblocking: {e}"))?;

    let shutdown_flag = Arc::new(std::sync::atomic::AtomicBool::new(false));
    let flag = shutdown_flag.clone();
    let active_count = Arc::new(std::sync::atomic::AtomicUsize::new(0));

    std::thread::Builder::new()
        .name("proxy-accept".into())
        .spawn(move || {
            accept_loop(listener, flag, blocked_file, active_count);
        })
        .map_err(|e| format!("spawn proxy thread: {e}"))?;

    // Give the listener a moment to start
    std::thread::sleep(Duration::from_millis(50));

    Ok(ProxyHandle { shutdown_flag })
}

fn accept_loop(
    listener: TcpListener,
    shutdown: Arc<std::sync::atomic::AtomicBool>,
    blocked_file: PathBuf,
    active_count: Arc<std::sync::atomic::AtomicUsize>,
) {
    // Short accept timeout so we can check shutdown flag
    listener.set_nonblocking(false).ok();

    for stream in listener.incoming() {
        if shutdown.load(std::sync::atomic::Ordering::SeqCst) {
            break;
        }

        let stream = match stream {
            Ok(s) => s,
            Err(_) => continue,
        };

        // Connection limit
        let count = active_count.load(std::sync::atomic::Ordering::SeqCst);
        if count >= MAX_CONNECTIONS {
            log_connection("REJECT", "connection limit", "LIMIT");
            drop(stream);
            continue;
        }

        active_count.fetch_add(1, std::sync::atomic::Ordering::SeqCst);
        let blocked = blocked_file.clone();
        let counter = active_count.clone();

        std::thread::Builder::new()
            .name("proxy-conn".into())
            .spawn(move || {
                handle_connection(stream, &blocked);
                counter.fetch_sub(1, std::sync::atomic::Ordering::SeqCst);
            })
            .ok();
    }
}

fn handle_connection(mut client: TcpStream, blocked_file: &PathBuf) {
    client.set_read_timeout(Some(READ_TIMEOUT)).ok();
    client.set_write_timeout(Some(READ_TIMEOUT)).ok();

    // Read the request line
    let mut buf = [0u8; 8192];
    let n = match client.read(&mut buf) {
        Ok(0) => return,
        Ok(n) => n,
        Err(_) => return,
    };

    let request = String::from_utf8_lossy(&buf[..n]);
    let first_line = request.lines().next().unwrap_or("");

    // Parse method and target
    let parts: Vec<&str> = first_line.split_whitespace().collect();
    if parts.len() < 2 {
        return;
    }

    let method = parts[0];
    let target = parts[1];

    if method.eq_ignore_ascii_case("CONNECT") {
        handle_connect(client, target, blocked_file);
    } else {
        // For non-CONNECT, send a simple error — the sandbox should force
        // CONNECT via proxy env vars for HTTPS traffic
        log_connection(method, target, "UNSUPPORTED");
        let _ = client.write_all(b"HTTP/1.1 405 Method Not Allowed\r\n\r\n");
    }
}

fn handle_connect(mut client: TcpStream, target: &str, blocked_file: &PathBuf) {
    // Parse host:port
    let (host, port) = match target.rsplit_once(':') {
        Some((h, p)) => (h.to_string(), p.parse::<u16>().unwrap_or(443)),
        None => (target.to_string(), 443),
    };

    // Check blocklist
    if is_blocked(&host, blocked_file) {
        log_connection("CONNECT", target, "BLOCKED");
        let _ = client.write_all(b"HTTP/1.1 403 Forbidden\r\n\r\nBlocked by copilot-sandbox\r\n");
        return;
    }

    // Reject connections to private/loopback IPs to prevent proxy bypass
    if is_private_target(&host) {
        log_connection("CONNECT", target, "BLOCKED-PRIVATE");
        let _ = client.write_all(b"HTTP/1.1 403 Forbidden\r\n\r\nPrivate IP blocked\r\n");
        return;
    }

    log_connection("CONNECT", target, "OK");

    // Connect to remote
    let addr = format!("{host}:{port}");
    let remote = match TcpStream::connect_timeout(
        &match addr.to_socket_addrs() {
            Ok(mut addrs) => match addrs.next() {
                Some(a) => a,
                None => {
                    log_connection("CONNECT", target, "DNS-FAIL");
                    let _ = client.write_all(b"HTTP/1.1 502 Bad Gateway\r\n\r\n");
                    return;
                }
            },
            Err(_) => {
                log_connection("CONNECT", target, "DNS-FAIL");
                let _ = client.write_all(b"HTTP/1.1 502 Bad Gateway\r\n\r\n");
                return;
            }
        },
        CONNECT_TIMEOUT,
    ) {
        Ok(s) => s,
        Err(e) => {
            log_connection("CONNECT", target, &format!("FAIL:{e}"));
            let _ = client.write_all(b"HTTP/1.1 502 Bad Gateway\r\n\r\n");
            return;
        }
    };

    // Send 200 to client
    if client
        .write_all(b"HTTP/1.1 200 Connection Established\r\n\r\n")
        .is_err()
    {
        return;
    }

    // Bidirectional relay
    relay(client, remote);
}

fn relay(client: TcpStream, remote: TcpStream) {
    let mut client_read = match client.try_clone() {
        Ok(c) => c,
        Err(_) => return,
    };
    let mut remote_write = match remote.try_clone() {
        Ok(r) => r,
        Err(_) => return,
    };
    let mut remote_read = remote;
    let mut client_write = client;

    // Set timeouts for relay
    client_read.set_read_timeout(Some(READ_TIMEOUT)).ok();
    remote_read.set_read_timeout(Some(READ_TIMEOUT)).ok();

    let t1 = std::thread::spawn(move || {
        let mut buf = [0u8; 8192];
        loop {
            match client_read.read(&mut buf) {
                Ok(0) | Err(_) => break,
                Ok(n) => {
                    if remote_write.write_all(&buf[..n]).is_err() {
                        break;
                    }
                }
            }
        }
        remote_write.shutdown(std::net::Shutdown::Both).ok();
    });

    let t2 = std::thread::spawn(move || {
        let mut buf = [0u8; 8192];
        loop {
            match remote_read.read(&mut buf) {
                Ok(0) | Err(_) => break,
                Ok(n) => {
                    if client_write.write_all(&buf[..n]).is_err() {
                        break;
                    }
                }
            }
        }
        client_write.shutdown(std::net::Shutdown::Both).ok();
    });

    t1.join().ok();
    t2.join().ok();
}

pub(crate) fn is_blocked(hostname: &str, blocked_file: &PathBuf) -> bool {
    let contents = match std::fs::read_to_string(blocked_file) {
        Ok(c) => c,
        Err(_) => return false,
    };
    is_blocked_in_content(hostname, &contents)
}

pub(crate) fn is_blocked_in_content(hostname: &str, contents: &str) -> bool {
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

pub(crate) fn is_private_target(host: &str) -> bool {
    // Reject IP literals that point to loopback or private ranges
    if let Ok(ip) = host.parse::<std::net::IpAddr>() {
        return match ip {
            std::net::IpAddr::V4(v4) => {
                v4.is_loopback() || v4.is_private() || v4.is_link_local() || v4.is_unspecified()
            }
            std::net::IpAddr::V6(v6) => v6.is_loopback() || v6.is_unspecified(),
        };
    }
    // Reject hostname patterns that resolve to localhost
    host == "localhost"
        || host.ends_with(".localhost")
        || host.ends_with(".local")
        || host == "0.0.0.0"
        || host == "[::]"
}

fn log_connection(method: &str, target: &str, status: &str) {
    let color = match status {
        "BLOCKED" | "BLOCKED-PRIVATE" | "LIMIT" => RED,
        "OK" => GREEN,
        _ => YELLOW,
    };
    let timestamp = chrono_now();
    eprintln!("{color}[proxy]{NC} {timestamp} {method} {target} → {status}");
}

fn chrono_now() -> String {
    use std::time::SystemTime;
    let now = SystemTime::now()
        .duration_since(SystemTime::UNIX_EPOCH)
        .unwrap_or_default();
    let secs = now.as_secs();
    let hours = (secs % 86400) / 3600;
    let mins = (secs % 3600) / 60;
    let s = secs % 60;
    format!("{hours:02}:{mins:02}:{s:02}")
}

use std::net::ToSocketAddrs;
