# Permissions Security Matrix & Rationale

AI developer harnesses enforce a permission boundary on tool executions. When the agent attempts to run terminal commands, it prompts the user unless the exact command or binary is explicitly listed in configuration settings under `permissions.allow` (e.g., `~/.gemini/antigravity-cli/settings.json` for Antigravity).

This document outlines the security classification, categories, and safety rationale for commands pre-approved by **`agentkit`**.

---

## 1. Safety Tiers

| Tier | Classification | Risk Level | Policy | Examples |
| :--- | :--- | :--- | :--- | :--- |
| **Tier 1** | Read-Only Inspection | Negligible | Pre-Approved | `ls`, `cat`, `grep`, `pwd`, `diff`, `find`, `which` |
| **Tier 2** | Non-Destructive Workspace Ops | Low | Pre-Approved | `mkdir`, `touch`, `cp`, `mv`, `chmod`, `ln` |
| **Tier 3** | Version Control Hygiene | Low | Pre-Approved | `git status`, `git diff`, `git log`, `git add`, `git commit` |
| **Tier 4** | Build & Test Harnesses | Low–Medium | Pre-Approved | `pytest`, `python3 -m unittest`, `npm test`, `cargo test` |
| **Tier 5** | Workstation Introspection | Negligible | Pre-Approved | `fastfetch`, `uname`, `hostname`, `whoami`, `date` |
| **Tier 6** | Destructive / Administrative | High / Critical | **Strictly Withheld (Requires User Approval)** | `rm -rf`, `sudo`, `dd`, `mkfs`, `fdisk` |

---

## 2. Pre-Approved Categories

### A. Core Inspection (`core_inspection`)
- **Commands**: `which`, `cat`, `head`, `tail`, `grep`, `ls`, `pwd`, `echo`, `diff`, `wc`, `find`, `file`, `tree`, `fastfetch`
- **Rationale**: These tools read files or display system metadata without modifying repository code or system state. Pre-approving them prevents constant approval fatigue during routine codebase exploration.

### B. Safe File Operations (`safe_file_operations`)
- **Commands**: `mkdir`, `touch`, `cp`, `mv`, `chmod`, `ln`
- **Rationale**: Allows the agent to scaffold project folders, prepare executable scripts (`chmod +x`), and stage files.
- **Safety Gate**: Recursive deletion (`rm -rf`) is **deliberately excluded**. Any attempt to delete files or directories must be explicitly confirmed by the user.

### C. Git Hygiene (`git_hygiene`)
- **Commands**: `git status`, `git diff`, `git log`, `git show`, `git branch`, `git checkout`, `git switch`, `git fetch`, `git pull`, `git add`, `git commit`, `git remote`, `git config`, `git init`, `git stash`, `git rebase`, `git worktree`, `git rev-parse`, `git ls-files`
- **Rationale**: Enables frictionless git hygiene, branch creation, atomic commits, repository checks, and subagent worktree isolation.
- **Safety Gate**: Destructive git commands such as `git push --force` and `git reset --hard` are omitted from pre-approval.

### D. GitHub Governance (`github_governance`)
- **Commands**: `gh`, `gh pr`, `gh issue`, `gh repo`, `gh auth`, `repokit`, `gh-standardise`, `gh-standardize`
- **Rationale**: Allows seamless coordination with Layer 3 repository governance (`repokit`), GitHub PR workflows, checking PR status (`gh pr checks`), and automating merge governance.

### E. Developer Test Runners & Runtimes (`test_runners_and_runtimes`)
- **Commands**: `python3 -m unittest discover tests`, `python3 -m unittest discover`, `pytest`, `python3`, `python`, `node`, `npm test`, `npm run`, `pnpm`, `yarn`, `mvn test`, `cargo test`, `go test`
- **Rationale**: The agent must rigorously test code before claiming completion. Pre-approving standard test runners ensures that verification steps execute smoothly without interactive stalls.

### F. Workstation Provisioning & Agent Tooling (`local_workflows`)
- **Commands**: `./bootstrap.sh`, `./install.sh`, `./plasma/restore-kde.sh`, `konsole`, `agy`, `agentkit`, `workstation-setup`, `ws-setup`
- **Rationale**: Enables orchestrating system setup, dotfile restoration, Layer 1 workstation tooling (`workstation-setup`), and Layer 2 agent standardisation tooling (`agentkit`) across machines.

### G. Context & Memory Optimization (`context_and_memory_optimization`)
- **Commands**: `headroom`, `repomix`
- **Rationale**: Enables automatic token suppression, output wrapping, and AST-compressed codebase indexing without approval friction.

---

## 3. Provisioned MCP & Tool Permissions (`mcp_tools`, `tools`)

In addition to terminal command execution permissions (`command(...)`), autonomous agents utilize Model Context Protocol (MCP) servers and harness-level meta-tools to interact with structured external services and persistent state.

### Rationale & Boundary Design
- **Eliminating Approval Fatigue**: Standard MCP tools (`memory`, `repomix`, `github`, `fetch`, `sqlite`) and invocation tools (`call_mcp_tool`) are invoked repeatedly during multi-agent workflows. Pre-approving trusted servers and methods removes repetitive interactive prompts, allowing seamless autonomous orchestration across conversational threads and subagents.
- **Safety Boundary**: Only explicitly registered, vetted MCP servers defined in `config/mcp.json` and `config/permissions.json` are pre-approved. Any unverified third-party MCP server or arbitrary external tool invocation remains subject to interactive user confirmation.

### A. Persistent Knowledge Graph (`memory`)
- **Permissions**: `mcp(memory)`, `mcp(memory/*)`, `mcp(memory/create_entities)`, `mcp(memory/create_relations)`, `mcp(memory/add_observations)`, `mcp(memory/delete_entities)`, `mcp(memory/delete_observations)`, `mcp(memory/delete_relations)`, `mcp(memory/read_graph)`, `mcp(memory/search_nodes)`, `mcp(memory/open_nodes)`
- **Rationale**: Enables agents to store and retrieve cross-session architecture decisions, cross-repo invariants, and developer preferences in the perpetual knowledge graph without stalling on approval prompts.

### B. AST Repository Packing (`repomix`)
- **Permissions**: `mcp(repomix)`, `mcp(repomix/*)`, `mcp(repomix/pack_codebase)`, `mcp(repomix/read_repomix_output)`, `mcp(repomix/grep_repomix_output)`, `mcp(repomix/pack_remote_repository)`, `mcp(repomix/generate_skill)`, `mcp(repomix/attach_packed_output)`
- **Rationale**: Enables subagents to compress entire codebases into structured AST-annotated context, reducing cross-repository token ingestion by ~70% without interactive prompts.

### C. GitHub Repository Governance (`github`)
- **Permissions**: `mcp(github)`, `mcp(github/*)`, `mcp(github/create_or_update_file)`, `mcp(github/list_commits)`, `mcp(github/search_code)`, `mcp(github/get_pull_request)`, `mcp(github/list_pull_requests)`, `mcp(github/create_issue)`, `mcp(github/update_issue)`, `mcp(github/list_issues)`, `mcp(github/push_files)`, `mcp(github/merge_pull_request)`, `mcp(github/get_file_contents)`, `mcp(github/create_pull_request)`, `mcp(github/create_branch)`, `mcp(github/fork_repository)`, `mcp(github/search_issues)`, `mcp(github/get_issue)`, `mcp(github/search_repositories)`, `mcp(github/create_repository)`, `mcp(github/search_users)`, `mcp(github/add_issue_comment)`, `mcp(github/create_pull_request_review)`, `mcp(github/get_pull_request_files)`, `mcp(github/get_pull_request_status)`, `mcp(github/update_pull_request_branch)`, `mcp(github/get_pull_request_comments)`, `mcp(github/get_pull_request_reviews)`
- **Rationale**: Allows agents to inspect issues, review pull request diffs, query GitHub APIs, and participate in Layer 3 governance natively via MCP.

### D. Web Fetching (`fetch`)
- **Permissions**: `mcp(fetch)`, `mcp(fetch/*)`, `mcp(fetch/fetch)`
- **Rationale**: Provides read-only retrieval and markdown conversion of online documentation, API references, and external specs.

### E. Database Queries (`sqlite`)
- **Permissions**: `mcp(sqlite)`, `mcp(sqlite/*)`, `mcp(sqlite/read_query)`, `mcp(sqlite/write_query)`, `mcp(sqlite/create_table)`, `mcp(sqlite/list_tables)`, `mcp(sqlite/describe_table)`
- **Rationale**: Facilitates local development database schema inspection and analytical queries against developer SQLite databases configured in `config/mcp.json`.

### F. Harness Invocation & Resource Access (`provisioned_tools`)
- **Permissions**: `call_mcp_tool`, `call_mcp_tool(*)`, `list_resources`, `read_resource`
- **Rationale**: Pre-approves the harness meta-tools that dispatch calls to configured MCP servers and read external server resources, preventing approval prompts at the harness invocation layer.

---

## 4. Auditing & Syncing New Commands

If an interactive session prompts you to allow a safe command, selecting **"Always allow"** appends it to your settings file.

To capture newly approved commands into this repository:
```bash
agentkit snapshot
```
Review the detected commands and add them to [config/permissions.json](../config/permissions.json) to distribute them across all your workstations.
