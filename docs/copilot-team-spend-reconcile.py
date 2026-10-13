"""Read-only September billing census; outputs aggregates, never user-level data.

Run: python3 docs/copilot-team-spend-reconcile.py
Requires bq query access to copilot-dev-e17a and gh enterprise billing access.
"""

import argparse
import csv
import io
import json
import subprocess
import sys
import time
from collections import defaultdict
from concurrent.futures import ThreadPoolExecutor, as_completed
from decimal import Decimal
from threading import Event
from urllib.parse import quote


MONTH = "2026-09"
PROJECT = "copilot-dev-e17a"
DATASET = f"{PROJECT}.copilot_metrics"
stop = Event()
QUERY = f"""
WITH usage AS (
  SELECT JSON_VALUE(raw_record, '$.user_id') user_id,
         ARRAY_AGG(LOWER(JSON_VALUE(raw_record, '$.user_login')) ORDER BY day DESC LIMIT 1)[OFFSET(0)] login,
         SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.ai_credits_used') AS FLOAT64)) * 0.01 gross
  FROM `{DATASET}.user_metrics`
  WHERE day >= DATE '{MONTH}-01' AND day < DATE_ADD(DATE '{MONTH}-01', INTERVAL 1 MONTH)
    AND scope = 'enterprise' AND scope_id = 'nav'
  GROUP BY user_id
), teams AS (
  SELECT JSON_VALUE(raw_record, '$.user_id') user_id,
         ARRAY_AGG(LOWER(JSON_VALUE(raw_record, '$.user_login')) ORDER BY day DESC LIMIT 1)[OFFSET(0)] login
  FROM `{DATASET}.user_teams`
  WHERE day >= DATE '{MONTH}-01' AND day < DATE_ADD(DATE '{MONTH}-01', INTERVAL 1 MONTH)
    AND scope = 'enterprise' AND scope_id = 'nav'
  GROUP BY user_id
)
SELECT COALESCE(t.login, u.login) login, u.login usage_login, u.gross,
       IF(u.login IS NULL, 'team_only', 'usage') source
FROM usage u FULL JOIN teams t USING (user_id)
"""


def command(*args):
    return subprocess.run(args, capture_output=True, text=True, check=True, timeout=45).stdout


def get_user(login):
    endpoint = f"enterprises/nav/settings/billing/ai_credit/usage?year=2026&month=9&user={quote(login, safe='')}"
    reason = "unknown"
    for attempt in range(3):
        if stop.is_set():
            return login, None, "stopped"
        try:
            response = json.loads(command("gh", "api", endpoint, "-H", "X-GitHub-Api-Version: 2026-03-10"))
            if response.get("user", "").lower() != login:
                return login, None, "user_mismatch"
            return login, response["usageItems"], None
        except subprocess.CalledProcessError as exc:
            reason = next((f"HTTP {status}" for status in ("404", "403", "429", "500", "502", "503")
                           if f"HTTP {status}" in exc.stderr), "request_error")
            if reason in ("HTTP 403", "HTTP 429"):
                stop.set()
                return login, None, reason
            if reason == "HTTP 404":
                return login, None, reason
        except subprocess.TimeoutExpired:
            reason = "timeout"
        except (ValueError, KeyError):
            reason = "invalid_response"
        if attempt < 2:
            time.sleep(2 ** attempt)
    return login, None, reason


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--max-users", type=int, default=0, help="For a quick read-only spot check; 0 scans all")
    args = parser.parse_args()
    rows = list(csv.DictReader(io.StringIO(command(
        "bq", "query", f"--project_id={PROJECT}", "--use_legacy_sql=false",
        "--max_rows=2000", "--format=csv", QUERY,
    ))))
    seats = json.loads(command("gh", "api", "enterprises/nav/copilot/billing/seats?per_page=100", "--paginate", "--slurp"))
    current_seats = {
        seat["assignee"]["login"].lower()
        for page in seats for seat in page["seats"] if seat.get("assignee", {}).get("login")
    }
    users = {row["login"]: row for row in rows if row["login"]}
    logins = sorted(users.keys() | current_seats)
    if args.max_users:
        logins = logins[:args.max_users]
    enterprise = json.loads(command(
        "gh", "api", "enterprises/nav/settings/billing/ai_credit/usage?year=2026&month=9",
        "-H", "X-GitHub-Api-Version: 2026-03-10",
    ))
    totals = defaultdict(lambda: defaultdict(Decimal))
    counts = defaultdict(int)
    failed = []
    failures = defaultdict(int)
    with ThreadPoolExecutor(max_workers=4) as pool:
        for processed, result in enumerate(as_completed([pool.submit(get_user, login) for login in logins]), 1):
            login, items, reason = result.result()
            if processed % 100 == 0:
                print(f"checked {processed}/{len(logins)}, failures {len(failed)}", file=sys.stderr, flush=True)
            if items is None:
                failed.append(login)
                failures[reason] += 1
                failures[users[login]["source"] if login in users else "current_seat_only"] += 1
                continue
            cohort = users[login]["source"] if login in users else "current_seat_only"
            counts[cohort] += 1
            if not items:
                counts["empty_reports"] += 1
            for item in items:
                key = f'{item["product"]} / {item["sku"]}'
                for field in ("grossAmount", "netAmount"):
                    totals[cohort][f"{key} / {field}"] += Decimal(str(item[field]))
    for item in enterprise["usageItems"]:
        key = f'{item["product"]} / {item["sku"]}'
        for field in ("grossAmount", "netAmount"):
            totals["enterprise"][f"{key} / {field}"] += Decimal(str(item[field]))
    user_totals = defaultdict(Decimal)
    for cohort, amounts in totals.items():
        if cohort != "enterprise":
            for key, value in amounts.items():
                user_totals[key] += value
    residual = {
        key: str(value - user_totals[key])
        for key, value in sorted(totals["enterprise"].items())
    }
    print(json.dumps({
        "month": MONTH, "request_count": len(logins), "failed_requests": len(failed),
        "failure_categories": dict(failures), "complete": not failed and not args.max_users,
        "source_counts": {
            "usage": sum(row["source"] == "usage" for row in rows),
            "team_only": sum(row["source"] == "team_only" for row in rows),
            "current_seats": len(current_seats), "current_seat_only": len(current_seats - users.keys()),
            "renamed_user_ids": sum(bool(row["usage_login"]) and row["usage_login"] != row["login"] for row in rows),
        }, "response_counts": dict(counts),
        "gross_from_user_metrics": str(sum(
            (Decimal(row["gross"]) for row in rows if row["gross"]), Decimal(0)
        )), "failed_user_metrics_gross": str(sum(
            (Decimal(users[login]["gross"]) for login in failed if login in users and users[login]["gross"]), Decimal(0)
        )), "enterprise_minus_fetched_users_usd": residual,
        "amounts_usd": {group: {key: str(value) for key, value in sorted(amounts.items())}
                           for group, amounts in sorted(totals.items())},
    }, indent=2))


if __name__ == "__main__":
    main()
