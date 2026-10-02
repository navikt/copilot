#!/usr/bin/env python3
"""Offline controls for the realistic task fixtures. Never starts an agent."""

import argparse
import hashlib
import json
import re
import shutil
import subprocess
import tempfile
import time
import xml.etree.ElementTree as ET
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parent
NODE_IMAGE = "node@sha256:be23f54a88d34e8824c741b19b91064094f92c1c97b194144bfc8b50d67258e2"
GRADLE_IMAGE = "gradle@sha256:68e94b44fa717b894a559f555aaa491b855f94a0de689080f73e56da6c4a7479"
NEXT_IMAGE = "nav-benchmark-next:local"


def docker_limits(memory: str, pids: str) -> list[str]:
    return [
        "docker", "run", "--rm", "--network", "none", "--read-only",
        "--cap-drop", "ALL", "--security-opt", "no-new-privileges",
        "--pids-limit", pids, "--memory", memory, "--cpus", "2",
        "--tmpfs", "/tmp:rw,exec,nosuid,size=256m",
        "-e", "HOME=/tmp",
    ]


def execute(command: list[str], timeout: int) -> subprocess.CompletedProcess[str]:
    try:
        result = subprocess.run(command, capture_output=True, text=True, timeout=timeout, check=False)
    except subprocess.TimeoutExpired as error:
        raise RuntimeError(f"Test timed out after {timeout}s: {command[-5:]}") from error
    if result.returncode not in (0, 1):
        raise RuntimeError(f"Container failure ({result.returncode}): {(result.stdout + result.stderr)[-1200:]}")
    return result


def next_build(task: Path) -> None:
    workspace = task / "workspace"
    node_modules = workspace / "node_modules"
    node_modules.symlink_to("/deps/node_modules")
    command = docker_limits("2g", "256") + [
        "-e", "NEXT_TELEMETRY_DISABLED=1",
        "--mount", f"type=bind,src={task},dst=/task",
        "--workdir", "/task/workspace", NEXT_IMAGE,
        "/deps/node_modules/.bin/next", "build", "--webpack",
    ]
    result = execute(command, 180)
    if result.returncode != 0:
        raise RuntimeError(f"Next.js build/typecheck failed: {(result.stdout + result.stderr)[-1800:]}")


def apply_control(root: Path, patch: Path) -> None:
    target = None
    chunks: list[tuple[list[str], list[str]]] = []
    old: list[str] = []
    new: list[str] = []

    def flush_chunk() -> None:
        nonlocal old, new
        if old or new:
            chunks.append((old, new))
            old, new = [], []

    def flush_file() -> None:
        flush_chunk()
        if target is None:
            return
        content = target.read_text()
        for before, after in chunks:
            needle = "".join(before)
            if not needle or content.count(needle) != 1:
                raise ValueError(f"Control does not apply uniquely: {target}")
            content = content.replace(needle, "".join(after), 1)
        target.write_text(content)
        chunks.clear()

    for line in patch.read_text().splitlines(keepends=True):
        if line.startswith("*** Update File: "):
            flush_file()
            path = line.removeprefix("*** Update File: ").strip()
            target = (root / path).resolve()
            if not target.is_relative_to(root.resolve()) or not target.is_file():
                raise ValueError(f"Invalid control path: {path}")
        elif line.startswith("@@"):
            flush_chunk()
        elif line.startswith("***"):
            flush_file()
            target = None
        elif target is not None and line[:1] in (" ", "-", "+"):
            if line[0] != "+":
                old.append(line[1:])
            if line[0] != "-":
                new.append(line[1:])
        else:
            raise ValueError(f"Invalid control patch line: {line!r}")


def run_tests(task: Path, suite: str) -> dict:
    if (task / "workspace/build.gradle.kts").is_file():
        source = task / "controls" / suite
        files = sorted(source.glob("*.kt"))
        if not files:
            raise ValueError(f"No {suite} tests in {task}")
        destination = task / "workspace/src/test/kotlin/no/nav/benchmark"
        for file in files:
            shutil.copy2(file, destination / file.name)
        cache = ROOT / "kotlin/.cache"
        if not cache.is_dir():
            raise RuntimeError("Warm the Kotlin dependency cache as described in kotlin/README.md")
        command = docker_limits("2g", "256") + [
            "--mount", f"type=bind,src={cache},dst=/cache",
            "--mount", f"type=bind,src={task / 'workspace'},dst=/work",
            "--workdir", "/work", "-e", "GRADLE_USER_HOME=/cache",
            GRADLE_IMAGE, "gradle", "test", "--offline", "--no-daemon",
            "--console=plain", "-q",
        ]
        for file in files:
            command.extend(("--tests", f"no.nav.benchmark.{file.stem}"))
        try:
            result = execute(command, 240)
        finally:
            for file in files:
                (destination / file.name).unlink()
        reports = task / "workspace/build/test-results/test"
        counts = []
        for file in files:
            report = reports / f"TEST-no.nav.benchmark.{file.stem}.xml"
            if not report.is_file():
                raise RuntimeError(f"No JUnit result for {file.name}: {result.stderr[-1200:]}")
            suite_result = ET.parse(report).getroot()
            tests = int(suite_result.attrib["tests"])
            expected = file.read_text().count("@Test")
            if tests != expected or tests == 0:
                raise RuntimeError(f"Incomplete JUnit result for {file.name}: {tests}/{expected}")
            counts.append((tests, int(suite_result.attrib["failures"]), int(suite_result.attrib["errors"])))
        total = sum(tests for tests, _, _ in counts)
        failed = sum(failures + errors for _, failures, errors in counts)
        return {"passed": total - failed, "total": total,
                "diagnostic": result.stderr[-1200:] if failed else ""}

    is_http = suite == "http"
    command = docker_limits("1g" if is_http else "512m", "128") + [
        "--mount", f"type=bind,src={task},dst=/task" + ("" if is_http else ",readonly"),
        "--workdir", "/task", NEXT_IMAGE if is_http else NODE_IMAGE,
        "node", "--test", "--test-reporter=tap",
    ]
    files = ([task / "controls/http.test.mjs"] if is_http else
             sorted((task / "controls" / suite).glob("*.test.ts")))
    if not files:
        raise ValueError(f"No {suite} tests in {task}")
    command.extend(str(path.relative_to(task)) for path in files)
    result = execute(command, 90)
    tests = re.search(r"^# tests (\d+)$", result.stdout, re.MULTILINE)
    expected = sum(len(re.findall(r"\btest\(", file.read_text())) for file in files)
    if tests is None or int(tests.group(1)) != expected or expected == 0:
        raise RuntimeError(f"Incomplete Node test results: {(result.stdout + result.stderr)[-1400:]}")
    passed = re.search(r"^# pass (\d+)$", result.stdout, re.MULTILINE)
    failed = re.search(r"^# fail (\d+)$", result.stdout, re.MULTILINE)
    if passed is None or failed is None or int(passed.group(1)) + int(failed.group(1)) != expected:
        raise RuntimeError(f"Incomplete Node assertions: {(result.stdout + result.stderr)[-1400:]}")
    return {"passed": int(passed.group(1)), "total": expected,
            "diagnostic": (result.stdout + result.stderr)[-1200:] if result.returncode else ""}


def passed(result: dict) -> bool:
    return result["passed"] == result["total"]


def fixture_revision(task: Path) -> str:
    digest = hashlib.sha256()
    for path in sorted(task.rglob("*")):
        if not path.is_file() or any(part in ("build", ".gradle", ".next", "node_modules")
                                     for part in path.relative_to(task).parts):
            continue
        digest.update(str(path.relative_to(task)).encode())
        digest.update(path.read_bytes())
    return digest.hexdigest()


def evaluate(task: Path, candidate: Path, task_name: str | None = None) -> dict:
    if not candidate.is_dir() or any(path.is_symlink() for path in candidate.rglob("*")
                                     if not any(part in ("build", ".gradle", ".next", "node_modules")
                                                for part in path.relative_to(candidate).parts)):
        raise ValueError("Candidate must be a directory without symlinks")
    started = time.monotonic()
    report = {
        "task": task_name or str(task.relative_to(ROOT)),
        "fixture_sha256": fixture_revision(task),
        "evaluated_at": datetime.now(timezone.utc).isoformat(),
        "model": None,
        "effort": None,
        "credits": None,
        "usage_verified": False,
    }
    with tempfile.TemporaryDirectory(prefix=".nav-benchmark-", dir=ROOT) as temporary:
        work = Path(temporary) / "task"
        shutil.copytree(task, work, ignore=shutil.ignore_patterns("build", ".gradle", ".next", "node_modules"))
        shutil.rmtree(work / "workspace")
        shutil.copytree(candidate, work / "workspace",
                        ignore=shutil.ignore_patterns("build", ".gradle", ".next", "node_modules"))
        report["candidate_sha256"] = fixture_revision(work / "workspace")
        try:
            if (task / "workspace/package.json").is_file():
                next_build(work)
            report["checks"] = {
                suite: run_tests(work, suite) for suite in ("regression", "acceptance")
            }
            if (task / "workspace/package.json").is_file():
                report["checks"]["http"] = run_tests(work, "http")
            report["status"] = "completed" if all(passed(value) for value in report["checks"].values()) else "failed"
        except RuntimeError as error:
            report["status"] = "untestable"
            report["error"] = str(error)
    report["evaluation_seconds"] = round(time.monotonic() - started, 3)
    return report


def evaluate_attempt(attempt: Path) -> dict:
    from prepare import TASKS, hashes
    manifest = json.loads((attempt / "manifest.json").read_text())
    if manifest["task"] not in TASKS:
        raise ValueError("Unknown task in attempt manifest")
    fixture = attempt / "evaluator/fixture"
    if fixture.is_symlink() or any(path.is_symlink() for path in fixture.rglob("*")):
        raise ValueError("Frozen fixture contains symlinks")
    if hashes(fixture) != manifest["fixture_files"]:
        raise ValueError("Frozen fixture no longer matches attempt manifest")
    return evaluate(fixture, attempt / "workspace", manifest["task"])


def check(task: Path) -> dict:
    results = {}
    for state in ("initial", "solution", "invalid"):
        with tempfile.TemporaryDirectory(prefix=".nav-benchmark-", dir=ROOT) as temporary:
            work = Path(temporary) / "task"
            shutil.copytree(task, work, ignore=shutil.ignore_patterns("build", ".gradle", ".next", "node_modules"))
            if state != "initial":
                apply_control(work, work / "controls" / f"{state}.patch")
            if (work / "workspace/package.json").is_file():
                next_build(work)
            results[state] = {
                suite: passed(run_tests(work, suite)) for suite in ("regression", "acceptance")
            }
            if (work / "workspace/package.json").is_file():
                results[state]["http"] = passed(run_tests(work, "http"))
    expected = {
        "initial": {"regression": True, "acceptance": False,
                    **({"http": False} if "http" in results["initial"] else {})},
        "solution": {"regression": True, "acceptance": True,
                     **({"http": True} if "http" in results["solution"] else {})},
    }
    if any(results[state] != value for state, value in expected.items()):
        raise AssertionError(f"{task.name}: control mismatch: {results}")
    if all(results["invalid"].values()):
        raise AssertionError(f"{task.name}: invalid patch escaped controls")
    return results


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("tasks", nargs="*", help="Task paths, default: all four tasks")
    parser.add_argument("--candidate", type=Path, help="Evaluate this completed workspace without starting an agent")
    parser.add_argument("--attempt", type=Path, help="Evaluate a prepared attempt against its frozen fixture")
    parser.add_argument("--output", type=Path, help="New JSON file for offline candidate evaluation")
    args = parser.parse_args()
    if args.attempt:
        if args.tasks or args.candidate or not args.output:
            parser.error("--attempt requires --output and no task or --candidate")
        report = evaluate_attempt(args.attempt.resolve())
        with args.output.open("x") as output:
            json.dump(report, output, indent=2, ensure_ascii=False)
            output.write("\n")
        print(json.dumps({"status": report["status"], "output": str(args.output)}))
        return
    tasks = ([Path(path).resolve() for path in args.tasks] if args.tasks else
             sorted(task for language in ("typescript", "kotlin")
                    for task in (ROOT / language).iterdir() if (task / "workspace").is_dir()))
    if args.candidate or args.output:
        if not args.candidate or not args.output or len(tasks) != 1:
            parser.error("candidate evaluation requires one task, --candidate and --output")
        if tasks[0] not in sorted(task for language in ("typescript", "kotlin")
                                 for task in (ROOT / language).iterdir() if (task / "workspace").is_dir()):
            parser.error("unknown task")
        report = evaluate(tasks[0], args.candidate.resolve())
        with args.output.open("x") as output:
            json.dump(report, output, indent=2, ensure_ascii=False)
            output.write("\n")
        print(json.dumps({"status": report["status"], "output": str(args.output)}))
        return
    for task in tasks:
        print(json.dumps({"task": str(task.relative_to(ROOT)), "controls": check(task)}, sort_keys=True))


if __name__ == "__main__":
    main()
