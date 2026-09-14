"""Seed pfm's own usage cache with MOCK limits for the README recording.

Writes the exact records the Limits tab reads (usagehook CacheRecord for
Claude acct-N.json, stats codexCacheRecord for codex-N.json), stamped fresh so
the sampler serves them from disk without a network call. Every number is
invented; no credential is read or written.
"""
import json
import os
import time
from datetime import datetime, timezone

home = os.path.expanduser("~")
cache = f"/tmp/cc-usage-{os.getuid()}"
os.makedirs(cache, mode=0o700, exist_ok=True)
now = int(time.time())
iso = lambda t: datetime.fromtimestamp(t, timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
stamp = datetime.fromtimestamp(now, timezone.utc).isoformat()

claude = [
    # id, config dir, 5h used, 5h reset (s), 7d used, 7d reset (s), fable used
    (1, f"{home}/.claude", 34, 2 * 3600 + 610, 61, 3 * 86400 + 4 * 3600, 22),
    (2, f"{home}/.cc/2", 8, 4 * 3600 + 1500, 27, 5 * 86400 + 7 * 3600, 9),
]
for acct, cfg, h5, h5r, d7, d7r, fable in claude:
    record = {
        "five_hour": {"utilization": float(h5), "resets_at": iso(now + h5r)},
        "seven_day": {"utilization": float(d7), "resets_at": iso(now + d7r)},
        "seven_day_opus": {"utilization": None, "resets_at": ""},
        "limits": [{
            "kind": "weekly_scoped",
            "scope": {"model": {"display_name": "Fable"}},
            "percent": float(fable), "resets_at": iso(now + d7r), "is_active": True,
        }],
        "config_dir": cfg,
        "fetched_at": stamp,
    }
    with open(f"{cache}/acct-{acct}.json", "w") as f:
        json.dump(record, f)

codex = [
    # id, home, plan, 5h used, 5h reset (s), weekly used, weekly reset (s)
    (1, f"{home}/.codex", "pro", 41, 3 * 3600 + 900, 29, 3 * 86400 + 13 * 3600),
    (2, f"{home}/.codex2", "plus", 12, 1 * 3600 + 2400, 55, 6 * 86400 + 2 * 3600),
]
for acct, chome, plan, h5, h5r, wk, wkr in codex:
    record = {
        "plan_type": plan,
        "rate_limit": {
            "primary_window": {"used_percent": float(h5), "reset_at": now + h5r, "limit_window_seconds": 18000},
            "secondary_window": {"used_percent": float(wk), "reset_at": now + wkr, "limit_window_seconds": 604800},
        },
        "source_version": 2,
        "codex_auth_path": f"{chome}/auth.json",
        "fetched_at": stamp,
    }
    with open(f"{cache}/codex-{acct}.json", "w") as f:
        json.dump(record, f)
print(f"seeded {len(claude)} claude + {len(codex)} codex mock usage records in {cache}")
