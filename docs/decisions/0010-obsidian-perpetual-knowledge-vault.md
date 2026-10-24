# ADR 0010: Obsidian Perpetual Knowledge Vault for Long-Term Agent Memory and Offline Collaboration

## Context

Prior to this decision, persistent cross-session knowledge and architectural contracts across Cooper Wallace's 4-layer workstation stack (`riced-kubuntu-os`, `workstation-setup`, `agentkit`, `repokit`) relied on the `@modelcontextprotocol/server-memory` MCP server (as established in ADR 0006). While this approach enabled cross-thread entity-relation persistence, operational practice across developer workstations identified three critical limitations:

1. **Opaque Storage & Lack of Human Visibility**:
   - The MCP memory server serializes entity relations and observations into an opaque JSON file (`~/.local/share/modelcontextprotocol/memory/knowledge-graph.json`).
   - The developer cannot naturally browse, audit, or search this memory store using standard desktop productivity tools without issuing synthetic MCP queries through an active agent conversation.

2. **Absence of Offline Human-in-the-Loop Collaboration**:
   - Between agent conversations, during thinking breaks, or overnight, the developer reflects on design decisions, sketches ideas, and refines task backlogs.
   - Under the prior architecture, there was no structured mechanism to communicate offline directives to subsequent agent runs. The developer was forced to either remember points for the next prompt or disrupt their flow by starting a live conversation.

3. **Lack of Visual Spatial Architecture Mapping**:
   - Text-based contracts and Markdown summaries struggle to capture multi-layered architectural boundaries, runtime toolchain dependencies, and multi-model agent DAGs at a glance.
   - Developers need spatial node-and-edge diagrams that render natively alongside living documentation.

To resolve these challenges, we adopt the philosophy of **"Files Over Apps"** and the **Obsidian Skills** open-source specification initiated by Steph Ango (kepano, CEO of Obsidian). By establishing a local, standard Obsidian Markdown vault as the primary agent memory layer, memory becomes completely transparent, durable, and editable by human and agent alike.

---

## Decision

1. **Adopt Obsidian Perpetual Knowledge Vault as Primary Long-Term Memory**:
   - Transition primary cross-session persistence from opaque JSON memory graphs to a local **Obsidian Perpetual Knowledge Vault** located at `$HOME/Documents/Vault` (or `$HOME/Vault`, configurable via `$AGENTKIT_VAULT_PATH`).
   - Adhere to the Obsidian Skills open standard: plaintext Markdown files, YAML frontmatter properties, bidirectional wikilinks (`[[Note Name]]`), callouts (`> [!note]`), and task checklists (`- [ ]`, `- [x]`).

2. **Standardized Vault Directory Architecture**:
   The vault enforces a standard layout:
   - `User-Input/`:
     - `pending/`: Offline user prompts, feedback, bug notes, and architectural adjustments created during conversation breaks.
     - `processed/`: Ingested user notes archived with timestamp prefixes (`YYYY-MM-DD-<name>`).
   - `Projects/`: Living project documentation (`agentkit.md`, `repokit.md`, `riced-kubuntu-os.md`, `workstation-setup.md`) tracking current architecture, branch states, and backlog items.
   - `Architecture/`: Cross-repo contracts, schemas, invariant definitions, and ADR summaries with bidirectional links.
   - `Sessions/`: Immutable records of agent session handoffs (`handoff-YYYY-MM-DD-<topic>.md`) and quota wrap-ups.
   - `Attachments/`: Static visual assets, charts, and diagrams.

3. **Visual Architecture Mapping via Open JSON Canvas (`.canvas`)**:
   - Adopt the official **JSON Canvas open specification** (`jsoncanvas.org`) for visual system diagrams (e.g. `Architecture/Workstation-Stack.canvas`).
   - Provide the `json-canvas` skill (`project/skills/json-canvas/SKILL.md`) to guide agents in programmatically constructing spatial node-and-edge maps with text, file, group, and color coordinates.

4. **Integration with Autonomous Lifecycle & Quota Wrap-Up**:
   - **Phase 1 Ingestion (`auto-lifecycle`)**: The Root Orchestrator inspects `$VAULT/User-Input/pending/` at session start (via `agentkit vault ingest`), incorporating offline directives into `plan.md` and archiving processed notes to `User-Input/processed/`.
   - **Phase 4 Handoff (`auto-lifecycle`)**: The Root Orchestrator authors a session handoff note to `$VAULT/Sessions/handoff-YYYY-MM-DD-<topic>.md` and updates `$VAULT/Projects/<project>.md` with finished tasks and in-flight work.
   - **Quota Wrap-Up (`quota-wrapup`)**: When quota exhaustion triggers the circuit breaker, the handoff note is written directly to `$VAULT/Sessions/handoff-YYYY-MM-DD-quota-wrapup.md` for offline human inspection in Obsidian.

5. **Tooling & CLI Automation (`agentkit vault`)**:
   - Equip the `agentkit` CLI with dedicated vault subcommands:
     - `agentkit vault init`: Scaffolds the standard vault directory layout and baseline project notes.
     - `agentkit vault ingest`: Discovers and compiles pending notes from `User-Input/pending/` into structured prompt context.
     - `agentkit vault handoff`: Generates timestamped session handoff notes in `Sessions/`.
     - `agentkit vault status`: Reports vault location, pending directives count, and recent session notes.

---

## Consequences

### Positive
- **Human-in-the-Loop Transparency**: All agent memory resides in human-readable Markdown files that can be viewed and edited in Obsidian on desktop and mobile.
- **Asynchronous Offline Collaboration**: The developer can queue tasks and feedback in `User-Input/pending/` between sessions without launching an agent harness.
- **Durability & Zero Vendor Lock-in**: Plaintext Markdown files and JSON Canvas diagrams remain accessible and versionable indefinitely without depending on proprietary cloud databases.
- **Visual Spatial Reasoning**: Infinite JSON Canvas maps allow instant visual inspection of the 4-layer workstation hierarchy and multi-agent DAGs.

### Neutral / Negative
- **Path Configuration**: Workstations must specify or adhere to standard vault paths (`$HOME/Documents/Vault` or `$HOME/Vault`).
- **File Discipline**: Agents and developers must follow frontmatter and wikilink conventions to maintain graph integrity.
