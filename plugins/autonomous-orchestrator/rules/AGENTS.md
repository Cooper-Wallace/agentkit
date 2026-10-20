# Autonomous Multi-Model Orchestration Standards

Welcome to the enterprise autonomous development environment. These directives govern multi-agent task distribution, context preservation, and source control hygiene.

---

## 1. Cognitive Architecture & Model Tiering

- **Root Orchestrator (Gemini 4 Argon)**:
  - The root session operates under Gemini 4 Argon. It is responsible for high-level requirement analysis, architectural contracts, task DAG generation, and adversarial reviews.
  - The root session MUST NEVER perform repetitive file edits, iterative syntax refactoring, or voluminous test runs directly.
- **Worker Subagents (Gemini 3.8 Flash)**:
  - Code implementation, localized refactoring, documentation syncing, and test runs MUST be delegated to subagents launched with `Model: "flash"` and `Workspace: "share"`.
- **Adversarial Auditor (Gemini 4 Argon)**:
  - Before requesting task completion or submitting a Pull Request, an adversarial review must evaluate the `git diff` against the initial specification.

---

## 2. Context Window Preservation & Anti-Pollution Rules

- **Zero-Pollution Delegation**:
  - Ephemeral worker subagents discard voluminous build logs, 500-line test outputs, and scratch explorations upon termination.
  - Workers must return only concise completion summaries (10-20 lines) to the root orchestrator.
- **Progressive Skill Retrieval**:
  - Do not load monolithic documentation upfront. Retrieve runbooks and specific guides on-demand via skills.
- **Artifact-Driven State Tracking**:
  - Maintain the source-of-truth task list and architectural contracts in markdown artifacts (`.md` files in the artifact directory).

---

## 3. Git Source Control & Trunk Hygiene

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
