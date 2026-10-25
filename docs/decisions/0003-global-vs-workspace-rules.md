# ADR 0003: Separation of Global Directives and Workspace Rules

## Context
AI developer directives can be placed in global configurations (`~/.gemini/config/` or `~/.gemini/GEMINI.md`) or scoped within individual repository workspaces (`<repo>/GEMINI.md`, `<repo>/.agents/rules/`).

Placing extensive, language-specific rules globally pollutes the context window for unrelated projects. Conversely, having no global rules means scratch sessions and new repositories lack core engineering principles.

## Decision
1. **Global Directives (`~/.gemini/GEMINI.md` and `~/.gemini/AGENTS.md`)**:
   Establish universal developer principles that are language-agnostic: Conventional Commits, clean architecture, idempotence, dynamic path requirements, and verification before task completion.
2. **Project-Scoped Directives (`templates/project-GEMINI.md`)**:
   Language-specific dependencies, framework architectures, and custom test runners are scaffolded into individual project repositories using `agentkit template`.

## Consequences
- **Positive**: Lightweight global context window overhead.
- **Positive**: Consistent baseline developer persona across all terminal sessions.
- **Positive**: Individual repositories remain self-documenting for team members.
