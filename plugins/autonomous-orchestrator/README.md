# Autonomous Orchestrator Plugin

An opinionated Antigravity plugin enabling state-of-the-art autonomous agent workflows with **Gemini 4 Argon** and **Gemini 3.8 Flash**.

---

## Capabilities

- **Cognitive Tiering**: Routes high-level architecture and adversarial reviews to **Gemini 4 Argon**, and rapid implementation/testing to **Gemini 3.8 Flash**.
- **Context Preservation**: Subagents execute in isolated scopes, preventing test log bloat from degrading the orchestrator context window.
- **Git Worktree Isolation**: Supports concurrent workers in independent git worktrees via `Workspace: "share"`.
- **Deterministic Hook Guards**:
  - `PreToolUse` (`scripts/guard-git-operations.sh`): Hard blocks direct pushes, force-pushes, or merges to `main`/`master`, and gates destructive deletions.
  - `Stop` (`scripts/verify-clean-completion.sh`): Prevents task termination if automated tests fail or if uncommitted changes exist in the worktree.
- **DevOps Integration**: Works seamlessly with `antigravity-standardise` for workstation permissions and `gh-standardise` for GitHub repository rulesets and squash merge enforcement.
