#!/usr/bin/env python3
"""Seed stale comments into a prepared workspace. Each comment states behaviour the task removes."""
import sys
from pathlib import Path

K = "src/main/kotlin/no/nav/benchmark/"
SEEDS = {
    "kotlin/status-feature": [
        (K + "TaskRepository.kt", "data class Task", "// Tasks are read-only: status never changes after creation."),
        (K + "TaskRepository.kt", "class TaskRepository", "// Read-only store: there is no way to update a task."),
        (K + "TaskService.kt", "class TaskService", "// Thin read-only wrapper: the service never mutates tasks."),
        (K + "TaskRoutes.kt", "fun Application.taskModule", "// Only GET routes: this API exposes no write operations."),
    ],
    "typescript/feature": [
        ("lib/saker.ts", "const saker", "// Fixed seed list: saker are never added at runtime."),
        ("lib/saker.ts", "export function listSaker", "// Returns the seed list; nothing else can modify it."),
        ("app/api/saker/route.ts", "export async function GET", "// Read-only endpoint: this route only supports GET."),
    ],
}


def seed(task: str, workspace: Path) -> None:
    for rel, anchor, comment in SEEDS[task]:
        path = workspace / rel
        lines = path.read_text().splitlines(keepends=True)
        index = next(i for i, line in enumerate(lines) if line.startswith(anchor))
        lines.insert(index, comment + "\n")
        path.write_text("".join(lines))


if __name__ == "__main__":
    seed(sys.argv[1], Path(sys.argv[2]))
