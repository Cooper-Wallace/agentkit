# AI Agent Plugins & Lifecycle Hooks Guide

This guide details the architecture of **Plugins** and **Lifecycle Hooks** in AI developer harnesses, provides templates for creating them, and outlines standard plugins, lifecycle hooks, and advanced execution patterns recommended across developer workstations under **`agentkit`** (Layer 2) and **`repokit`** (Layer 3).

---

## 1. Overview & Concepts

| Concept | File / Manifest | Purpose | Trigger / Lifecycle Point |
| :--- | :--- | :--- | :--- |
| **Lifecycle Hooks** | `.agents/hooks.json` | Runs arbitrary shell commands at agent loop milestones. | `PreInvocation`, `PostInvocation`, `PreToolUse`, `PostToolUse`, `Stop` |
| **Plugins** | `.agents/plugins/<name>/plugin.json` | Self-contained bundles uniting rules, skills, hooks, and MCP servers. | Loaded on workspace launch; toggleable via harness CLI or configuration. |
| **Plugin Registry** | `plugins.json` | Explicit registration of external plugin directories. | Loaded globally (`~/.gemini/config/`) or locally (`.agents/`). |

---

## 2. Standard Lifecycle Hooks

The following lifecycle hooks represent core automation and safety components implemented across workstations:

### A. PreToolUse Subagent Delegation Guard
- **Lifecycle Event**: `PreToolUse` matching `edit_file|write_file`
- **Script**: `scripts/guard-subagent-delegation.sh`
- **Purpose**: Enforces cognitive tiering by blocking the Root Orchestrator (Gemini 4 Argon, Claude Opus 5.5, Claude Sonnet 3.7, or Gemini 3.8 Flash High) from directly editing project code, tests, or UI files, ensuring implementation work is delegated to Flash worker subagents.

### B. PreToolUse Git & Filesystem Safety Gate
- **Lifecycle Event**: `PreToolUse` matching `run_command`
- **Script**: `scripts/guard-git-operations.sh`
- **Purpose**: A deterministic programmatic circuit-breaker blocking direct pushes, force-pushes, or merges to `main`/`master`, while prompting for human confirmation on destructive commands (`rm -rf`, `mkfs`, `dd`).

### C. PreToolUse Quota Guardian & Fallback Router
- **Lifecycle Event**: `PreToolUse` matching `invoke_subagent`
- **Script**: `scripts/guard-quota.py`
- **Purpose**: Intercepts subagent delegation calls, checks remaining token quotas via `RetrieveUserQuotaSummary`, dynamically reroutes subagents to fallback model families when quotas fall below 15%, and enforces graceful wrap-up when quotas fall below 5%.

### D. Stop Quality & Completion Gate
- **Lifecycle Event**: `Stop`
- **Script**: `scripts/verify-clean-completion.sh`
- **Purpose**: Intercepts agent loop termination. Rejects completion if automated test suites fail or if uncommitted changes linger in the git working tree.

### E. PostToolUse Auto-Formatter & Linter
- **Lifecycle Event**: `PostToolUse` matching `write_to_file|edit_file`
- **Purpose**: Automatically runs fast language formatters (e.g., `ruff format` on `.py`, `prettier` on `.ts/.json`, `gofmt` on `.go`) immediately after the agent writes or modifies a file.

### F. PreInvocation Git Hygiene Watcher
- **Lifecycle Event**: `PreInvocation`
- **Purpose**: Runs `git status --porcelain` at the start of an agent turn. If unstaged scratch files or a detached `HEAD` state are detected, it surfaces a concise alert in the agent's context.

---

## 3. The `PreToolUse` Overwrite Pattern for Output Token Compression

### Why `PreToolUse` Command Wrapping is Required

In autonomous coding loops, verbose commands (such as full test suite executions via `pytest` or `npm test`, voluminous `git diff` outputs, package listings, and directory trees) can emit tens of thousands of tokens per turn into `transcript.jsonl`. This rapidly exhausts the LLM context window and forces premature compaction.

Standard compression approaches fail in modern harnesses due to two hard constraints:
1. **`PostToolUse` Hook Immutability**: Under standard lifecycle contracts, `PostToolUse` handlers can only return `{}` on stdout. They can trigger asynchronous notifications or logging, but they **cannot mutate, truncate, or rewrite** the tool output buffer after execution has completed.
2. **Network Proxy Incompatibility**: External HTTP/REST proxy layers (e.g., `headroom proxy --port 8787`) are incompatible with native backend transports, authentication channels, and internal RPC protocols.

### The Solution: Transparent `PreToolUse` Overwrite

To achieve token compression without violating lifecycle constraints, **`agentkit`** employs the **`PreToolUse` `overwrite` pattern** via `scripts/filter-and-wrap.py` and `scripts/headroom-wrap.py`:

```mermaid
flowchart TD
    A["Agent calls run_command"] --> B["PreToolUse Hook: filter-and-wrap.py"]
    B -->|Check safety gate| C{Destructive or trunk push?}
    C -->|Yes| D["Block / Ask user confirmation"]
    C -->|No| E{Verbose inspection / test command?}
    E -->|No / Daemon / Pipe| F["Allow original command unchanged"]
    E -->|Yes| G["Emit overwrite payload:<br/>headroom-wrap.py -- &lt;cmd&gt;"]
    G --> H["Harness executes headroom-wrap.py"]
    H --> I["Subprocess executes target command"]
    I --> J{Output &gt; 1,500 chars?}
    J -->|No| K["Pass raw output through"]
    J -->|Yes| L["Hash & cache raw output in<br/>~/.cache/antigravity-headroom/&lt;hash&gt;.txt"]
    L --> M["Compress JSON / test logs (60-90% savings)"]
    M --> N["Append 1-line CCR retrieval breadcrumb"]
    N --> O["Emit compressed text to stdout<br/>(Recorded in transcript.jsonl)"]
```

### Hook Protocol & Overwrite Payload

When a candidate command is intercepted, `filter-and-wrap.py` returns an `overwrite` JSON payload on stdout:

```json
{
  "decision": "allow",
  "overwrite": {
    "CommandLine": "python3 ~/.gemini/config/plugins/autonomous-orchestrator/scripts/headroom-wrap.py -- pytest tests -v"
  }
}
```

### Reversible CCR (Contextual Compression with Retrieval)

`headroom-wrap.py` enforces **Reversible CCR**:
- **Exit Status Preservation**: Captures standard output and standard error while deterministically preserving `proc.returncode` (`sys.exit(proc.returncode)`).
- **Local Disk Cache**: Outputs exceeding `COMPRESSION_THRESHOLD` (default 1,500 chars) are hashed (`SHA-256[:12]`) and stored at `~/.cache/antigravity-headroom/<hash>.txt` with automatic LRU pruning (capped at 100 cache files).
- **Content-Aware Compression**: Employs `headroom-ai` (if installed) or deterministic built-in engines (`JSON SmartCrusher` and `LogCrusher`) to collapse repetitive passing lines and schema padding while preserving all failure tracebacks.
- **Retrieval Breadcrumb**: Appends a clear reference line:
  ```text
  [Headroom CCR: compressed 14,200 -> 1,850 chars (87% saved). Full output cached at ~/.cache/antigravity-headroom/a1b2c3d4e5f6.txt]
  ```
  Any agent requiring the full unabridged output can simply read the cached file with native `read_file`.

---

## 4. Standard Harness Plugins (Layer 2 Governance)

The following production plugins are bundled with `agentkit` to govern autonomous multi-model orchestration, token economy, and quota safety:

### 1. `autonomous-orchestrator` (Cognitive Tiering & Headroom CCR)
- **Purpose**: Enforces cognitive tiering, context window preservation, Headroom CCR token compression, and git safety gates across Gemini 4 Argon, Claude Opus 5.5, Claude Sonnet 3.7, and Gemini 3.8 Flash.
- **Components & Architecture**:
  - **Hooks**:
    - `PreToolUse` matching `edit_file|write_file` -> `scripts/guard-subagent-delegation.sh`: Blocks the Root Orchestrator from directly modifying project source code, tests, or UI files, mandating delegation to Flash subagents.
    - `PreToolUse` matching `run_command` -> `scripts/guard-git-operations.sh`: Hard-blocks direct pushes or merges to `main`/`master`, and gates destructive operations (`rm -rf`, `mkfs`, `dd`).
    - `PreToolUse` matching `run_command` -> `scripts/filter-and-wrap.py`: Intercepts verbose test, inspection, and diff commands, rewriting them to execute under `scripts/headroom-wrap.py` for 60%–90% token compression with Reversible CCR caching in `~/.cache/antigravity-headroom/<hash>.txt`.
    - `Stop` -> `scripts/verify-clean-completion.sh`: Rejects agent completion if automated tests fail or if uncommitted changes linger in the git working tree.
  - **Rules**: `rules/AGENTS.md` (Cognitive tiering hierarchy, pure-edit subagent isolation, prefix-matchable command shapes, conventional commits).
  - **Skills**: `skills/auto-lifecycle/SKILL.md` (4-phase autonomous feature lifecycle: Contract Freeze, Worker Implementation, Adversarial Review, Promotion & Cleanup).

### 2. `quota-guardian` (Quota-Aware Routing, Complexity Mapping & Wrap-Up Gate)
- **Purpose**: Autonomous quota-aware model routing, subagent task complexity mapping, and graceful wrap-up circuit-breaker to eliminate hung subagents and data loss during token quota exhaustion.
- **Components & Architecture**:
  - **Hooks**:
    - `PreToolUse` matching `invoke_subagent` -> executes `scripts/guard-quota.py` (timeout: 10s).
  - **Scripts**:
    - `scripts/quota_status.py`: Interrogates the Antigravity Language Server RPC or local cache, categorizes usage into quota groups, and formats tabular/JSON output for CLI and hooks.
    - `scripts/guard-quota.py`: Intercepts `invoke_subagent` calls. Evaluates task complexity tag (`[complexity: low|med|high]`), checks remaining quota in the target model family, and automatically rewrites the subagent `Model` argument to a healthy fallback family when quota drops below 15%. If all groups drop below 5%, denies execution with `QUOTA WRAP-UP:`.
  - **Configuration (`config/quota.json`)**:
    - `thresholds`: `{"switch_threshold": 0.15, "wrapup_threshold": 0.05, "cache_ttl_seconds": 60}`
    - `groups`: Maps model identifiers to 3 quota groups:
      - `gemini`: Gemini 3.8 Flash, Flash Lite
      - `claude`: Claude Opus 5.5, Claude Sonnet 3.7
      - `argon`: Gemini 4 Argon
    - `complexity_to_model`:
      - `low`: `flash_lite` (file moves/renames, mechanical find-replace, formatting, doc typos, simple one-command inspection).
      - `med`: `flash` (feature code, tests, localized refactors).
      - `high`: `flash` (with escalation to `pro` for architecture, multi-repo design, or adversarial review when Argon quota is healthy).
    - `fallbacks`: Declarative fallback routing sequences (`flash_lite` -> `inherit`, `flash` -> `inherit`, `pro` -> `flash` -> `inherit`).
  - **Language Server RPC Endpoint**:
    - Interrogates `RetrieveUserQuotaSummary` over HTTP/JSON:
      `POST http://$ANTIGRAVITY_LS_ADDRESS/exa.language_server_pb.LanguageServerService/RetrieveUserQuotaSummary`
      Headers: `Content-Type: application/json`, `x-codeium-csrf-token: $ANTIGRAVITY_CSRF_TOKEN`
  - **Environment Variables**:
    - `AGENTKIT_QUOTA_GUARD`: Set to `0` or `false` to disable the pre-tool subagent guard (useful during offline runs or emergency bypass).
    - `AGENTKIT_QUOTA_FIXTURE`: Absolute path to a mock JSON quota response for unit tests and deterministic simulation.
    - `AGENTKIT_QUOTA_CACHE`: Custom file path for caching the language server quota response (defaults to `~/.cache/antigravity-quota/summary.json`).
  - **Graceful Wrap-Up Protocol**:
    - When all quota families drop below 5%, `guard-quota.py` denies delegation with `QUOTA WRAP-UP:`.
    - Enforces the `quota-wrapup` protocol: Cease spawning subagents, run local test suite, commit WIP changes on the feature branch, generate a structured `handoff.md`, and advise the user of the exact quota reset time and recommended `/model` switch.

---

## 5. Workstation Stack Integration Plugins (Architecture)

The following modular plugins coordinate across the 4-layer workstation stack:

### 1. `rko-workstation-companion` (Layer 0 Integration)
- **Purpose**: Integrates the AI agent harness with Cooper Wallace's Kubuntu rice ([`riced-kubuntu-os`](file:///home/cooper/Projects/riced-kubuntu-os)).
- **Components**:
  - **Skills**: `snapshot-plasma` (sanitizes and exports KDE Plasma 6 configs), `audit-packages` (diffs manual apt/flatpak installs against manifests).
  - **Rules**: OS baseline specifications (Kubuntu 26.04 LTS, KDE Plasma 6, Wayland).
  - **Hooks**: Auto-verifies dotfile symlink integrity after workstation provisioning.

### 2. `repokit-devops-governance` (Layer 3 Integration)
- **Purpose**: Integrates the AI agent harness with repository DevOps standards ([`repokit`](file:///home/cooper/Projects/repokit)).
- **Components**:
  - **Skills**: `audit-repo-standards` (runs `repokit audit` on active workspace), `configure-ruleset` (applies GitHub branch protection).
  - **Rules**: Conventional Commits specification rules, PR title formatting guidelines.
  - **Hooks**: `PreToolUse` check ensuring branches follow `feat/`, `fix/`, `docs/`, `refactor/` naming conventions.

### 3. `agentkit-safety-kit` (Layer 2 Governance)
- **Purpose**: Comprehensive workstation defense-in-depth bundle.
- **Components**:
  - **Rules**: Strict dynamic path enforcement, forbidden disk commands.
  - **Hooks**: Pre-command safety gate.
  - **MCP Server**: Sandboxed terminal runner or credential leak scanner (checking for `.env` or API tokens).

---

## 6. Templates Available in this Repository

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

## 7. Activating a Plugin or Hook

When you are ready to implement a specific plugin or hook:

### To activate a project hook:
```bash
cp ~/Projects/agentkit/templates/hooks.json .agents/hooks.json
# Edit .agents/hooks.json to set "enabled": true for desired hooks
```

### To scaffold a new plugin:
```bash
cp -r ~/Projects/agentkit/templates/plugin .agents/plugins/my-custom-plugin
# Edit .agents/plugins/my-custom-plugin/plugin.json and set "disabled": false
```
