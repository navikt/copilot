#!/usr/bin/env python3
"""Offline controls for the realistic task fixtures. Never starts an agent."""

import argparse
import hashlib
import io
import json
import re
import shutil
import subprocess
import tarfile
import tempfile
import time
import uuid
import xml.etree.ElementTree as ET
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parent
NODE_IMAGE = "node@sha256:be23f54a88d34e8824c741b19b91064094f92c1c97b194144bfc8b50d67258e2"
GRADLE_IMAGE = "gradle@sha256:68e94b44fa717b894a559f555aaa491b855f94a0de689080f73e56da6c4a7479"
NEXT_IMAGE = "nav-benchmark-next:local"
OASIS_IMAGE = "nav-benchmark-oasis:local"
GRADLE_CACHE = "nav-benchmark-gradle-cache"
VOLUMES: dict[Path, str] = {}
KOTLIN_RUNS: dict[Path, tuple[dict[str, bytes], str]] = {}


def next_image(task: Path) -> str:
    return OASIS_IMAGE if "@navikt/oasis" in (task / "workspace/package.json").read_text() else NEXT_IMAGE


def task_mount(task: Path, readonly: bool = False) -> list[str]:
    """Copy the task into a Docker volume once. Bind mounts are not available on every
    runtime (Colima without host mounts), so containers never see host paths."""
    if task not in VOLUMES:
        name = f"nav-benchmark-{uuid.uuid4().hex}"
        subprocess.run(["docker", "volume", "create", name], check=True, capture_output=True)
        VOLUMES[task] = name
        archive = io.BytesIO()
        with tarfile.open(fileobj=archive, mode="w") as tar:
            tar.add(task, arcname=".")
        subprocess.run(["docker", "run", "--rm", "-i", "--network", "none",
                        "--mount", f"type=volume,src={name},dst=/task", NODE_IMAGE, "tar", "-x", "--no-same-owner", "-C", "/task"],
                       input=archive.getvalue(), check=True, capture_output=True)
    return ["--mount", f"type=volume,src={VOLUMES[task]},dst=/task" + (",readonly" if readonly else "")]


def read_from_volume(task: Path, directory: str) -> dict[str, bytes]:
    result = subprocess.run(["docker", "run", "--rm", "--network", "none", *task_mount(task, readonly=True),
                             NODE_IMAGE, "sh", "-c", f"cd /task/{directory} 2>/dev/null && tar -c . || true"],
                            capture_output=True, check=True)
    if not result.stdout:
        return {}
    with tarfile.open(fileobj=io.BytesIO(result.stdout)) as tar:
        return {Path(member.name).name: tar.extractfile(member).read() for member in tar.getmembers() if member.isfile()}


def release(task: Path) -> None:
    KOTLIN_RUNS.pop(task, None)
    name = VOLUMES.pop(task, None)
    if name:
        subprocess.run(["docker", "volume", "rm", "-f", name], capture_output=True, check=False)


def docker_limits(memory: str, pids: str) -> list[str]:
    return [
        "docker", "run", "--rm", "--name", f"nav-benchmark-{uuid.uuid4().hex}",
        "--network", "none", "--read-only",
        "--cap-drop", "ALL", "--security-opt", "no-new-privileges",
        "--pids-limit", pids, "--memory", memory, "--cpus", "2",
        "--tmpfs", "/tmp:rw,exec,nosuid,size=256m",
        "-e", "HOME=/tmp",
    ]


def execute(command: list[str], timeout: int) -> subprocess.CompletedProcess[str]:
    try:
        result = subprocess.run(command, capture_output=True, text=True, timeout=timeout, check=False)
    except subprocess.TimeoutExpired as error:
        # Killing the client does not stop the container; remove it by name.
        subprocess.run(["docker", "rm", "-f", command[command.index("--name") + 1]], capture_output=True, check=False)
        raise RuntimeError(f"Test timed out after {timeout}s: {command[-5:]}") from error
    if result.returncode not in (0, 1):
        raise RuntimeError(f"Container failure ({result.returncode}): {(result.stdout + result.stderr)[-1200:]}")
    return result


def next_build(task: Path, image: str) -> None:
    workspace = task / "workspace"
    node_modules = workspace / "node_modules"
    node_modules.symlink_to("/deps/node_modules")
    command = docker_limits("2g", "256") + [
        "-e", "NEXT_TELEMETRY_DISABLED=1", *task_mount(task),
        "--workdir", "/task/workspace", image,
        "/deps/node_modules/.bin/next", "build", "--webpack",
    ]
    result = execute(command, 600)
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


def gradle_test(task: Path) -> tuple[dict[str, bytes], str]:
    """One offline Gradle run with every withheld suite; JVM start-up dominates on slow hosts."""
    if subprocess.run(["docker", "volume", "inspect", GRADLE_CACHE], capture_output=True).returncode:
        raise RuntimeError("Warm the Kotlin dependency cache as described in kotlin/README.md")
    destination = "/task/workspace/src/test/kotlin/no/nav/benchmark"
    files = sorted((task / "controls").glob("*/*.kt"))
    tests = " ".join(f"--tests no.nav.benchmark.{file.stem}" for file in files)
    copies = " && ".join(f"cp /task/controls/{file.parent.name}/{file.name} {destination}/" for file in files)
    # Only withheld tests run (--tests filter), so the candidate's own tests are never
    # evidence; dropping them keeps an uncompilable agent test from making it untestable.
    script = (f"rm -rf src/test && mkdir -p {destination} && {copies} && "
              f"rm -rf build/test-results/test; gradle test --offline --no-daemon --console=plain -q {tests}; "
              f"rc=$?; exit $rc")
    command = docker_limits("2g", "256") + [
        # The candidate build runs arbitrary code: share the warmed cache read-only.
        "--mount", f"type=volume,src={GRADLE_CACHE},dst=/cache,readonly", *task_mount(task),
        "--workdir", "/task/workspace", "-e", "GRADLE_USER_HOME=/task/.gradle-home",
        "-e", "GRADLE_RO_DEP_CACHE=/cache/caches",
        GRADLE_IMAGE, "sh", "-c", script,
    ]
    result = execute(command, 600)
    return read_from_volume(task, "workspace/build/test-results/test"), result.stderr


def run_tests(task: Path, suite: str, image: str = NEXT_IMAGE) -> dict:
    if (task / "workspace/build.gradle.kts").is_file():
        files = sorted((task / "controls" / suite).glob("*.kt"))
        if not files:
            raise ValueError(f"No {suite} tests in {task}")
        if task not in KOTLIN_RUNS:
            KOTLIN_RUNS[task] = gradle_test(task)
        reports, stderr = KOTLIN_RUNS[task]
        result = subprocess.CompletedProcess([], 0, "", stderr)
        counts = []
        for file in files:
            report = reports.get(f"TEST-no.nav.benchmark.{file.stem}.xml")
            if report is None:
                raise RuntimeError(f"No JUnit result for {file.name}: {result.stderr[-1200:]}")
            suite_result = ET.fromstring(report)
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
        *task_mount(task, readonly=not is_http),
        "--workdir", "/task", image,
        "node", "--test", "--test-reporter=tap",
    ]
    files = ([task / "controls/http.test.mjs"] if is_http else
             sorted((task / "controls" / suite).glob("*.test.ts")))
    if not files:
        raise ValueError(f"No {suite} tests in {task}")
    command.extend(str(path.relative_to(task)) for path in files)
    result = execute(command, 240)
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
        is_next = (task / "workspace/package.json").is_file()
        image = next_image(task) if is_next else NEXT_IMAGE
        try:
            if is_next:
                next_build(work, image)
            report["checks"] = {
                suite: run_tests(work, suite, image) for suite in ("regression", "acceptance")
            }
            if is_next:
                report["checks"]["http"] = run_tests(work, "http", image)
            report["status"] = "completed" if all(passed(value) for value in report["checks"].values()) else "failed"
        except RuntimeError as error:
            report["status"] = "untestable"
            report["error"] = str(error)
        finally:
            release(work)
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
    invalids = sorted(path.stem for path in (task / "controls").glob("invalid*.patch"))
    if not invalids:
        raise ValueError(f"No invalid control in {task}")
    for state in ("initial", "solution", *invalids):
        with tempfile.TemporaryDirectory(prefix=".nav-benchmark-", dir=ROOT) as temporary:
            work = Path(temporary) / "task"
            shutil.copytree(task, work, ignore=shutil.ignore_patterns("build", ".gradle", ".next", "node_modules"))
            if state != "initial":
                apply_control(work, work / "controls" / f"{state}.patch")
            is_next = (work / "workspace/package.json").is_file()
            image = next_image(work) if is_next else NEXT_IMAGE
            try:
                if is_next:
                    next_build(work, image)
                results[state] = {
                    suite: passed(run_tests(work, suite, image)) for suite in ("regression", "acceptance")
                }
                if is_next:
                    results[state]["http"] = passed(run_tests(work, "http", image))
            finally:
                release(work)
    expected = {
        "initial": {"regression": True, "acceptance": False,
                    **({"http": False} if "http" in results["initial"] else {})},
        "solution": {"regression": True, "acceptance": True,
                     **({"http": True} if "http" in results["solution"] else {})},
    }
    if any(results[state] != value for state, value in expected.items()):
        raise AssertionError(f"{task.name}: control mismatch: {results}")
    for state in invalids:
        if all(results[state].values()):
            raise AssertionError(f"{task.name}: {state} patch escaped controls")
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
