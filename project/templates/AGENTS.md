# {{PROJECT_NAME}} - Agent Directives & Architecture

Welcome to the **{{PROJECT_NAME}}** repository. This document provides project-specific architecture context, coding standards, and operational guidelines for AI pair programming sessions.

---

## 1. Project Overview

- **Description**: {{PROJECT_DESCRIPTION}}
- **Primary Language / Framework**: {{PRIMARY_LANGUAGE}}
- **Repository Architecture**:
  - `src/` or `bin/`: Core application source code
  - `tests/`: Automated unit and integration test suite
  - `docs/`: Architecture documentation and ADRs

---

## 2. Engineering & Git Hygiene

- **Branching Model**: Use short-lived feature branches (`feat/`, `fix/`, `docs/`, `refactor/`, `test/`).
- **Commit Format**: Conventional Commits 1.0.0 (`type(scope): summary`).
- **Merge Standard**: Squash & Merge only via GitHub Pull Requests, governed by `repokit`.

---

## 3. Testing & Verification

Before submitting changes or marking tasks complete, run the project test suite:
```bash
{{TEST_COMMAND}}
```

---

## 4. Safety & Environment Rules

- Do not hardcode absolute system paths; resolve paths relative to the project root or use `$HOME`.
- Scripts and tooling must support `--dry-run` and be fully idempotent.
- Sensitive environment variables and secrets must remain in untracked `.env` files.
- Agent harness configuration and directives are governed by `agentkit` / `ak`.
