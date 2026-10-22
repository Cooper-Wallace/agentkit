# ADR 0008: Harness/Project Bifurcation, Exclusive Agentkit Naming, and AugursAptitude Consolidation

## Context
As `agentkit` matured from an Antigravity-specific configuration manager into a universal Layer 2 agent architecture platform across Cooper Wallace's workstations, three architectural friction points arose:

1. **Dual Naming & Alias Ambiguity (`ak` vs. `agentkit`)**:
   - The CLI was previously invocable as both `agentkit` and its short alias `ak`. While convenient for brief manual typing, dual aliases introduced permission fragmentation in `config/permissions.json`, documentation inconsistency, ambiguity in automated scripts, and potential collisions with external CLI packages (e.g., AlmaLinux `ak`, Amazon Alexa `ak`).
   - Agent harness permission systems generalize auto-approvals via prefix-matching (`command(agentkit)`). Supporting multiple aliases forced duplicated entries in security catalogs and created risk when subagents invoked one binary form while permissions approved another.
2. **Entangled Harness Infrastructure and Domain Knowledge**:
   - The repository previously interleaved agent harness configurations (`plugins/`, `hooks/`, `rules/`) with project scaffolding templates (`templates/`) at the root level.
   - As agent capabilities expanded, the repository increasingly served two distinct roles:
     - **Harness Governance**: Managing harness-level lifecycle hooks (`PreToolUse`, `PostToolUse`, `Stop`), Headroom CCR token compression, cognitive model profiles (`argon-flash`, `opus-flash`), and universal agent behavioral directives (`AGENTS.md`).
     - **Project Enablement**: Providing reusable engineering skills, analytical tools, and project directives to downstream software applications.
   - Without an explicit architectural boundary, harness runtime mechanisms risked bleeding into domain workflows.
3. **Redundant Skill Maintenance Across Quant Projects (`AugursAptitude`)**:
   - The standalone repository `AugursAptitude` was created to house shared machine learning, time-series modeling, and data lake pipelines across quantitative trading (`AugursBazaar`) and baseball player projection analytics (`AugursSaber`).
   - Maintaining `AugursAptitude` as a separate git repository introduced versioning friction, repository sprawl, and synchronization lag whenever foundational ML skills or DuckDB pipeline guidelines evolved.

---

## Decision

1. **Standardize Exclusively on `agentkit` (Retire `ak`)**:
   - The primary CLI executable is exclusively named **`agentkit`**. The alias `ak` is removed from `config/permissions.json`, documentation, shell guides, and templates.
   - All harness permissions, documentation, and programmatic invocations standardize on `agentkit`, ensuring deterministic prefix-matching and zero naming ambiguity.

2. **Bifurcate Repository Architecture (`harness/` vs. `project/`)**:
   - Restructure the repository into two strictly partitioned domains:
     - **`harness/` (Agent Harness Runtime & Governance)**:
       - `plugins/`: Multi-model orchestration plugins (e.g., `autonomous-orchestrator` with `hooks.json` and lifecycle guards).
       - `hooks/`: Declarative lifecycle hook manifests and execution guards.
       - `rules/`: Global agent developer directives and behavioral rules (`AGENTS.md`).
       - `skills/`: Harness-level operational skills (e.g., `auto-lifecycle` runbook for multi-model autonomous coordination).
     - **`project/` (Reusable Knowledge, Tools & Scaffolding)**:
       - `skills/`: Reusable domain engineering and machine learning skills.
       - `tools/`: Shared project utility scripts, diagnostic helpers, and data tools.
       - `templates/`: Project scaffolding templates (`AGENTS.md`, `project-GEMINI.md`, `hooks.json`, `plugins.json`).

3. **Consolidate `AugursAptitude` into Reusable `project/skills/`**:
   - Absorb all foundational machine learning and data engineering assets from `AugursAptitude` directly into `project/skills/`:
     - **`timeseries-ml`**: Temporal modeling, walk-forward validation, zero lookahead bias, and "as-of" join semantics.
     - **`feature-engineering`**: Leak-free feature transforms, rolling time windows, scaling, and out-of-fold target encoding.
     - **`model-evaluation`**: Adversarial validation, calibration, metric selection under imbalance, and temporal performance decay.
     - **`duckdb-pipeline`**: Columnar analytics, Parquet data lake querying, schema lineage, and zero-copy transformations.
     - **`llm-structured-output`**: Deterministic JSON extraction, Pydantic schemas, and tool calling patterns.
     - **`prompt-engineering`**: Chain-of-thought orchestration, few-shot conditioning, and token conservation.
   - Downstream projects (`AugursBazaar`, `AugursSaber`) reuse these canonical skills without duplicating foundational intelligence.

4. **Introduce CLI Skill & Tool Management Subcommands**:
   - Add first-class skill and tool discovery subcommands to `agentkit`:
     - `agentkit skill list`: Discover available reusable skills with frontmatter metadata.
     - `agentkit skill info <name>`: Inspect parsed skill frontmatter, descriptions, and full instruction markdown.
     - `agentkit skill install <name> [--copy] [--target <path>]`: Install a skill into the active project (defaulting to a relative symlink under `./skills/<name>`, or a standalone copy via `--copy`).
     - `agentkit tool list`: Discover and list available utility scripts and helper tools in `project/tools/`.

5. **Establish 3-Tier Skill Taxonomy**:
   - **Tier 1: Harness Operational Skills** (`harness/plugins/.../skills/`, `harness/skills/`): Meta-skills governing model cognitive tiering, 4-phase autonomous workflows, and subagent lifecycle controls.
   - **Tier 2: Foundational Engineering & ML Skills** (`project/skills/`): Domain-agnostic predictive modeling, data pipeline, and system architecture skills distributed by `agentkit`.
   - **Tier 3: Project-Specific Custom Skills** (`<consumer-repo>/skills/`): Proprietary domain skills tailored to specific business logic (e.g., SABR volatility modeling in `AugursBazaar`, Statcast bat-tracking metrics in `AugursSaber`).

---

## Consequences

- **Positive (Clarity & Security)**: Complete elimination of `ak` eliminates alias confusion, prevents catalog drift in `config/permissions.json`, and guarantees reliable prefix-matchable tool permissions.
- **Positive (Clean Architecture)**: The `harness/` vs. `project/` bifurcation creates clean separation of concerns between agent harness execution mechanics and portable project engineering knowledge.
- **Positive (Unified Intelligence & Decommissioning)**: Consolidating `AugursAptitude` unifies all ML and quantitative data engineering standards in `agentkit`, providing a single distribution hub and paving a clear path to formally archive and decommission the standalone `AugursAptitude` repository.
- **Positive (Zero Duplication)**: Downstream consumer repositories (`AugursSaber`, `AugursBazaar`) consume verified skills via symlinks or targeted copies, eliminating duplicate maintenance and version drift.
- **Tradeoff**: Downstream projects consuming `AugursAptitude` must update their skill ingestion to `agentkit skill install` or update existing symlinks to reference `agentkit/project/skills/`.
