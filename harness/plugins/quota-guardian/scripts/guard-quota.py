#!/usr/bin/env python3
"""guard-quota.py: PreToolUse hook for invoke_subagent enforcing quota monitoring and complexity routing."""

import json
import os
import re
import sys
from pathlib import Path
from typing import Any, Dict, Optional

SCRIPT_DIR = Path(__file__).resolve().parent
if str(SCRIPT_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIR))

from quota_status import _resolve_group_id, fetch_raw, load_config, summarize


def _touches_protected_scope(prompt: str) -> bool:
    """Return True if prompt indicates testing, refactoring, debugging, or security work."""
    return bool(re.search(r"\b(test|refactor|debug|security)\b", prompt, re.IGNORECASE))


def _resolve_model_group(model_name: str, config: Dict[str, Any]) -> Optional[str]:
    """Map a model identifier or name to its canonical quota group."""
    if not model_name or not isinstance(model_name, str):
        return None
    m_lower = model_name.strip().lower()
    if not m_lower or m_lower == "inherit":
        return None

    model_group_map = {
        "flash_lite": "gemini",
        "flash": "gemini",
        "pro": "argon",
    }
    if m_lower in model_group_map:
        return model_group_map[m_lower]

    if "argon" in m_lower:
        return "argon"
    if "claude" in m_lower or "opus" in m_lower or "sonnet" in m_lower:
        return "claude"
    if "gemini" in m_lower or "flash" in m_lower:
        return "gemini"

    return _resolve_group_id(m_lower, config)


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

    # 4. Load quota config and evaluate current health
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

    # 5. Inspect prompt for [complexity: low|med|high] tag and resolve target model
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
            argon_rem = groups.get("argon", {}).get("remainingFraction", 1.0)
            if argon_rem > switch_thresh:
                target_model = "pro"
            else:
                target_model = "flash"

    # Safety guardrail: upgrade low / flash_lite to flash if prompt indicates heavy engineering work
    if target_model == "flash_lite" or (complexity == "low" and model_was_default):
        if _touches_protected_scope(prompt):
            target_model = "flash"

    # 6. Map target model to group & resolve root model group
    model_group_map = {
        "flash_lite": "gemini",
        "flash": "gemini",
        "pro": "argon",
    }
    target_group = model_group_map.get(target_model)
    if not target_group and target_model != "inherit":
        target_group = _resolve_model_group(target_model, config)

    # Determine root_group from payload or orchestration config
    root_group = None
    root_model_input = (
        payload.get("modelName")
        or payload.get("rootModel")
        or payload.get("session", {}).get("model")
        or ""
    )
    if root_model_input:
        root_group = _resolve_model_group(str(root_model_input), config)

    if root_group is None:
        orch_path = Path.home() / ".gemini" / "config" / "orchestration.json"
        if orch_path.is_file():
            try:
                orch_cfg = json.loads(orch_path.read_text(encoding="utf-8"))
                root_model = (
                    orch_cfg.get("model")
                    or orch_cfg.get("root_model")
                    or orch_cfg.get("active_profile")
                    or ""
                )
                if root_model:
                    root_group = _resolve_model_group(str(root_model), config)
            except Exception:
                pass

    if target_model == "inherit" and root_group:
        target_group = root_group

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
        fallback_model = None
        fallback_group = None

        for cand in fb_candidates:
            if cand == "inherit":
                if root_group is not None:
                    if root_group != target_group:
                        cand_rem = groups.get(root_group, {}).get("remainingFraction", 1.0)
                        if cand_rem > switch_thresh:
                            fallback_model = "inherit"
                            fallback_group = root_group
                            break
                    else:
                        # root_group == target_group: inherit is in the SAME degraded group
                        cand_rem = rem_frac
                else:
                    # root_group is None: check if any OTHER group in groups has remainingFraction > switch_threshold
                    other_healthy_group = None
                    for g_name, g_data in groups.items():
                        if g_name != target_group and g_data.get("remainingFraction", 1.0) > switch_thresh:
                            other_healthy_group = g_name
                            break
                    if other_healthy_group:
                        fallback_model = "inherit"
                        fallback_group = other_healthy_group
                        break
            else:
                cand_group = model_group_map.get(cand) or _resolve_model_group(cand, config)
                cand_rem = groups.get(cand_group, {}).get("remainingFraction", 1.0) if cand_group else 1.0
                if cand_rem > switch_thresh:
                    fallback_model = cand
                    fallback_group = cand_group
                    break

        if fallback_model is None:
            # Check other healthy groups if fb_candidates did not yield a healthy model
            argon_rem = groups.get("argon", {}).get("remainingFraction", 1.0)
            gemini_rem = groups.get("gemini", {}).get("remainingFraction", 1.0)
            claude_rem = groups.get("claude", {}).get("remainingFraction", 1.0)

            if target_group != "argon" and argon_rem > switch_thresh:
                fallback_model = "pro"
                fallback_group = "argon"
            elif target_group != "gemini" and gemini_rem > switch_thresh:
                fallback_model = "flash"
                fallback_group = "gemini"
            elif claude_rem > switch_thresh:
                fallback_model = "inherit"
                fallback_group = "claude"
            else:
                fallback_model = "inherit"
                fallback_group = None

        pct = int(round(group_info.get("remainingFraction", 0.0) * 100))
        reset = group_info.get("resetTime") or "unknown"
        group_desc = f" ({fallback_group} group)" if fallback_group else ""
        reason = (
            f"Quota degraded for {target_group} ({pct}% remaining, resets {reset}). "
            f"Rerouted subagent to {fallback_model}{group_desc}."
        )
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
