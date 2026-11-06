package vault

import (
	"fmt"
	"strings"
	"time"
)

func getDashboardTemplate() string {
	todayISO := time.Now().Format("2006-01-02")
	return fmt.Sprintf(`---
tags:
  - dashboard
  - hub
type: dashboard
last_updated: %s
updated_by: agentkit
---

# 🌌 Workstation AgentVault

> [!info] Autonomous Workstation Knowledge Graph
> Central intelligence repository linking the 4-layer developer workstation stack, active applications, architecture contracts, and agent session history.

## 🚀 Active Projects

`+"```dataview"+`
TABLE type, status, last_updated
FROM "Projects"
SORT file.name ASC
`+"```"+`

- [[Projects/agentkit|agentkit]] — Layer 2: Agent Harness Governance & Directive Standards
- [[Projects/repokit|repokit]] — Layer 3: GitHub Governance, Squash Merges & PR Automation
- [[Projects/riced-kubuntu-os|riced-kubuntu-os]] — Layer 0: Kubuntu OS Packages & KDE Plasma System Config
- [[Projects/workstation-setup|workstation-setup]] — Layer 1: Developer Toolchains & Environment Bridging
- [[Projects/augurs-bazaar|augurs-bazaar]] — Cross-exchange prediction market trading platform
- [[Projects/augurs-saber|augurs-saber]] — Algorithmic execution & market making engine

## 🏛️ Architecture & Governance

- [[Architecture/00-Overview|Architecture Overview]] — 4-layer workstation stack & component interactions
- [[Architecture/contracts|System Contracts]] — Model tiering, zero-prompt discipline, quota rules
- [[Architecture/decisions/README|Architecture Decision Records (ADRs)]] — Record of architectural choices

## 📥 Directives & User Inputs

> [!todo] User Input Queue
> Place notes, tasks, and directives into `+"`User-Input/pending/`"+` for ingestion into active agent contexts.
- [[User-Input/README|User Input Guide]]

## 📜 Recent Sessions

`+"```dataview"+`
TABLE date, summary
FROM "Sessions"
SORT file.ctime DESC
LIMIT 5
`+"```"+`
- [[Sessions/README|Sessions Archive]]
`, todayISO)
}

func getUserInputReadmeTemplate() string {
	todayISO := time.Now().Format("2006-01-02")
	return fmt.Sprintf(`---
tags:
  - user-input
  - directives
type: reference
last_updated: %s
updated_by: agentkit
---

# User Input & Offline Directives Inbox

> [!info] Purpose
> This directory serves as an offline inbox for user guidance, prompt drafts, task requests, and backlog directives.

## Workflow

1. **Write**: Place your markdown file in `+"`User-Input/pending/<your-note>.md`"+`.
2. **Ingest**: The agent runs `+"`agentkit vault ingest`"+` (or `+"`agentkit vault ingest --project <name>`"+`).
3. **Archive**: Processed notes are automatically moved to `+"`User-Input/processed/YYYY-MM-DD-<your-note>.md`"+` with an audit stamp:
   > [!check] Ingested by agentkit on <iso_timestamp>

> [!note] Project-Specific Inputs
> You can also write offline notes directly inside project notes like [[Projects/agentkit|agentkit]] under the `+"`## Offline User Input & Notes`"+` section.
`, todayISO)
}

func getArchitectureOverviewTemplate() string {
	todayISO := time.Now().Format("2006-01-02")
	return fmt.Sprintf(`---
tags:
  - architecture
  - overview
type: architecture
last_updated: %s
updated_by: agentkit
---

# Workstation Stack Architecture Overview

> [!info] 4-Layer Workstation Stack
> The developer workstation environment is strictly partitioned into 4 distinct layers:
> - **Layer 0**: [[Projects/riced-kubuntu-os|riced-kubuntu-os]] (OS packages, KDE Plasma dotfiles)
> - **Layer 1**: [[Projects/workstation-setup|workstation-setup]] (Language toolchains, shell profiles)
> - **Layer 2**: [[Projects/agentkit|agentkit]] (Agent harness standards, permissions, quota governance)
> - **Layer 3**: [[Projects/repokit|repokit]] (GitHub governance, squash merges, semantic checks)

## System Boundaries
See [[Architecture/contracts|System Contracts]] for operational guarantees and model tiering specifications.
`, todayISO)
}

func getArchitectureContractsTemplate() string {
	todayISO := time.Now().Format("2006-01-02")
	return fmt.Sprintf(`---
tags:
  - architecture
  - contracts
type: architecture
last_updated: %s
updated_by: agentkit
---

# System Architecture Contracts & Standards

> [!note] Architectural Invariants
> Directives governing cognitive tiering, zero-prompt command discipline, and quota wrap-up.

## 1. Cognitive Architecture & Model Tiering
- **Root Orchestrator**: Gemini 4 Argon (`+"`pro`"+`) / Flash High (`+"`flash`"+`) / Claude Opus 5.5 (`+"`opus`"+`).
- **Subagent Workers**: Partitioned by complexity (`+"`low`"+`: `+"`flash_lite`"+`, `+"`med`"+`: `+"`flash`"+`, `+"`high`"+`: `+"`flash`"+`/`+"`pro`"+`).
- Zero project editing by Root Orchestrator: All code implementation delegated to worker subagents.

## 2. Quota Awareness & Graceful Wrap-Up
- Automated fallback routing when provider quota falls below 15%%.
- Circuit breaker and graceful wrap-up when all provider quotas reach critical levels (<5%%).

## 3. Tooling & Ecosystem Links
- [[Projects/agentkit|agentkit]]
- [[Projects/repokit|repokit]]
- [[Projects/riced-kubuntu-os|riced-kubuntu-os]]
- [[Projects/workstation-setup|workstation-setup]]
`, todayISO)
}

func getArchitectureDecisionsReadmeTemplate() string {
	todayISO := time.Now().Format("2006-01-02")
	return fmt.Sprintf(`---
tags:
  - architecture
  - adr
type: reference
last_updated: %s
updated_by: agentkit
---

# Architecture Decision Records (ADRs)

> [!info] ADR Repository
> Stores records of significant architectural choices and structural contracts.
`, todayISO)
}

func getSessionsReadmeTemplate() string {
	todayISO := time.Now().Format("2006-01-02")
	return fmt.Sprintf(`---
tags:
  - sessions
  - history
type: reference
last_updated: %s
updated_by: agentkit
---

# Agent Session Archive

> [!info] Session Logs
> Completed agent pair-programming summaries, handoffs, and audit trails are logged here.
`, todayISO)
}

type projectMetaInfo struct {
	Tags     []string
	Title    string
	Desc     string
	LongDesc string
	Layer    string
	Siblings string
	Tasks    []string
}

var knownProjects = map[string]projectMetaInfo{
	"agentkit": {
		Tags:     []string{"project", "layer-2", "agentkit"},
		Title:    "agentkit",
		Desc:     "Layer 2: Agent Harness Governance & Directive Standards",
		LongDesc: "Audits, enforces, and standardizes harness-agnostic AI agent permissions, directives, cognitive model profiles, and quota governance across workstations.",
		Layer:    "Layer 2 (Agent Harness Governance)",
		Siblings: "[[Projects/riced-kubuntu-os|riced-kubuntu-os]] (Layer 0), [[Projects/workstation-setup|workstation-setup]] (Layer 1), [[Projects/repokit|repokit]] (Layer 3)",
		Tasks: []string{
			"Maintain pre-approved permissions catalog",
			"Enforce quota guardian thresholds and fallback routing",
			"Expand Obsidian vault synchronization and ingest workflows",
		},
	},
	"repokit": {
		Tags:     []string{"project", "layer-3", "repokit"},
		Title:    "repokit",
		Desc:     "Layer 3: GitHub Governance, Squash Merges & PR Automation",
		LongDesc: "Automates GitHub repository governance, branch protection rulesets, squash merge enforcement, and semantic PR checks.",
		Layer:    "Layer 3 (GitHub Repository Governance)",
		Siblings: "[[Projects/agentkit|agentkit]] (Layer 2)",
		Tasks: []string{
			"Automate branch protection rulesets across repositories",
			"Enforce Conventional Commits 1.0.0 squash merge titles",
		},
	},
	"riced-kubuntu-os": {
		Tags:     []string{"project", "layer-0", "riced-kubuntu-os"},
		Title:    "riced-kubuntu-os",
		Desc:     "Layer 0: Kubuntu OS Packages & KDE Plasma System Config",
		LongDesc: "Manages Kubuntu base packages, KDE Plasma 6 configurations, and system-level dotfiles.",
		Layer:    "Layer 0 (Base Operating System)",
		Siblings: "[[Projects/workstation-setup|workstation-setup]] (Layer 1)",
		Tasks: []string{
			"Maintain declarative package manifests",
			"Sync KDE Plasma configuration snapshots",
		},
	},
	"workstation-setup": {
		Tags:     []string{"project", "layer-1", "workstation-setup"},
		Title:    "workstation-setup",
		Desc:     "Layer 1: Developer Toolchains & Environment Bridging",
		LongDesc: "Configures language runtimes (Node.js, Python, uv), shell profiles (~/.profile, ~/.bashrc), keyrings, and environment bridging.",
		Layer:    "Layer 1 (Developer Toolchains)",
		Siblings: "[[Projects/riced-kubuntu-os|riced-kubuntu-os]] (Layer 0), [[Projects/agentkit|agentkit]] (Layer 2)",
		Tasks: []string{
			"Manage developer SDK and CLI installations",
			"Provide idempotent bootstrap and setup scripts",
		},
	},
	"augurs-bazaar": {
		Tags:     []string{"project", "augurs-bazaar"},
		Title:    "augurs-bazaar",
		Desc:     "Cross-Exchange Prediction Market Trading Platform",
		LongDesc: "Decentralized and multi-exchange prediction market data ingestion, cross-venue pricing, and order routing platform.",
		Layer:    "Application Layer",
		Siblings: "[[Projects/augurs-saber|augurs-saber]]",
		Tasks: []string{
			"Real-time market feed ingestion",
			"Liquidity pool indexing and pricing models",
		},
	},
	"augurs-saber": {
		Tags:     []string{"project", "augurs-saber"},
		Title:    "augurs-saber",
		Desc:     "Algorithmic Execution & Market Making Engine",
		LongDesc: "High-throughput order execution, risk management, and market making engine paired with augurs-bazaar.",
		Layer:    "Application Layer",
		Siblings: "[[Projects/augurs-bazaar|augurs-bazaar]]",
		Tasks: []string{
			"Market making latency optimization",
			"Execution risk limits and circuit breakers",
		},
	},
}

// GetProjectTemplate returns a structured markdown template for a project note.
func GetProjectTemplate(projectName string) string {
	todayISO := time.Now().Format("2006-01-02")
	meta, ok := knownProjects[projectName]
	if !ok {
		meta = projectMetaInfo{
			Tags:     []string{"project", projectName},
			Title:    projectName,
			Desc:     fmt.Sprintf("Project %s", projectName),
			LongDesc: fmt.Sprintf("Workspace project note for %s.", projectName),
			Layer:    "Application",
			Siblings: "None",
			Tasks:    []string{"Initialize project tasks"},
		}
	}

	var taskLines []string
	for _, t := range meta.Tasks {
		taskLines = append(taskLines, fmt.Sprintf("- [ ] %s", t))
	}
	tasksMD := strings.Join(taskLines, "\n")

	var tagLines []string
	for _, t := range meta.Tags {
		tagLines = append(tagLines, fmt.Sprintf("  - %s", t))
	}
	tagsYAML := strings.Join(tagLines, "\n")

	return fmt.Sprintf(`---
tags:
%s
type: project
last_updated: %s
updated_by: agentkit
status: active
---

# %s

> [!info] %s
> %s

## Architecture & Contracts
- Layer: %s
- Sibling Systems: %s
- Specification: [[Architecture/contracts|System Contracts]]
- Decisions: [[Architecture/decisions/README|ADRs]]

## Active Tasks
%s

## Offline User Input & Notes
> [!todo] Pending Notes
> Add task requests or design considerations here for automatic ingestion.
`, tagsYAML, todayISO, meta.Title, meta.Desc, meta.LongDesc, meta.Layer, meta.Siblings, tasksMD)
}
