# agentkit - Agent Directives & Architecture

Welcome to the **agentkit** repository. This repository defines, audits, and enforces harness-agnostic AI agent standards, command permissions, cognitive model profiles, quota governance, and developer guidelines across Cooper Wallace's workstations.

---

## 1. Repository Layout

```text
.
├── AGENTS.md                 # Primary canonical agent instructions and repository map
├── GEMINI.md                 # Symlink to AGENTS.md
├── README.md                 # Human-oriented quickstart and usage documentation
├── LICENSE                   # MIT License
├── install.sh                # Symlinks agentkit to ~/.local/bin
│
├── bin/
│   └── agentkit              # Primary Python 3 CLI tool (audit, apply, diff, snapshot, template, profile, quota, skill, tool)
│
├── config/
│   ├── permissions.json      # Declarative catalog of pre-approved commands by category
│   ├── mcp.json              # Standard MCP server configurations and environment requirements
│   ├── profiles.json         # Cognitive model tiering profiles (argon-flash, flash-high, opus-flash, sonnet-flash)
│   ├── quota.json            # Quota thresholds, provider groups, complexity mappings, and fallback chains
│   └── settings.default.json # Baseline configuration defaults (theme, non-workspace access)
│
├── harness/                  # Harness-level governance, lifecycle hooks, and plugins
│   ├── plugins/              # Standard harness plugins
│   │   ├── autonomous-orchestrator/ # Multi-model cognitive tiering (Argon/Flash) & Headroom CCR
│   │   │   ├── hooks.json    # Subagent delegation, Headroom CCR, git safety gates
│   │   │   ├── rules/        # Cognitive tiering & zero-prompt discipline rules
│   │   │   ├── skills/       # 4-phase autonomous feature lifecycle runbook (auto-lifecycle)
│   │   │   └── scripts/      # Lifecycle hook scripts (headroom-wrap, guard-*)
│   │   └── quota-guardian/   # Quota monitoring, fallback routing, and graceful wrap-up gate
│   │       ├── hooks.json    # PreToolUse invoke_subagent interception
│   │       ├── plugin.json   # Plugin metadata manifest
│   │       └── scripts/      # Quota status query and guard scripts
│   ├── hooks/                # Lifecycle hooks configuration
│   ├── rules/                # Universal agent directives (AGENTS.md)
│   └── skills/               # Harness operational skills
│
├── project/                  # Reusable cross-project skills, tools, and scaffolding templates
│   ├── skills/               # Consolidated engineering & ML skills (timeseries-ml, feature-engineering, etc.)
│   ├── tools/                # Reusable project utility scripts
│   └── templates/            # Project scaffolding templates (AGENTS.md, project-GEMINI.md, hooks.json)
│
├── docs/                     # Documentation suite
│   ├── permissions-matrix.md # Security classification and rationale for permissions
│   ├── gemini-md-guide.md    # Guide on agent cascading rules
│   ├── mcp-guide.md          # MCP standardization & decoupled secrets guide
│   ├── plugins-and-hooks-guide.md # Guide on lifecycle hooks and plugin architectures
│   ├── replication-guide.md  # Multi-machine setup checklist
│   ├── settings-reference.md # Complete anatomy of CLI settings
│   └── decisions/            # Architecture Decision Records (ADRs)
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
    ├── test_cli.py           # Automated unit test suite
    └── test_quota_cli.py     # Quota profiles, CLI commands, and routing tests
```

---

## 2. Guiding Principles for Agent Interactions

1. **Idempotence & Non-Destructive Operations**:
   - The CLI (`agentkit`) must never erase existing custom permissions, active models, or trusted workspaces from configuration files.
   - Modifying operations must create automated, timestamped backups (`settings.json.bak.<timestamp>`).
   - All state-altering subcommands must respect the `--dry-run` flag.

2. **Harness-Agnostic Abstraction**:
   - Operations support `--harness` (default `antigravity`), abstracting settings schemas and permission formats across developer harnesses.
   - Directives canonicalize on `AGENTS.md` with harness-specific symlinks (`GEMINI.md`).

3. **Cognitive Architecture & Model Tiering**:
   - **Root Orchestrator**:
     - **Primary Tier (`argon-flash`)**: Gemini 4 Argon (`Model: "pro"` / `"inherit"`) when Argon quota is available AND task involves complex architecture, multi-repo planning, or adversarial diff review.
     - **Standard Tier (`flash-high`)**: Gemini 3.8 Flash with High reasoning (`Model: "flash"` / `"inherit"`) for standard feature and lifecycle workflows (replaces retired Gemini 3.0/3.1 Pro).
     - **Fallback Tiers (`opus-flash` / `sonnet-flash`)**: Claude Opus 5.5 / Claude Sonnet 3.7 (`Model: "opus"` / `"claude-opus-5.5"` / `"inherit"`) when Gemini quota groups are exhausted or rate-limited.
     - Root Orchestrator MUST NEVER directly edit project source code, tests, or UI files (via `edit_file` or `write_file`) — only artifact files in the session brain directory (`plan.md`). All code implementation and test updates MUST be delegated to worker subagents.
   - **Subagent Complexity Tiers**:
     - `low`: `flash_lite` (file moves/renames, mechanical find-replace, formatting, doc typos, simple one-command inspection).
     - `med`: `flash` (feature code, tests, localized refactors).
     - `high`: `flash` (cross-module refactors, complex debugging, security), with escalation to `pro` (Gemini 4 Argon) for architecture, multi-repo design, or adversarial review when Argon quota is healthy.
     - Tag convention: place `[complexity: low|med|high]` at the start of subagent prompts.
   - **Disjoint-File Parallel Worker Partitioning**:
     - Partition tasks across concurrent subagents operating on strictly disjoint file sets for race-free parallel execution.

4. **Strict Test Coverage**:
   - Any modifications to the permission catalog, parser, merger, profiles, quota logic, or audit engine must be accompanied by updated unit tests in `tests/`.
   - Run tests with `python3 -m unittest discover tests -v`.

5. **Conventional Commits**:
   - Adhere strictly to Conventional Commits 1.0.0.

6. **Dynamic Paths**:
   - Never hardcode `/home/cooper`. Use `$HOME` in bash and `Path.home()` in Python.

7. **Minimally Verbose & Deterministic CLI Design**:
   - Design CLIs across projects to be clean, self-documenting, and minimally verbose without unnecessary abbreviation or cryptic shortening.
   - Because developer tools in this ecosystem are predominantly orchestrated agentically (via AI agent workflows, tool calls, scripts, and CI runners), semantic clarity, predictability, and unambiguous command shapes take precedence over rapid human keystroke shortcuts.
   - Top-level operational shorthands are restricted to explicit, high-frequency state toggles (such as `agentkit wake` and `agentkit sleep` for immediate manual power overrides), while arbitrary contractions (`caff`, `cfg`, `mgr`, `sync-tmp`) are strictly prohibited.

---

## 3. Quota Awareness & Graceful Wrap-Up

- **Pre-Flight Quota Inspection**:
  - Check remaining quota with `agentkit quota status` before launching large subagent fan-outs.
- **The 3 Quota Groups**:
  - **Gemini Group**: Gemini 3.8 Flash, Gemini 3.8 Flash Lite (standard and high-throughput subagent execution).
  - **Claude + GPT Group**: Claude Opus 5.5, Claude Sonnet 3.7 (fallback root orchestrator and cross-family execution).
  - **Gemini 4 Argon Group**: Dedicated deep reasoning quota (architectural planning, complex system design, adversarial critique).
- **Automated Fallback Routing**:
  - The `quota-guardian` plugin automatically intercepts subagent delegation via `PreToolUse`.
  - When the target model's quota group drops below **15% remaining capacity**, `quota-guardian` dynamically reroutes subagents to a healthy fallback group without interrupting execution.
- **Graceful Wrap-Up Circuit Breaker**:
  - When all eligible quota groups drop below **5% remaining capacity**, `quota-guardian` denies further delegation with `QUOTA WRAP-UP:`.
  - Agents MUST immediately follow the `quota-wrapup` protocol:
    1. Cease spawning new subagents.
    2. Run local test suites to verify system stability.
    3. Create a clean WIP commit on the active feature branch (`git commit -m "chore(wip): save state prior to quota wrap-up"`).
    4. Generate a comprehensive handoff note in the Obsidian AgentVault (`$HOME/Documents/AgentVault/Sessions/handoff-YYYY-MM-DD-quota-wrapup.md`), update `Projects/<project>.md`, and mirror `handoff.md` in conversation artifacts.
    5. Advise the user of the exact quota reset timestamp and recommend an advisory root model switch.
- **Advisory Root Model Switching**:
  - The root session model cannot be switched programmatically via API during an active session.
  - When root quota is depleted, the agent must provide clear advisory guidance instructing the user to switch models via the chat UI picker or the `/model` slash command.

---

## 4. Perpetual Long-Term Memory & Offline Collaboration (Obsidian AgentVault)

- **Local Obsidian AgentVault**:
  - The local Obsidian AgentVault (`$HOME/Documents/AgentVault` or `$HOME/AgentVault`, configurable via `$AGENTKIT_AGENTVAULT_PATH` or `$AGENTKIT_VAULT_PATH`) serves as the long-term, human-in-the-loop memory store across sessions, adhering to the open Obsidian Skills standard and "files over apps" philosophy.
  - **Automatic Workspace Whitelisting**: The AgentVault directory (`$HOME/Documents/AgentVault` or `$HOME/AgentVault`) as well as core repositories (`agentkit` and `repokit` at `$HOME/Projects/agentkit` and `$HOME/Projects/repokit`) are automatically whitelisted in Antigravity's `trustedWorkspaces` via `agentkit apply` and `agentkit vault init` to eliminate file read/write prompts during autonomous workflows and cross-repo introspection.
- **Offline Human-Agent Collaboration Protocol**:
  - **Ingest at Session Start (Phase 1)**: Agents must check `User-Input/pending/` at the beginning of each session (or run `agentkit vault ingest`) to process offline user directives, feedback, and backlog notes written between conversations. Processed directives are archived to `User-Input/processed/`.
  - **Project Living Notes**: Agents maintain project knowledge bases in `Projects/<project>.md` (e.g. `agentkit.md`, `repokit.md`), keeping active architecture, current branch/commit, and task checklists synchronized.
  - **Handoff at Session Wrap-Up (Phase 4)**: Agents author structured handoff notes to `Sessions/handoff-YYYY-MM-DD-<topic>.md`, capturing completed deliverables, test verification results, and offline tasks.
  - **Visual Architecture with JSON Canvas**: Visual architecture maps and multi-agent DAGs are maintained in `Architecture/` using the open JSON Canvas specification (`.canvas`).

---

## 5. Git Workflow & Version Control Standards

- **Branching Strategy: One Branch Per Change**:
  - Every discrete task, feature, bug fix, refactor, or test improvement must operate on its own dedicated short-lived feature branch branched directly from `main`:
    - `feat/<feature-name>`: New functionality
    - `fix/<bug-name>`: Bug fixes
    - `docs/<doc-name>`: Documentation updates
    - `refactor/<target>`: Structural code improvements
    - `test/<suite>`: Test additions or refactoring
  - Never stack unrelated features or accumulate unrelated changes on the same branch. Keep scope narrow, focused, and atomic.
- **Commit Hygiene: One Atomic Commit Per Subagent**:
  - Each worker subagent assigned to a deliverable must produce a single atomic Conventional Commit (`<type>(<scope>): <short summary>`) representing its completed, tested unit of work.
  - Workers must never leave uncommitted changes or produce messy WIP commits.
  - Commit messages must strictly follow Conventional Commits 1.0.0: `<type>(<scope>): <short summary>`
  - Types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`.
  - Use imperative, present-tense verbs ("add", "fix", "update", not "added" or "fixes").
  - Scope is optional but encouraged for modular repositories.
- **Multi-Agent Concurrency: Isolated Git Worktrees for Concurrent Threads**:
  - When multiple agent threads or subagents work concurrently within the same repository, they MUST operate in isolated git worktrees (`Workspace: "share"` in Antigravity or `git worktree add`) to prevent `.git/index.lock` collisions, dirty working tree races, and write contention.
  - Concurrent workers must never operate on the same physical working directory simultaneously.
- **Trunk Hygiene & Merge Governance**:
  - Repositories follow the **Squash and Merge** model governed by `repokit`.
  - Squash titles default to `PR_TITLE` and bodies to `PR_BODY`. Direct pushes to `main` and local trunk merges are strictly prohibited.
- **Human-Only Trunk Promotion & Auto-Merge Prohibition**:
  - Merging Pull Requests into `main` is strictly a human-in-the-loop action. Autonomous agents MUST NEVER execute `gh pr merge`, enable GitHub auto-merge, or automate trunk merging. Agents are restricted to opening Pull Requests (`gh pr create`), posting adversarial review audits, and verifying check suites (`gh pr checks`). PR promotion into `main` must only occur via manual human review and approval in the GitHub web UI once all quality gates pass.
