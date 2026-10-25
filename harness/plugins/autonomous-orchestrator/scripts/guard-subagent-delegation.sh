#!/usr/bin/env bash
# guard-subagent-delegation.sh: PreToolUse hook enforcing subagent delegation for project code edits (agentkit).
set -euo pipefail

# Support CLI test / dry-run mode
if [[ "${1:-}" == "--dry-run" ]]; then
  echo '{"status": "dry-run", "message": "guard-subagent-delegation.sh is operational"}'
  exit 0
elif [[ "${1:-}" == "--test" && -n "${2:-}" ]]; then
  INPUT_JSON="$2"
else
  INPUT_JSON="$(cat)"
fi

# Extract relevant fields from hook input payload
if command -v jq >/dev/null 2>&1; then
  TARGET_PATH="$(echo "$INPUT_JSON" | jq -r '.toolCall.args.Path // ""')"
  TRANSCRIPT_PATH="$(echo "$INPUT_JSON" | jq -r '.transcriptPath // ""')"
  ARTIFACT_DIR="$(echo "$INPUT_JSON" | jq -r '.artifactDirectoryPath // ""')"
  MODEL_NAME="$(echo "$INPUT_JSON" | jq -r '.modelName // ""')"
else
  TARGET_PATH="$(python3 -c 'import sys, json; print(json.load(sys.stdin).get("toolCall", {}).get("args", {}).get("Path", ""))' <<< "$INPUT_JSON" 2>/dev/null || true)"
  TRANSCRIPT_PATH="$(python3 -c 'import sys, json; print(json.load(sys.stdin).get("transcriptPath", ""))' <<< "$INPUT_JSON" 2>/dev/null || true)"
  ARTIFACT_DIR="$(python3 -c 'import sys, json; print(json.load(sys.stdin).get("artifactDirectoryPath", ""))' <<< "$INPUT_JSON" 2>/dev/null || true)"
  MODEL_NAME="$(python3 -c 'import sys, json; print(json.load(sys.stdin).get("modelName", ""))' <<< "$INPUT_JSON" 2>/dev/null || true)"
fi

# 1. Developer bypass override (supports AGY_ALLOW_ROOT_EDIT or AGENTKIT_ALLOW_ROOT_EDIT)
if [[ "${AGY_ALLOW_ROOT_EDIT:-0}" == "1" || "${AGENTKIT_ALLOW_ROOT_EDIT:-0}" == "1" ]]; then
  echo '{"decision": "allow", "reason": "Bypassed via AGY_ALLOW_ROOT_EDIT=1 or AGENTKIT_ALLOW_ROOT_EDIT=1"}'
  exit 0
fi

# 2. Allow if Path is empty, inside artifact directory, scratch, or .gemini configurations
if [[ -z "$TARGET_PATH" ]]; then
  echo '{"decision": "allow"}'
  exit 0
fi

if [[ "$TARGET_PATH" == *"/.gemini/"* ]] || \
   [[ "$TARGET_PATH" == *"/brain/"* ]] || \
   { [[ -n "$ARTIFACT_DIR" ]] && [[ "$TARGET_PATH" == "$ARTIFACT_DIR"* ]]; }; then
  echo '{"decision": "allow"}'
  exit 0
fi

# 3. Allow if running inside a Flash / Lite subagent
MODEL_LOWER="$(echo "$MODEL_NAME" | tr '[:upper:]' '[:lower:]')"
if [[ "$MODEL_LOWER" == *"flash"* ]] || [[ "$MODEL_LOWER" == *"lite"* ]]; then
  echo '{"decision": "allow"}'
  exit 0
fi

# 4. Check if conversation is a subagent via transcript inspection
IS_SUBAGENT=0
if [[ -n "$TRANSCRIPT_PATH" && -f "$TRANSCRIPT_PATH" ]]; then
  FIRST_INPUT=""
  if command -v jq >/dev/null 2>&1; then
    FIRST_INPUT="$(jq -r 'select(.type=="USER_INPUT") | .content // ""' "$TRANSCRIPT_PATH" 2>/dev/null | head -n 1 || true)"
  else
    FIRST_INPUT="$(python3 -c '
import sys, json
try:
    with open(sys.argv[1]) as f:
        for line in f:
            d = json.loads(line)
            if d.get("type") == "USER_INPUT":
                print(d.get("content", ""))
                break
except Exception:
    pass
' "$TRANSCRIPT_PATH" 2>/dev/null || true)"
  fi

  # Helper regex: detect subagent role indicators, worker identities, or agentkit subagent markers
  if echo "$FIRST_INPUT" | grep -Eiq '(You are|Worker|Subagent|pure-edit|Flash|<SUBAGENT|agentkit)'; then
    IS_SUBAGENT=1
  fi
fi

if [[ "$IS_SUBAGENT" -eq 1 ]]; then
  echo '{"decision": "allow"}'
  exit 0
fi

# 5. Root Orchestrator Gatekeeping (switchable root orchestrators: Argon 4, Opus 5.5, Sonnet 3.7, Pro)
# Strict deny mode
if [[ "${AGY_STRICT_SUBAGENT_DENY:-0}" == "1" || "${AGENTKIT_STRICT_SUBAGENT_DENY:-0}" == "1" ]]; then
  echo '{"decision": "deny", "reason": "ORCHESTRATOR VIOLATION: Root session (Argon 4 / Opus 5.5 / Sonnet 3.7) must not call edit_file/write_file on project files directly. Delegate implementation to a Flash subagent via invoke_subagent(Model=\"flash\")."}'
  exit 0
fi

# Check if invoke_subagent was called since latest USER_INPUT
HAS_SUBAGENT_SINCE_INPUT=0
if [[ -n "$TRANSCRIPT_PATH" && -f "$TRANSCRIPT_PATH" ]]; then
  if command -v python3 >/dev/null 2>&1; then
    HAS_SUBAGENT_SINCE_INPUT="$(python3 -c '
import sys, json
has_subagent = False
try:
    with open(sys.argv[1]) as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                step = json.loads(line)
                step_type = step.get("type", "")
                if step_type == "USER_INPUT":
                    has_subagent = False
                elif "invoke_subagent" in line:
                    has_subagent = True
            except Exception:
                pass
    print("1" if has_subagent else "0")
except Exception:
    print("0")
' "$TRANSCRIPT_PATH" 2>/dev/null || echo "0")"
  else
    HAS_SUBAGENT_SINCE_INPUT="$(awk '
      /USER_INPUT/ { subagent=0 }
      /invoke_subagent/ { subagent=1 }
      END { print (subagent ? 1 : 0) }
    ' "$TRANSCRIPT_PATH" 2>/dev/null || echo "0")"
  fi
fi

if [[ "$HAS_SUBAGENT_SINCE_INPUT" -eq 0 ]]; then
  echo '{"decision": "deny", "reason": "ORCHESTRATOR VIOLATION: Root session (Argon 4 / Opus 5.5 / Sonnet 3.7) must not call edit_file/write_file on project files directly. Delegate implementation to a Flash subagent via invoke_subagent(Model=\"flash\")."}'
  exit 0
fi

# If a subagent was dispatched or permitted, allow
echo '{"decision": "allow"}'
exit 0
