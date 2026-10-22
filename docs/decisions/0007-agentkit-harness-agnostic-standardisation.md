# ADR 0007: Migration to Agentkit & Harness-Agnostic Standardization

## Context
Originally created as `antigravity-standardise` (`agy-standardise`), this repository managed permissions, rules, and configuration exclusively for the Google Antigravity CLI (`agy`). As autonomous AI coding workflows matured across Cooper Wallace's workstations, several architectural limitations emerged:

1. **Harness Coupling**: The tooling was tightly bound to Antigravity CLI's configuration paths and schemas (`~/.gemini/antigravity-cli/settings.json`), preventing the reuse of permissions catalogs, MCP definitions, and lifecycle policies across alternative or emerging AI harnesses (e.g., Claude Code, Aider, OpenCode).
2. **Token Quota Exhaustion & Rate Limits**: Relying on a single root model (Gemini 4 Argon) introduces single-point-of-failure risks during heavy development sessions when daily rate limits or quota caps are encountered. A seamless mechanism to swap the root orchestrator (e.g., to Claude Opus 5.5) while retaining cost-efficient Gemini 3.8 Flash subagents was absent.
3. **Ecosystem Nomenclature Alignment**: Layer 3 of the workstation stack recently migrated from `gh-standardise` to `repokit`. Retaining `antigravity-standardise` for Layer 2 maintained an awkward asymmetry (`repokit` vs. `antigravity-standardise`).

## Decision
1. **Migration to Agentkit (`agentkit` / `ak`)**:
   - The project and primary CLI executable are renamed to **`agentkit`**, accompanied by the concise daily alias **`ak`**.
   - Legacy aliases (`agy-standardise` and `antigravity-standardise`) have been retired; the project uses `agentkit` (and concise alias `ak`) exclusively moving forward.
2. **Harness Abstraction via `--harness`**:
   - The CLI architecture abstracts harness-specific configuration paths and formats behind a modular adapter interface.
   - Commands accept `--harness <name>` (defaulting to `antigravity`). Future harness adapters (e.g., `claude`, `aider`) can map declarative permissions and MCP servers to their native configuration structures.
3. **Switchable Model Profiles**:
   - Declarative model profiles are introduced (`config/orchestration.json` / `config/profiles.json`), enabling rapid switching of cognitive tiers:
     - **`argon-flash`**: Gemini 4 Argon root planner -> Gemini 3.8 Flash subagents (Default primary configuration).
     - **`opus-flash`**: Claude Opus 5.5 root planner -> Gemini 3.8 Flash subagents (Automated fallback when hitting Argon token / rate quotas).
     - **`sonnet-flash`**: Claude Sonnet 3.7 root planner -> Gemini 3.8 Flash subagents.
     - **`pro-flash`**: Gemini 3.0 Pro root planner -> Gemini 3.8 Flash subagents.
   - Supported via `agentkit apply --model-profile <name>`.
4. **Canonicalization on `AGENTS.md`**:
   - All repository-level and global directives standardize on **`AGENTS.md`** as the canonical single source of truth.
   - Harness-specific instruction files (such as `GEMINI.md` or `CLAUDE.md`) are maintained as filesystem symlinks to `AGENTS.md`, preventing rule drift across harnesses.

## Consequences
- **Positive**: Unified workstation stack nomenclature across all 4 layers:
  - **Layer 0**: `riced-kubuntu-os` (RKO)
  - **Layer 1**: `workstation-setup`
  - **Layer 2**: `agentkit` (`ak`)
  - **Layer 3**: `repokit`
- **Positive**: Seamless token-quota mitigation: developers can instantly switch root reasoning models to Claude Opus 5.5 without altering subagent architectures or project directives.
- **Positive**: Future-proof extensibility: newly adopted AI harnesses inherit pre-approved permission catalogs, MCP configurations, and safety policies.
- **Positive**: Clean nomenclature: retiring legacy aliases ensures a single, consistent standard across all documentation, tooling, and terminal workflows.
- **Tradeoff**: Introducing profile switching and harness flags expands CLI options and test matrix scope.
