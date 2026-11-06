# Architecture Decision Records (ADRs)

This directory documents the architectural decision records for `agentkit`, capturing structural contracts, engineering invariants, runtime policies, and technical trade-offs across Cooper Wallace's developer workstation ecosystem.

---

## Index of Architectural Decisions

| ADR | Title | Status | Date | Primary Scope |
| :--- | :--- | :--- | :--- | :--- |
| [ADR 0001](0001-permission-merging-strategy.md) | Non-Destructive Set-Union Merging Strategy for Permissions | Accepted | 2026-09-08 | Harness Permissions & Settings Merging |
| [ADR 0002](0002-dual-naming-convention.md) | Dual Naming Convention (`antigravity-standardise` and `agy-standardise`) | Superseded (by 0008) | 2026-09-08 | CLI Naming & Shorthand Conventions |
| [ADR 0003](0003-global-vs-workspace-rules.md) | Separation of Global Directives and Workspace Rules | Accepted | 2026-09-08 | Agent Instruction Hierarchy & Scope |
| [ADR 0004](0004-plugin-and-hook-templating.md) | Templating Strategy for Lifecycle Hooks and Plugins | Accepted | 2026-09-08 | Lifecycle Hook Scaffolding |
| [ADR 0005](0005-declarative-mcp-management-and-env-advisories.md) | Declarative MCP Management and Environment Variable Advisories | Accepted | 2026-09-08 | Model Context Protocol Tooling & Secrets |
| [ADR 0006](0006-token-compression-and-cross-repo-memory.md) | PreToolUse Token Compression (Headroom CCR) and Cross-Repo Memory Architecture | Accepted | 2026-10-26 | Headroom CCR & Context Optimization |
| [ADR 0007](0007-agentkit-harness-agnostic-standardisation.md) | Migration to Agentkit & Harness-Agnostic Standardization | Accepted | 2026-10-26 | Harness Abstraction & Multi-Tool Standard |
| [ADR 0008](0008-harness-project-bifurcation-and-aptitude-consolidation.md) | Harness/Project Bifurcation, Exclusive Agentkit Naming, and AugursAptitude Consolidation | Accepted | 2026-10-26 | Repository Bifurcation (`harness/` vs `project/`) |
| [ADR 0009](0009-quota-aware-model-routing.md) | Quota-Aware Model Routing, Subagent Complexity Tiers, and Graceful Wrap-Up Protocol | Accepted | 2026-10-26 | Cognitive Model Tiering & Quota Governance |
| [ADR 0010](0010-obsidian-perpetual-knowledge-vault.md) | Obsidian AgentVault for Long-Term Agent Memory and Offline Collaboration | Accepted | 2026-10-26 | AgentVault Storage, JSON Canvas & Offline Memory |
| [ADR 0011](0011-git-tree-management-and-worktree-isolation.md) | Git Tree Management: Single-Branch Changes, Atomic Subagent Commits, and Worktree Isolation | Accepted | 2026-10-26 | Git Invariants, Worktree Concurrency & Branching |
| [ADR 0012](0012-wake-lock-and-visual-state.md) | Cross-Platform Agent Wake Lock and Visual State | Accepted | 2026-10-26 | Sleep Prevention Engine, D-Bus Inhibit & Tray |
| [ADR 0013](0013-dock-based-caffeine-hud-and-visual-design.md) | Dock-Based Caffeine HUD and Visual Design | Accepted | 2026-11-06 | Floating HUD Pill, Multi-Agent Arbiter & Circuit Breakers |
