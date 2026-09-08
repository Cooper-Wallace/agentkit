# Permissions Security Matrix & Rationale

Google Antigravity CLI enforces a permission boundary on tool executions. When the agent attempts to run terminal commands, it prompts the user unless the exact command or binary is explicitly listed in `~/.gemini/antigravity-cli/settings.json` under `permissions.allow`.

This document outlines the security classification, categories, and safety rationale for commands pre-approved by `antigravity-standardise`.

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
- **Commands**: `git status`, `git diff`, `git log`, `git show`, `git branch`, `git checkout`, `git switch`, `git fetch`, `git pull`, `git add`, `git commit`, `git remote`, `git config`, `git init`, `git stash`, `git rebase`
- **Rationale**: Enables frictionless git hygiene, branch creation, atomic commits, and repository checks.
- **Safety Gate**: Destructive git commands such as `git push --force` and `git reset --hard` are omitted from pre-approval.

### D. GitHub Governance (`github_governance`)
- **Commands**: `gh`, `gh pr`, `gh issue`, `gh repo`, `gh auth`, `gh-standardise`, `gh-standardize`
- **Rationale**: Allows seamless coordination with GitHub PR workflows, checking PR status (`gh pr checks`), and automating merge governance.

### E. Developer Test Runners & Runtimes (`test_runners_and_runtimes`)
- **Commands**: `python3 -m unittest discover tests`, `python3 -m unittest discover`, `pytest`, `python3`, `python`, `node`, `npm test`, `npm run`, `pnpm`, `yarn`, `mvn test`, `cargo test`, `go test`
- **Rationale**: The agent must rigorously test code before claiming completion. Pre-approving standard test runners ensures that verification steps execute smoothly.

### F. Workstation Provisioning & Scripts (`local_workflows`)
- **Commands**: `./bootstrap.sh`, `./install.sh`, `./plasma/restore-kde.sh`, `konsole`, `agy`, `agy-standardise`, `antigravity-standardise`
- **Rationale**: Enables orchestrating system setup, dotfile restoration, and Antigravity compliance tooling across machines.

---

## 3. Auditing & Syncing New Commands

If an interactive session prompts you to allow a safe command, selecting **"Always allow"** appends it to `settings.json`.

To capture newly approved commands into this repository:
```bash
agy-standardise snapshot
```
Review the detected commands and add them to [config/permissions.json](../config/permissions.json) to distribute them across all your workstations.
