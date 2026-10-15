#!/usr/bin/env python3
"""Benchmark nav-pilot startup latency and parent subprocess spawns.

Inspired by navikt/cplt hack/bench-startup.py (#769). Runs nav-pilot in an
isolated temporary HOME/XDG/NAV_PILOT_CONFIG environment with fake client
shims and a blackholed network so developer state in ~/.copilot and
~/.nav-pilot is never touched.

Usage:
    python3 cli/nav-pilot/hack/bench-startup.py
    python3 cli/nav-pilot/hack/bench-startup.py --check-spawns
    python3 cli/nav-pilot/hack/bench-startup.py --save /tmp/before.json
    python3 cli/nav-pilot/hack/bench-startup.py --compare /tmp/before.json --check-spawns
"""

from __future__ import annotations

import argparse
import collections
import datetime
import json
import os
import pathlib
import platform
import shutil
import socket
import statistics
import subprocess
import sys
import tempfile
import threading
from typing import Any

REPO_ROOT = pathlib.Path(__file__).resolve().parents[3]
NAV_PILOT_DIR = REPO_ROOT / "cli" / "nav-pilot"

# Maximum allowed subprocess spawns per scenario after warmup.
SPAWN_BUDGETS: dict[str, dict[str, int]] = {
    "version": {"git": 0, "cplt": 0, "opencode": 0, "copilot": 0},
    "help": {"git": 0, "cplt": 0, "opencode": 0, "copilot": 0},
    "config_get": {"git": 0, "cplt": 0, "opencode": 0, "copilot": 0},
    "list": {"git": 0, "cplt": 0, "opencode": 0, "copilot": 0},
    "copilot_launch": {"git": 0, "cplt": 1, "opencode": 0, "copilot": 0},
    "opencode_launch": {"git": 0, "cplt": 1, "opencode": 0, "copilot": 0},
    "pakke_copilot_launch": {"git": 0, "cplt": 1, "opencode": 0, "copilot": 0},
}


class BlackholeServer:
    """Accepts TCP connections and holds them open without replying."""

    def __init__(self) -> None:
        self._sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._sock.bind(("127.0.0.1", 0))
        self._sock.listen(64)
        self.addr = f"127.0.0.1:{self._sock.getsockname()[1]}"
        self._conns: list[socket.socket] = []
        self._closed = False
        self._thread = threading.Thread(target=self._serve, daemon=True)
        self._thread.start()

    def _serve(self) -> None:
        while not self._closed:
            try:
                conn, _ = self._sock.accept()
                self._conns.append(conn)
            except OSError:
                break

    def close(self) -> None:
        self._closed = True
        try:
            self._sock.close()
        except OSError:
            pass
        for conn in self._conns:
            try:
                conn.close()
            except OSError:
                pass


SHIM_SOURCE = r"""package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
)

func main() {
	name := filepath.Base(os.Args[0])
	if logPath := os.Getenv("NAV_PILOT_BENCH_SPAWN_LOG"); logPath != "" {
		if f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			fmt.Fprintf(f, "%s\n", name)
			f.Close()
		}
	}
	if name == "git" {
		realGit := os.Getenv("NAV_PILOT_BENCH_REAL_GIT")
		cmd := exec.Command(realGit, os.Args[1:]...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			os.Exit(1)
		}
		return
	}
	if slices.Contains(os.Args[1:], "--version") {
		switch name {
		case "cplt":
			fmt.Println("cplt 2026.09.24-192459-38642b4")
		case "opencode":
			fmt.Println("1.18.25")
		default:
			fmt.Println("GitHub Copilot CLI 1.0.40")
		}
		return
	}
	if name == "opencode" && slices.Contains(os.Args[1:], "session") {
		fmt.Println("[]")
		return
	}
}
"""


def build_binaries(tmp_dir: pathlib.Path, existing_binary: str | None) -> tuple[pathlib.Path, pathlib.Path]:
    if existing_binary:
        nav_pilot_bin = pathlib.Path(existing_binary).resolve()
        if not nav_pilot_bin.exists():
            sys.exit(f"error: binary not found: {nav_pilot_bin}")
    else:
        nav_pilot_bin = tmp_dir / "nav-pilot"
        subprocess.run(
            ["go", "build", "-o", str(nav_pilot_bin), "."],
            cwd=NAV_PILOT_DIR,
            check=True,
        )

    shim_src = tmp_dir / "shim.go"
    shim_src.write_text(SHIM_SOURCE, encoding="utf-8")
    shim_bin = tmp_dir / "shim"
    subprocess.run(
        ["go", "build", "-o", str(shim_bin), str(shim_src)],
        cwd=tmp_dir,
        check=True,
    )
    return nav_pilot_bin, shim_bin


def setup_environment(
    tmp_dir: pathlib.Path,
    nav_pilot_bin: pathlib.Path,
    shim_bin: pathlib.Path,
    hole_addr: str,
) -> tuple[pathlib.Path, pathlib.Path, pathlib.Path, dict[str, str]]:
    home_dir = tmp_dir / "home"
    xdg_config = tmp_dir / "xdg_config"
    xdg_state = tmp_dir / "xdg_state"
    xdg_cache = tmp_dir / "xdg_cache"
    fake_bin = tmp_dir / "fake_bin"
    repo_dir = tmp_dir / "repo"
    pakke_dir = tmp_dir / "pakke"

    for d in (home_dir, xdg_config, xdg_state, xdg_cache, fake_bin, repo_dir, pakke_dir):
        d.mkdir(parents=True, exist_ok=True)

    real_git = shutil.which("git")
    if not real_git:
        sys.exit("error: git not found in PATH")

    for name in ("cplt", "opencode", "git"):
        os.symlink(shim_bin, fake_bin / name)
    shutil.copy2(shim_bin, fake_bin / "copilot")

    # Initialize consumer repo and a local git-backed Tier 1 agentpakke with a
    # compatibility.copilot requirement so the launch version gate runs too.
    git_env = {
        **os.environ,
        "HOME": str(home_dir),
        "GIT_CONFIG_NOSYSTEM": "1",
        "GIT_AUTHOR_NAME": "bench",
        "GIT_AUTHOR_EMAIL": "bench@nav.no",
        "GIT_COMMITTER_NAME": "bench",
        "GIT_COMMITTER_EMAIL": "bench@nav.no",
    }
    subprocess.run([real_git, "init", "-q", "-b", "main", str(repo_dir)], env=git_env, check=True)
    subprocess.run(
        [real_git, "-C", str(repo_dir), "commit", "-q", "--allow-empty", "-m", "init"],
        env=git_env,
        check=True,
    )

    (pakke_dir / ".nav-pilot").mkdir(parents=True, exist_ok=True)
    (pakke_dir / ".nav-pilot" / "agentpakke.json").write_text(
        json.dumps(
            {
                "contractVersion": "1",
                "name": "plattform",
                "description": "plattform",
                "layout": {"agents": "agents", "skills": "skills"},
                "clients": {"copilot": {"primaryAgents": ["grillmester"]}},
            }
        ),
        encoding="utf-8",
    )
    (pakke_dir / "skills").mkdir(parents=True, exist_ok=True)
    (pakke_dir / "agents").mkdir(parents=True, exist_ok=True)
    (pakke_dir / "agents" / "grillmester.agent.md").write_text(
        "---\nname: grillmester\ndescription: fra plattform\n---\nplattform\n",
        encoding="utf-8",
    )
    subprocess.run([real_git, "init", "-q", "-b", "main", str(pakke_dir)], env=git_env, check=True)
    subprocess.run([real_git, "-C", str(pakke_dir), "add", "."], env=git_env, check=True)
    subprocess.run(
        [real_git, "-C", str(pakke_dir), "commit", "-q", "-m", "init pakke"],
        env=git_env,
        check=True,
    )

    spawn_log = tmp_dir / "spawns.log"
    config_path = home_dir / ".nav-pilot" / "config.toml"

    now_iso = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    (home_dir / ".copilot").mkdir(parents=True, exist_ok=True)
    (home_dir / ".copilot" / "cache.json").write_text(
        json.dumps({"last_checked": now_iso, "latest_version": "2026.09.01-120000-aaaaaaa"}),
        encoding="utf-8",
    )

    env = {
        **os.environ,
        "HOME": str(home_dir),
        "NAV_PILOT_CONFIG": str(config_path),
        "XDG_CONFIG_HOME": str(xdg_config),
        "XDG_STATE_HOME": str(xdg_state),
        "XDG_CACHE_HOME": str(xdg_cache),
        "PATH": f"{fake_bin}{os.pathsep}{nav_pilot_bin.parent}",
        "NO_COLOR": "1",
        "NAV_PILOT_TELEMETRY_ENABLED": "false",
        "DO_NOT_TRACK": "1",
        "NAV_PILOT_TELEMETRY_ENDPOINT": f"http://{hole_addr}/v1/metrics",
        "HTTP_PROXY": f"http://{hole_addr}",
        "HTTPS_PROXY": f"http://{hole_addr}",
        "http_proxy": f"http://{hole_addr}",
        "https_proxy": f"http://{hole_addr}",
        "NO_PROXY": "",
        "no_proxy": "",
        "NAV_PILOT_BENCH_SPAWN_LOG": str(spawn_log),
        "NAV_PILOT_BENCH_REAL_GIT": real_git,
    }

    # Install pakke into repo and set default source to REPO_ROOT (navikt/copilot checkout)
    subprocess.run(
        [str(nav_pilot_bin), "install", "plattform", "--source", str(pakke_dir), "--repo", "--yes"],
        cwd=repo_dir,
        env=env,
        check=True,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    subprocess.run(
        [str(nav_pilot_bin), "config", "set", "source", str(REPO_ROOT)],
        cwd=repo_dir,
        env=env,
        check=True,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )

    return repo_dir, pakke_dir, spawn_log, env


def percentile(sorted_vals: list[float], pct: float) -> float:
    if not sorted_vals:
        return 0.0
    idx = (len(sorted_vals) - 1) * pct
    lo = int(idx)
    hi = min(lo + 1, len(sorted_vals) - 1)
    frac = idx - lo
    return sorted_vals[lo] * (1.0 - frac) + sorted_vals[hi] * frac


def run_scenario(
    nav_pilot_bin: pathlib.Path,
    cwd: pathlib.Path,
    env: dict[str, str],
    spawn_log: pathlib.Path,
    args: list[str],
    warmup: int,
    runs: int,
) -> dict[str, Any]:
    cmd = [str(nav_pilot_bin), *args]

    for _ in range(warmup):
        if spawn_log.exists():
            spawn_log.unlink()
        proc = subprocess.run(cmd, cwd=cwd, env=env, capture_output=True, text=True)
        if proc.returncode != 0:
            sys.exit(f"error: {' '.join(cmd)} failed ({proc.returncode}):\n{proc.stderr}")

    samples_ms: list[float] = []
    last_spawns: dict[str, int] = {"git": 0, "cplt": 0, "opencode": 0, "copilot": 0}

    for _ in range(runs):
        if spawn_log.exists():
            spawn_log.unlink()
        t0 = datetime.datetime.now(datetime.timezone.utc)
        ns0 = time_ns()
        proc = subprocess.run(cmd, cwd=cwd, env=env, capture_output=True, text=True)
        ns1 = time_ns()
        _ = t0
        if proc.returncode != 0:
            sys.exit(f"error: {' '.join(cmd)} failed ({proc.returncode}):\n{proc.stderr}")
        samples_ms.append((ns1 - ns0) / 1_000_000.0)

        counts: collections.Counter[str] = collections.Counter()
        if spawn_log.exists():
            for line in spawn_log.read_text(encoding="utf-8").splitlines():
                name = line.strip()
                if name:
                    counts[name] += 1
        last_spawns = {
            "git": counts["git"],
            "cplt": counts["cplt"],
            "opencode": counts["opencode"],
            "copilot": counts["copilot"],
        }

    s = sorted(samples_ms)
    return {
        "args": args,
        "runs": runs,
        "min_ms": round(s[0], 2),
        "median_ms": round(statistics.median(s), 2),
        "mean_ms": round(statistics.mean(s), 2),
        "p95_ms": round(percentile(s, 0.95), 2),
        "max_ms": round(s[-1], 2),
        "stdev_ms": round(statistics.stdev(s), 2) if len(s) > 1 else 0.0,
        "spawns": last_spawns,
    }


def time_ns() -> int:
    import time

    return time.perf_counter_ns()


def format_spawns(spawns: dict[str, int]) -> str:
    parts = [f"{k}={v}" for k, v in spawns.items() if v > 0]
    return ", ".join(parts) if parts else "0"


def print_table(results: dict[str, Any], baseline: dict[str, Any] | None) -> None:
    scenarios = results["scenarios"]
    base_scenarios = baseline.get("scenarios", {}) if baseline else {}

    print(f"Platform: {results['platform']} | Runs: {results['runs']} (warmup: {results['warmup']})\n")
    if base_scenarios:
        print("| Scenario | Median | P95 | Min | Max | Baseline | Delta | Spawns |")
        print("| :--- | ---: | ---: | ---: | ---: | ---: | ---: | :--- |")
    else:
        print("| Scenario | Median | P95 | Min | Max | Stdev | Spawns |")
        print("| :--- | ---: | ---: | ---: | ---: | ---: | :--- |")

    for name, data in scenarios.items():
        med = data["median_ms"]
        p95 = data["p95_ms"]
        mn = data["min_ms"]
        mx = data["max_ms"]
        stdev = data["stdev_ms"]
        spawns_str = format_spawns(data["spawns"])
        if base_scenarios and name in base_scenarios:
            base_med = base_scenarios[name]["median_ms"]
            diff = med - base_med
            pct = (diff / base_med * 100.0) if base_med > 0 else 0.0
            print(
                f"| `{name}` | {med:.2f} ms | {p95:.2f} ms | {mn:.2f} ms | {mx:.2f} ms "
                f"| {base_med:.2f} ms | {diff:+.2f} ms ({pct:+.1f}%) | `{spawns_str}` |"
            )
        else:
            print(
                f"| `{name}` | {med:.2f} ms | {p95:.2f} ms | {mn:.2f} ms | {mx:.2f} ms "
                f"| {stdev:.2f} ms | `{spawns_str}` |"
            )


def check_spawn_budgets(results: dict[str, Any]) -> bool:
    ok = True
    print("\nSubprocess spawn budget check:")
    for name, data in results["scenarios"].items():
        budget = SPAWN_BUDGETS.get(name, {})
        actual = data["spawns"]
        violations = []
        for bin_name, limit in budget.items():
            got = actual.get(bin_name, 0)
            if got > limit:
                violations.append(f"{bin_name}={got} (budget <={limit})")
        if violations:
            ok = False
            print(f"  FAIL  {name}: {', '.join(violations)}")
        else:
            print(f"  PASS  {name}: {format_spawns(actual)}")
    return ok


def main() -> int:
    parser = argparse.ArgumentParser(description="Benchmark nav-pilot startup latency and subprocess spawns.")
    parser.add_argument("--runs", type=int, default=15, help="Timed runs per scenario (default: 15)")
    parser.add_argument("--warmup", type=int, default=3, help="Warmup runs per scenario (default: 3)")
    parser.add_argument("--binary", type=str, default=None, help="Path to pre-built nav-pilot binary")
    parser.add_argument("--no-build", action="store_true", help="Use ./cli/nav-pilot/nav-pilot if --binary is not set")
    parser.add_argument("--save", type=str, default=None, help="Save JSON benchmark results to path")
    parser.add_argument("--compare", type=str, default=None, help="Compare against saved JSON baseline")
    parser.add_argument("--check-spawns", action="store_true", help="Fail if parent subprocess spawns exceed budget")
    args = parser.parse_args()

    binary_arg = args.binary
    if args.no_build and not binary_arg:
        binary_arg = str(NAV_PILOT_DIR / "nav-pilot")

    hole = BlackholeServer()
    try:
        with tempfile.TemporaryDirectory(prefix="nav-pilot-bench-") as tmp:
            tmp_dir = pathlib.Path(tmp)
            nav_pilot_bin, shim_bin = build_binaries(tmp_dir, binary_arg)
            repo_dir, pakke_dir, spawn_log, env = setup_environment(tmp_dir, nav_pilot_bin, shim_bin, hole.addr)

            scenario_defs: list[tuple[str, list[str]]] = [
                ("version", ["--version"]),
                ("help", ["--help"]),
                ("config_get", ["config", "get", "client"]),
                ("list", ["list"]),
                ("copilot_launch", ["--", "-p", "hei"]),
                ("opencode_launch", ["--client", "opencode", "--", "run", "hei"]),
                ("pakke_copilot_launch", ["--source", str(pakke_dir), "--", "-p", "hei"]),
            ]

            scenarios: dict[str, Any] = {}
            for name, sc_args in scenario_defs:
                scenarios[name] = run_scenario(
                    nav_pilot_bin,
                    repo_dir,
                    env,
                    spawn_log,
                    sc_args,
                    args.warmup,
                    args.runs,
                )

            results: dict[str, Any] = {
                "timestamp": datetime.datetime.now(datetime.timezone.utc).isoformat(),
                "platform": f"{platform.system()} {platform.release()} ({platform.machine()})",
                "runs": args.runs,
                "warmup": args.warmup,
                "scenarios": scenarios,
            }
    finally:
        hole.close()

    baseline = None
    if args.compare:
        baseline = json.loads(pathlib.Path(args.compare).read_text(encoding="utf-8"))

    print_table(results, baseline)

    if args.save:
        out_path = pathlib.Path(args.save)
        out_path.parent.mkdir(parents=True, exist_ok=True)
        out_path.write_text(json.dumps(results, indent=2) + "\n", encoding="utf-8")
        print(f"\nSaved benchmark results to {out_path}")

    if args.check_spawns and not check_spawn_budgets(results):
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
