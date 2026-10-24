---
name: obsidian-vault
description: "Navigate, read, update, and manage the Obsidian Perpetual Knowledge Vault for long-term agent memory, offline human-agent collaboration, project knowledge bases, and session handoffs."
---

# Obsidian Perpetual Knowledge Vault Skill

This skill teaches AI agents how to navigate, read, update, and maintain the developer's local **Obsidian Perpetual Knowledge Vault** (`~/Documents/Vault` or `~/Vault`). Based on the open-source **Obsidian Skills** specification initiated by Steph Ango (kepano, CEO of Obsidian) and the "files over apps" philosophy, this vault serves as the persistent, human-readable long-term memory layer across agent conversations.

---

## 1. Core Philosophy: Files Over Apps & Human-in-the-Loop Offline Collaboration

Traditional agent memory often relies on opaque, ephemeral vector stores or JSON graph blobs (`@modelcontextprotocol/server-memory`) that developers cannot easily inspect, curate, or manipulate outside of active agent sessions. 

The **Obsidian Perpetual Knowledge Vault** replaces opaque memory mechanisms with standard, durable Markdown files and visual JSON Canvas diagrams:
- **Files Over Apps**: Plaintext Markdown files in a local directory remain readable, version-controllable, and portable for decades without vendor lock-in.
- **Asynchronous Offline Collaboration**: During breaks between conversations or overnight, the developer opens Obsidian on their desktop or mobile device, inspects previous session handoffs, updates project tasks, adjusts architectural guidelines, and drops offline directives into `User-Input/pending/`.
- **Bidirectional Ingestion & Synthesis**: When a new conversation session starts, the agent immediately inspects `User-Input/pending/` and active project notes in `Projects/`, inlining offline directives directly into Phase 1 planning artifacts (`plan.md`). At session completion, the agent writes a structured handoff note to `Sessions/handoff-YYYY-MM-DD-<topic>.md` and updates `Projects/<project>.md`.

---

## 2. Vault Location & Environment Resolution

Agents must locate the Obsidian Vault dynamically in order of precedence:
1. `$AGENTKIT_VAULT_PATH` environment variable (if set).
2. `$HOME/Documents/Vault` (standard desktop default).
3. `$HOME/Vault` (alternate standard path).

> [!IMPORTANT]
> Never hardcode `/home/cooper/Documents/Vault`. Always resolve dynamically using `$HOME` in bash, `Path.home() / "Documents" / "Vault"` in Python, or relative paths.

---

## 3. Vault Standard Directory Structure

```text
~/Documents/Vault/
├── User-Input/
│   ├── pending/                  # Unread user directives, offline prompts, and feedback
│   └── processed/                # Ingested directives archived with processing timestamps
├── Projects/                     # Cross-project living documentation and state tracking
│   ├── agentkit.md               # Architecture, roadmap, active DAGs, and known issues
│   ├── repokit.md                # GitHub rulesets, squash merge policies, PR hooks
│   ├── riced-kubuntu-os.md       # Kubuntu OS packages, KDE Plasma 6 configurations
│   └── workstation-setup.md      # Toolchains, shell profiles, keyring setups
├── Architecture/                 # System boundaries, contracts, and ADR summaries
│   ├── Workstation-Stack.canvas  # Visual JSON Canvas diagram of the 4-layer workstation stack
│   ├── contracts/                # Cross-repo schemas and invariant definitions
│   └── adrs/                     # Architecture Decision Record summaries with wikilinks
├── Sessions/                     # Immutable records of agent turns and handoff notes
│   ├── handoff-2026-10-24-vault.md
│   └── handoff-YYYY-MM-DD-<topic>.md
└── Attachments/                  # Diagrams, images, and visual artifacts
```

---

## 4. Operational Workflows for Agents

### 4.1 Ingestion Workflow (Session Start / Phase 1)
At the start of every autonomous feature lifecycle or debugging session:
1. **Check for Pending Directives**:
   Inspect `$VAULT/User-Input/pending/` using `agentkit vault ingest` or by listing files:
   ```bash
   agentkit vault ingest
   ```
2. **Synthesize Directives**:
   - Read all `.md` files located in `User-Input/pending/`.
   - Incorporate user requests, bug reports, and priority adjustments into the Phase 1 `plan.md`.
3. **Archive Processed Directives**:
   Move each processed file to `User-Input/processed/YYYY-MM-DD-<original-filename>` to prevent duplicate processing:
   ```bash
   mv "$VAULT/User-Input/pending/directive.md" "$VAULT/User-Input/processed/$(date +%Y-%m-%d)-directive.md"
   ```
4. **Read Target Project Note**:
   Read `$VAULT/Projects/<project-name>.md` to load recent architecture decisions, open tasks, and subsystem invariants.

### 4.2 Handoff & Persistence Workflow (Session Wrap-Up / Phase 4 & Quota Wrap-Up)
When completing a feature lifecycle, preparing a Pull Request, or executing a Quota Wrap-Up:
1. **Author Session Handoff**:
   Write a markdown note to `$VAULT/Sessions/handoff-YYYY-MM-DD-<topic>.md` (or run `agentkit vault handoff --topic <topic>`).
   Ensure it includes frontmatter, completed items, in-flight work, test status, and resumption instructions.
2. **Update Project Knowledge Base**:
   Update `$VAULT/Projects/<project-name>.md`:
   - Check off completed tasks (`- [x] Task name`).
   - Add newly discovered tasks or backlog items (`- [ ] New item`).
   - Update the "Current State" section with recent branch names, commit hashes, and test suite health.
3. **Update Architecture & ADR Notes**:
   If cross-repo contracts, schemas, or architectural invariants changed, create or update the relevant note in `$VAULT/Architecture/` and add appropriate wikilinks.

---

## 5. Obsidian Markdown Formatting Standards

To ensure optimal rendering in the Obsidian desktop application and graph view, agents must adhere to the following formatting standards:

### 5.1 YAML Frontmatter Properties
Every note created in the vault must begin with standard YAML frontmatter:

```markdown
---
type: session-handoff # project | architecture | adr | user-input | session-handoff
date: 2026-10-24
project: agentkit
status: complete # active | complete | pending | blocked
tags:
  - agentkit
  - orchestration
  - architecture
---
```

### 5.2 Obsidian Wikilinks
Connect notes across the vault using bidirectional wikilinks:
- Target an entire note: `[[agentkit]]` or `[[Projects/agentkit]]`
- Target a specific section heading: `[[agentkit#Current Architecture]]`
- Provide custom display text: `[[Projects/agentkit|AgentKit Core Documentation]]`
- Link to visual JSON Canvas files: `[[Architecture/Workstation-Stack.canvas|Workstation Architecture Canvas]]`

### 5.3 Obsidian Callouts
Use standard Obsidian callout blocks to highlight key takeaways, alerts, and pending action items:

```markdown
> [!note] Architectural Invariant
> Agentkit commands must execute idempotently and support non-destructive `--dry-run` operations.

> [!important] Quota Warning
> Gemini quota group reached 12% headroom. Rerouted subsequent worker subagents to Claude Sonnet.

> [!todo] Pending Manual Steps
> Run `agentkit apply` on secondary laptop once PR #42 is squashed and merged.

> [!check] Verification Passed
> Unit test suite: 28 tests passing (0 failures, 0 errors).
```

### 5.4 Checklists & Task State
Represent task progress using GitHub-compatible markdown checklists:
- `- [ ]` Incomplete or pending task
- `- [x]` Completed task
- `- [/]` In-progress or partially completed task (recognized by Obsidian plugins)

---

## 6. Example Handoff Note Template

```markdown
---
type: session-handoff
date: 2026-10-24
project: agentkit
topic: obsidian-vault-skill
status: complete
tags:
  - session
  - handoff
  - obsidian
---

# Session Handoff: Obsidian Perpetual Knowledge Vault Integration

## 1. Executive Summary
Integrated the Obsidian Perpetual Knowledge Vault specification into `agentkit`, establishing a durable, human-readable offline collaboration memory layer replacing opaque memory servers.

## 2. Completed Deliverables
- [x] Created `project/skills/obsidian-vault/SKILL.md` and symlinked into harness.
- [x] Created `project/skills/json-canvas/SKILL.md` for visual diagrams.
- [x] Updated `auto-lifecycle` and `quota-wrapup` runbooks to ingest pending input and output handoffs.
- [x] Documented architecture in `[[Architecture/adrs/0010-obsidian-perpetual-knowledge-vault|ADR 0010]]`.

## 3. Test & Verification Status
> [!check] Automated Test Verification
> Unit tests: `python3 -m unittest discover tests -v` passed with 100% compliance.

## 4. Next Steps & Offline Backlog
- [ ] Review `[[Workstation-Stack.canvas]]` diagram in Obsidian.
- [ ] Add custom prompts into `User-Input/pending/` for tomorrow's sprint.
