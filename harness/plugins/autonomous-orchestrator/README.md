# Autonomous Orchestrator Plugin

An opinionated Antigravity plugin enabling state-of-the-art autonomous agent workflows with **Gemini 4 Argon**, **Gemini 3.8 Flash (High reasoning)**, **Claude Opus 5.5**, and **Claude Sonnet 3.7** root orchestrators delegating to **Gemini 3.8 Flash / Flash Lite** subagents.

---

## Capabilities

- **Cognitive Tiering**: Routes high-level architecture, task DAG generation, and adversarial reviews to **Gemini 4 Argon** (primary) or **Gemini 3.8 Flash High** (standard), and discrete implementation tasks to **Gemini 3.8 Flash** or **Flash Lite** subagents.
- **Complexity-Aware Subagent Routing**: Partition tasks by complexity tag (`[complexity: low|med|high]`):
  - `low`: `flash_lite` (file moves/renames, mechanical find-replace, formatting, doc typos, simple one-command inspection).
  - `med`: `flash` (feature code, tests, localized refactors).
  - `high`: `flash` (cross-module refactors, complex debugging, security), with escalation to `pro` (Gemini 4 Argon) for architecture, multi-repo design, or adversarial review when Argon quota is healthy.
- **Context Preservation**: Subagents execute in isolated scopes, preventing test log bloat from degrading the orchestrator context window.
- **Disjoint-File Parallel Worker Partitioning**: Partitions multi-component features across concurrent subagents operating on strictly disjoint file sets for race-free parallel execution.
- **Zero-Prompt Command Shape Discipline**: Pure-edit workers use native file editing tools without terminal calls; test/build workers prioritize workspace-relative generic commands when in the project root (e.g. `.venv/bin/pytest tests -v`, `git status`) and avoid shell chaining or complex `-c` one-liners that trigger manual approval prompts.
- **Git Worktree Isolation**: Supports concurrent workers in independent git worktrees via `Workspace: "share"`.
- **Deterministic Hook Guards**:
  - `SessionStart` (`scripts/guard-agentkit-compliance.sh`): Verifies Antigravity harness configuration against agentkit standards at session startup. Injects actionable alerts if settings are out of sync, or automatically synchronizes them when `AGENTKIT_AUTO_APPLY=1` is set. Bypassable via `AGENTKIT_SKIP_SESSION_AUDIT=1`.
  - `PreToolUse` (`scripts/guard-subagent-delegation.sh`): Blocks the Root Orchestrator from directly modifying project source code, tests, or UI files, enforcing delegation to worker subagents.
  - `PreToolUse` (`scripts/guard-git-operations.sh`): Hard blocks direct pushes, force-pushes, or merges to `main`/`master`, and gates destructive deletions.
  - `PreToolUse` (`scripts/filter-and-wrap.py` & `scripts/headroom-wrap.py`): Transparently wraps verbose test, diff, and inspection commands to compress output tokens by 60–90% with Reversible CCR (Contextual Compression with Retrieval) caching (`~/.cache/antigravity-headroom/<hash>.txt`).
  - `Stop` (`scripts/verify-clean-completion.sh`): Prevents task termination if automated tests fail or if uncommitted changes exist in the worktree.
- **Quota Integration**: Operates alongside the `quota-guardian` plugin for real-time quota polling (`RetrieveUserQuotaSummary`), automated fallback rerouting (<15%), and graceful wrap-up enforcement (<5%).
- **Cross-Repository Memory & AST Code Compression**:
  - **AST Structural Packing (`repomix` MCP)**: Integrates `repomix --mcp` to generate token-efficient codebase skeletons and interface signatures without reading monolithic raw source trees.
- **DevOps Integration**: Works seamlessly with `agentkit` for workstation permissions and `repokit` for GitHub repository rulesets and squash merge enforcement.

---

## Configuration & Environment Variables

| Variable | Default | Description |
|---|---|---|
| `AGENTKIT_AUTO_APPLY` | `0` | Automatically runs `agentkit apply` and notifies the session when non-compliant configuration is detected at `SessionStart`. |
| `AGENTKIT_SKIP_SESSION_AUDIT` | `0` | Bypasses `SessionStart` compliance auditing when set to `1`. |
| `AGY_ALLOW_ROOT_EDIT` | `0` | Bypasses `PreToolUse` orchestrator delegation guard when set to `1`. |
| `AGENTKIT_ALLOW_ROOT_EDIT` | `0` | Alias for `AGY_ALLOW_ROOT_EDIT`. |
| `AGY_ALLOW_DIRTY_STOP` | `0` | Bypasses `Stop` clean worktree verification when set to `1`. |
| `AGY_STRICT_SUBAGENT_DENY` | `0` | Strictly denies root orchestrator direct project edits when set to `1`. |
