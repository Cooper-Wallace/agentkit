# antigravity-standardise - Agent Directives & Architecture

Welcome to the **antigravity-standardise** repository. This repository defines, audits, and enforces Antigravity CLI standards, command permissions, and developer guidelines across Cooper Wallace's workstations.

---

## 1. Repository Layout

```text
.
├── AGENTS.md                 # Primary agent instructions and repository map
├── GEMINI.md                 # Symlink to AGENTS.md
├── README.md                 # Human-oriented quickstart and usage documentation
├── LICENSE                   # MIT License
├── install.sh                # Symlinks agy-standardise to ~/.local/bin
│
├── bin/
│   └── agy-standardise       # Python 3 CLI tool (audit, configure/apply, diff, snapshot, template)
│
├── config/
│   ├── permissions.json      # Declarative catalog of pre-approved commands by category
│   └── settings.default.json # Baseline configuration defaults (theme, non-workspace access)
│
├── templates/
│   ├── GEMINI.md             # Canonical global developer directives template
│   ├── AGENTS.md             # Symlink to GEMINI.md
│   ├── project-GEMINI.md     # Scaffolding template for managed repositories
│   └── rules/                # Contextual rules (bash-safety.md, conventional-commits.md)
│
├── docs/                     # Documentation suite
│   ├── permissions-matrix.md # Security classification and rationale for permissions
│   ├── gemini-md-guide.md    # Guide on Antigravity cascading rules
│   ├── replication-guide.md  # Multi-machine setup checklist
│   ├── settings-reference.md # Complete anatomy of Antigravity CLI settings
│   └── decisions/            # Architecture Decision Records (ADRs)
│
└── tests/
    └── test_cli.py           # Automated unit test suite
```

---

## 2. Guiding Principles for Agent Interactions

1. **Idempotence & Non-Destructive Operations**:
   - The CLI (`agy-standardise`) must never erase existing custom permissions, active models, or trusted workspaces from `~/.gemini/antigravity-cli/settings.json`.
   - Modifying operations must create automated, timestamped backups (`settings.json.bak.<timestamp>`).
   - All state-altering subcommands must respect the `--dry-run` flag.

2. **Strict Test Coverage**:
   - Any modifications to the permission catalog, parser, merger, or audit engine must be accompanied by updated unit tests in `tests/test_cli.py`.
   - Run tests with `python3 -m unittest discover tests -v`.

3. **Conventional Commits**:
   - Adhere strictly to Conventional Commits 1.0.0.

4. **Dynamic Paths**:
   - Never hardcode `/home/cooper`. Use `$HOME` in bash and `Path.home()` in Python.
