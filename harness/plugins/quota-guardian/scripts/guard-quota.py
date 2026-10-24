#!/usr/bin/env python3
"""guard-quota.py: PreToolUse hook for invoke_subagent enforcing quota monitoring and complexity routing."""

import json
import os
import re
import sys
from pathlib import Path
from typing import Any, Dict

SCRIPT_DIR = Path(__file__).resolve().parent
if str(SCRIPT_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIR))

from quota_status import fetch_raw, load_config, summarize


def main() -> int:
    # 1. Support --dry-run
    if "--dry-run" in sys.argv:
        print(json.dumps({"status": "dry-run", "message": "guard-quota.py is operational"}))
        return 0

    # 2. Check bypass environment variable
    if os.environ.get("AGENTKIT_QUOTA_GUARD") in ("off", "0"):
        print(json.dumps({"decision": "allow", "reason": "Bypassed via AGENTKIT_QUOTA_GUARD"}))
        return 0

    # 3. Read input payload
    input_json = ""
    if len(sys.argv) > 1 and sys.argv[1] == "--test":
        input_json = sys.argv[2] if len(sys.argv) > 2 else "{}"
    else:
        try:
            input_json = sys.stdin.read()
        except Exception:
            input_json = "{}"

    try:
        payload = json.loads(input_json) if input_json.strip() else {}
    except Exception:
        payload = {}

    tool_call = payload.get("toolCall", {})
    args = tool_call.get("args", {})
    if not args and ("Model" in payload or "Prompt" in payload):
        args = payload

    original_model = args.get("Model")
    prompt = args.get("Prompt", "")

    # 4. Inspect prompt for [complexity: low|med|high] tag
    complexity_match = re.search(r"\[complexity:\s*(low|med|high)\]", prompt, re.IGNORECASE)
    complexity = complexity_match.group(1).lower() if complexity_match else None

    # Model resolution
    model_was_default = original_model is None or original_model == "" or original_model == "inherit"
    target_model = original_model if original_model else "inherit"

    if model_was_default and complexity:
        if complexity == "low":
            target_model = "flash_lite"
        elif complexity == "med":
            target_model = "flash"
        elif complexity == "high":
            target_model = "flash"

    # Safety guardrail: upgrade low / flash_lite to flash if prompt indicates heavy engineering work
    if target_model == "flash_lite" or (complexity == "low" and model_was_default):
        if re.search(r"\b(test|refactor|debug|security)\b", prompt, re.IGNORECASE):
            target_model = "flash"

    # 5. Load quota config and evaluate current health
    config = load_config()
    raw_data = fetch_raw()
    summary = summarize(raw_data, config)

    thresholds = config.get("thresholds", {})
    wrapup_thresh = thresholds.get("wrapup_threshold", 0.05)
    switch_thresh = thresholds.get("switch_threshold", 0.15)
    groups = summary.get("groups", {})

    # Check if all groups are critically low
    all_groups_critical = bool(groups) and all(
        g.get("remainingFraction", 1.0) <= wrapup_thresh for g in groups.values()
    )

    if all_groups_critical:
        reason = (
            "QUOTA WRAP-UP: All model quotas are critically low. Stop delegating new subagents. "
            "Follow the quota-wrapup skill: run test suite, commit WIP to feature branch, "
            "write handoff.md, and notify user."
        )
        print(json.dumps({"decision": "deny", "reason": reason}))
        return 0

    # 6. Map target model to group
    model_group_map = {
        "flash_lite": "gemini",
        "flash": "gemini",
        "pro": "argon",
    }
    target_group = model_group_map.get(target_model)
    if not target_group and target_model != "inherit":
        m_lower = target_model.lower()
        if "claude" in m_lower or "opus" in m_lower or "sonnet" in m_lower:
            target_group = "claude"

    group_info = groups.get(target_group, {}) if target_group else None
    is_degraded = False
    if group_info:
        rem_frac = group_info.get("remainingFraction", 1.0)
        if rem_frac <= switch_thresh:
            is_degraded = True

    # 7. Quota degradation fallback routing
    if is_degraded:
        fallbacks = config.get("fallbacks", {})
        fb_candidates = fallbacks.get(target_model, ["inherit"])
        fallback_model = "inherit"
        for cand in fb_candidates:
            cand_group = model_group_map.get(cand)
            cand_rem = groups.get(cand_group, {}).get("remainingFraction", 1.0) if cand_group else 1.0
            if cand_rem > switch_thresh:
                fallback_model = cand
                break

        pct = int(round(group_info.get("remainingFraction", 0.0) * 100))
        reset = group_info.get("resetTime") or "unknown"
        reason = f"Quota degraded for {target_group} ({pct}% remaining, resets {reset}). Rerouted subagent to {fallback_model}."
        print(
            json.dumps(
                {
                    "decision": "allow",
                    "overwrite": {"Model": fallback_model},
                    "reason": reason,
                }
            )
        )
        return 0

    # 8. Overwrite Model if mapped by complexity or safety upgrade
    if target_model != (original_model or "inherit"):
        print(
            json.dumps(
                {
                    "decision": "allow",
                    "overwrite": {"Model": target_model},
                }
            )
        )
        return 0

    # 9. Healthy execution
    print(json.dumps({"decision": "allow"}))
    return 0


if __name__ == "__main__":
    sys.exit(main())
