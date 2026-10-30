# Autonomous Orchestrator Plugin Directives

These runtime directives govern the hook behaviors and environment overrides of the `autonomous-orchestrator` plugin. Core cognitive model tiering, subagent complexity routing, AgentVault memory, and git hygiene rules are defined in root workspace directives (`AGENTS.md`).

---

## 1. Lifecycle Hook Behaviors & Environment Overrides

- **`SessionStart` Compliance Guard (`scripts/guard-agentkit-compliance.sh`)**:
  - Automatically audits Antigravity harness settings against agentkit declarative standards on session start.
  - Set `AGENTKIT_AUTO_APPLY=1` to auto-heal discrepancies via `agentkit apply`.
  - Set `AGENTKIT_SKIP_SESSION_AUDIT=1` to bypass session start compliance checks.

- **`PreToolUse` Subagent Delegation Gate (`scripts/guard-subagent-delegation.sh`)**:
  - Blocks Root Orchestrators (Argon 4, Opus 5.5, Sonnet 3.7) from calling `edit_file` or `write_file` directly on project files.
  - Edits inside Obsidian AgentVault (`$HOME/Documents/AgentVault` or `$HOME/AgentVault`), `~/.gemini/`, and session brain directories are always permitted.
  - Set `AGY_ALLOW_ROOT_EDIT=1` (or `AGENTKIT_ALLOW_ROOT_EDIT=1`) to override this gate during emergency debugging.

- **`PreToolUse` Headroom Token Compression (`scripts/filter-and-wrap.py` & `headroom-wrap.py`)**:
  - Compresses command outputs >= 1,500 chars by 60–90% to conserve context window tokens.
  - Full uncompressed outputs are cached for Reversible CCR retrieval at `~/.cache/antigravity-headroom/<hash>.txt`.
  - Set `AGY_DISABLE_TOKEN_COMPRESSION=1` to bypass output compression.

- **`Stop` Quality Gate (`scripts/verify-clean-completion.sh`)**:
  - Prevents session completion if git working tree contains uncommitted modifications or if unit tests fail.
  - Read-only conversational turns (no mutating tools invoked since last user prompt) stop cleanly without triggering test suites.
  - Set `AGY_ALLOW_DIRTY_STOP=1` to bypass dirty worktree and test verification gates.
