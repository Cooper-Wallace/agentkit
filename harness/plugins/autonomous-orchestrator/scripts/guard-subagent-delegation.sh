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
  TARGET_PATH="$(echo "$INPUT_JSON" | jq -r '.toolCall.args.Path // ""' 2>/dev/null || true)"
  TRANSCRIPT_PATH="$(echo "$INPUT_JSON" | jq -r '.transcriptPath // ""' 2>/dev/null || true)"
  ARTIFACT_DIR="$(echo "$INPUT_JSON" | jq -r '.artifactDirectoryPath // ""' 2>/dev/null || true)"
  MODEL_NAME="$(echo "$INPUT_JSON" | jq -r '.modelName // ""' 2>/dev/null || true)"
else
  TARGET_PATH="$(python3 -c 'import sys, json; print(json.loads(sys.stdin.read() or "{}").get("toolCall", {}).get("args", {}).get("Path", ""))' <<< "$INPUT_JSON" 2>/dev/null || true)"
  TRANSCRIPT_PATH="$(python3 -c 'import sys, json; print(json.loads(sys.stdin.read() or "{}").get("transcriptPath", ""))' <<< "$INPUT_JSON" 2>/dev/null || true)"
  ARTIFACT_DIR="$(python3 -c 'import sys, json; print(json.loads(sys.stdin.read() or "{}").get("artifactDirectoryPath", ""))' <<< "$INPUT_JSON" 2>/dev/null || true)"
  MODEL_NAME="$(python3 -c 'import sys, json; print(json.loads(sys.stdin.read() or "{}").get("modelName", ""))' <<< "$INPUT_JSON" 2>/dev/null || true)"
fi

# 1. Developer bypass override (supports AGY_ALLOW_ROOT_EDIT or AGENTKIT_ALLOW_ROOT_EDIT)
if [[ "${AGY_ALLOW_ROOT_EDIT:-0}" == "1" || "${AGENTKIT_ALLOW_ROOT_EDIT:-0}" == "1" ]]; then
  echo '{"decision": "allow", "reason": "Bypassed via AGY_ALLOW_ROOT_EDIT=1 or AGENTKIT_ALLOW_ROOT_EDIT=1"}'
  exit 0
fi

# 2. Allow if Path is empty, inside artifact directory, scratch, .gemini configurations, or Obsidian AgentVault
if [[ -z "$TARGET_PATH" ]]; then
  echo '{"decision": "allow"}'
  exit 0
fi

AGENTVAULT_DOCS="${HOME}/Documents/AgentVault"
AGENTVAULT_HOME="${HOME}/AgentVault"
CUSTOM_VAULT="${AGENTKIT_AGENTVAULT_PATH:-${AGENTKIT_VAULT_PATH:-}}"

if [[ "$TARGET_PATH" == *"/.gemini/"* ]] || \
   [[ "$TARGET_PATH" == *"/brain/"* ]] || \
   { [[ -n "$ARTIFACT_DIR" ]] && [[ "$TARGET_PATH" == "$ARTIFACT_DIR"* ]]; } || \
   [[ "$TARGET_PATH" == "${AGENTVAULT_DOCS}"* ]] || \
   [[ "$TARGET_PATH" == "${AGENTVAULT_HOME}"* ]] || \
   { [[ -n "$CUSTOM_VAULT" ]] && [[ "$TARGET_PATH" == "${CUSTOM_VAULT}"* ]]; }; then
  echo '{"decision": "allow"}'
  exit 0
fi

# 3. Allow if running inside a Flash / Lite / Haiku subagent
MODEL_LOWER="$(echo "$MODEL_NAME" | tr '[:upper:]' '[:lower:]')"
if [[ "$MODEL_LOWER" == *"flash"* ]] || [[ "$MODEL_LOWER" == *"lite"* ]] || [[ "$MODEL_LOWER" == *"haiku"* ]]; then
  echo '{"decision": "allow"}'
  exit 0
fi

# 4. Check if conversation is a subagent via env vars or transcript inspection
IS_SUBAGENT="${IS_SUBAGENT:-${AGY_IS_SUBAGENT:-0}}"

if [[ "$IS_SUBAGENT" != "1" && "$IS_SUBAGENT" != "true" && -n "$TRANSCRIPT_PATH" && -f "$TRANSCRIPT_PATH" ]]; then
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

  # Helper regex: detect subagent role indicators, worker identities, or subagent markers
  if echo "$FIRST_INPUT" | grep -Eiq '(\[complexity:[[:space:]]*(low|med|high)\]|You are (Worker|a .* subagent)|<SUBAGENT)'; then
    IS_SUBAGENT=1
  fi
fi

if [[ "$IS_SUBAGENT" == "1" || "$IS_SUBAGENT" == "true" ]]; then
  echo '{"decision": "allow"}'
  exit 0
fi

# 5. Root Orchestrator Gatekeeping (switchable root orchestrators: Argon 4, Opus 5.5, Sonnet 3.7, Pro)
# Root orchestrator must never directly edit project source code, tests, or UI files.
echo '{"decision": "deny", "reason": "ORCHESTRATOR VIOLATION: Root session (Argon 4 / Opus 5.5 / Sonnet 3.7) must not call edit_file/write_file on project files directly. Delegate implementation to a Flash subagent via invoke_subagent(Model=\"flash\")."}'
exit 0
