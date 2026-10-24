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
    4. Generate a comprehensive `handoff.md` detailing finished deliverables, pending DAG items, and resumption commands.
    5. Advise the user of the exact quota reset timestamp and recommend an advisory root model switch.
- **Advisory Root Model Switching**:
  - The root session model cannot be switched programmatically via API during an active session.
  - When root quota is depleted, the agent must provide clear advisory guidance instructing the user to switch models via the chat UI picker or the `/model` slash command.

---

## 4. Engineering & Git Hygiene

- **Branching Model**: Use short-lived feature branches (`feat/`, `fix/`, `docs/`, `refactor/`, `test/`).
- **Commit Format**: Conventional Commits 1.0.0 (`type(scope): summary`).
- **Merge Standard**: Squash & Merge only via GitHub Pull Requests, governed by `repokit`.

---

## 5. Testing & Verification

Before submitting changes or marking tasks complete, run the project test suite:
```bash
{{TEST_COMMAND}}
```

---

## 6. Safety & Environment Rules

- Do not hardcode absolute system paths; resolve paths relative to the project root or use `$HOME`.
- Scripts and tooling must support `--dry-run` and be fully idempotent.
- Sensitive environment variables and secrets must remain in untracked `.env` files.
- Agent harness configuration and directives are governed by `agentkit`.
