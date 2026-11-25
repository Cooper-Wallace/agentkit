# Agent Developer Directives & Cross-Machine Standards

Welcome to Cooper Wallace's development workspace. This document establishes universal behavioral guidelines, git hygiene rules, cognitive model tiering, quota management, safety protocols, and verification standards for AI pair programming sessions across all workstations (managed via `agentkit` and `repokit`).

---

## 1. Engineering Philosophy & Persona

- **Clean Architecture & Minimal Dependencies**: Favor clear, maintainable solutions over complex abstractions. Use standard library capabilities when appropriate before adding external dependencies.
- **Standardized Repository Architecture**:
  - `bin/` or `src/`: Core executables and application logic.
  - `tests/`: Automated unit and integration test suite.
  - `docs/`: Architecture documentation and Architecture Decision Records (ADRs).
  - Agent Harness Repositories: Bifurcated into `harness/` (`plugins/`, `hooks/`, `rules/`, `skills/`) for harness-level governance and runtime hooks, and `project/` (`skills/`, `tools/`, `templates/`) for reusable cross-project domain skills, utility tools, and scaffolding templates.
- **Idempotent Automation**: All shell scripts and automation tools must be safe to execute multiple times without causing side effects, duplicated lines, or state corruption. Include a `--dry-run` flag wherever files are modified or external services are called.
- **Documentation Integrity**: Maintain existing comments, type hints, and docstrings. When refactoring or updating code, ensure documentation accurately mirrors the new behavior.
- **Minimally Verbose & Deterministic CLI Design**:
  - Design CLIs across projects to be clean, self-documenting, and minimally verbose without unnecessary abbreviation or cryptic shortening.
  - Because developer tools in this ecosystem are predominantly orchestrated agentically (via AI agent workflows, tool calls, scripts, and CI runners), semantic clarity, predictability, and unambiguous command shapes take precedence over rapid human keystroke shortcuts.
  - Top-level operational shorthands are restricted to explicit, high-frequency state toggles (such as `agentkit wake` and `agentkit sleep` for immediate manual power overrides), while arbitrary contractions (`caff`, `cfg`, `mgr`, `sync-tmp`) are strictly prohibited.

---

## 2. Cognitive Architecture & Model Tiering

- **Root Orchestrator (Switchable High-Tier Profiles)**:
  - The root session operates under a high-capability reasoning model with switchable profiles:
    - **Primary Tier (`carbon-flash` [default] / `argon-flash`)**: **Orchestrator (Carbon)** (`gemini-pro-orchestrator`) and **Gemini 4 Carbon** (`High` / `Medium` / `Low`), alongside **Orchestrator (Argon)** (`gemini-pro-orchestrator-argon`) and **Gemini 4 Argon** (`High` / `Medium` / `Low`) (`Model: "pro"` / `"inherit"`) when Carbon/Argon quota is available AND the task involves complex architecture, multi-repo planning, or adversarial diff auditing.
      - **Two-Phase Plan & Execute (`Orchestrator (Carbon)` / `Orchestrator (Argon)`)**: Phase 1 runs on the Carbon/Argon planning model in read-only `plan_mode` to investigate, resolve design decisions, and hand off an approved plan via `exit_plan_mode`. Phase 2 runs in-place on a fast execution model (`Gemini 4 Neon`, `Gemini 4 Hydrogen`, or `Gemini 3.8 Flash`) to execute the approved plan or dispatch parallel subagents across disjoint file sets, returning to the planner on `[plan review]`.
    - **Standard Tier (`neon-high` / `hydrogen-high` / `flash-high`)**: **Orchestrator (Neon)** (`gemini-pro-orchestrator-neon`) and **Gemini 4 Neon** (`High` / `XHigh` / `Medium` / `Low`; profile `neon-high`; default `settings.default.json` model `Gemini 4 Neon (High)`), alongside **Gemini 4 Hydrogen** (`High` / `XHigh` / `Medium` / `Low`; profile `hydrogen-high`) and **Gemini 3.8 Flash (High)** (profile `flash-high`) in the Gemini quota group for fast feature and lifecycle execution with zero Argon/Carbon quota consumption.
    - **Fallback Tiers (`opus-flash` / `sonnet-flash`)**: **Claude Opus 5.5** / **Claude Sonnet 5.5** (`Low` / `Medium` / `High`; `Model: "opus"` / `"inherit"`) when Gemini quota groups are exhausted or rate-limited.
  - The Root Orchestrator (Orchestrator (Carbon) / Carbon 4 / Orchestrator (Argon) / Argon 4 / Orchestrator (Neon) / Neon High / Hydrogen High / Flash High / Opus 5.5 / Sonnet 5.5) is responsible for high-level requirement analysis, architectural contracts, task DAG generation, and adversarial reviews.
  - **Standalone Root Orchestrator Isolation**: When not in a post-`exit_plan_mode` execution phase, the Root Orchestrator (`Gemini 4 Carbon`, `Gemini 4 Argon`, `Claude Opus 5.5`, `Claude Sonnet 5.5`) MUST NEVER directly edit project source code, tests, or UI files (via `edit_file` or `write_file`) — only artifact files in the session brain directory (e.g., `plan.md`) may be written directly. All standalone code implementation, post-review bugfixes, threshold tuning, and test updates MUST be delegated to worker subagents (`invoke_subagent`).
- **Subagent Complexity Tiers**:
  - Subagents are partitioned into discrete complexity tiers based on task scope and routed accordingly:
    - **`low`**: `flash_lite` (file moves/renames, mechanical find-replace, formatting, doc typos, simple one-command inspection).
    - **`med`**: `flash` (feature code, tests, localized refactors).
    - **`high`**: `flash` (cross-module refactors, complex debugging, security), with escalation to `pro` (Gemini 4 Carbon / Gemini 4 Argon) for architecture, multi-repo design, or adversarial review when Carbon/Argon quota is healthy.
  - **Tag Convention**: Orchestrators must place `[complexity: low|med|high]` at the start of subagent prompts.
- **Disjoint-File Parallel Worker Partitioning**:
  - Multi-component tasks (e.g., backend logic, frontend UI, documentation, and unit tests) MUST be partitioned across concurrent subagents operating on strictly disjoint file sets so workers execute in parallel without write contention.

---

## 3. Quota Awareness & Graceful Wrap-Up

- **Pre-Flight Quota Inspection**:
  - Check remaining quota with `agentkit quota status` before launching large subagent fan-outs.
- **The 3 Quota Groups**:
  - **Gemini Group** (`gemini`): `Orchestrator (Neon)`, `Gemini 4 Neon` (`Low` / `Medium` / `High` / `XHigh`), `Gemini 4 Hydrogen` (`Low` / `Medium` / `High` / `XHigh`), `Gemini 3.8 Flash` (`Low` / `Medium` / `High`), `Gemini 3.8 Flash Lite` (`flash_lite`) — standard root execution and high-throughput subagent execution.
  - **Claude + GPT Group** (`claude`): `Claude Opus 5.5`, `Claude Sonnet 5.5`, `GPT-OSS 120B` (fallback root orchestrator and cross-family execution).
  - **Gemini 4 Carbon / Argon Group** (`argon` canonical group ID): `Orchestrator (Carbon)`, `Gemini 4 Carbon` (`Low` / `Medium` / `High`), `Orchestrator (Argon)`, `Gemini 4 Argon` (`Low` / `Medium` / `High`) — dedicated deep reasoning, two-phase planning, and adversarial critique quota.
- **Automated Routing**:
  - The `quota-guardian` plugin automatically intercepts subagent delegation via `PreToolUse`.
  - When the target model's quota group drops below **15% remaining capacity**, `quota-guardian` automatically reroutes subagents to a healthy fallback group without interrupting execution.
- **Graceful Wrap-Up Circuit Breaker**:
  - When all eligible quota groups drop below **5% remaining capacity**, `quota-guardian` denies further delegation with `QUOTA WRAP-UP:`.
  - Agents MUST immediately follow the `quota-wrapup` protocol:
    1. Cease spawning new subagents.
    2. Run local test suites to verify system stability.
    3. Create a clean WIP commit on the active feature branch (`git commit -m "chore(wip): save state prior to quota wrap-up"`).
    4. Generate a comprehensive handoff note in the Obsidian AgentVault (`$HOME/Documents/AgentVault/Sessions/handoff-YYYY-MM-DD-quota-wrapup.md`), update `Projects/<project>.md`, and mirror `handoff.md` in conversation artifacts.
    5. Advise the user of the exact quota reset timestamp and recommend an advisory root model switch.
- **Advisory Root Model Switching**:
  - The root session model cannot be switched programmatically via API during an active session.
  - When root quota is depleted, the agent must provide clear advisory guidance instructing the user to switch models via the chat UI picker or the `/model` slash command.

---

## 4. Perpetual Long-Term Memory & Offline Collaboration (Obsidian AgentVault)

- **Local Obsidian AgentVault**:
  - The local Obsidian AgentVault (`$HOME/Documents/AgentVault` or `$HOME/AgentVault`, configurable via `$AGENTKIT_AGENTVAULT_PATH` or `$AGENTKIT_VAULT_PATH`) serves as the durable, human-in-the-loop long-term memory layer across agent conversations, adhering to the open Obsidian Skills standard and "files over apps" philosophy.
  - **Automatic Workspace Whitelisting**: The AgentVault directory (`$HOME/Documents/AgentVault` or `$HOME/AgentVault`) as well as core repositories (`agentkit` and `repokit` at `$HOME/Projects/agentkit` and `$HOME/Projects/repokit`) are automatically whitelisted in Antigravity's `trustedWorkspaces` via `agentkit apply` and `agentkit vault init` to eliminate file read/write prompts during autonomous workflows and cross-repo introspection.
- **Bifurcated Hybrid Storage Contract**:
  - **Repo-Specific Specifications**: Architecture Decision Records (ADRs) and static technical specifications live directly inside `<repo>/docs/` (`docs/decisions/`) versioned alongside project code.
  - **Cross-Repo System State**: High-level cross-repo architecture maps (`workstation-stack.canvas`), living project boards (`Projects/<repo>.md`), offline input queues (`User-Input/`), and immutable session handoffs (`Sessions/`) live in the Git-backed `AgentVault` repository.
- **Offline Human-Agent Collaboration Protocol**:
  - **Ingest at Session Start (Phase 1)**: Agents must check `User-Input/pending/` at the beginning of each session (or run `agentkit vault ingest`) to process offline user directives, feedback, and backlog notes written between conversations. Processed directives are archived to `User-Input/processed/`.
  - **Project Living Notes**: Agents maintain project knowledge bases in `Projects/<project>.md` (e.g. `agentkit.md`, `repokit.md`), keeping active architecture, current branch/commit, and task checklists synchronized.
  - **Handoff at Session Wrap-Up (Phase 4)**: Agents author structured handoff notes to `Sessions/handoff-YYYY-MM-DD-<topic>.md`, capturing completed deliverables, test verification results, and offline tasks.
  - **Visual Architecture with JSON Canvas**: Visual architecture maps and multi-agent DAGs are maintained in `Architecture/` using the open JSON Canvas specification (`.canvas`).

---

## 5. Git Workflow & Version Control Standards

- **Branching Strategy: One Branch Per Change**:
  - Every discrete task, feature, bug fix, refactor, or test improvement must operate on its own dedicated short-lived feature branch branched directly from `main`:
    - `feat/<feature-name>`: New functionality
    - `fix/<bug-name>`: Bug fixes
    - `docs/<doc-name>`: Documentation updates
    - `refactor/<target>`: Structural code improvements
    - `test/<suite>`: Test additions or refactoring
  - Before creating a feature branch or isolated worktree, agents MUST run `git fetch origin main` and branch from `origin/main` (`git worktree add -b feat/<name> <path> origin/main`) so worktrees are never branched from a stale local `main` after GitHub web UI merges.
  - Never stack unrelated features or accumulate unrelated changes on the same branch. Keep scope narrow, focused, and atomic.
- **Commit Hygiene: One Atomic Commit Per Subagent**:
  - Each worker subagent assigned to a deliverable must produce a single atomic Conventional Commit (`<type>(<scope>): <short summary>`) representing its completed, tested unit of work.
  - Workers must never leave uncommitted changes or produce messy WIP commits.
  - Commit messages must strictly follow Conventional Commits 1.0.0: `<type>(<scope>): <short summary>`
  - Types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`.
  - Use imperative, present-tense verbs ("add", "fix", "update", not "added" or "fixes").
  - Scope is optional but encouraged for modular repositories.
- **Multi-Agent Concurrency: Isolated Git Worktrees for Concurrent Threads**:
  - When multiple agent threads or subagents work concurrently within the same repository, they MUST operate in isolated git worktrees (`Workspace: "share"` in Antigravity or `git worktree add`) to prevent `.git/index.lock` collisions, dirty working tree races, and write contention.
  - Concurrent workers must never operate on the same physical working directory simultaneously.
- **Trunk Hygiene & Merge Governance**:
  - Repositories follow the **Squash and Merge** model governed by `repokit`.
  - Squash titles default to `PR_TITLE` and bodies to `PR_BODY`. Direct pushes to `main` and local trunk merges are strictly prohibited.
- **Human-Only Trunk Promotion & Auto-Merge Prohibition**:
  - Merging Pull Requests into `main` is strictly a human-in-the-loop action. Autonomous agents MUST NEVER execute `gh pr merge`, enable GitHub auto-merge, or automate trunk merging. Agents are restricted to opening Pull Requests (`gh pr create`), posting adversarial review audits, and verifying check suites (`gh pr checks`). PR promotion into `main` must only occur via manual human review and approval in the GitHub web UI once all quality gates pass.

---

## 6. Safety Protocols & System Boundaries

- **Dynamic Path Resolution**:
  - Never hardcode the absolute home directory (`/home/cooper`).
  - Always resolve dynamically using `$HOME`, `Path.home()`, or relative project roots.
- **Destructive Command Gatekeeping**:
  - Destructive file removals (`rm -rf`, recursive deletion) and raw disk operations (`dd`, `mkfs`) are strictly prohibited from automated execution without explicit user review.
  - Safe operations (`mkdir`, `touch`, `cp`, `mv`, `chmod +x`) are permitted.
- **Secrets & Credentials Protection**:
  - Never track, print, or commit `.env` files, OAuth tokens, SSH keys, or private API keys.
  - Ensure sensitive files are explicitly listed in `.gitignore`.

---

## 7. Verification & Testing Standards

- **Test Before Completion**:
  - Never mark a feature, fix, or refactor complete without verifying functionality through automated tests or deterministic manual validation steps.
  - Test commands:
    - Python: `python3 -m unittest discover tests -v` or `pytest`
    - Node.js: `npm test` or `pnpm test`
    - Go: `go test ./...`
    - Rust: `cargo test`
    - Java: `mvn test`
- **Guarded Failure & Safe Degrade Coverage**:
  - Every feature or bugfix test suite must include at least one guarded failure, validation rejection, or safe degrade-mode test alongside happy-path tests.
- **Audit & Status Inspection**:
  - Inspect `git status` and `git diff` before committing to ensure no unintended scratch files or debug statements leak into repositories.

---

## 8. Tooling & Ecosystem Awareness

- **`repokit`**: Automates GitHub repository governance, branch protection rulesets, squash merge enforcement, and semantic PR checks.
- **`riced-kubuntu-os` (RKO)**: Manages Kubuntu OS packages, KDE Plasma 6 configurations, and system-level dotfiles.
- **`agentkit`**: Audits and synchronizes agent harness settings, command permissions, model profiles, and developer directives across machines.
