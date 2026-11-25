# Agentic Workflow: {{PROJECT_NAME}}

> **Stack**: {{PRIMARY_LANGUAGE}} | **Verify Gate**: `{{TEST_COMMAND}}`

## 1. Tooling & Environment

- **Orchestration**: `agentkit` directives, isolated git worktrees (`Workspace: "share"` / `git worktree add`), and `{{TEST_COMMAND}}` verification gate.
- **Grounding**: Inspect installed package docs/signatures and local repo files before editing.

## 2. Phase 0: Contracts-First Decomposition

Before dispatching parallel implementation workers:
1. Lock data schemas, typed interfaces/seams, and `AC-*` mappings in `evidence/intent.md` and `evidence/design.md`.
2. Partition deliverables across **strictly disjoint file sets** so concurrent subagents never collide on shared files or `.git/index.lock`.

## 3. Parallel Worktrees & Steering Prompts

- **Branching**: Fetch `origin/main` first; create one branch per discrete change (`feat/*`, `fix/*`, `test/*`).
- **Complexity Tagging**: Prefix every `invoke_subagent` prompt with `[complexity: low|med|high]`:
  - `low`: mechanical edits, renames, doc typos
  - `med`: feature modules, unit/property tests, localized refactors
  - `high`: cross-module architecture, security boundaries, adversarial diff review
- **Atomic Commits**: Each worker finishes with a passing `{{TEST_COMMAND}}` run and one atomic Conventional Commit (`<type>(<scope>): <summary>`).

## 4. Human Interventions

- Approving Phase 0 architectural contracts and scope trade-offs.
- Provisioning credentials/secrets in local `.env` or CI secret stores.
- **Trunk Promotion**: Only humans execute PR merges into `main` via the GitHub web UI after all checks pass.

## 5. Review & Verification Gates

1. Run `{{TEST_COMMAND}}` locally after final edits (must include happy-path + guarded-failure tests).
2. Perform adversarial diff review against `evidence/intent.md` (`AC-*`) and `AGENTS.md` layer import rules.
3. Update `evidence/verify.md` (including **Known Gaps**) before opening a PR.
