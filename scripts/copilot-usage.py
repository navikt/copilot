#!/usr/bin/env python3
"""Read exact Copilot model usage from session-store.db, or estimate OpenCode usage."""

from __future__ import annotations  # macOS /usr/bin/python3 is 3.9

import argparse
import json
import re
import sqlite3
import sys
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


PRICING_TS = Path(__file__).resolve().parent.parent / "apps/my-copilot/src/lib/model-pricing.ts"


def load_prices(path: Path) -> dict[str, list[tuple[int, tuple[float, ...]]]]:
    """model-pricing.ts as {model id: [(tier limit, (input, cache read, cache write, output))]}.

    USD per 1M tokens. "GPT-6 Luna (Default, ≤ 272K)" becomes gpt-6-luna with
    limit 272000; its "Long context" entry is the price above that. A model
    without cacheWrite pays the input price for cache writes.
    """
    base: dict[str, tuple[int, tuple[float, ...]]] = {}
    long: dict[str, tuple[float, ...]] = {}
    for block in re.findall(r'\{[^{}]*model: "[^{}]*\}', path.read_text()):
        name = re.search(r'model: "([^"]+)"', block).group(1)

        def num(key: str, required: bool = True) -> float | None:
            m = re.search(rf"\b{key}: ([0-9.]+)", block)
            if not m and required:
                sys.exit(f"{path}: {name} has no {key}")
            return float(m.group(1)) if m else None

        rate = (num("input"), num("cachedInput"), num("cacheWrite", False) or num("input"), num("output"))
        model = name.split(" (")[0].lower().replace(" ", "-")
        limit = re.search(r"≤ (\d+)K", name)
        if "Long context" in name:
            long[model] = rate
        elif model not in base:  # first entry wins: "(fast mode)" variants share a base name
            base[model] = (int(limit.group(1)) * 1000 if limit else 10**12, rate)
    if not base:
        sys.exit(f"{path}: no model prices found")
    prices: dict[str, list[tuple[int, tuple[float, ...]]]] = {}
    for model, (limit, rate) in base.items():
        prices[model] = [(limit, rate)]
        if model in long:
            if limit >= 10**12:
                sys.exit(f"{path}: {model} has a Long context price but no ≤ limit")
            prices[model].append((10**12, long.pop(model)))
    if long:
        sys.exit(f"{path}: {', '.join(long)} has a Long context price but no Default entry")
    return prices


def opencode_rows(
    export: dict, prices: dict, since_ms: int, slug: str, run: int, title: str
) -> list[str]:
    """Rows in the export_rows layout from `opencode session export` (v2).

    total_nano_aiu is an ESTIMATE from list price, not a billed number.
    OpenCode's input excludes cache reads and writes and its output excludes
    reasoning, so input_tokens is their sum (as in assistant_usage_events) and
    reasoning is priced as output. 1 credit = $0.01: nano-credits = USD * 1e11.
    Unknown models get "unknown", which a credit cap must treat as over.
    A child session (info.parentID set) is a subagent: col 17 names its agent.
    """
    rows = []
    subagent = export.get("info", {}).get("agent", "") if export.get("info", {}).get("parentID") else ""
    for msg in export.get("messages", []):
        times = msg.get("time", {})
        if msg.get("type") != "assistant" or times.get("created", 0) < since_ms:
            continue
        t = msg.get("tokens", {})
        fresh, out, reason = t.get("input", 0), t.get("output", 0), t.get("reasoning", 0)
        read, write = t.get("cache", {}).get("read", 0), t.get("cache", {}).get("write", 0)
        model = msg.get("model", {}).get("id", "")
        total_in = fresh + read + write
        nano: object = "unknown"
        if tiers := prices.get(model.split("/")[-1]):
            pi, pc, pw, po = next((r for limit, r in tiers if total_in <= limit), tiers[-1][1])
            nano = round((fresh * pi + read * pc + write * pw + (out + reason) * po) * 1e5)
        duration = times["completed"] - times["created"] if "completed" in times else ""
        rows.append("|".join(clean(v) for v in (
            slug, run, title, msg.get("id"), "", model, msg.get("model", {}).get("variant"),
            total_in, out, read, write, reason, nano, duration, "",
            msg.get("finish"), subagent, "",
        )))
    return rows


def opencode_children(export: dict) -> list[str]:
    """Child session ids, from the subagent tool calls in an export (v2.1.3:
    state.metadata.sessionID). `session list` shows top-level sessions only."""
    ids = []
    for msg in export.get("messages", []):
        for part in msg.get("content") or []:
            sid = (part.get("state", {}).get("metadata") or {}).get("sessionID") if part.get("type") == "tool" and part.get("name") == "subagent" else None
            if sid and sid not in ids:
                ids.append(sid)
    return ids


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

    # OpenCode estimate. Fake prices and fake token counts, never real ones.
    with tempfile.TemporaryDirectory() as directory:
        ts = Path(directory) / "model-pricing.ts"
        ts.write_text(
            '{ model: "Fake One (Default, ≤ 100K)", input: 1, cachedInput: 0.1, cacheWrite: 1.25, output: 10 },\n'
            '{ model: "Fake One (Long context, 100K)", input: 2, cachedInput: 0.2, output: 20 },\n'
            '{ model: "Fake Two (Default)", input: 3, cachedInput: 0.3, output: 30 },\n'
            '{ model: "Fake Two (fast mode) (Default)", input: 9, cachedInput: 9, output: 9 },\n'
        )
        prices = load_prices(ts)
        assert prices["fake-one"] == [(100_000, (1, 0.1, 1.25, 10)), (10**12, (2, 0.2, 2, 20))]
        assert prices["fake-two"] == [(10**12, (3, 0.3, 3, 30))]
        assert "gpt-6-luna" in load_prices(PRICING_TS)

        # Long context listed before Default: same tiers, not long-context for everything.
        ts.write_text(
            '{ model: "Fake One (Long context, 100K)", input: 2, cachedInput: 0.2, output: 20 },\n'
            '{ model: "Fake One (Default, ≤ 100K)", input: 1, cachedInput: 0.1, cacheWrite: 1.25, output: 10 },\n'
        )
        assert load_prices(ts)["fake-one"] == prices["fake-one"]

        def exits(text: str, needle: str) -> None:
            ts.write_text(text)
            try:
                load_prices(ts)
            except SystemExit as e:
                assert needle in str(e), e
            else:
                raise AssertionError(f"no exit for {text!r}")

        exits('{ model: "Fake One (Default)", input: 1, cachedInput: 0.1, output: 10 },\n'
              '{ model: "Fake One (Long context, 100K)", input: 2, cachedInput: 0.2, output: 20 },\n',
              "fake-one has a Long context price but no ≤ limit")
        exits("export const MODEL_PRICING = [];\n", "no model prices found")
        exits('{ model: "Fake One (Default)", input: 1, output: 10 },\n', "Fake One (Default) has no cachedInput")

        def msg(created: int, model: str, fresh: int, read: int, out: int, reason: int) -> dict:
            return {"type": "assistant", "id": f"m{created}", "agent": "golden-nav-pilot", "finish": "stop",
                    "time": {"created": created, "completed": created + 7},
                    "model": {"id": model, "variant": "default"},
                    "tokens": {"input": fresh, "output": out, "reasoning": reason,
                               "cache": {"read": read, "write": 0}}}

        export = {"messages": [
            msg(5, "fake-one", 1, 1, 1, 1),  # before since_ms: an earlier turn
            {"type": "user", "time": {"created": 20}},
            msg(20, "fake-one", 600, 400, 150, 50),
            msg(30, "provider/fake-one", 100_000, 1, 0, 0),  # 100001 > 100K
            msg(40, "nope", 1, 0, 0, 0),
        ]}
        rows = opencode_rows(export, prices, 10, "t1", 2, "ttl")
        # 600*1 + 400*0.1 + (150+50)*10 = 2640 USD per 1M = 0.264 credits
        assert rows[0] == "t1|2|ttl|m20||fake-one|default|1000|150|400|0|50|264000000|7||stop||", rows[0]
        assert rows[1].split("|")[12] == str(round((100_000 * 2 + 0.2) * 1e5))
        assert rows[2].split("|")[12] == "unknown"
        assert len(rows) == 3

        # A real OpenCode v2.1.3 export (fake LLM, then every token count, id,
        # path, text and timestamp replaced by fakes): pins the export shape.
        real = json.loads((Path(__file__).parent / "testdata/opencode-session-export-v2.json").read_text())
        rows = [r.split("|") for r in opencode_rows(real, prices | {"gpt-6-luna": prices["fake-one"]}, 1050, "t4", 1, "ttl")]
        assert len(rows) == 1  # turn two only; turn one was created before 1050
        # input 67 + cache read 88; output 74, reasoning 81 priced as output
        assert rows[0][3:13] == ["fake_id_6", "", "gpt-6-luna", "default", "155", "74", "88", "0", "81",
                                 str(round((67 * 1 + 88 * 0.1 + (74 + 81) * 10) * 1e5))], rows[0]
        assert rows[0][13] == "20" and rows[0][15:17] == ["stop", ""]  # agent "build" is the primary agent, not a subagent
        assert opencode_children(real) == []

        # A delegating parent and its child (fake ids and numbers). The child
        # is continued once, so its id appears twice but is listed once.
        tool = {"type": "tool", "name": "subagent", "state": {"status": "completed", "metadata": {"sessionID": "ses_child"}}}
        parent = {"messages": [{"type": "assistant", "content": [tool, {"type": "text"}]},
                               {"type": "assistant", "content": [tool]}, {"type": "user"}]}
        assert opencode_children(parent) == ["ses_child"]
        child = {"info": {"id": "ses_child", "parentID": "ses_parent", "agent": "fake-explorer"},
                 "messages": [msg(20, "fake-one", 10, 0, 1, 0)]}
        row = opencode_rows(child, prices, 10, "t1", 2, "ttl")[0].split("|")
        assert row[16] == "fake-explorer" and row[12] == str(round((10 * 1 + 1 * 10) * 1e5)), row


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

    opencode_parser = subparsers.add_parser("opencode")  # export JSON on stdin
    opencode_parser.add_argument("--since-ms", required=True, type=int)
    opencode_parser.add_argument("--slug", required=True)
    opencode_parser.add_argument("--run", required=True, type=int)
    opencode_parser.add_argument("--title", required=True)
    opencode_parser.add_argument("--pricing", type=Path, default=PRICING_TS)

    subparsers.add_parser("opencode-children")  # export JSON on stdin

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
    if args.command == "opencode":
        rows = opencode_rows(
            json.load(sys.stdin), load_prices(args.pricing),
            args.since_ms, args.slug, args.run, args.title,
        )
        if rows:
            print("\n".join(rows))
        return
    if args.command == "opencode-children":
        for sid in opencode_children(json.load(sys.stdin)):
            print(sid)
        return
    parser.error("choose cursor, export, opencode, opencode-children or --selftest")


if __name__ == "__main__":
    main()
