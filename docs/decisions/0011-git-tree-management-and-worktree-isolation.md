# ADR 0011: Git Tree Management: Single-Branch Changes, Atomic Subagent Commits, and Worktree Isolation

## Status
Accepted

---

## Context

Autonomous multi-model software engineering under `agentkit` orchestrates high-capability root reasoning models (Gemini 4 Argon, Gemini 3.8 Flash High, Claude Opus 5.5, Claude Sonnet 3.7) alongside parallel worker subagents (Gemini 3.8 Flash, Flash Lite). These agents collaborate across short-lived feature branches on multi-stage task DAGs, with code promotion governed by `repokit` PR rulesets.

As autonomous agent teams scaled to multi-threaded parallel execution across complex repositories, three critical operational bottlenecks and race conditions emerged in Git tree management:

1. **Feature Stacking & Scope Creep**:
   - Agent workflows frequently accumulated unrelated changes, minor bug fixes, unrelated refactors, and test additions onto a single sprawling feature branch.
   - This scope creep complicated code reviews, made automated squash-and-merge evaluation by `repokit` error-prone, and prevented surgical rollbacks when regressions occurred.

2. **Commit Fragmentation & Dirty Working Tree Residue**:
   - Subagents assigned to specific components frequently completed edits without committing their changes, returning control to the parent orchestrator with a dirty working tree.
   - Conversely, other subagents produced erratic sequences of micro-commits, unverified checkpoints, or unstructured WIP messages that degraded repository hygiene and broke `git bisect` reproducibility.

3. **Git Index Lock & Working Tree Collisions Under Parallel Fan-Outs**:
   - When the root orchestrator launched concurrent subagents within the same repository directory (e.g., Worker A on backend logic, Worker B on frontend UI, Worker C on documentation), all subagents operated on the same physical filesystem directory.
   - This caused catastrophic Git lock contention (`fatal: Unable to create '.git/index.lock': File exists`), dirty state overwrite races, and file thrashing across concurrent processes.

To establish deterministic stability and source control hygiene, this ADR formalizes three immutable Git tree management invariants across all developer workspaces and agent harness directives.

---

## Decision

We establish three architectural invariants governing all AI agent operations and developer workflows:

### 1. Invariant 1: One Branch Per Change
- **Dedicated Short-Lived Branches**: Every discrete task, feature, bug fix, documentation update, or refactor MUST operate on its own dedicated short-lived feature branch branched directly from `main`:
  - `feat/<feature-name>`: New capabilities or functional enhancements.
  - `fix/<bug-name>`: Bug fixes and issue remediations.
  - `docs/<doc-name>`: Documentation, guides, and ADR additions.
  - `refactor/<target>`: Structural code improvements without behavioral changes.
  - `test/<suite>`: Test additions, harnesses, or test framework refactoring.
- **Strict Scope Isolation**: Never stack unrelated features or accumulate independent changes on the same branch. If a secondary improvement or bug is discovered during active feature development, it must be spun out into a distinct task and dedicated branch.

### 2. Invariant 2: One Atomic Commit Per Subagent
- **Atomic Subagent Deliverables**: Each worker subagent assigned to implement a task or deliverable must produce exactly **ONE atomic Conventional Commit** (`<type>(<scope>): <short summary>`) representing its completed, tested unit of work.
- **Clean Exit Requirement**: Worker subagents must never exit with uncommitted working tree modifications. Before returning results to the caller, the worker must run verification tests, stage all relevant files, and commit them.
- **Conventional Commits 1.0.0**: Commit messages must adhere strictly to Conventional Commits specifications with imperative, present-tense verbs (`feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`).
- **No WIP Commits**: Subagents must not produce messy intermediate or WIP commits during standard execution. The only exception is the `quota-wrapup` emergency circuit breaker, which creates a single consolidated commit (`chore(wip): save progress before quota wrap-up`) prior to halting execution.

### 3. Invariant 3: Isolated Git Worktrees for Concurrent Threads
- **Isolated Physical Working Trees**: When multiple agent threads, subagents, or processes work concurrently within the same Git repository, they MUST operate in isolated Git worktrees (`Workspace: "share"` in Antigravity or `git worktree add`).
- **Zero Shared Directory Concurrency**: Concurrent workers must never operate on the same physical working directory simultaneously. This completely isolates `.git/index.lock`, prevents dirty working tree races, and guarantees race-free parallel execution.
- **Lifecycle Cleanup**: Once subagents terminate, the root orchestrator cleans up idle subagents via `manage_subagents(Action="kill_all")`, automatically releasing associated worktrees.

---

## Integration with Agent Lifecycle & Directives

1. **Autonomous Lifecycle (`auto-lifecycle`)**:
   - **Phase 1 (Contract Freeze)**: The Root Orchestrator enforces branch isolation, ensuring work begins on a clean dedicated branch (`feat/<name>` or `fix/<name>`).
   - **Phase 2 (Implementation)**: Subagents are dispatched with `Workspace: "share"` for worktree isolation. Subagent prompts include explicit directives to stage files and generate exactly one atomic Conventional Commit upon passing tests.
   - **Phase 3 (Adversarial Review)**: The adversarial review inspects `git status`, `git diff`, and `git log -n 1 --stat` to verify the subagent's atomic commit and ensure zero dirty residue.
2. **Quota Guardian (`quota-wrapup`)**:
   - Maintains worktree boundaries and single-commit hygiene even under abrupt token exhaustion.
3. **Harmonized Directives (`AGENTS.md`)**:
   - Standardized across `harness/rules/AGENTS.md`, root `AGENTS.md`, `project/templates/AGENTS.md`, and plugin-specific directives.

---

## Consequences

### Positive
- **Linear, Bisectable History**: Every subagent deliverable forms a clean, atomic commit with valid Conventional Commit metadata, ensuring `git bisect` works cleanly without broken intermediate states.
- **Zero Lock Collisions Under High Concurrency**: Parallel subagents running across disjoint file sets no longer trigger `.git/index.lock` collisions or file overwrite races.
- **Predictable PR Governance**: Short-lived single-purpose branches streamline code reviews and integrate seamlessly with `repokit` automated squash-and-merge policies.
- **Traceability**: Each subagent's output maps directly to a discrete, verifiable Git commit hash.

### Neutral / Negative
- **Worktree Disk Footprint**: Maintaining multiple concurrent worktrees temporarily consumes additional disk space while subagents are active.
- **Prompt Discipline**: Orchestrators must consistently include commit instructions in worker prompts to prevent subagents from leaving uncommitted changes.
