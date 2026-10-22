#!/usr/bin/env bash
# guard-git-operations.sh: PreToolUse hook to intercept dangerous git commands and destructive filesystem operations.
set -euo pipefail

# Support CLI test / dry-run mode
if [[ "${1:-}" == "--test" && -n "${2:-}" ]]; then
  CMD="$2"
elif [[ "${1:-}" == "--dry-run" ]]; then
  echo '{"status": "dry-run", "message": "guard-git-operations.sh is operational"}'
  exit 0
else
  # Read full JSON input from stdin
  INPUT_JSON="$(cat)"
  if command -v jq >/dev/null 2>&1; then
    CMD="$(echo "$INPUT_JSON" | jq -r '.toolCall.args.CommandLine // ""')"
  else
    CMD="$(python3 -c 'import sys, json; data=json.load(sys.stdin); print(data.get("toolCall", {}).get("args", {}).get("CommandLine", ""))' <<< "$INPUT_JSON")"
  fi
fi

# 1. Block direct push to main / master or force pushes
if echo "$CMD" | grep -Eq 'git\s+(push\s+.*(main|master)|push\s+.*(-f\b|--force\b)|merge\s+(main|master)|checkout\s+(main|master)\s*(&&|;)\s*git\s+merge|switch\s+(main|master)\s*(&&|;)\s*git\s+merge)'; then
  echo '{"decision": "deny", "reason": "SECURITY VIOLATION: Direct pushes, force-pushes, or trunk merges to main/master are strictly blocked by enterprise policy. All changes must be pushed to an isolated feature branch (feat/*, fix/*) and submitted via Pull Request."}'
  exit 0
fi

# 2. Gate destructive file removals and raw disk operations
if echo "$CMD" | grep -Eq '(\brm\s+-[a-zA-Z]*r[a-zA-Z]*f|\brm\s+-[a-zA-Z]*f[a-zA-Z]*r|\bdd\s+if=|\bmkfs\b)'; then
  echo '{"decision": "ask", "reason": "DESTRUCTIVE OPERATION DETECTED: This command performs irreversible recursive deletion or raw disk mutation. Explicit user approval is required."}'
  exit 0
fi

# 3. Default: allow tool execution
echo '{"decision": "allow"}'
exit 0
