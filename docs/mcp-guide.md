# Antigravity MCP Server Standardization Guide

The **Model Context Protocol (MCP)** enables Antigravity to connect with external tools, databases, and services (such as GitHub, SQLite, DevTools, or internal APIs). 

This guide explains how `antigravity-standardise` manages MCP servers declaratively while keeping credentials strictly decoupled and secure.

---

## 1. The Twelve-Factor Secrets Architecture

> [!IMPORTANT]
> **Zero Credential Exposure**: Never commit raw API keys, Personal Access Tokens, or passwords to version control.

Instead of hardcoding sensitive secrets in git, `config/mcp.json` references environment variables using the `${VARIABLE_NAME}` syntax:

```json
{
  "mcpServers": {
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "${GITHUB_PERSONAL_ACCESS_TOKEN}"
      }
    }
  }
}
```

When Antigravity spawns the MCP server process, it inherits your workstation shell environment, passing the exported token securely to the server.

---

## 2. Declarative Catalog & Standard Servers

`antigravity-standardise` provides a declarative catalog in `config/mcp.json` establishing **5 standard MCP servers** across all developer workstations:

| Server | Binary / Package | Role & Scope | Dependencies |
| :--- | :--- | :--- | :--- |
| **`github`** | `npx @modelcontextprotocol/server-github` | Repository inspection, PR reviews, issue management. | `node`, `npx`, `GITHUB_PERSONAL_ACCESS_TOKEN` |
| **`fetch`** | `npx @modelcontextprotocol/server-fetch` | Web document fetching and markdown conversion. | `node`, `npx` |
| **`sqlite`** | `uvx mcp-server-sqlite` | Local SQLite database querying and diagnostic state. | `uvx`, `SQLITE_DEV_DB` |
| **`memory`** | `npx @modelcontextprotocol/server-memory` | Persistent cross-repository knowledge graph. | `node`, `npx` |
| **`repomix`** | `npx repomix --mcp` | AST-compressed codebase packing and structural skeletons. | `node`, `npx` |

Each server configuration in `config/mcp.json` is paired with diagnostic requirements under `_requirements`:

```json
{
  "_requirements": {
    "github": {
      "description": "GitHub integration for repository, PR, and issue management via MCP.",
      "required_bins": ["npx", "node"],
      "required_env": ["GITHUB_PERSONAL_ACCESS_TOKEN"],
      "setup_hint": "Create a GitHub PAT with repo scope and add 'export GITHUB_PERSONAL_ACCESS_TOKEN=\"...\"' to ~/.bashrc"
    },
    "memory": {
      "description": "Persistent cross-repository knowledge graph for architectural contracts, entity relationships, and multi-session decisions.",
      "required_bins": ["npx", "node"],
      "required_env": [],
      "setup_hint": "Run 'workstation-setup toolchains' to install Node.js and npx."
    },
    "repomix": {
      "description": "AST-compressed repository packing for low-token modular cross-repository code inspection.",
      "required_bins": ["npx", "node"],
      "required_env": [],
      "setup_hint": "Run 'workstation-setup toolchains' to install Node.js and npx."
    }
  }
}
```

---

## 3. Active Diagnostics via `agy-standardise`

When you run `agy-standardise audit` or `agy-standardise apply`, the tool:
1. Verifies that `~/.gemini/config/mcp_config.json` contains all 5 standard server definitions.
2. Checks whether required command-line binaries (e.g. `npx`, `uvx`) are present in `$PATH` (`shutil.which`).
3. Checks whether required environment variables are set in your current shell (`os.environ`).
4. Prints clear, actionable setup advisories for any missing variables.

Example Diagnostic Output:
```text
==> MCP Server Diagnostics
  ✔ MCP Configuration: All 5 standard servers configured
  ⚠ MCP 'github': Missing environment variable: GITHUB_PERSONAL_ACCESS_TOKEN
  ✔ MCP 'fetch': Dependencies and environment ready
  ⚠ MCP 'sqlite': Missing required binary in PATH: uvx
  ⚠ MCP 'sqlite': Missing environment variable: SQLITE_DEV_DB
  ✔ MCP 'memory': Dependencies and environment ready
  ✔ MCP 'repomix': Dependencies and environment ready

==> MCP Environment Setup Advisories
  ℹ github: Create a GitHub PAT with repo scope and add 'export GITHUB_PERSONAL_ACCESS_TOKEN="..."' to ~/.bashrc
  ℹ sqlite: Install uv (pip install uv) and add 'export SQLITE_DEV_DB="$HOME/dev.db"' to ~/.bashrc
```

---

## 4. Cross-Repository Memory & AST Token Optimization

In modular architectures spanning multiple projects (such as `antigravity-standardise`, `riced-kubuntu-os`, `gh-standardise`, and `workstation-setup`), agents frequently suffer from context compaction and token bloat when cross-referencing codebases. The `memory` and `repomix` MCP servers solve these problems:

### A. Perpetual Cross-Repo Knowledge Graph (`memory`)
The `@modelcontextprotocol/server-memory` server runs locally to maintain an enduring knowledge graph of entities, relations, architectural contracts, and historical decisions.
- **Cross-Session Continuity**: Prevents repeated rediscovery of subsystem boundaries, API contracts, and conventions across disjoint agent sessions.
- **Zero Prompt Bloat**: Rather than duplicating monolithic architectural documentation into every prompt, agents dynamically read and write relational facts into the persistent knowledge graph on demand.

### B. AST Structural Code Compression (`repomix`)
Running `repomix --mcp` exposes repository packing and structure-extraction tools directly over MCP.
- **AST Skeleton Pruning**: Uses Tree-sitter AST parsing to strip implementation bodies while retaining method signatures, exported types, interfaces, and docstrings.
- **70%–80% Token Reduction**: Instead of loading thousands of lines of raw source code across multiple repositories, agents inspect compact structural codebase skeletons, drastically lowering prompt token consumption and preserving context window capacity.

---

## 5. Workstation Onboarding Steps

To enable your pre-configured MCP servers on any machine:

1. **Apply Configuration**:
   ```bash
   agy-standardise apply
   ```
2. **Review Advisories**:
   Inspect the advisory output or run:
   ```bash
   agy-standardise audit -v
   ```
3. **Export Required Environment Variables**:
   Add the requested tokens to your `~/.bashrc` or `~/.zshrc`:
   ```bash
   export GITHUB_PERSONAL_ACCESS_TOKEN="ghp_yourTokenHere"
   export SQLITE_DEV_DB="$HOME/dev.db"
   ```
   Reload your shell:
   ```bash
   source ~/.bashrc
   ```

---

## 6. Adding New Standard MCP Servers

To standardize a new MCP server across all your machines:

1. Open [`config/mcp.json`](../config/mcp.json).
2. Add the server entry to `"mcpServers"` (using `${ENV_VAR}` for any sensitive values).
3. Add the diagnostic requirements and actionable setup hint to `"_requirements"`.
4. Run `agy-standardise apply` to sync to your local `~/.gemini/config/mcp_config.json`.
5. Commit and push:
   ```bash
   git add config/mcp.json
   git commit -m "feat(mcp): add postgres query server definition"
   ```
