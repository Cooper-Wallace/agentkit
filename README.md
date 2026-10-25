# agentkit

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Conventional Commits](https://img.shields.io/badge/Conventional%20Commits-1.0.0-yellow.svg)](https://conventionalcommits.org)
[![Python: 3.10+](https://img.shields.io/badge/Python-3.10%2B-blue.svg)](https://python.org)

The harness-agnostic AI agent configuration, permissions catalog, directives, and orchestration toolkit across Cooper Wallace's workstations.

---

## The Philosophy

When utilizing autonomous AI agents and pair-programming harnesses across multiple machines (such as desktop workstations, laptops, and remote dev environments), developers encounter two recurring friction points:

1. **Permission Approval Fatigue**: Constant interactive interruptions prompting the user to allow basic read, git, inspection, or test commands (`command(git status)`, `command(ls)`, `command(pytest)`).
2. **Configuration & Context Drift**: Fresh workstations lack the engineering directives, safety guidelines, tool permissions, and model tiering established on primary machines.

`agentkit` eliminates both issues:
- It maintains a **declarative, tiered catalog of pre-approved commands** in `config/permissions.json`, granting immediate access to safe inspection, testing, and git operations while strictly withholding destructive operations (`rm -rf`, `sudo`, disk formatters).
- It provides an **idempotent, harness-agnostic CLI (`agentkit`)** that audits local settings, merges standard permissions without overwriting custom workspaces or models, creates automated backups, manages switchable model profiles, distributes reusable domain skills and tools, and links universal developer directives (`AGENTS.md` / `GEMINI.md`).

---

## 4-Layer Workstation Ecosystem

`agentkit` operates as **Layer 2** in a modular 4-layer workstation engineering architecture:

| Layer | System | Responsibility |
|---|---|---|
| **Layer 0** | **`riced-kubuntu-os` (RKO)** | Base OS configuration, KDE Plasma 6 desktop styling, system dotfiles, and system packages. |
| **Layer 1** | **`workstation-setup`** | Developer toolchains (Node.js, Python, uv), shell profiles (`~/.profile`, `~/.bashrc`), keyrings, and environment bridging. |
| **Layer 2** | **`agentkit`** (CLI: `agentkit`) | Harness-agnostic AI agent configuration, CLI permissions catalog, global agent directives (`AGENTS.md` / `GEMINI.md`), MCP server catalogs, switchable model profiles, project skill/tool distribution, and plugin lifecycle hooks. |
| **Layer 3** | **`repokit`** | GitHub repository governance, branch protection rulesets, squash-and-merge enforcement, and semantic PR checks. |

By separating Layer 2 from Layer 1, `agentkit` maintains strict focus on AI agent governance and safety policies without performing runtime toolchain installations or modifying shell profiles directly.

---

## Harness-Agnostic Architecture

`agentkit` provides an extensible architecture designed to manage multiple AI developer harnesses:

- **Harness Flag (`--harness`)**: Commands accept `--harness <name>` (default: `antigravity`). Modular adapters handle harness-specific configuration formats, permission schemas, and settings files.
- **Canonical `AGENTS.md` Directives**: All repository and global directives are centered on `AGENTS.md` as the canonical standard. Harness-specific directive files (such as `GEMINI.md` or `CLAUDE.md`) are maintained as symlinks to `AGENTS.md`.
- **Modular Lifecycle Hooks**: Standardized hook manifests allow running cognitive tiering guards, Headroom CCR token compression, and git safety gates across supported harnesses.

---

## Switchable Model Profiles

`agentkit` manages cognitive tiering configurations to optimize model intelligence, token quotas, and response latency:

| Profile | Root Orchestrator | Worker Subagents | Primary Use Case |
|---|---|---|---|
| **`argon-flash`** | Gemini 4 Argon | Gemini 3.8 Flash | **Primary / Default**: Deepest reasoning and architectural precision (when Argon quota is available) with fast Flash subagents. |
| **`flash-high`** | Gemini 3.8 Flash (High reasoning) | Gemini 3.8 Flash | **Standard**: Fast lifecycle and feature development with high reasoning depth (replaces retired Pro). |
| **`opus-flash`** | Claude Opus 5.5 | Gemini 3.8 Flash | **Quota / Token Limit Fallback**: High-capability root planner when Gemini quota groups are exhausted or rate-limited. |
| **`sonnet-flash`** | Claude Sonnet 3.7 | Gemini 3.8 Flash | **High-Throughput**: High-speed reasoning, refactoring, and code synthesis with Flash execution. |

### Subagent Complexity Tiers

Subagents are routed to optimal models based on task complexity tags placed at the start of prompts (`[complexity: low|med|high]`):
- **`low`**: `flash_lite` (file moves/renames, mechanical find-replace, formatting, doc typos, simple one-command inspection).
- **`med`**: `flash` (feature code, tests, localized refactors).
- **`high`**: `flash` (cross-module refactors, complex debugging, security), with escalation to `pro` (Gemini 4 Argon) for architecture, multi-repo design, or adversarial review when Argon quota is healthy.

Switch active profiles seamlessly:
```bash
agentkit profile list
agentkit profile set flash-high
agentkit profile set opus-flash
```

---

## Features

- **Harness-Agnostic Governance**: Support for Google Antigravity CLI and extensible harness targets via `--harness`.
- **Switchable Cognitive Profiles**: Switch between `argon-flash`, `flash-high`, `opus-flash`, and `sonnet-flash` profiles to navigate token quotas and workload complexity.
- **Quota Telemetry & Fallback Routing**: Query real-time token quotas with `agentkit quota status` and calculate optimal model assignments with `agentkit quota route`.
- **Tiered Command Permissions**:
  - Pre-approves safe core inspection tools (`cat`, `grep`, `which`, `ls`, `find`, `file`, `diff`, `fastfetch`).
  - Pre-approves non-destructive file operations (`mkdir`, `touch`, `cp`, `mv`, `chmod`, `ln`) while gating unreviewed deletions.
  - Pre-approves complete git hygiene and GitHub workflows (`git status`, `git diff`, `git log`, `git checkout`, `gh pr`, `repokit`).
  - Pre-approves multi-language test harnesses (`pytest`, `unittest`, `npm test`, `cargo test`, `go test`).
  - Pre-approves agentkit ecosystem binaries (`agentkit`, `workstation-setup`, `ws-setup`).
  - Pre-approves provisioned MCP servers and cross-thread memory tools (`memory`, `repomix`, `github`, `fetch`) and harness invocation tools (`call_mcp_tool`), removing approval fatigue across conversation threads.
- **Reusable Domain Skills & Utility Tools**:
  - Distributes consolidated engineering and machine learning skills (`project/skills/`) from `AugursAptitude` (`timeseries-ml`, `feature-engineering`, `model-evaluation`, `duckdb-pipeline`, `llm-structured-output`, `prompt-engineering`).
  - Provides frictionless project integration via `agentkit skill install <name> [--copy]` and inspection via `agentkit skill info <name>`.
  - Discovers reusable project scripts and analytical utilities via `agentkit tool list`.
- **Obsidian AgentVault & Offline Collaboration**:
  - Replaces opaque MCP memory stores with a durable, human-readable local Obsidian Markdown vault (`~/Documents/AgentVault` or `~/AgentVault`) adhering to the open **Obsidian Skills** specification by Steph Ango (kepano).
  - Automatically whitelisted in Antigravity's `trustedWorkspaces` (alongside core repositories `~/Projects/agentkit` and `~/Projects/repokit`) via `agentkit apply` and `agentkit vault init` to eliminate read/write confirmation prompts during autonomous workflows and cross-repo introspection.
  - Enables asynchronous offline collaboration: developers review session handoffs, refine project living notes, and drop offline directives into `User-Input/pending/` during conversation breaks.
  - Generates visual architecture maps and multi-agent workflows using the open **JSON Canvas** (`.canvas`) specification (`jsoncanvas.org`).
  - Integrated CLI commands (`agentkit vault init`, `agentkit vault ingest`, `agentkit vault handoff`, `agentkit vault status`).
- **Non-Destructive Set-Union Merging**:
  - `apply` merges missing baseline permissions into `~/.gemini/antigravity-cli/settings.json`.
  - Never clobbers machine-specific `trustedWorkspaces`, model preferences, or custom tool permissions.
  - Generates timestamped backups (`settings.json.bak.<timestamp>`) prior to write operations.
  - Full `--dry-run` simulation mode.
- **Visual Audit Engine**:
  - Validates workstation configuration against standards, reporting compliance score and missing permissions.
- **Snapshot & Learning Sync**:
  - Detects newly approved commands on the local machine (`snapshot`) so you can commit them back to version control.
- **Universal Agent Directives**:
  - Deploys canonical developer rules (`templates/AGENTS.md` and symlinked `GEMINI.md`) emphasizing Conventional Commits, clean architecture, bash safety, and verification before completion.
- **Declarative MCP Server Management**:
  - Declarative catalog (`config/mcp.json`) for 4 standard Model Context Protocol servers: `github`, `fetch`, `memory` (cross-repository knowledge graph), and `repomix` (AST-compressed repository code maps).
  - All sensitive credentials decoupled via environment variable interpolations (`${GITHUB_PERSONAL_ACCESS_TOKEN}`).
  - Active diagnostics: `audit` and `apply` verify CLI dependencies (`npx`, `node`) and inspect `os.environ` and shell profiles (`~/.profile`, `~/.bashrc`), providing actionable setup advisories for missing variables.
  - Clear architectural boundary: `agentkit` audits runtime and environment readiness while delegating OS/toolchain provisioning and shell environment injection to `workstation-setup`.
- **Project Scaffolding**:
  - Scaffolds modular project-level `AGENTS.md` and `GEMINI.md` files with custom test commands and architecture maps.
- **Autonomous Orchestrator & Plugin Governance**:
  - Bundles standard plugins (`autonomous-orchestrator`, `quota-guardian`) for multi-model workflows, quota management, and token optimization.
  - Automatically deploys and audits standard plugins in `~/.gemini/config/plugins/` with executable lifecycle hook scripts.
  - **Cognitive Tiering & Subagent Guard**: Enforces root planning via Argon/Opus and code execution via Flash subagents with `PreToolUse` delegation enforcement (`scripts/guard-subagent-delegation.sh`).
  - **Quota Guardian & Fallback Routing**: Intercepts subagent delegation (`PreToolUse` on `invoke_subagent`), queries language server quota telemetry (`RetrieveUserQuotaSummary`), dynamically reroutes subagents when quotas drop below 15%, and gates execution with a graceful wrap-up protocol at 5% remaining capacity.
  - **Headroom CCR Token Compression**: Intercepts verbose test, inspection, and diff commands via `PreToolUse` overwrite (`filter-and-wrap.py` -> `headroom-wrap.py`), shrinking output tokens by 60%–90% while caching raw logs in `~/.cache/antigravity-headroom/<hash>.txt` for reversible retrieval.
  - **Deterministic Hook Gates**: Hard blocks direct pushes and trunk merges to `main` (`guard-git-operations.sh`), gates destructive deletions, and prevents task termination on test failures or dirty trees (`verify-clean-completion.sh`).
  - Pre-approves Git worktree operations (`git worktree`, `git rev-parse`, `git ls-files`) for concurrent subagent isolation.
- **Git Tree Management & Concurrency Isolation**:
  - Enforces three foundational tree management invariants across all agent workflows:
    - **One Branch Per Change**: Dedicated, short-lived feature branches (`feat/`, `fix/`, `docs/`, `refactor/`, `test/`) with zero stacked unrelated changes.
    - **One Commit Per Subagent**: Each worker subagent produces a single atomic Conventional Commit representing its completed and tested deliverable.
    - **Isolated Git Worktrees for Concurrent Threads**: Guarantees isolated worktrees (`Workspace: "share"` or `git worktree add`) for concurrent threads within the same repository, eliminating `.git/index.lock` collisions, dirty tree races, and write contention.

---

## Installation

### Quick Install (Symlink to `~/.local/bin`)
```bash
git clone https://github.com/Cooper-Wallace/agentkit.git ~/Projects/agentkit
cd ~/Projects/agentkit
bash install.sh
```

Ensure `~/.local/bin` is in your `$PATH`:
```bash
export PATH="$HOME/.local/bin:$PATH"
```

### CLI Executable
`agentkit` is the exclusive CLI tool. The installer links the executable into `~/.local/bin`:
- `agentkit` (primary executable)

---

## Commands & Usage

### 1. Audit Current Workstation
Audit the current machine's settings, permissions, global directives, and environment:
```bash
agentkit audit

# Verbose category breakdown:
agentkit audit -v

# Target specific harness:
agentkit audit --harness antigravity
```

Example audit report:
```text
Auditing AI Agent Standards (harness: antigravity)
Target Settings: /home/cooper/.gemini/antigravity-cli/settings.json
  ✔ Settings file exists (/home/cooper/.gemini/antigravity-cli/settings.json)
  ✔ Standard Permissions: All pre-approved commands active
  ✔ Global Agent Directives installed (/home/cooper/.gemini/AGENTS.md)
  ✔ Directives Symlink (/home/cooper/.gemini/GEMINI.md -> AGENTS.md)
  ✔ Workspace Boundary: 'allowNonWorkspaceAccess' enabled
  ✔ Active Profile: 'argon-flash' (Gemini 4 Argon -> Gemini 3.8 Flash)
  ✔ UI Theme: 'colorScheme' set to 'dark'
  ✔ Environment PATH includes /home/cooper/.local/bin

────────────────────────────────────────────────────────────
[SUCCESS] Agent harness configuration is fully compliant! (8/8 checks passed)
```

---

### 2. Apply Standards to Local Machine
Merge all standard permissions, create a timestamped backup, and symlink global directives:
```bash
agentkit apply
```

#### Safe Dry-Run Mode
Simulate changes without writing to disk:
```bash
agentkit apply --dry-run
```

#### Also Link into Home Directory
```bash
agentkit apply --link-home
```

---

### 3. Manage Cognitive Model Profiles
Inspect and switch cognitive tiering profiles:
```bash
# List available model profiles:
agentkit profile list

# Switch to standard Flash High profile:
agentkit profile set flash-high

# Switch to Claude Opus 5.5 fallback profile:
agentkit profile set opus-flash

# Switch back to primary Argon profile:
agentkit profile set argon-flash
```

---

### 4. Quota Telemetry & Subagent Route Optimization
Query real-time token quotas and calculate optimal subagent routing:
```bash
# Check quota status across Gemini, Claude, and Argon groups:
agentkit quota status

# Check quota status in JSON format:
agentkit quota status --json

# Determine optimal model recommendation for a task complexity:
agentkit quota route --complexity low
agentkit quota route --complexity med
agentkit quota route --complexity high
```

---

### 5. Diff Local Configuration against Standard Baseline
Review which standard permissions are missing or which custom permissions exist locally:
```bash
agentkit diff
```

---

### 6. Snapshot Local Commands to Sync Back to Git
When you click **"Always allow"** during ad-hoc sessions, inspect those newly added commands:
```bash
agentkit snapshot
```

To output as JSON:
```bash
agentkit snapshot --json
```

Add any commands you wish to standardize to `config/permissions.json` and push to your repository.

---

### 7. Scaffold Project-Level Directives
Scaffold project-specific agent directives (`AGENTS.md` and `GEMINI.md` symlink) into any repository:
```bash
cd ~/Projects/my-new-project
agentkit template --name "auth-service" --lang "Python / FastAPI" --test "pytest"
```

---

### 8. Install Conventional Commits Git Hook
Install a local `commit-msg` hook to enforce Conventional Commits:
```bash
agentkit hook install
```

---

### 9. Manage Reusable Project Skills
Discover, inspect, and install domain engineering and machine learning skills into repositories:
```bash
# List available skills:
agentkit skill list

# Inspect detailed instructions and runbook for a skill:
agentkit skill info timeseries-ml

# Install skill into current project (symlinked under ./skills/<skill-name>):
agentkit skill install timeseries-ml

# Install skill as an independent file copy:
agentkit skill install duckdb-pipeline --copy

# Install into a custom target directory:
agentkit skill install feature-engineering --target ~/Projects/AugursSaber/skills/feature-engineering
```

---

### 10. Discover Reusable Project Tools
Discover and inspect verified utility scripts and helper tools:
```bash
# List all available project utility scripts:
agentkit tool list
```

---

### 11. Obsidian AgentVault Management
Initialize, audit, and orchestrate long-term agent memory and offline human-agent collaboration via the local Obsidian AgentVault (`~/Documents/AgentVault` or `~/AgentVault`, configurable via `$AGENTKIT_AGENTVAULT_PATH` or `$AGENTKIT_VAULT_PATH`):

```bash
# Initialize standard vault directory layout and baseline project notes:
agentkit vault init

# Inspect vault status, location, pending directive count, and recent sessions:
agentkit vault status

# Ingest and display pending offline directives from User-Input/pending/:
agentkit vault ingest

# Ingest and archive pending directives to User-Input/processed/:
agentkit vault ingest --process

# Generate a structured session handoff note in Sessions/:
agentkit vault handoff --topic "auth-migration"
```

---

## Reusable Engineering & ML Skills (`AugursAptitude` Consolidation)

With the consolidation of **`AugursAptitude`** into `agentkit` (`project/skills/`), foundational machine learning and quantitative engineering intelligence is unified into a single repository:

### Core Reusable Skills
- **`timeseries-ml`**: Production guidelines for non-stationary temporal modeling, walk-forward cross-validation, "as-of" point-in-time joins, and eliminating lookahead bias across sequential datasets.
- **`feature-engineering`**: Standardized feature transformation, rolling aggregations, lag operators, scaling techniques, and out-of-fold target encoding.
- **`model-evaluation`**: Adversarial validation, calibration curves, classification/regression metrics under class imbalance, and performance decay monitoring.
- **`duckdb-pipeline`**: High-performance in-process columnar analytics, Parquet data lake querying, schema lineage, and zero-copy transformations with DuckDB.
- **`llm-structured-output`**: Deterministic schema extraction, Pydantic validation, instructor patterns, and structured function calling with LLMs.
- **`prompt-engineering`**: Chain-of-thought orchestration, few-shot prompting patterns, token conservation, and context injection techniques.

### Consumer Project Reuse
Downstream consumer repositories—including quantitative trading pipelines (**`AugursBazaar`**) and baseball player projection models (**`AugursSaber`**)—share identical temporal modeling, data lake, and feature engineering foundations. Rather than maintaining duplicated copies or drifting implementations:
- Consumer projects install standardized skills via `agentkit skill install <skill_name>`.
- Symlink-based installation ensures downstream agents always execute the latest validated engineering standards.
- Standalone copy installations (`--copy`) support fully air-gapped or self-contained repository distributions without duplicating foundational intelligence across projects.

---

## Extensibility: Lifecycle Hooks & Plugin Templates

`agentkit` provides execution hooks (`PreToolUse`, `PostToolUse`, `PreInvocation`, `Stop`) and shareable plugin bundles. This repository includes opt-in templates and architectural recommendations without imposing live runtime side-effects prematurely:

- **Lifecycle Hooks Template**: [`templates/hooks.json`](templates/hooks.json) (Pre-configured examples: command safety guard, auto-formatting, git status watcher).
- **Plugin Bundle Archetype**: [`templates/plugin/`](templates/plugin/) (Structure for packaging rules, skills, hooks, and MCP servers).
- **Plugin Registration Manifest**: [`templates/plugins.json`](templates/plugins.json).
- **Comprehensive Guide & Recommendations**: [docs/plugins-and-hooks-guide.md](docs/plugins-and-hooks-guide.md).

---

## Repository Structure

```text
.
├── AGENTS.md                     # Directives for AI agents working in this repository
├── GEMINI.md                     # Symlink to AGENTS.md
├── README.md                     # Human-oriented documentation
├── LICENSE                       # MIT License
├── install.sh                    # Symlinks CLI to ~/.local/bin (agentkit)
│
├── bin/
│   └── agentkit                  # Primary Python 3 CLI executable (audit, apply, diff, snapshot, template, profile, skill, tool)
│
├── config/
│   ├── permissions.json          # Categorized catalog of pre-approved commands
│   ├── mcp.json                  # Declarative catalog of standard MCP servers & requirements
│   ├── profiles.json             # Cognitive model tiering profiles (argon-flash, opus-flash, etc.)
│   └── settings.default.json     # Standard baseline preferences
│
├── harness/                      # Harness-level governance, lifecycle hooks, and plugins
│   ├── plugins/                  # Standard harness plugins
│   │   ├── autonomous-orchestrator/ # Multi-model orchestration (Argon/Flash) & Headroom CCR
│   │   └── quota-guardian/       # Quota telemetry, fallback routing, and wrap-up gate
│   ├── hooks/                    # Standard lifecycle hooks configuration
│   ├── rules/                    # Universal agent directives (AGENTS.md)
│   └── skills/                   # Harness operational skills
│
├── project/                      # Reusable cross-project skills, tools, and scaffolding templates
│   ├── skills/                   # Consolidated engineering & ML skills (timeseries-ml, feature-engineering, etc.)
│   ├── tools/                    # Reusable project utility scripts
│   └── templates/                # Project scaffolding templates (AGENTS.md, project-GEMINI.md, hooks.json)
│
├── docs/                         # Comprehensive documentation
│   ├── permissions-matrix.md     # Permission classifications and safety tiers
│   ├── gemini-md-guide.md        # Guide on agent cascading rules
│   ├── mcp-guide.md              # MCP standardization & decoupled secrets guide
│   ├── plugins-and-hooks-guide.md# Guide on lifecycle hooks and plugin architectures
│   ├── replication-guide.md      # Multi-machine setup checklist
│   ├── settings-reference.md     # Settings schema and options reference
│   └── decisions/                # Architecture Decision Records (ADRs)
│       ├── 0001-permission-merging-strategy.md
│       ├── 0002-dual-naming-convention.md
│       ├── 0003-global-vs-workspace-rules.md
│       ├── 0004-plugin-and-hook-templating.md
│       ├── 0005-declarative-mcp-management-and-env-advisories.md
│       ├── 0006-token-compression-and-cross-repo-memory.md
│       ├── 0007-agentkit-harness-agnostic-standardisation.md
│       ├── 0008-harness-project-bifurcation-and-aptitude-consolidation.md
│       ├── 0009-quota-aware-model-routing.md
│       ├── 0010-obsidian-perpetual-knowledge-vault.md
│       └── 0011-git-tree-management-and-worktree-isolation.md
│
└── tests/
    └── test_cli.py               # Automated unit tests
```

---

## Multi-Machine Replication & RKO Integration

For machines provisioned with **`riced-kubuntu-os` (RKO)**, `agentkit` integrates seamlessly into Stage `03` (`dotfiles`):

```bash
# Clone and configure in 2 commands:
git clone https://github.com/Cooper-Wallace/agentkit.git ~/Projects/agentkit
bash ~/Projects/agentkit/install.sh
agentkit apply
```

See [docs/replication-guide.md](docs/replication-guide.md) for full details.

---

## Testing

Run the automated test suite:
```bash
python3 -m unittest discover tests -v
```

---

## License

This project is licensed under the [MIT License](LICENSE).
