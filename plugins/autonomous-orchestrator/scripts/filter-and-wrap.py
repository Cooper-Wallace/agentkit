#!/usr/bin/env python3
"""PreToolUse hook for run_command: intercepts verbose commands for Headroom token compression while preserving security gates."""

import json
import os
import re
import sys
from pathlib import Path

VERBOSE_PATTERNS = [
    r"\bpytest\b",
    r"\bunittest\b",
    r"\bnpm\s+test\b",
    r"\bpnpm\s+test\b",
    r"\byarn\s+test\b",
    r"\bcargo\s+test\b",
    r"\bgo\s+test\b",
    r"\bmvn\s+test\b",
    r"\bgit\s+.*?\b(diff|log|show)\b",
    r"\bfind\s+",
    r"\bgrep\s+",
    r"\bcat\s+",
    r"\bdiff\s+",
    r"\bdpkg\s+-l\b",
    r"\bapt-cache\b",
    r"\btree\b",
]


def is_verbose_command(cmd: str) -> bool:
    """Check if command matches verbose inspection or test patterns."""
    for pattern in VERBOSE_PATTERNS:
        if re.search(pattern, cmd):
            return True
    return False


def main() -> None:
    # 1. Support --dry-run
    if len(sys.argv) > 1 and sys.argv[1] == "--dry-run":
        print(json.dumps({"status": "dry-run", "message": "filter-and-wrap.py is operational"}))
        sys.exit(0)

    # 2. Support --test <cmd> or read JSON from stdin
    if len(sys.argv) > 1 and sys.argv[1] == "--test":
        cmd_arg = sys.argv[2] if len(sys.argv) > 2 else ""
        payload = {
            "toolCall": {
                "name": "run_command",
                "args": {
                    "CommandLine": cmd_arg
                }
            }
        }
    else:
        try:
            raw_input = sys.stdin.read()
            payload = json.loads(raw_input) if raw_input.strip() else {}
        except Exception:
            payload = {}

    # 3. Disable toggle override
    if os.environ.get("AGY_DISABLE_TOKEN_COMPRESSION") == "1":
        print(json.dumps({"decision": "allow"}))
        sys.exit(0)

    # 4. Extract fields from toolCall payload
    tool_call = payload.get("toolCall", {}) if isinstance(payload, dict) else {}
    tool_name = tool_call.get("name", "") if isinstance(tool_call, dict) else ""
    args = tool_call.get("args", {}) if isinstance(tool_call, dict) else {}
    command_line = args.get("CommandLine", "").strip() if isinstance(args, dict) else ""
    is_daemon = bool(args.get("IsDaemon", False)) if isinstance(args, dict) else False

    # 5. Safety Preservation (never downgrade guard-git-operations.sh)
    if command_line:
        git_security_re = re.compile(
            r"git\s+(push\s+.*(main|master)|push\s+.*(-f\b|--force\b)|merge\s+(main|master)|checkout\s+(main|master)\s*(&&|;)\s*git\s+merge|switch\s+(main|master)\s*(&&|;)\s*git\s+merge)"
        )
        if git_security_re.search(command_line):
            print(json.dumps({
                "decision": "deny",
                "reason": "SECURITY VIOLATION: Direct pushes, force-pushes, or trunk merges to main/master are strictly blocked."
            }))
            sys.exit(0)

        destructive_re = re.compile(
            r"(\brm\s+-[a-zA-Z]*r[a-zA-Z]*f|\brm\s+-[a-zA-Z]*f[a-zA-Z]*r|\bdd\s+if=|\bmkfs\b)"
        )
        if destructive_re.search(command_line):
            print(json.dumps({
                "decision": "ask",
                "reason": "DESTRUCTIVE OPERATION DETECTED: Explicit user approval is required."
            }))
            sys.exit(0)

    # 6. Skip conditions (return {"decision": "allow"} without overwrite)
    if tool_name != "run_command" or not command_line:
        print(json.dumps({"decision": "allow"}))
        sys.exit(0)

    if is_daemon:
        print(json.dumps({"decision": "allow"}))
        sys.exit(0)

    if "headroom-wrap.py" in command_line:
        print(json.dumps({"decision": "allow"}))
        sys.exit(0)

    # Skip any shell redirection/pipe/chain operator (|, >, >>, <<, &&, ;)
    if re.search(r"(\||\>\>|\>|\<\<|\&\&|\;)", command_line):
        print(json.dumps({"decision": "allow"}))
        sys.exit(0)

    # Skip git mutation subcommands
    git_mutation_re = re.compile(
        r"\bgit\s+.*?\b(commit|push|checkout|switch|add|merge|rebase|stash|init|clone)\b"
    )
    if git_mutation_re.search(command_line):
        print(json.dumps({"decision": "allow"}))
        sys.exit(0)

    # 7. Check verbose inspection/test patterns
    if is_verbose_command(command_line):
        wrapper_path = Path(__file__).resolve().parent / "headroom-wrap.py"
        print(json.dumps({
            "decision": "allow",
            "overwrite": {
                "CommandLine": f"python3 {wrapper_path} -- {command_line}"
            }
        }))
        sys.exit(0)

    # Default allow
    print(json.dumps({"decision": "allow"}))
    sys.exit(0)


if __name__ == "__main__":
    main()
