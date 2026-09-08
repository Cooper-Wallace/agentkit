# Antigravity Plugins & Lifecycle Hooks Guide

This guide details the architecture of Antigravity **Plugins** and **Lifecycle Hooks**, provides templates for creating them, and outlines high-value standard plugins and hooks recommended for future adoption across developer workstations.

---

## 1. Overview & Concepts

| Concept | File / Manifest | Purpose | Trigger / Lifecycle Point |
| :--- | :--- | :--- | :--- |
| **Lifecycle Hooks** | `.agents/hooks.json` | Runs arbitrary shell commands at agent loop milestones. | `PreInvocation`, `PostInvocation`, `PreToolUse`, `PostToolUse` |
| **Plugins** | `.agents/plugins/<name>/plugin.json` | Self-contained bundles uniting rules, skills, hooks, and MCP servers. | Loaded on workspace launch; toggleable via `agy plugin {enable,disable}`. |
| **Plugin Registry** | `plugins.json` | Explicit registration of external plugin directories. | Loaded globally (`~/.gemini/config/`) or locally (`.agents/`). |

---

## 2. Potential Standard Hooks (Recommendations)

The following lifecycle hooks represent potential automation and safety candidates for future implementation:

### A. PreToolUse Shell Safety Gate
- **Lifecycle Event**: `PreToolUse` matching `run_command`
- **Concept**: A lightweight script inspecting the proposed `CommandLine` before execution. If it detects unapproved recursive file deletions (`rm -rf`), disk formatting (`mkfs`), raw block writes (`dd`), or accidental fork bombs, it exits with non-zero to abort execution before any command runs.
- **Why Valuable**: Acts as a hard programmatic circuit-breaker backing up the declarative permission boundary.

### B. PostToolUse Auto-Formatter & Linter
- **Lifecycle Event**: `PostToolUse` matching `write_to_file|replace_file_content`
- **Concept**: Automatically runs fast language formatters (e.g., `ruff format` on `.py`, `prettier` on `.ts/.json`, `gofmt` on `.go`) immediately after the agent writes or modifies a file.
- **Why Valuable**: Keeps code cleanly formatted and lint-free throughout agent turns without requiring extra prompting turns.

### C. PreInvocation Git Hygiene Watcher
- **Lifecycle Event**: `PreInvocation`
- **Concept**: Runs `git status --porcelain` at the start of an agent turn. If unstaged scratch files or a detached `HEAD` state are detected, it surfaces a concise alert in the agent's context.
- **Why Valuable**: Prevents the agent from operating on dirty states or committing unintended debug artifacts.

### D. PostToolUse Commit-Msg Verifier
- **Lifecycle Event**: `PostToolUse` matching `run_command`
- **Concept**: Intercepts `git commit` commands to verify that commit messages adhere strictly to Conventional Commits 1.0.0 before they hit the local repository tree.

---

## 3. Potential Standard Plugins (Recommendations)

The following modular plugins could be bundled into `antigravity-standardise` and registered across workstations:

### 1. `rko-workstation-companion`
- **Purpose**: Integrates Antigravity with Cooper Wallace's Kubuntu rice ([`riced-kubuntu-os`](file:///home/cooper/Projects/riced-kubuntu-os)).
- **Components**:
  - **Skills**: `snapshot-plasma` (sanitizes and exports KDE Plasma 6 configs), `audit-packages` (diffs manual apt/flatpak installs against manifests).
  - **Rules**: OS baseline specifications (Kubuntu 26.04 LTS, KDE Plasma 6, Wayland).
  - **Hooks**: Auto-verifies dotfile symlink integrity after workstation provisioning.

### 2. `gh-devops-governance`
- **Purpose**: Integrates Antigravity with repository DevOps standards ([`gh-standardise`](file:///home/cooper/Projects/gh-standardise)).
- **Components**:
  - **Skills**: `audit-repo-standards` (runs `gh-standardise audit` on active workspace), `configure-ruleset` (applies GitHub branch protection).
  - **Rules**: Conventional Commits specification rules, PR title formatting guidelines.
  - **Hooks**: `PreToolUse` check ensuring branches follow `feat/`, `fix/`, `docs/`, `refactor/` naming conventions.

### 3. `developer-safety-kit`
- **Purpose**: Comprehensive workstation defense-in-depth bundle.
- **Components**:
  - **Rules**: Strict dynamic path enforcement, forbidden disk commands.
  - **Hooks**: Pre-command safety gate.
  - **MCP Server**: Sandboxed terminal runner or credential leak scanner (checking for `.env` or API tokens).

---

## 4. Templates Available in this Repository

This repository includes ready-to-use templates for future implementation:

1. [`templates/hooks.json`](../templates/hooks.json): Sample lifecycle hooks with `command-safety-guard`, `code-linter-formatter`, and `git-hygiene-reminder` pre-configured (disabled by default).
2. [`templates/plugins.json`](../templates/plugins.json): Standard schema for registering plugin search paths.
3. [`templates/plugin/`](../templates/plugin/): Complete plugin directory archetype containing:
   - `plugin.json`: Plugin metadata manifest
   - `hooks.json`: Plugin-scoped lifecycle hooks
   - `mcp_config.json`: MCP server configuration
   - `rules/AGENTS.md`: Plugin rules
   - `skills/sample-skill/SKILL.md`: Procedural skill runbook

---

## 5. Activating a Plugin or Hook

When you are ready to implement a specific plugin or hook:

### To activate a project hook:
```bash
cp ~/Projects/antigravity-standardise/templates/hooks.json .agents/hooks.json
# Edit .agents/hooks.json to set "enabled": true for desired hooks
```

### To scaffold a new plugin:
```bash
cp -r ~/Projects/antigravity-standardise/templates/plugin .agents/plugins/my-custom-plugin
# Edit .agents/plugins/my-custom-plugin/plugin.json and set "disabled": false
```
