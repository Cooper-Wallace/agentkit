#!/usr/bin/env python3
"""quota_status.py: Query language server quota summary and route complexity tiers."""

import argparse
import json
import os
import sys
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any, Dict, List, Optional

SCRIPT_DIR = Path(__file__).resolve().parent
PLUGIN_DIR = SCRIPT_DIR.parent
REPO_ROOT = PLUGIN_DIR.parent.parent
DEFAULT_CONFIG_PATH = REPO_ROOT / "config" / "quota.json"


def load_config() -> Dict[str, Any]:
    """Load quota configuration from disk or fall back to defaults."""
    cfg_env = os.environ.get("AGENTKIT_QUOTA_CONFIG")
    cfg_path = Path(cfg_env).expanduser().resolve() if cfg_env else DEFAULT_CONFIG_PATH
    if cfg_path.is_file():
        try:
            with open(cfg_path, "r", encoding="utf-8") as f:
                return json.load(f)
        except Exception:
            pass

    return {
        "thresholds": {
            "switch_threshold": 0.15,
            "wrapup_threshold": 0.05,
            "cache_ttl_seconds": 60,
        },
        "groups": {
            "gemini": "gemini",
            "claude": "claude",
            "argon": "argon",
        },
        "complexity_to_model": {
            "low": {
                "model": "flash_lite",
                "group": "gemini",
                "use": "file moves, renames, mechanical find-replace, formatting, doc typos, simple one-command inspection",
            },
            "med": {
                "model": "flash",
                "group": "gemini",
                "use": "feature code, tests, localized refactors",
            },
            "high": {
                "model": "flash",
                "escalate_to": "pro",
                "group": "gemini",
                "escalate_group": "argon",
                "use": "cross-module refactors, debugging, security, architecture; pro only for review/architecture/debugging when Argon quota healthy",
            },
        },
        "fallbacks": {
            "flash_lite": ["inherit"],
            "flash": ["inherit"],
            "pro": ["flash", "inherit"],
            "inherit": [],
        },
    }


def get_cache_path() -> Path:
    """Return local disk cache file path."""
    custom_cache = os.environ.get("AGENTKIT_QUOTA_CACHE")
    if custom_cache:
        return Path(custom_cache).expanduser().resolve()
    return Path.home() / ".cache" / "agentkit" / "quota.json"


def get_synthetic_healthy_payload() -> Dict[str, Any]:
    """Return synthetic healthy payload to fail open on network/server errors."""
    return {
        "groups": [
            {
                "displayName": "gemini",
                "buckets": [
                    {
                        "bucketId": "gemini-default",
                        "displayName": "Gemini Models",
                        "window": "24h",
                        "remainingFraction": 1.0,
                        "disabled": False,
                        "resetTime": "unknown",
                    }
                ],
            },
            {
                "displayName": "claude",
                "buckets": [
                    {
                        "bucketId": "claude-default",
                        "displayName": "Claude Models",
                        "window": "24h",
                        "remainingFraction": 1.0,
                        "disabled": False,
                        "resetTime": "unknown",
                    }
                ],
            },
            {
                "displayName": "argon",
                "buckets": [
                    {
                        "bucketId": "argon-default",
                        "displayName": "Argon Models",
                        "window": "24h",
                        "remainingFraction": 1.0,
                        "disabled": False,
                        "resetTime": "unknown",
                    }
                ],
            },
        ]
    }


def fetch_raw() -> Dict[str, Any]:
    """Fetch raw quota summary payload from fixture, language server, or cache."""
    # 1. Check offline test fixture
    fixture = os.environ.get("AGENTKIT_QUOTA_FIXTURE")
    if fixture and Path(fixture).is_file():
        try:
            with open(fixture, "r", encoding="utf-8") as f:
                return json.load(f)
        except Exception:
            pass

    # 2. Query language server if environment variables are set
    ls_address = os.environ.get("ANTIGRAVITY_LS_ADDRESS")
    csrf_token = os.environ.get("ANTIGRAVITY_CSRF_TOKEN")

    if ls_address and csrf_token:
        url = f"http://{ls_address}/exa.language_server_pb.LanguageServerService/RetrieveUserQuotaSummary"
        headers = {
            "x-codeium-csrf-token": csrf_token,
            "Content-Type": "application/json",
        }
        req = urllib.request.Request(url, data=b"{}", headers=headers, method="POST")
        try:
            with urllib.request.urlopen(req, timeout=3.0) as resp:
                raw_bytes = resp.read()
                data = json.loads(raw_bytes.decode("utf-8"))
                # Cache response
                cache_file = get_cache_path()
                try:
                    cache_file.parent.mkdir(parents=True, exist_ok=True)
                    with open(cache_file, "w", encoding="utf-8") as cf:
                        json.dump(data, cf)
                except Exception:
                    pass
                return data
        except Exception:
            # Fail open to cache
            pass

    # 3. Fall back to cached data
    cache_file = get_cache_path()
    if cache_file.is_file():
        try:
            with open(cache_file, "r", encoding="utf-8") as cf:
                return json.load(cf)
        except Exception:
            pass

    # 4. Fail open to synthetic healthy payload
    return get_synthetic_healthy_payload()


def _resolve_group_id(name: str, config: Dict[str, Any]) -> Optional[str]:
    """Match a group display name or keyword to a canonical group ID."""
    name_lower = name.lower()
    groups_cfg = config.get("groups", {})

    for key, val in groups_cfg.items():
        if isinstance(val, str):
            if val.lower() in name_lower or key.lower() in name_lower:
                return key.lower()
        elif isinstance(val, list):
            for pattern in val:
                if str(pattern).lower() in name_lower:
                    return key.lower()

    if "argon" in name_lower:
        return "argon"
    if "claude" in name_lower:
        return "claude"
    if "gemini" in name_lower or "flash" in name_lower:
        return "gemini"
    return None


def summarize(raw_data: Dict[str, Any], config: Dict[str, Any]) -> Dict[str, Any]:
    """Calculate remainingFraction, evaluate exhaustion/critical/low status per group."""
    thresholds = config.get("thresholds", {})
    switch_thresh = thresholds.get("switch_threshold", 0.15)
    wrapup_thresh = thresholds.get("wrapup_threshold", 0.05)

    groups_data = raw_data.get("groups", [])
    if not groups_data and "buckets" in raw_data:
        groups_data = [{"displayName": "default", "buckets": raw_data.get("buckets", [])}]

    # Canonical buckets grouped by target ID
    grouped_buckets: Dict[str, List[Dict[str, Any]]] = {
        "gemini": [],
        "claude": [],
        "argon": [],
    }

    for g in groups_data:
        disp_name = g.get("displayName") or g.get("name") or ""
        gid = _resolve_group_id(disp_name, config) or "gemini"
        if gid not in grouped_buckets:
            grouped_buckets[gid] = []

        for b in g.get("buckets", []):
            if b.get("disabled", False):
                continue
            grouped_buckets[gid].append(b)

    summary_groups: Dict[str, Any] = {}
    earliest_resets: List[str] = []

    for gid, buckets in grouped_buckets.items():
        if not buckets:
            rem_frac = 1.0
            window = "24h"
            reset_time = "unknown"
        else:
            fractions = []
            resets = []
            windows = []
            for b in buckets:
                rf = b.get("remainingFraction")
                if rf is None:
                    rf = b.get("remaining")
                if rf is None:
                    rf = b.get("remaining_fraction", 1.0)
                try:
                    rf = float(rf)
                except (ValueError, TypeError):
                    rf = 1.0
                fractions.append(rf)

                rt = b.get("resetTime") or b.get("reset_time") or ""
                if rt and rt != "unknown":
                    resets.append(str(rt))

                w = b.get("window") or b.get("windowMinutes") or ""
                if w:
                    windows.append(str(w))

            rem_frac = min(fractions) if fractions else 1.0
            reset_time = sorted(resets)[0] if resets else "unknown"
            window = windows[0] if windows else "24h"

        if reset_time != "unknown":
            earliest_resets.append(reset_time)

        is_exhausted = rem_frac <= 0.0
        is_critical = rem_frac <= wrapup_thresh
        is_low = rem_frac <= switch_thresh

        if is_exhausted:
            status = "exhausted"
        elif is_critical:
            status = "critical"
        elif is_low:
            status = "low"
        else:
            status = "healthy"

        summary_groups[gid] = {
            "name": gid,
            "remainingFraction": rem_frac,
            "remainingPercent": round(rem_frac * 100, 1),
            "window": window,
            "resetTime": reset_time,
            "status": status,
            "exhausted": is_exhausted,
            "critical": is_critical,
            "low": is_low,
        }

    all_critical = all(g["critical"] for g in summary_groups.values()) if summary_groups else False
    earliest_reset = sorted(earliest_resets)[0] if earliest_resets else "unknown"

    return {
        "groups": summary_groups,
        "earliest_reset": earliest_reset,
        "all_critical": all_critical,
        "wrapup_needed": all_critical,
    }


def route(
    complexity: str,
    summary: Dict[str, Any],
    config: Dict[str, Any],
    task_type: Optional[str] = None,
) -> Dict[str, Any]:
    """Route complexity tiers to optimal model based on current quota health."""
    c_tier = config.get("complexity_to_model", {}).get(complexity)
    thresholds = config.get("thresholds", {})
    switch_thresh = thresholds.get("switch_threshold", 0.15)
    fallbacks = config.get("fallbacks", {})

    groups = summary.get("groups", {})
    gemini_rem = groups.get("gemini", {}).get("remainingFraction", 1.0)
    argon_rem = groups.get("argon", {}).get("remainingFraction", 1.0)
    claude_rem = groups.get("claude", {}).get("remainingFraction", 1.0)

    gemini_healthy = gemini_rem > switch_thresh
    argon_healthy = argon_rem > switch_thresh
    claude_healthy = claude_rem > switch_thresh

    comp_lower = complexity.lower()

    if comp_lower == "low":
        model = c_tier.get("model", "flash_lite") if c_tier else "flash_lite"
        if gemini_healthy:
            return {
                "model": model,
                "group": "gemini",
                "reason": f"Low complexity routed to {model}; Gemini quota healthy ({round(gemini_rem * 100, 1)}%)",
            }
        fb_list = fallbacks.get(model, ["inherit"])
        fb_model = fb_list[0] if fb_list else "inherit"
        return {
            "model": fb_model,
            "group": "inherit",
            "reason": f"Gemini quota degraded ({round(gemini_rem * 100, 1)}%); low complexity falling back to {fb_model}",
        }

    if comp_lower == "med":
        model = c_tier.get("model", "flash") if c_tier else "flash"
        if gemini_healthy:
            return {
                "model": model,
                "group": "gemini",
                "reason": f"Medium complexity routed to {model}; Gemini quota healthy ({round(gemini_rem * 100, 1)}%)",
            }
        fb_list = fallbacks.get(model, ["inherit"])
        fb_model = fb_list[0] if fb_list else "inherit"
        return {
            "model": fb_model,
            "group": "inherit",
            "reason": f"Gemini quota degraded ({round(gemini_rem * 100, 1)}%); medium complexity falling back to {fb_model}",
        }

    # High complexity tier
    base_model = c_tier.get("model", "flash") if c_tier else "flash"
    escalate_to = c_tier.get("escalate_to", "pro") if c_tier else "pro"

    # Route to pro when Argon quota healthy
    if argon_healthy:
        return {
            "model": escalate_to,
            "group": "argon",
            "reason": f"High complexity escalated to {escalate_to}; Argon quota healthy ({round(argon_rem * 100, 1)}%)",
        }

    if gemini_healthy:
        return {
            "model": base_model,
            "group": "gemini",
            "reason": f"Argon quota degraded ({round(argon_rem * 100, 1)}%); high complexity routed to {base_model}",
        }

    # Both degraded -> fallback to inherit
    return {
        "model": "inherit",
        "group": "inherit",
        "reason": "Argon and Gemini quotas degraded; high complexity falling back to inherit",
    }


def print_status_table(summary: Dict[str, Any]) -> None:
    """Print human-readable tabular representation of quota summary."""
    groups = summary.get("groups", {})
    headers = ["Group", "Remaining %", "Window", "Reset Time", "Status"]
    rows = []
    for gid in ["gemini", "claude", "argon"]:
        g = groups.get(gid)
        if not g:
            continue
        pct_str = f"{g['remainingPercent']}%"
        rows.append([gid, pct_str, g["window"], g["resetTime"], g["status"]])

    col_widths = [len(h) for h in headers]
    for row in rows:
        for idx, cell in enumerate(row):
            col_widths[idx] = max(col_widths[idx], len(str(cell)))

    header_fmt = "  ".join(f"{{:<{w}}}" for w in col_widths)
    separator = "  ".join("-" * w for w in col_widths)
    row_fmt = "  ".join(f"{{:<{w}}}" for w in col_widths)

    print(header_fmt.format(*headers))
    print(separator)
    for row in rows:
        print(row_fmt.format(*row))


def main() -> int:
    if "--dry-run" in sys.argv:
        print(json.dumps({"status": "dry-run", "message": "quota_status.py is operational"}))
        return 0

    parser = argparse.ArgumentParser(description="Quota status and complexity routing.")
    subparsers = parser.add_subparsers(dest="command")

    status_parser = subparsers.add_parser("status", help="Show current quota status summary.")
    status_parser.add_argument("--json", action="store_true", help="Output summary in JSON format.")

    route_parser = subparsers.add_parser("route", help="Route task complexity to optimal model.")
    route_parser.add_argument(
        "--complexity",
        choices=["low", "med", "high"],
        required=True,
        help="Task complexity tier.",
    )
    route_parser.add_argument("--task-type", help="Optional task type descriptor.")
    route_parser.add_argument("--json", action="store_true", help="Output route decision in JSON format.")

    args = parser.parse_args()

    config = load_config()
    raw_data = fetch_raw()
    summary = summarize(raw_data, config)

    if args.command == "status" or args.command is None:
        if getattr(args, "json", False):
            print(json.dumps(summary, indent=2))
        else:
            print_status_table(summary)
        return 0

    if args.command == "route":
        decision = route(args.complexity, summary, config, task_type=getattr(args, "task_type", None))
        if args.json:
            print(json.dumps(decision, indent=2))
        else:
            print(f"Model:   {decision['model']}")
            print(f"Group:   {decision['group']}")
            print(f"Reason:  {decision['reason']}")
        return 0

    return 1


if __name__ == "__main__":
    sys.exit(main())
