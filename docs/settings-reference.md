# Antigravity CLI Settings Reference

Google Antigravity CLI stores user preferences, permission overrides, and security workspace boundaries in `~/.gemini/antigravity-cli/settings.json`.

---

## 1. Schema & Anatomy

A complete, standard `settings.json` has the following structure:

```json
{
  "allowNonWorkspaceAccess": true,
  "colorScheme": "dark",
  "model": "Gemini 3.8 Flash (High)",
  "permissions": {
    "allow": [
      "command(which)",
      "command(cat)",
      "command(git status)",
      "command(pytest)"
    ]
  },
  "trustedWorkspaces": [
    "/home/cooper",
    "/home/cooper/Projects",
    "/home/cooper/Documents/AgentVault"
  ]
}
```

### Key Fields

| Field | Type | Description | Default |
| :--- | :--- | :--- | :--- |
| `allowNonWorkspaceAccess` | boolean | Allows the agent to read/write files and execute commands outside explicitly trusted workspaces. | `true` |
| `colorScheme` | string | Terminal UI color theme (`"dark"` or `"light"`). | `"dark"` |
| `model` | string | Preferred default AI model (e.g., `"Gemini 3.8 Flash (High)"`). | `"Gemini 3.8 Flash (High)"` |
| `permissions` | object | Contains security rule overrides for tool actions. | `{}` |
| `permissions.allow` | list[string] | List of pre-approved commands in `command(...)` format. | `[]` |
| `trustedWorkspaces` | list[string] | Absolute filesystem paths trusted for workspace operations, unprompted file access, and local hook discovery. Includes Obsidian AgentVault (`~/Documents/AgentVault` or `~/AgentVault`) automatically via `agentkit apply` and `agentkit vault init`. | `[]` |

> [!TIP]
> **Automatic Whitelisting of Obsidian AgentVault (`trustedWorkspaces`)**:
> During autonomous multi-agent workflows, agents regularly inspect `$HOME/Documents/AgentVault/User-Input/pending/` for offline user directives, persist session handoffs to `Sessions/`, and maintain living project documentation in `Projects/`. To prevent interactive confirmation prompts from interrupting autonomous workers, `agentkit apply` and `agentkit vault init` automatically ensure the active AgentVault path (`$HOME/Documents/AgentVault` or `$HOME/AgentVault`) is present in `trustedWorkspaces`.

---

## 2. Permission String Syntax

Antigravity CLI supports exact and binary-level command matching:

- **Binary Match**: `command(ls)`
  Pre-approves any execution of `ls`, regardless of arguments (e.g., `ls -la`, `ls /tmp`).
- **Compound / Subcommand Match**: `command(git status)`
  Pre-approves specific invocations or subcommands while leaving other subcommands gated.
- **Full Shell String**: `command(python3 -m unittest discover tests)`
  Pre-approves exact chained test invocations.

---

## 3. CLI Management Commands

Inside the Antigravity TUI:
- `/permissions`: Interactive editor to view, add, or revoke command permissions.
- `/settings`: Interactive viewer and toggle for user preferences (such as theme and copyOnSelect).

Using `agentkit` (or `ak`):
- `agentkit audit`: Validates current `settings.json` against the standard baseline.
- `agentkit apply`: Merges missing baseline permissions while preserving custom workspaces.
- `agentkit diff`: Compares active permissions with the baseline.
- `agentkit snapshot`: Lists permissions approved on the machine that are not yet tracked in git.
