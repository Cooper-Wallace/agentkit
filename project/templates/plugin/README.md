# Antigravity Plugin Archetype

This directory provides a reference template for packaging **Skills**, **Rules**, **Hooks**, and **MCP Servers** into a single deployable Antigravity plugin.

---

## Directory Layout

```text
my-plugin/
├── plugin.json       # Plugin manifest (name, description, version, disabled)
├── hooks.json        # Lifecycle hooks (PreToolUse, PostToolUse, PreInvocation)
├── mcp_config.json   # MCP servers declared by this plugin
├── rules/            # Contextual rules (AGENTS.md)
│   └── AGENTS.md
└── skills/           # Actionable runbooks with progressive disclosure
    └── <skill-name>/
        └── SKILL.md
```

---

## How to Deploy a Plugin

1. Copy this archetype into `.agents/plugins/<plugin-name>` within your project (or into a global shared directory).
2. Update `plugin.json` with your plugin's unique name and remove `"disabled": true` when ready to activate.
3. Manage the plugin using native Antigravity CLI commands:
   ```bash
   agy plugin list
   agy plugin enable <plugin-name>
   agy plugin disable <plugin-name>
   ```
