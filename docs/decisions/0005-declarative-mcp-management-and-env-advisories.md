# ADR 0005: Declarative MCP Management and Environment Variable Advisories

## Context
Model Context Protocol (MCP) servers extend Antigravity with external capabilities. Historically, configuring MCP servers across multiple workstations led to two recurring issues:
1. **Secret Leakage Risks**: Storing MCP configurations directly in version control often accidentally exposes raw API keys, PATs, or database credentials.
2. **Silent Failure & Onboarding Ambiguity**: A fresh machine cloned without documentation does not indicate what environment variables or CLI tools need to be provisioned for an MCP server to connect successfully.

## Decision
1. **Store Declarative MCP Configs in `config/mcp.json`**:
   Configurations are tracked in git, but all tokens and secrets are decoupled using environment variable interpolations (e.g. `${GITHUB_PERSONAL_ACCESS_TOKEN}`).
2. **Metadata Requirements & Actionable Advisories**:
   Each MCP server includes metadata in `_requirements` declaring required binaries and environment variables, along with a human-readable setup hint.
3. **Active Diagnostic Engine in `agy-standardise`**:
   The CLI audits both the registration of the server in `~/.gemini/config/mcp_config.json` and the presence of binaries (`shutil.which`) and environment variables (`os.environ`).
4. **Idempotent Set-Union Merging with Backups**:
   `agy-standardise apply` merges standard servers into `~/.gemini/config/mcp_config.json`, preserving any custom local servers and backing up existing configurations (`.bak.<timestamp>`).

## Consequences
- **Positive**: Zero secret exposure in git history.
- **Positive**: Instant machine parity—cloning on a new machine provisions all MCP definitions automatically.
- **Positive**: Actionable diagnostics eliminate debugging of silent connection failures.
