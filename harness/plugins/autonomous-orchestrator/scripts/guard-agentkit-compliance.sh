#!/usr/bin/env bash
# guard-agentkit-compliance.sh: SessionStart hook enforcing agentkit configuration compliance.
set -euo pipefail

# 1. CLI test / dry-run mode
if [[ "${1:-}" == "--dry-run" ]]; then
  echo '{"status": "dry-run", "message": "guard-agentkit-compliance.sh is operational"}'
  exit 0
fi

# 2. Stdin handling: in --test mode, do not block on stdin
if [[ "${1:-}" == "--test" ]]; then
  if [[ -n "${2:-}" ]]; then
    INPUT_JSON="$2"
  else
    INPUT_JSON="{}"
  fi
else
  INPUT_JSON="$(cat)"
fi

# 3. Bypass check: AGENTKIT_SKIP_SESSION_AUDIT=1
if [[ "${AGENTKIT_SKIP_SESSION_AUDIT:-0}" == "1" ]]; then
  echo '{}'
  exit 0
fi

# 4. Binary resolution for agentkit
RESOLVED_BIN=""
if [[ -n "${AGENTKIT_BIN:-}" && -x "${AGENTKIT_BIN}" ]]; then
  RESOLVED_BIN="${AGENTKIT_BIN}"
elif command -v agentkit >/dev/null 2>&1; then
  RESOLVED_BIN="$(command -v agentkit)"
elif [[ -x "${HOME}/.local/bin/agentkit" ]]; then
  RESOLVED_BIN="${HOME}/.local/bin/agentkit"
else
  SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  REPO_CANDIDATE="$(cd "${SCRIPT_DIR}/../../../../bin" 2>/dev/null && pwd || true)/agentkit"
  if [[ -x "${REPO_CANDIDATE}" ]]; then
    RESOLVED_BIN="${REPO_CANDIDATE}"
  fi
fi

if [[ -z "${RESOLVED_BIN}" || ! -x "${RESOLVED_BIN}" ]]; then
  echo '{}'
  exit 0
fi

# 5. Execute agentkit audit
AUDIT_EXIT=0
AUDIT_OUTPUT="$("${RESOLVED_BIN}" audit 2>&1)" || AUDIT_EXIT=$?

if [[ "${AUDIT_EXIT}" -eq 0 ]]; then
  echo '{}'
  exit 0
fi

# 6. Audit failed: auto-heal or alert
if [[ "${AGENTKIT_AUTO_APPLY:-0}" == "1" ]]; then
  "${RESOLVED_BIN}" apply >/dev/null 2>&1 || true
  cat << 'EOF'
{"injectSteps": [{"systemMessage": {"systemMessage": "[agentkit auto-heal] Antigravity harness configuration was automatically synchronized with agentkit standards via 'agentkit apply'."}}]}
EOF
  exit 0
else
  cat << 'EOF'
{"injectSteps": [{"systemMessage": {"systemMessage": "⚠️ [agentkit] Antigravity harness configuration is out of sync with standards. Run 'agentkit apply' to synchronize pre-approved permissions, directives, and plugins."}}]}
EOF
  exit 0
fi
