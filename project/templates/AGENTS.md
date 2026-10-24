# {{PROJECT_NAME}} - Agent Directives & Architecture

Welcome to the **{{PROJECT_NAME}}** repository. This document provides project-specific architecture context, coding standards, model tiering guidelines, and operational rules for AI pair programming sessions.

---

## 1. Project Overview

- **Description**: {{PROJECT_DESCRIPTION}}
- **Primary Language / Framework**: {{PRIMARY_LANGUAGE}}
- **Repository Architecture**:
  - `src/` or `bin/`: Core application source code
  - `tests/`: Automated unit and integration test suite
  - `docs/`: Architecture documentation and ADRs

---

## 2. Cognitive Architecture & Model Tiering

- **Root Orchestrator**:
  - **Primary Tier (`argon-flash`)**: Gemini 4 Argon (`Model: "pro"` / `"inherit"`) when Argon quota is available AND task is complex (architecture, multi-repo planning, adversarial review).
  - **Standard Tier (`flash-high`)**: Gemini 3.8 Flash with High reasoning (`Model: "flash"` / `"inherit"`) for standard feature and lifecycle workflows.
  - **Fallback Tiers (`opus-flash` / `sonnet-flash`)**: Claude Opus 5.5 / Claude Sonnet 3.7 (`Model: "opus"` / `"claude-opus-5.5"` / `"inherit"`) when Gemini quota groups are exhausted or rate-limited.
  - The Root Orchestrator MUST NEVER directly edit project source code, tests, or UI files — only artifact files in the session brain directory (e.g., `plan.md`). All code implementation, post-review bugfixes, and test updates MUST be delegated to worker subagents.
- **Subagent Complexity Tiers**:
  - `low`: `flash_lite` (file moves/renames, mechanical find-replace, formatting, doc typos, simple one-command inspection).
  - `med`: `flash` (feature code, tests, localized refactors).
  - `high`: `flash` (cross-module refactors, complex debugging, security), with escalation to `pro` (Gemini 4 Argon) for architecture, multi-repo design, or adversarial review when Argon quota is healthy.
  - Tag convention: place `[complexity: low|med|high]` at the start of subagent prompts.
- **Disjoint-File Parallel Worker Partitioning**:
  - Multi-component tasks MUST be partitioned across concurrent subagents operating on strictly disjoint file sets so workers execute in parallel without write contention.

---

## 3. Quota Awareness & Graceful Wrap-Up

- **Pre-Flight Quota Inspection**:
  - Check remaining quota with `agentkit quota status` before launching large subagent fan-outs.
- **The 3 Quota Groups**:
  - **Gemini Group**: Gemini 3.8 Flash, Gemini 3.8 Flash Lite (standard and high-throughput subagent execution).
  - **Claude + GPT Group**: Claude Opus 5.5, Claude Sonnet 3.7 (fallback root orchestrator and cross-family execution).
  - **Gemini 4 Argon Group**: Dedicated deep reasoning quota (architectural planning, complex system design, adversarial critique).
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
  - The local Obsidian AgentVault (`$HOME/Documents/AgentVault` or `$HOME/AgentVault`, configurable via `$AGENTKIT_AGENTVAULT_PATH` or `$AGENTKIT_VAULT_PATH`) serves as the long-term, human-in-the-loop memory store across sessions, adhering to the open Obsidian Skills standard and "files over apps" philosophy.
  - **Automatic Workspace Whitelisting**: The AgentVault directory (`$HOME/Documents/AgentVault` or `$HOME/AgentVault`) is automatically whitelisted in Antigravity's `trustedWorkspaces` via `agentkit apply` and `agentkit vault init` to eliminate file read/write prompts during autonomous workflows.
- **Offline Human-Agent Collaboration Protocol**:
  - **Ingest at Session Start (Phase 1)**: Check `User-Input/pending/` at session start (or run `agentkit vault ingest`) to process offline user directives, feedback, and backlog notes written between conversations. Archive processed directives to `User-Input/processed/`.
  - **Project Living Notes**: Maintain project knowledge base in `Projects/<project>.md`, keeping architecture, current branch/commit, and task checklists aligned.
  - **Handoff at Session Wrap-Up (Phase 4)**: Write structured handoff notes to `Sessions/handoff-YYYY-MM-DD-<topic>.md`, capturing completed deliverables, test verification results, and offline tasks.
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

---

## 6. Testing & Verification

Before submitting changes or marking tasks complete, run the project test suite:
```bash
{{TEST_COMMAND}}
```

---

## 7. Safety & Environment Rules

- Do not hardcode absolute system paths; resolve paths relative to the project root or use `$HOME`.
- Scripts and tooling must support `--dry-run` and be fully idempotent.
- Sensitive environment variables and secrets must remain in untracked `.env` files.
- Agent harness configuration and directives are governed by `agentkit`.
