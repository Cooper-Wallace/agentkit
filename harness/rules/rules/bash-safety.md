---
trigger: model_decision
description: Best practices for authoring, auditing, and executing bash scripts safely and idempotently.
---

# Bash Safety and Idempotence Guidelines

When writing or executing shell scripts on developer workstations:

1. **Strict Error Handling**:
   - Always include `set -euo pipefail` at the top of scripts to fail immediately on command failure, unset variables, and pipeline errors.
2. **Idempotence**:
   - Every script must be safe to run multiple times without duplicating data or breaking state.
   - Use `mkdir -p` instead of `mkdir`.
   - Use `ln -sf` for symlinks.
   - Guard file appends with grep checks before appending lines.
3. **Dry-Run Capability**:
   - Scripts that perform disk writes, file removals, or package changes must support `--dry-run`.
4. **Dynamic Paths**:
   - Never hardcode home paths like `/home/cooper`.
   - Use `SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"` to resolve script-relative assets.
   - Use `$HOME` for user-level directories.
