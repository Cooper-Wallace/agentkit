# Agent Developer Directives & Cross-Machine Standards

Welcome to Cooper Wallace's development workspace. This document establishes universal behavioral guidelines, git hygiene rules, safety protocols, and verification standards for AI pair programming sessions across all workstations (managed via `agentkit` and `repokit`).

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

---

## 2. Git Workflow & Version Control Standards

- **Branching Strategy**:
  - Always work in short-lived feature branches branched from `main`:
    - `feat/<feature-name>`: New functionality
    - `fix/<bug-name>`: Bug fixes
    - `docs/<doc-name>`: Documentation updates
    - `refactor/<target>`: Structural code improvements
    - `test/<suite>`: Test additions or refactoring
- **Conventional Commits 1.0.0**:
  - Commit messages must strictly follow the format: `<type>(<scope>): <short summary>`
  - Types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`
  - Use imperative, present-tense verbs ("add", "fix", "update", not "added" or "fixes").
  - Scope is optional but encouraged for modular repositories.
- **Trunk Hygiene & Merge Governance**:
  - Repositories follow the **Squash and Merge** model governed by `repokit`.
  - Squash titles default to `PR_TITLE` and bodies to `PR_BODY`.

---

## 3. Safety Protocols & System Boundaries

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

## 4. Verification & Testing Standards

- **Test Before Completion**:
  - Never mark a feature, fix, or refactor complete without verifying functionality through automated tests or deterministic manual validation steps.
  - Test commands:
    - Python: `python3 -m unittest discover tests -v` or `pytest`
    - Node.js: `npm test` or `pnpm test`
    - Go: `go test ./...`
    - Rust: `cargo test`
    - Java: `mvn test`
- **Audit & Status Inspection**:
  - Inspect `git status` and `git diff` before committing to ensure no unintended scratch files or debug statements leak into repositories.

---

## 5. Tooling & Ecosystem Awareness

- **`repokit`**: Automates GitHub repository governance, branch protection rulesets, squash merge enforcement, and semantic PR checks.
- **`riced-kubuntu-os` (RKO)**: Manages Kubuntu OS packages, KDE Plasma 6 configurations, and system-level dotfiles.
- **`agentkit`**: Audits and synchronizes agent harness settings, command permissions, model profiles, and developer directives across machines.
