# Autonomous Multi-Model Orchestration Standards

Welcome to the enterprise autonomous development environment. These directives govern multi-agent task distribution, context preservation, quota safety, and source control hygiene.

---

## 1. Cognitive Architecture & Model Tiering

- **Root Orchestrator (Switchable High-Tier Profiles)**:
  - The root session operates under a high-capability reasoning model with switchable profiles:
    - **Primary Tier (`argon-flash`)**: Gemini 4 Argon (`Model: "pro"` / `"inherit"`) when Argon quota is available AND the task involves complex architecture, multi-repo planning, or adversarial diff auditing.
    - **Standard Tier (`flash-high`)**: Gemini 3.8 Flash with High reasoning (`Model: "flash"` / `"inherit"`) for standard feature and lifecycle workflows (replaces retired Gemini 3.0/3.1 Pro).
    - **Fallback Tiers (`opus-flash` / `sonnet-flash`)**: Claude Opus 5.5 / Claude Sonnet 3.7 (`Model: "opus"` / `"claude-opus-5.5"` / `"inherit"`) when Gemini quota groups are exhausted or rate-limited.
  - The Root Orchestrator (Argon 4 / Flash High / Opus 5.5 / Sonnet 3.7) is responsible for high-level requirement analysis, architectural contracts, task DAG generation (Phase 1 planning), and adversarial reviews (Phase 3 review).
  - The Root Orchestrator MUST NEVER directly edit project source code, tests, or UI files (via `edit_file` or `write_file`) — only artifact files in the session brain directory (e.g., `plan.md`) may be written directly by the root orchestrator. All code implementation, post-review bugfixes, threshold tuning, and test updates — even single-file follow-up adjustments after user feedback — MUST be delegated to Flash worker subagents.
- **Subagent Complexity Tiers**:
  - Subagents are partitioned into discrete complexity tiers based on task scope and routed accordingly:
    - **`low`**: `flash_lite` (file moves/renames, mechanical find-replace, formatting, doc typos, simple one-command inspection).
    - **`med`**: `flash` (feature code, tests, localized refactors).
    - **`high`**: `flash` (cross-module refactors, complex debugging, security), with escalation to `pro` (Gemini 4 Argon) for architecture, multi-repo design, or adversarial review when Argon quota is healthy.
  - **Tag Convention**: Orchestrators must place `[complexity: low|med|high]` at the start of subagent prompts.
- **Worker Subagents (Gemini 3.8 Flash / Flash Lite)**:
  - Code implementation, localized refactoring, documentation syncing, and test runs MUST be delegated to subagents launched with `Model: "flash"` (or `flash_lite` for low-complexity tasks) and `Workspace: "share"` (or `Workspace: "inherit"` when the active workspace is `$HOME` rather than a VCS repository root).
  - **One Atomic Commit Per Subagent**: Each worker subagent assigned to a deliverable must produce a single atomic Conventional Commit (`<type>(<scope>): <short summary>`) representing its completed, tested unit of work. Workers must not leave uncommitted changes or produce messy WIP commits.
  - **Disjoint-File Parallel Worker Partitioning & Isolated Worktrees**: Multi-component tasks (e.g., backend logic, frontend UI, documentation, and unit tests) MUST be partitioned across concurrent subagents operating on strictly disjoint file sets. Furthermore, when multiple threads or subagents work concurrently within the same repository, they MUST execute in isolated git worktrees (`Workspace: "share"` in Antigravity or `git worktree add`) to prevent `.git/index.lock` collisions, dirty working tree races, and write contention. Concurrent workers must never operate on the same physical working directory simultaneously.
- **Adversarial Auditor (Argon 4 / Flash High / Opus 5.5 / Sonnet 3.7)**:
  - Before requesting task completion or submitting a Pull Request, an adversarial review must evaluate the `git diff` against the initial specification.
- **Crash & Compaction Recovery Protocol**:
  - Whenever resuming after a crash, context compaction, or interrupted turn, the Root Orchestrator MUST check subagent completeness via `manage_subagents(Action="list")`, `transcript.jsonl`, and `git status` before dispatching new work.

---

## 2. Quota Awareness & Graceful Wrap-Up

- **Pre-Flight Quota Inspection**:
  - Run `agentkit quota status` before dispatching large subagent fan-outs to verify quota headroom across provider groups.
- **The 3 Quota Groups**:
  - **Gemini Group**: Gemini 3.8 Flash, Gemini 3.8 Flash Lite (standard and high-throughput subagent execution).
  - **Claude + GPT Group**: Claude Opus 5.5, Claude Sonnet 3.7 (fallback root orchestrator and cross-family execution).
  - **Gemini 4 Argon Group**: Dedicated deep reasoning quota (architectural planning, complex system design, adversarial critique).
- **Automated Fallback Routing**:
  - The `quota-guardian` plugin automatically intercepts `invoke_subagent` calls via `PreToolUse`.
  - When the target model's quota group drops below **15% remaining capacity**, `quota-guardian` dynamically reroutes the subagent to a healthy fallback group without interrupting execution.
- **Graceful Wrap-Up Circuit Breaker**:
  - When all eligible quota groups drop below **5% remaining capacity**, `quota-guardian` denies further subagent delegation with `QUOTA WRAP-UP:`.
  - Agents MUST immediately follow the `quota-wrapup` protocol:
    1. Cease spawning new subagents.
    2. Run local test suites to verify system stability.
    3. Create a clean WIP commit on the active feature branch (`git commit -m "chore(wip): save state prior to quota wrap-up"`).
    4. Generate a comprehensive `handoff.md` detailing finished deliverables, pending DAG items, and resumption commands.
    5. Advise the user of the exact quota reset timestamp and recommend an advisory root model switch.
- **Advisory Root Model Switching**:
  - The root session model cannot be switched programmatically via API during an active session.
  - When root quota is depleted, the agent must provide clear advisory guidance instructing the user to switch models via the chat UI picker or the `/model` slash command.

---

## 3. Context Window Preservation, Token Suppression & Cross-Repo Memory

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
- **Perpetual Cross-Repository Memory (Obsidian AgentVault)**:
  - Maintain cross-repo architectural contracts, living project documentation, and system boundaries across the 4-layer workstation stack (`riced-kubuntu-os`, `workstation-setup`, `agentkit` [CLI `agentkit`], `repokit`) in the local **Obsidian AgentVault** (`$HOME/Documents/AgentVault` or `$HOME/AgentVault`, configurable via `$AGENTKIT_AGENTVAULT_PATH` or `$AGENTKIT_VAULT_PATH`).
  - **Automatic Workspace Whitelisting**: AgentVault (`$HOME/Documents/AgentVault` or `$HOME/AgentVault`) is automatically whitelisted in Antigravity's `trustedWorkspaces` via `agentkit apply` and `agentkit vault init` to prevent confirmation prompts during autonomous execution.
  - **Offline Human-Agent Collaboration Protocol**: The Root Orchestrator inspects `User-Input/pending/` and target project notes in `Projects/<project>.md` during Phase 1 planning to ingest offline user feedback and directives, and writes structured session summaries to `Sessions/handoff-YYYY-MM-DD-<topic>.md` at Phase 4 wrap-up. Visual architecture diagrams are maintained using JSON Canvas (`.canvas`) in `Architecture/`.
- **Cross-Repo Structural AST Compression (`repomix` MCP)**:
  - When inspecting sibling repositories, prefer AST-compressed structural skeletons (`repomix` MCP or `repomix --compress`) over raw multi-file reads to reduce cross-repo token ingestion by ~70%.

---

## 4. Subagent Autonomy & Zero-Prompt Command Discipline

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

## 5. Git Source Control & Trunk Hygiene

- **Branching Strategy: One Branch Per Change**:
  - Every discrete task, feature, bug fix, or refactor must operate on its own dedicated short-lived feature branch branched directly from `main`:
    - `feat/<feature-name>`: New functionality
    - `fix/<bug-name>`: Bug fixes
    - `docs/<doc-name>`: Documentation
    - `refactor/<target>`: Structural improvements
    - `test/<suite>`: Test additions
  - Never stack unrelated features or accumulate unrelated changes on the same branch.
- **Commit Hygiene: One Atomic Commit Per Subagent**:
  - Each worker subagent assigned to a deliverable must produce a single atomic Conventional Commit (`<type>(<scope>): <short summary>`) representing its completed, tested unit of work.
  - Workers must not leave uncommitted changes or produce messy WIP commits.
  - Every commit message must adhere to Conventional Commits 1.0.0: `<type>(<scope>): <short summary>`
  - Types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`.
- **Multi-Agent Concurrency: Isolated Git Worktrees for Concurrent Threads**:
  - When multiple agent threads or subagents work concurrently within the same repository, they MUST operate in isolated git worktrees (`Workspace: "share"` in Antigravity or `git worktree add`) to prevent `.git/index.lock` collisions, dirty working tree races, and write contention. Concurrent workers must never operate on the same physical working directory simultaneously.
- **Strict Trunk Protection & Governance**:
  - Never run `git push origin main`, `git push origin master`, or force-pushes. Direct local merges into `main` are prohibited.
  - Repositories follow the Squash and Merge model governed by `repokit`.
- **Test Before Completion**:
  - Never mark work complete without running automated tests. The harness `Stop` hook will reject termination if tests fail or if the working tree is dirty.

---

## 6. Tooling & Ecosystem Awareness

- **Layer 0**: `riced-kubuntu-os` (RKO) - Kubuntu OS packages, KDE Plasma 6 configurations, and system-level dotfiles.
- **Layer 1**: `workstation-setup` - Developer toolchains (Node.js, Python, uv), shell profiles (`~/.profile`, `~/.bashrc`), keyrings, and environment bridging.
- **Layer 2**: `agentkit` (CLI `agentkit`) - Audits and synchronizes agent harness settings, command permissions, global agent directives (`AGENTS.md`), MCP server catalogs, and plugin lifecycle hooks.
- **Layer 3**: `repokit` - Automates GitHub repository governance, branch protection rulesets, squash merge enforcement, and semantic PR checks.
