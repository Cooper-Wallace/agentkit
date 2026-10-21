# Autonomous Orchestrator Plugin

An opinionated Antigravity plugin enabling state-of-the-art autonomous agent workflows with **Gemini 4 Argon** and **Gemini 3.8 Flash**.

---

## Capabilities

- **Cognitive Tiering**: Routes high-level architecture and adversarial reviews to **Gemini 4 Argon**, and rapid implementation/testing to **Gemini 3.8 Flash**.
- **Context Preservation**: Subagents execute in isolated scopes, preventing test log bloat from degrading the orchestrator context window.
- **Disjoint-File Parallel Worker Partitioning**: Partitions multi-component features across concurrent Flash subagents operating on strictly disjoint file sets for race-free parallel execution.
- **Zero-Prompt Command Shape Discipline**: Pure-edit workers use native file editing tools without terminal calls; test/build workers prioritize workspace-relative generic commands when in the project root (e.g. `.venv/bin/pytest tests -v`, `git status`) and avoid shell chaining or complex `-c` one-liners that trigger manual approval prompts.
- **Git Worktree Isolation**: Supports concurrent workers in independent git worktrees via `Workspace: "share"`.
- **Deterministic Hook Guards**:
  - `PreToolUse` (`scripts/guard-subagent-delegation.sh`): Blocks the Root Orchestrator (Argon) from directly modifying project source code, tests, or UI files, enforcing delegation to Gemini 3.8 Flash subagents.
  - `PreToolUse` (`scripts/guard-git-operations.sh`): Hard blocks direct pushes, force-pushes, or merges to `main`/`master`, and gates destructive deletions.
  - `PreToolUse` (`scripts/filter-and-wrap.py` & `scripts/headroom-wrap.py`): Transparently wraps verbose test, diff, and inspection commands to compress output tokens by 60–90% with Reversible CCR (Contextual Compression with Retrieval) caching (`~/.cache/antigravity-headroom/<hash>.txt`).
  - `Stop` (`scripts/verify-clean-completion.sh`): Prevents task termination if automated tests fail or if uncommitted changes exist in the worktree.
- **Cross-Repository Memory & AST Code Compression**:
  - **Perpetual Memory (`memory` MCP)**: Integrates `@modelcontextprotocol/server-memory` to maintain a persistent knowledge graph of entities, relations, and architectural constraints across modular repositories.
  - **AST Structural Packing (`repomix` MCP)**: Integrates `repomix --mcp` to generate token-efficient codebase skeletons and interface signatures without reading monolithic raw source trees.
- **DevOps Integration**: Works seamlessly with `antigravity-standardise` for workstation permissions and `gh-standardise` for GitHub repository rulesets and squash merge enforcement.
