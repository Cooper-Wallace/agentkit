# {{PROJECT_NAME}} - Agent Directives

{{PROJECT_DESCRIPTION}}

## Stack & Commands

- **Primary Language / Runtime**: {{PRIMARY_LANGUAGE}}
- **Single Verification Gate**: Run before committing or reporting completion:
  ```bash
  {{TEST_COMMAND}}
  ```

## Layer Import Rules

Dependencies flow strictly downward; **never import upward** across layers:

| Layer | Purpose | May Import From | Must Not Import From |
| :--- | :--- | :--- | :--- |
| `src/` | Core domain logic, adapters, CLIs | Standard library, declared deps | `tests/`, `docs/`, `evidence/` |
| `tests/` | Unit, property, & integration suites | `src/`, test fixtures | `docs/`, `evidence/` |
| `docs/` & `evidence/` | ADRs, specs, & evidence artifacts | Standalone Markdown / links | `src/` runtime execution |

## Project Rules

1. **Clean Code & DRY**: Keep functions small and single-purpose; extract shared helpers instead of duplicating logic. CLIs must use clear, non-abbreviated flags and support `--dry-run` + idempotency where mutating.
2. **Guarded Failure & Property Tests**: Every feature or bugfix requires guarded-failure / safe-degrade tests alongside happy paths, plus property-based or oracle invariants for non-trivial transforms.
3. **Fail-Fast Startup & Allow-Listed JSON Logs**: Validate required env/config and schemas at startup before processing; emit structured JSON logs containing only explicitly allow-listed non-sensitive fields.
4. **Installed-Docs Grounding**: Verify third-party APIs, CLI flags, and schemas against installed package docstrings/help or local docs before writing code—never guess signatures.
5. **One Branch Per Change & Atomic Commits**: Branch from `origin/main` (`feat/*`, `fix/*`, `docs/*`, `refactor/*`, `test/*`). Concurrent agents must use isolated git worktrees and produce one atomic Conventional Commit per deliverable.
6. **Human-Only PR Merge**: Agents may open PRs and verify CI checks, but MUST NEVER run `gh pr merge` or enable auto-merge.
7. **Dynamic Paths**: Resolve paths relative to repo root or `$HOME` / `Path.home()`; never hardcode user home directories.
8. **No Secrets**: Keep credentials and API keys in untracked `.env` files; never log, print, or commit secrets.

## Read More Only When You Need It

| Resource | When to Read |
| :--- | :--- |
| [README.md](README.md) | Setup, usage examples, and user-facing overview |
| [docs/ENGINEERING.md](docs/ENGINEERING.md) | Deep module conventions, build details, and style |
| [docs/decisions/](docs/decisions/) | Architecture Decision Records (ADRs) for past choices |
| [evidence/intent.md](evidence/intent.md) | Acceptance criteria (`AC-*`), scope boundaries, and evidence pack (`design.md`, `verify.md`, `operate.md`) |
