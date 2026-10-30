# Quota Guardian Plugin Directives

Delta directives for `quota-guardian`. Refer to global `AGENTS.md` for universal cognitive tiering, model architecture, and git worktree isolation standards.

---

## 1. Complexity Tagging Requirement

All `invoke_subagent` prompts MUST include the standard tag `[complexity: low|med|high]` at the start of the prompt:
- `low`: Routes to `flash_lite` (auto-upgraded to `flash` if prompt touches tests, refactoring, debugging, or security).
- `med`: Routes to `flash`.
- `high`: Escalates to `pro` (Gemini 4 Argon) when Argon quota is healthy (> 15%); otherwise routes to `flash`.

## 2. Quota Guard Bypass & Inspection

- Pre-flight quota check: `agentkit quota status` or `agentkit quota route --complexity <low|med|high>`.
- Hook bypass: Set `AGENTKIT_QUOTA_GUARD=0` or `AGENTKIT_QUOTA_GUARD=off` in the environment to disable the `PreToolUse` guard.

## 3. Strict Denial Adherence on Quota Wrap-Up

When `guard-quota.py` denies `invoke_subagent` with `QUOTA WRAP-UP: All model quotas are critically low`:
1. Immediately cease invoking new subagents (do not retry or attempt model bypass).
2. Execute the `quota-wrapup` skill: run tests, commit WIP to the active feature branch, author handoff note in Obsidian AgentVault, and advise the user.
