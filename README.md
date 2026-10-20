# antigravity-standardise (`agy-standardise`)

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Conventional Commits](https://img.shields.io/badge/Conventional%20Commits-1.0.0-yellow.svg)](https://conventionalcommits.org)
[![Python: 3.10+](https://img.shields.io/badge/Python-3.10%2B-blue.svg)](https://python.org)

An opinionated developer CLI and declarative configuration space to define, audit, and standardise the **Google Antigravity CLI (`agy`)** across multiple developer workstations.

---

## The Philosophy

When using AI pair programmers across multiple machines (such as desktop workstations, laptops, and remote dev environments), developers encounter two recurring friction points:

1. **Permission Approval Fatigue**: Constant interactive interruptions prompting the user to allow basic read, git, inspection, or test commands (`command(git status)`, `command(ls)`, `command(pytest)`).
2. **Configuration & Context Drift**: Fresh workstations lack the engineering directives, safety guidelines, and tool permissions established on primary machines.

`antigravity-standardise` eliminates both issues:
- It maintains a **declarative, tiered catalog of pre-approved commands** in `config/permissions.json`, granting immediate access to safe inspection, testing, and git operations while strictly withholding destructive operations (`rm -rf`, `sudo`, disk formatters).
- It provides an **idempotent CLI (`agy-standardise`)** that audits local settings, merges standard permissions without overwriting custom workspaces or models, creates automated backups, and links universal developer directives (`GEMINI.md` / `AGENTS.md`).

---

## Features

- **Tiered Command Permissions**:
  - Pre-approves safe core inspection tools (`cat`, `grep`, `which`, `ls`, `find`, `file`, `diff`, `fastfetch`).
  - Pre-approves non-destructive file operations (`mkdir`, `touch`, `cp`, `mv`, `chmod`, `ln`) while gating unreviewed deletions.
  - Pre-approves complete git hygiene and GitHub workflows (`git status`, `git diff`, `git log`, `git checkout`, `gh pr`, `gh-standardise`).
  - Pre-approves multi-language test harnesses (`pytest`, `unittest`, `npm test`, `cargo test`, `go test`).
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
  - Deploys canonical developer rules (`templates/GEMINI.md` and `AGENTS.md`) emphasizing Conventional Commits, clean architecture, bash safety, and verification before completion.
- **Declarative MCP Server Management**:
  - Declarative catalog (`config/mcp.json`) for Model Context Protocol servers (e.g. GitHub, SQLite, Fetch) referencing decoupled environment variables (`${GITHUB_PERSONAL_ACCESS_TOKEN}`).
  - Active diagnostics: `audit` and `apply` verify CLI dependencies (`npx`, `uvx`) and inspect `os.environ`, providing actionable setup advisories for missing variables.
- **Project Scaffolding**:
  - Scaffolds modular project-level `GEMINI.md` files with custom test commands and architecture maps.
- **Autonomous Orchestrator & Plugin Governance**:
  - Bundles the standard `autonomous-orchestrator` plugin for state-of-the-art multi-model workflows with Gemini 4 Argon and Gemini 3.8 Flash.
  - Automatically deploys and audits standard plugins in `~/.gemini/config/plugins/` with executable lifecycle hook scripts.
  - Hardens version control workflows with deterministic hook gates (`PreToolUse` blocking direct pushes and merges to `main`, and `Stop` verifying clean worktrees and green test suites).
  - Pre-approves Git worktree operations (`git worktree`, `git rev-parse`, `git ls-files`) for concurrent subagent isolation.

---

## Installation

### Quick Install (Symlink to `~/.local/bin`)
```bash
git clone https://github.com/Cooper-Wallace/antigravity-standardise.git ~/Projects/antigravity-standardise
cd ~/Projects/antigravity-standardise
bash install.sh
```

Ensure `~/.local/bin` is in your `$PATH`:
```bash
export PATH="$HOME/.local/bin:$PATH"
```

### Dual Command Aliases
The installer creates two symlinks in `~/.local/bin`:
- `agy-standardise` (concise CLI alias)
- `antigravity-standardise` (full name)

---

## Commands & Usage

### 1. Audit Current Workstation
Audit the current machine's `settings.json`, permissions, global directives, and environment:
```bash
agy-standardise audit

# Verbose category breakdown:
agy-standardise audit -v
```

Example audit report:
```text
Auditing Antigravity CLI Standards
Target Settings: /home/cooper/.gemini/antigravity-cli/settings.json
  ✔ Settings file exists (/home/cooper/.gemini/antigravity-cli/settings.json)
  ✔ Standard Permissions: All 74 pre-approved commands active
  ✔ Global Agent Directives installed (/home/cooper/.gemini/GEMINI.md)
  ✔ Workspace Boundary: 'allowNonWorkspaceAccess' enabled
  ✔ UI Theme: 'colorScheme' set to 'dark'
  ✔ Environment PATH includes /home/cooper/.local/bin

────────────────────────────────────────────────────────────
[SUCCESS] Antigravity CLI configuration is fully compliant! (6/6 checks passed)
```

---

### 2. Apply Standards to Local Machine
Merge all standard permissions, create a timestamped backup, and symlink global directives:
```bash
agy-standardise apply
```

#### Safe Dry-Run Mode
Simulate changes without writing to disk:
```bash
agy-standardise apply --dry-run
```

#### Also Link into Home Directory
```bash
agy-standardise apply --link-home
```

---

### 3. Diff Local Configuration against Standard Baseline
Review which standard permissions are missing or which custom permissions exist locally:
```bash
agy-standardise diff
```

---

### 4. Snapshot Local Commands to Sync Back to Git
When you click **"Always allow"** in Antigravity during ad-hoc sessions, inspect those newly added commands:
```bash
agy-standardise snapshot
```

To output as JSON:
```bash
agy-standardise snapshot --json
```

Add any commands you wish to standardize to `config/permissions.json` and push to your repository.

---

### 5. Scaffold Project-Level `GEMINI.md`
Scaffold project-specific agent directives into any repository:
```bash
cd ~/Projects/my-new-project
agy-standardise template --name "auth-service" --lang "Python / FastAPI" --test "pytest"
```

---

### 6. Install Conventional Commits Git Hook
Install a local `commit-msg` hook to enforce Conventional Commits:
```bash
agy-standardise hook install
```

---

## Extensibility: Lifecycle Hooks & Plugin Templates

Antigravity CLI provides execution hooks (`PreToolUse`, `PostToolUse`, `PreInvocation`) and shareable plugin bundles. This repository includes opt-in templates and architectural recommendations without imposing live runtime side-effects prematurely:

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
├── install.sh                    # Symlinks CLI to ~/.local/bin
│
├── bin/
│   ├── agy-standardise           # Primary Python 3 CLI executable
│   └── antigravity-standardise   # Alias symlink
│
├── config/
│   ├── permissions.json          # Categorized catalog of pre-approved commands
│   ├── mcp.json                  # Declarative catalog of standard MCP servers & requirements
│   └── settings.default.json     # Standard baseline preferences
│
├── templates/
│   ├── GEMINI.md                 # Universal developer agent directives
│   ├── AGENTS.md                 # Symlink to GEMINI.md
│   ├── project-GEMINI.md         # Template for project-specific scaffolding
│   ├── hooks.json                # Template for Antigravity lifecycle hooks
│   ├── plugins.json              # Template for plugin registration manifest
│   ├── plugin/                   # Complete archetype bundle for Antigravity plugins
│   └── rules/                    # Reusable rules (bash-safety.md, conventional-commits.md)
│
├── docs/                         # Comprehensive documentation
│   ├── permissions-matrix.md     # Permission classifications and safety tiers
│   ├── gemini-md-guide.md        # Guide on Antigravity cascading rules
│   ├── mcp-guide.md              # MCP standardization & decoupled secrets guide
│   ├── plugins-and-hooks-guide.md# Guide on lifecycle hooks and plugin architectures
│   ├── replication-guide.md      # Multi-machine setup checklist
│   ├── settings-reference.md     # Settings schema and options reference
│   └── decisions/                # Architecture Decision Records (ADRs)
│       ├── 0001-permission-merging-strategy.md
│       ├── 0002-dual-naming-convention.md
│       ├── 0003-global-vs-workspace-rules.md
│       ├── 0004-plugin-and-hook-templating.md
│       └── 0005-declarative-mcp-management-and-env-advisories.md
│
└── tests/
    └── test_cli.py               # 20 automated unit tests
```

---

## Multi-Machine Replication & RKO Integration

For machines provisioned with **`riced-kubuntu-os` (RKO)**, `agy-standardise` integrates seamlessly into Stage `03` (`dotfiles`):

```bash
# Clone and configure in 2 commands:
git clone https://github.com/Cooper-Wallace/antigravity-standardise.git ~/Projects/antigravity-standardise
bash ~/Projects/antigravity-standardise/install.sh
agy-standardise apply
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
