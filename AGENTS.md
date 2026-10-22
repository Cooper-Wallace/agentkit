# agentkit - Agent Directives & Architecture

Welcome to the **agentkit** repository. This repository defines, audits, and enforces harness-agnostic AI agent standards, command permissions, cognitive model profiles, and developer guidelines across Cooper Wallace's workstations.

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
│   └── agentkit              # Primary Python 3 CLI tool (audit, apply, diff, snapshot, template, profile, skill, tool)
│
├── config/
│   ├── permissions.json      # Declarative catalog of pre-approved commands by category
│   ├── mcp.json              # Standard MCP server configurations and environment requirements
│   ├── profiles.json         # Cognitive model tiering profiles (argon-flash, opus-flash, etc.)
│   └── settings.default.json # Baseline configuration defaults (theme, non-workspace access)
│
├── harness/                  # Harness-level governance, lifecycle hooks, and plugins
│   ├── plugins/              # Multi-model orchestration plugin (autonomous-orchestrator)
│   │   └── autonomous-orchestrator/
│   │       ├── hooks.json    # Subagent delegation, Headroom CCR, git safety gates
│   │       ├── rules/        # Cognitive tiering & zero-prompt discipline rules
│   │       ├── skills/       # 4-phase autonomous feature lifecycle runbook (auto-lifecycle)
│   │       └── scripts/      # Lifecycle hook scripts (headroom-wrap, guard-*)
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
│
└── tests/
    └── test_cli.py           # Automated unit test suite
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

3. **Cognitive Model Profiles**:
   - Manage tiering via switchable profiles (`argon-flash`, `opus-flash`, `sonnet-flash`, `pro-flash`).
   - Support seamless fallback to `opus-flash` when token quotas or daily limits are encountered.

4. **Strict Test Coverage**:
   - Any modifications to the permission catalog, parser, merger, profiles, or audit engine must be accompanied by updated unit tests in `tests/test_cli.py`.
   - Run tests with `python3 -m unittest discover tests -v`.

5. **Conventional Commits**:
   - Adhere strictly to Conventional Commits 1.0.0.

6. **Dynamic Paths**:
   - Never hardcode `/home/cooper`. Use `$HOME` in bash and `Path.home()` in Python.
