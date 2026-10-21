# Autonomous Multi-Model Orchestration Standards

Welcome to the enterprise autonomous development environment. These directives govern multi-agent task distribution, context preservation, and source control hygiene.

---

## 1. Cognitive Architecture & Model Tiering

- **Root Orchestrator (Gemini 4 Argon)**:
  - The root session operates under Gemini 4 Argon. It is responsible for high-level requirement analysis, architectural contracts, task DAG generation, and adversarial reviews.
  - The root session MUST NEVER directly edit project source code, tests, or UI files (via edit_file or write_file) — only artifact files in the session brain directory (e.g. plan.md) may be written directly by the root orchestrator. All code implementation, post-review bugfixes, threshold tuning, and test updates — even single-file follow-up adjustments after user feedback — MUST be delegated to Flash worker subagents.
- **Worker Subagents (Gemini 3.8 Flash)**:
  - Code implementation, localized refactoring, documentation syncing, and test runs MUST be delegated to subagents launched with `Model: "flash"` and `Workspace: "share"` (or `Workspace: "inherit"` when the active workspace is `$HOME` rather than a VCS repository root).
  - **Disjoint-File Parallel Worker Partitioning**: Multi-component tasks (e.g., backend logic, frontend UI, documentation, and unit tests) MUST be partitioned across concurrent Flash subagents operating on strictly disjoint file sets so workers execute in parallel without write contention.
- **Adversarial Auditor (Gemini 4 Argon)**:
  - Before requesting task completion or submitting a Pull Request, an adversarial review must evaluate the `git diff` against the initial specification.
- **Crash & Compaction Recovery Protocol**:
  - Whenever resuming after a crash, context compaction, or interrupted turn, the Root Orchestrator MUST check subagent completeness via manage_subagents(Action="list"), transcript.jsonl, and git status before dispatching new work.

---

## 2. Context Window Preservation, Token Suppression & Cross-Repo Memory

- **Zero-Pollution Delegation**:
  - Ephemeral worker subagents discard voluminous build logs, 500-line test outputs, and scratch explorations upon termination.
  - Workers must return only concise completion summaries (10-20 lines) to the root orchestrator.
- **Progressive Skill Retrieval**:
  - Do not load monolithic documentation upfront. Retrieve runbooks and specific guides on-demand via skills.
- **Artifact-Driven State Tracking**:
  - Maintain the source-of-truth task list and architectural contracts in markdown artifacts (`.md` files in the artifact directory).
- **Automated Tool Output Token Suppression (Headroom CCR)**:
  - Verbose test, log, and diff commands executed via `run_command` are automatically intercepted by `PreToolUse` (`filter-and-wrap.py` -> `headroom-wrap.py`), compressing outputs >= 1,500 chars by 60-90%.
  - **Reversible CCR Retrieval**: Full uncompressed outputs are cached locally at `~/.cache/antigravity-headroom/<hash>.txt`. If an agent needs raw stack trace lines omitted by compression, it MUST read the cached file slice via `read_file` rather than re-running the test suite.
- **Perpetual Cross-Repository Memory (`memory` MCP)**:
  - Maintain cross-repo architectural contracts, shared schemas, and system boundaries across the 4-layer workstation stack (`riced-kubuntu-os`, `workstation-setup`, `antigravity-standardise`, `gh-standardise`) in the persistent `memory` knowledge graph.
  - The Root Orchestrator queries the knowledge graph during Phase 1 planning and injects relevant cross-repo constraints into Flash worker prompts, avoiding redundant multi-repo file exploration.
- **Cross-Repo Structural AST Compression (`repomix` MCP)**:
  - When inspecting sibling repositories, prefer AST-compressed structural skeletons (`repomix` MCP or `repomix --compress`) over raw multi-file reads to reduce cross-repo token ingestion by ~70%.

---

## 3. Subagent Autonomy & Zero-Prompt Command Discipline

- **Pure-Edit Worker Isolation**:
  - Subagents assigned exclusively to file edits (frontend UI, configuration, or documentation) MUST be instructed to use ONLY native `read_file` and `edit_file` tools (`NEVER call run_command`), guaranteeing zero terminal approval prompts.
- **Prefix-Matchable Command Shapes**:
  - Antigravity CLI generalizes `"always allow"` command permissions by prefix-matching the binary and subcommand. Shell chaining operators (`&&`, `;`, `||`, `|`), command substitutions (`$(...)`), and `cd <dir> && <cmd>` disable prefix matching and stall subagents waiting for manual human approval.
  - When the working directory is already the target project root, agents MUST prioritize workspace-relative generic commands (`pytest`, `.venv/bin/pytest tests -v`, `git status`, `git diff`) and NEVER invoke `/absolute/path/to/.venv/bin/python3 -c ...` which requires manual human approval. Reserve `-C` or `$HOME/...` paths only for cross-directory invocations when CWD is `$HOME`.
  - Preferred command examples:
    - Git: `git status`, `git diff` (or `git -C /path/to/repo <subcommand>` when running cross-directory from `$HOME`)
    - Pytest: `.venv/bin/pytest tests -v` or `pytest tests -v` (or `/path/to/repo/.venv/bin/pytest /path/to/repo/tests` from `$HOME`)
    - Unittest: `python3 -m unittest discover -s tests -v` (or `python3 -m unittest discover -s /path/to/repo/tests -v` from `$HOME`)
- **Subagent Lifecycle Cleanup**:
  - Once Phase 3/4 verification completes, the Root Orchestrator MUST terminate idle subagents via `manage_subagents(Action="kill_all")`.

---

## 4. Git Source Control & Trunk Hygiene

- **Strict Trunk Protection**:
  - Never run `git push origin main`, `git push origin master`, or force-pushes.
  - Direct local merges into `main` are prohibited.
  - Always operate on short-lived branches:
    - `feat/<feature-name>`: New functionality
    - `fix/<bug-name>`: Bug fixes
    - `docs/<doc-name>`: Documentation
    - `refactor/<target>`: Structural improvements
    - `test/<suite>`: Test additions
- **Conventional Commits 1.0.0**:
  - Every commit message must adhere to: `<type>(<scope>): <short summary>`
  - Types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`.
- **Test Before Completion**:
  - Never mark work complete without running automated tests. The harness `Stop` hook will reject termination if tests fail or if the working tree is dirty.
