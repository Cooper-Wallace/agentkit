---
name: auto-lifecycle
description: "Use whenever executing feature tasks, bug fixes, post-run log reviews, or iterative user feedback adjustments across Argon 4 / Flash High / Opus 5.5 root orchestrators and Flash subagents with automated test verification, and adversarial diff review."
---

# Autonomous Multi-Model Lifecycle Runbook

This skill teaches the Root Orchestrator (Gemini 4 Argon, Gemini 3.8 Flash High, Claude Opus 5.5, or Claude Sonnet 3.7) how to execute features autonomously under Auto mode without human micro-management, while guaranteeing zero trunk contamination, quota headroom awareness, and rigorous adversarial verification.

---

## The 4-Phase Lifecycle

```mermaid
flowchart LR
    P1["Phase 1: Ingest & Task DAG<br/>(Argon 4 / Flash High / Opus 5.5)"] --> P2A["Phase 2: Worker A [complexity: med]<br/>(Backend & Tests)"]
    P1 --> P2B["Phase 2: Worker B [complexity: low]<br/>(Pure-Edit UI/Docs)"]
    P2A --> P3["Phase 3: Adversarial Review<br/>(Argon 4 / Flash High / Opus 5.5)"]
    P2B --> P3
    P3 --> P4["Phase 4: Promotion & Vault Handoff<br/>(PR + Vault + kill_all)"]
```

### Phase 1: Contract Freeze, Offline Ingestion & Task DAG (Root Orchestrator)
1. **Analyze Requirements, Offline User Directives & Cross-Repo Context**:
   - **Offline User Input Ingestion**: Inspect the Obsidian AgentVault at `$HOME/Documents/AgentVault` (or `$HOME/AgentVault`) via `agentkit vault ingest` or by reading `User-Input/pending/` and `Projects/<project>.md`. Ingest any offline user feedback, tasks, or directives left between agent conversations, and incorporate them directly into `plan.md`. Move processed input notes to `User-Input/processed/`.
   - **Cross-Repo Contracts & Project Context**: Read `$VAULT/Projects/<project>.md` for living architecture constraints and active task status. Query cross-repo contracts, shared schemas, and system boundaries across the 4-layer workstation stack (`riced-kubuntu-os`, `workstation-setup`, `agentkit` [CLI `agentkit`], `repokit`).
   - When inspecting sibling repositories, use `repomix` AST compression (`repomix` MCP or `repomix --compress`) to retrieve structural skeletons without bloating context.
   - Inspect local project files, existing tests, and architecture constraints.
2. **Quota Headroom Pre-Check**:
   - Run `agentkit quota status` before freezing large DAG fan-outs to ensure target quota groups (Gemini, Claude, Argon) have sufficient headroom (>15%).
3. **Draft Plan Artifact**: Create an executable markdown plan in the artifact directory (`brain/<conversation-id>/plan.md`).
   - Define exact files to modify.
   - Define test contracts and acceptance criteria.
   - Outline subtask DAG with explicit complexity tiers (`[complexity: low|med|high]`) for parallel worker execution.
4. **Branch Isolation (One Branch Per Change)**: Ensure the active branch is a dedicated short-lived feature branch (`feat/<name>`, `fix/<name>`, `docs/<name>`, `refactor/<name>`, `test/<name>`) branched directly from `main`. Never stack unrelated changes or accumulate multiple features on the same branch.

### Phase 2: Autonomous Implementation (Worker Subagents)
1. **Strict Subagent Execution**: ALL code, test, and UI edits (including initial implementation as well as iterative tuning after user review feedback) must be executed by subagents (`Workspace: "inherit"` or `"share"`), never directly by the Root Orchestrator (Argon 4 / Flash High / Opus 5.5 / Sonnet 3.7) in the root session.
2. **Complexity-Aware Model Tiering**:
   - **`low`**: Launch with `Model: "flash_lite"` (file moves/renames, mechanical find-replace, formatting, doc typos, simple one-command inspection).
   - **`med`**: Launch with `Model: "flash"` (feature code, tests, localized refactors).
   - **`high`**: Launch with `Model: "flash"` (cross-module refactors, complex debugging, security), with escalation to `Model: "pro"` (Gemini 4 Argon) for architecture, multi-repo design, or adversarial review when Argon quota is healthy (>15%).
   - Prefix every subagent prompt with the complexity tag: `[complexity: low|med|high]`.
3. **Automated Fallback & Wrap-Up Protection**:
   - `quota-guardian` automatically monitors quota via `RetrieveUserQuotaSummary` and reroutes subagents to healthy fallback groups if a group drops below 15%.
   - If all quota groups drop below 5%, `quota-guardian` denies further subagent delegation with `QUOTA WRAP-UP:`. Follow the graceful wrap-up protocol immediately (stop delegation, run tests, commit WIP, write `handoff.md`, advise user of reset time).
4. **Disjoint-File Parallel Worker Pool**: Split the Phase 1 DAG into parallel subagents partitioned by file domain (e.g., Worker A: Backend & Unit Tests; Worker B: Frontend/UI; Worker C: Architecture Docs) so workers execute in parallel without write contention.
5. **Automated Tool Output Token Suppression (Headroom CCR)**:
   - Note that `PreToolUse` automatically intercepts and wraps verbose test/inspection commands via `headroom-wrap.py` (`filter-and-wrap.py` -> `headroom-wrap.py`), compressing outputs >= 1,500 chars by 60-90%.
   - Full uncompressed outputs are cached locally at `~/.cache/antigravity-headroom/<hash>.txt` for reversible CCR retrieval via `read_file` when specific stack trace slices are required.
6. **Launch Worker Subagents in Isolated Worktrees**: Delegate coding and testing using `invoke_subagent`:
   - **Separate Worktrees for Concurrent Threads**: Always set `Workspace: "share"` (which provisions an isolated Git worktree) when inside a Git repository root to prevent `.git/index.lock` collisions, dirty working tree races, and write contention across concurrent threads. Concurrent workers must never operate in the same physical working directory simultaneously. Use `Workspace: "inherit"` with strictly disjoint file boundaries only when the active session workspace is `$HOME` rather than a repository root.
   - Pass the exact spec contract from Phase 1.
7. **Zero-Prompt Subagent Prompting & Atomic Commit Instruction**: Include explicit autonomy and commit constraints in every `invoke_subagent` prompt:
   - **One Commit Per Subagent**: Instruct each worker subagent: *"Upon verifying all changes pass automated tests, stage your modifications and produce exactly ONE atomic Conventional Commit (`git add <files>` and `git commit -m '<type>(<scope>): <short summary>'`). Do NOT leave uncommitted modifications or produce multiple messy WIP commits."*
   - **Pure-Edit Workers**: For workers modifying code, UI, or documentation without running tests/commands: *"Use ONLY `read_file` and `edit_file`. NEVER call `run_command`."*
   - **Test/Build Workers**: For workers running tests or build commands: *"Prioritize workspace-relative generic commands (`.venv/bin/pytest tests -v`, `git status`, `git diff`) when inside the repo root. Never use `cd ... && ...`, shell chains (`&&`, `;`, `|`), or `/path/to/.venv/bin/python3 -c ...` one-liners. Reserve absolute or `-C` paths only when cross-directory invocations from `$HOME` are required."*
   - Instruct workers: *"Return only a concise 10-line summary of files modified, commit hash/message, and test verification results."*
8. **Reactive Wakeup**: When the subagents complete, the agent harness reactively resumes the Root Orchestrator.

### Phase 3: Adversarial Critique & Self-Healing (Argon 4 / Flash High / Opus 5.5)
1. **Inspect Git Diff & Subagent Commits**:
   - Run `git status`, `git diff`, and `git log -n 1 --stat` to inspect changes and verify that each worker subagent produced a single atomic Conventional Commit representing its completed deliverable without uncommitted working tree residue.
2. **Audit Checkpoints**:
   - Review logic against edge cases, error handling, and potential race conditions.
   - Verify that no secrets, `.env` files, or temporary scratch statements leaked.
   - Verify that commit messages comply with Conventional Commits 1.0.0 and represent a coherent, single unit of work.
3. **Self-Healing & User Review Feedback Loop**: When either the adversarial review OR the user reports a regression, tuning issue, or requested adjustment: the Root Orchestrator diagnoses the logs/diff and updates `plan.md`, but **MUST dispatch a subagent (`Model: "flash"`)** to implement the fix and update unit tests — never editing project files directly in the root session.

### Phase 4: Verification, Vault Memory Persistence & PR Promotion Gate
1. **Deterministic Quality Gate**: Verify that the test suite passes locally. The `Stop` hook will reject session termination if tests fail or if the worktree is dirty.
2. **Obsidian AgentVault Persistence**:
   - Author a structured session handoff note to `$HOME/Documents/AgentVault/Sessions/handoff-YYYY-MM-DD-<topic>.md` (or run `agentkit vault handoff --topic <topic>`), detailing completed deliverables, active branch, test verification status, and next steps for offline review.
   - Update the target project knowledge base in `$HOME/Documents/AgentVault/Projects/<project>.md`: check off completed tasks (`- [x]`), record newly discovered backlog items (`- [ ]`), and update the "Current State" summary.
   - If cross-repo contracts or architectural boundaries were modified, update `$VAULT/Architecture/` notes and visual JSON Canvas diagrams (`[[Architecture/Workstation-Stack.canvas]]`).
3. **Subagent Pool Cleanup**: Terminate all idle worker subagents via `manage_subagents(Action="kill_all")` before finalizing the session.
4. **Push Feature Branch**:
   ```bash
   git push -u origin feat/<feature-name>
   ```
5. **Create Pull Request & Post Review Audit**:
   Use the GitHub CLI (`gh pr create`) or GitHub MCP to open a PR and verify check suites:
   ```bash
   gh pr create --title "feat(scope): concise description" \
                --body "### Summary... ### Verification..."
   gh pr checks
   ```
   Post the Phase 3 adversarial review report to the PR using `gh pr comment` or directly in the PR description.
6. **Human-Only Trunk Promotion Gate**: Autonomous agents MUST NEVER execute `gh pr merge`, enable GitHub auto-merge, or automate trunk merging. Phase 4 concludes by presenting the PR URL and verification report to the user for human review and manual squash merge via the GitHub web UI once all quality gates pass.
