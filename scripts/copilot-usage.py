#!/usr/bin/env python3
"""Read exact Copilot model usage from session-store.db."""

import argparse
import sqlite3
import tempfile
from pathlib import Path


USAGE_COLUMNS = (
    "id",
    "turn_index",
    "model",
    "reasoning_effort",
    "input_tokens",
    "output_tokens",
    "cache_read_tokens",
    "cache_write_tokens",
    "reasoning_tokens",
    "total_nano_aiu",
    "duration_ms",
    "time_to_first_token_ms",
    "finish_reason",
    "agent_id",
    "parent_tool_call_id",
)


def connect(path: Path) -> sqlite3.Connection:
    return sqlite3.connect(f"{path.resolve().as_uri()}?mode=ro", uri=True)


def cursor(path: Path) -> int:
    with connect(path) as database:
        row = database.execute(
            "SELECT COALESCE(MAX(id), 0) FROM assistant_usage_events"
        ).fetchone()
    return int(row[0])


def clean(value: object) -> str:
    if value is None:
        return ""
    return str(value).replace("|", "\\u007c").replace("\n", "\\n")


def export_rows(
    path: Path, session_id: str, after_id: int, slug: str, run: int
) -> list[str]:
    columns = ", ".join(USAGE_COLUMNS)
    with connect(path) as database:
        rows = database.execute(
            f"""
            SELECT {columns}
            FROM assistant_usage_events
            WHERE session_id = ? AND id > ?
            ORDER BY id
            """,
            (session_id, after_id),
        ).fetchall()
    prefix = (slug, run, session_id)
    return ["|".join(clean(value) for value in (*prefix, *row)) for row in rows]


def selftest() -> None:
    with tempfile.TemporaryDirectory() as directory:
        path = Path(directory) / "session-store.db"
        with sqlite3.connect(path) as database:
            database.execute(
                """
                CREATE TABLE assistant_usage_events (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    session_id TEXT NOT NULL,
                    turn_index INTEGER,
                    model TEXT NOT NULL,
                    reasoning_effort TEXT,
                    input_tokens INTEGER,
                    output_tokens INTEGER,
                    cache_read_tokens INTEGER,
                    cache_write_tokens INTEGER,
                    reasoning_tokens INTEGER,
                    total_nano_aiu INTEGER,
                    duration_ms INTEGER,
                    time_to_first_token_ms INTEGER,
                    finish_reason TEXT,
                    agent_id TEXT,
                    parent_tool_call_id TEXT
                )
                """
            )
            database.executemany(
                """
                INSERT INTO assistant_usage_events (
                    session_id, turn_index, model, reasoning_effort,
                    input_tokens, output_tokens, cache_read_tokens,
                    cache_write_tokens, reasoning_tokens, total_nano_aiu,
                    duration_ms, time_to_first_token_ms, finish_reason,
                    agent_id, parent_tool_call_id
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                [
                    ("other", 0, "old", None, 10, 1, 0, 10, 0, 100, 20, 5, "stop", None, None),
                    ("wanted", 1, "gpt-6-sol", "high", 100, 20, 60, 30, 4, 2500, 900, 80, "stop", "review", "call|1"),
                ],
            )

        assert cursor(path) == 2
        rows = export_rows(path, "wanted", 1, "t1", 3)
        assert rows == [
            "t1|3|wanted|2|1|gpt-6-sol|high|100|20|60|30|4|2500|900|80|stop|review|call\\u007c1"
        ]


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--selftest", action="store_true")
    subparsers = parser.add_subparsers(dest="command")

    cursor_parser = subparsers.add_parser("cursor")
    cursor_parser.add_argument("database", type=Path)

    export_parser = subparsers.add_parser("export")
    export_parser.add_argument("database", type=Path)
    export_parser.add_argument("--session", required=True)
    export_parser.add_argument("--after", required=True, type=int)
    export_parser.add_argument("--slug", required=True)
    export_parser.add_argument("--run", required=True, type=int)

    args = parser.parse_args()
    if args.selftest:
        selftest()
        return
    if args.command == "cursor":
        print(cursor(args.database))
        return
    if args.command == "export":
        rows = export_rows(
            args.database, args.session, args.after, args.slug, args.run
        )
        if rows:
            print("\n".join(rows))
        return
    parser.error("choose cursor, export or --selftest")


if __name__ == "__main__":
    main()
