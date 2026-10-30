#!/usr/bin/env bash
# verify-clean-completion.sh: Stop hook to prevent autonomous agent termination on dirty git status or failing tests.
set -euo pipefail

# Support CLI test / dry-run mode
if [[ "${1:-}" == "--dry-run" ]]; then
  echo '{"status": "dry-run", "decision": "stop", "message": "verify-clean-completion.sh is operational"}'
  exit 0
fi

# Recursion circuit breaker: prevent infinite test invocation loops
if [[ "${AGY_VERIFY_RUNNING:-0}" == "1" ]]; then
  echo '{"decision": "stop", "reason": "Recursion circuit breaker: verification already running"}'
  exit 0
fi
export AGY_VERIFY_RUNNING=1

if [[ "${1:-}" == "--test" && -n "${2:-}" ]]; then
  INPUT_JSON="$2"
else
  INPUT_JSON="$(cat)"
fi

# Extract terminationReason, workspacePaths, and transcriptPath
if command -v jq >/dev/null 2>&1; then
  TERM_REASON="$(echo "$INPUT_JSON" | jq -r '.terminationReason // ""' 2>/dev/null || true)"
  WORKSPACE_PATH="$(echo "$INPUT_JSON" | jq -r '.workspacePaths[0] // ""' 2>/dev/null || true)"
  TRANSCRIPT_PATH="$(echo "$INPUT_JSON" | jq -r '.transcriptPath // ""' 2>/dev/null || true)"
else
  TERM_REASON="$(python3 -c 'import sys, json; data=json.load(sys.stdin); print(data.get("terminationReason", ""))' <<< "$INPUT_JSON" 2>/dev/null || true)"
  WORKSPACE_PATH="$(python3 -c 'import sys, json; paths=json.load(sys.stdin).get("workspacePaths", []); print(paths[0] if paths else "")' <<< "$INPUT_JSON" 2>/dev/null || true)"
  TRANSCRIPT_PATH="$(python3 -c 'import sys, json; data=json.load(sys.stdin); print(data.get("transcriptPath", ""))' <<< "$INPUT_JSON" 2>/dev/null || true)"
fi

# Resolve workspace working directory if provided
if [[ -n "$WORKSPACE_PATH" && -d "$WORKSPACE_PATH" ]]; then
  cd "$WORKSPACE_PATH"
fi

# Allow non-standard terminations (e.g. user cancellations, errors, max invocations) to exit cleanly
if [[ "$TERM_REASON" != "NO_TOOL_CALL" && -n "$TERM_REASON" ]]; then
  echo '{"decision": "stop"}'
  exit 0
fi

# Check for developer bypass override
if [[ "${AGY_ALLOW_DIRTY_STOP:-0}" == "1" ]]; then
  echo '{"decision": "stop", "reason": "Bypassed via AGY_ALLOW_DIRTY_STOP=1"}'
  exit 0
fi

# Read-only turn gate: if transcript indicates no mutating tools were invoked since last USER_INPUT, exit cleanly
if [[ -n "$TRANSCRIPT_PATH" && -f "$TRANSCRIPT_PATH" ]]; then
  MUTATING_DETECTED=0
  if command -v python3 >/dev/null 2>&1; then
    MUTATING_DETECTED="$(python3 -c '
import sys, json

transcript_path = sys.argv[1]
mutating_tools = {
    "edit_file", "write_file", "replace_file_content",
    "multi_replace_file_content", "invoke_subagent", "run_command"
}

has_mutating = False

try:
    with open(transcript_path, "r", encoding="utf-8") as f:
        lines = f.readlines()

    last_user_idx = -1
    for i, line in enumerate(lines):
        if "\"type\":\"USER_INPUT\"" in line or "\"type\": \"USER_INPUT\"" in line:
            last_user_idx = i

    if last_user_idx >= 0:
        for line in lines[last_user_idx + 1:]:
            line = line.strip()
            if not line:
                continue
            try:
                step = json.loads(line)
                tool_calls = step.get("tool_calls") or []
                for tc in tool_calls:
                    name = tc.get("name") or tc.get("tool_name") or ""
                    if name in mutating_tools:
                        has_mutating = True
                        break
                tc_single = step.get("tool_call") or step.get("toolCall")
                if isinstance(tc_single, dict):
                    name = tc_single.get("name") or ""
                    if name in mutating_tools:
                        has_mutating = True
                        break
            except Exception:
                for mt in mutating_tools:
                    if f"\"{mt}\"" in line:
                        has_mutating = True
                        break
            if has_mutating:
                break
    else:
        has_mutating = True
except Exception:
    has_mutating = True

print("1" if has_mutating else "0")
' "$TRANSCRIPT_PATH" 2>/dev/null || echo "1")"
  fi

  if [[ "$MUTATING_DETECTED" == "0" ]]; then
    echo '{"decision": "stop"}'
    exit 0
  fi
fi

# 1. Inspect Git Working Tree for uncommitted / unstaged changes
if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  DIRTY_FILES="$(git status --porcelain 2>/dev/null || true)"
  if [[ -n "$DIRTY_FILES" ]]; then
    # Format top 5 dirty files for agent feedback
    DIRTY_SAMPLE="$(echo "$DIRTY_FILES" | head -n 5 | sed 's/^/  • /')"
    echo "{\"decision\": \"continue\", \"reason\": \"QUALITY GATE BLOCKED: Uncommitted changes or unstaged files detected in repository:\n${DIRTY_SAMPLE}\nEnsure all modifications are cleanly committed to your feature branch with Conventional Commits before concluding.\"}"
    exit 0
  fi
fi

# 2. Automated Test Verification
TEST_FAILURES=0
FAILED_SUITE=""

# Python verification
if [[ -f "pytest.ini" || -f "pyproject.toml" || -d "tests" ]]; then
  if command -v pytest >/dev/null 2>&1; then
    AGY_VERIFY_RUNNING=0 pytest -q >/dev/null 2>&1 || { TEST_FAILURES=1; FAILED_SUITE="pytest"; }
  elif command -v python3 >/dev/null 2>&1; then
    AGY_VERIFY_RUNNING=0 python3 -m unittest discover tests -v >/dev/null 2>&1 || { TEST_FAILURES=1; FAILED_SUITE="python unittest"; }
  fi
fi

# Node.js verification
if [[ -f "package.json" ]] && grep -q '"test"' package.json; then
  if command -v npm >/dev/null 2>&1; then
    npm test >/dev/null 2>&1 || { TEST_FAILURES=1; FAILED_SUITE="npm test"; }
  fi
fi

# Cargo / Rust verification
if [[ -f "Cargo.toml" ]]; then
  if command -v cargo >/dev/null 2>&1; then
    cargo test -q >/dev/null 2>&1 || { TEST_FAILURES=1; FAILED_SUITE="cargo test"; }
  fi
fi

# Go verification
if [[ -f "go.mod" ]]; then
  if command -v go >/dev/null 2>&1; then
    go test ./... >/dev/null 2>&1 || { TEST_FAILURES=1; FAILED_SUITE="go test"; }
  fi
fi

if [[ "$TEST_FAILURES" -ne 0 ]]; then
  echo "{\"decision\": \"continue\", \"reason\": \"QUALITY GATE BLOCKED: Test suite (${FAILED_SUITE}) failed. All automated unit and integration tests must pass deterministically before completing this task.\"}"
  exit 0
fi

# All gates passed: allow agent execution loop to stop cleanly
echo '{"decision": "stop"}'
exit 0
