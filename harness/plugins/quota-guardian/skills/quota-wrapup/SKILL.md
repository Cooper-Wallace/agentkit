---
name: quota-wrapup
description: Gracefully conclude agent sessions upon token quota exhaustion, preserve working tree state, and generate handoff artifacts.
---

# Quota Wrap-up Runbook

This skill is invoked when subagent delegation is halted or model quotas reach critical levels (`<= 5%`). It ensures that no work is lost, the codebase remains in a verifiable state, and handoff context is cleanly persisted for resumption.

---

## Phase 1: Halt Subagent Delegation

1. **Stop Delegations**: Cease calling `invoke_subagent`. Do not retry failed delegations or attempt workarounds with alternative model names.
2. **Collect Active Results**: Allow currently running subagents to return their results or messages before proceeding.

---

## Phase 2: Run Verification & Automated Tests

Execute the repository test suite to establish the baseline health of current changes:

```bash
python3 -m unittest discover tests -v
```

- If tests pass, record passing status in the handoff document.
- If tests fail, document the failing tests and root cause in `handoff.md` so the resuming agent has immediate visibility.

---

## Phase 3: Git Working Tree Hygiene, One-Commit Safety & Worktree State

1. **One Branch Per Change**:
   - Ensure the working tree is located on the dedicated short-lived feature branch (`feat/<name>`, `fix/<name>`, etc.).
   - Never commit wrap-up progress directly to `main` or `master`.
2. **Worktree Concurrency Safety**:
   - If running within an isolated git worktree (`Workspace: "share"`), verify that active index locks (`.git/index.lock`) are clear and uncommitted edits are confined to this worktree.
   - Do not clobber sibling worktrees used by concurrent subagent threads.
3. **Inspect Working Tree**:
   ```bash
   git status
   git diff --stat
   ```
4. **Produce Single Atomic Commit (One Commit per Subagent / Wrap-up)**:
   - If a subagent completed its deliverable, ensure it produced a single atomic Conventional Commit.
   - If wrap-up interrupts execution mid-flight with uncommitted changes, stage and consolidate all remaining progress into exactly ONE atomic WIP commit on the active feature branch:
   ```bash
   git add -A
   git commit -m "chore(wip): save progress before quota wrap-up"
   ```
   > [!IMPORTANT]
   > Do NOT push to `main` or `master`. Keep the commit on the short-lived feature branch.

---

## Phase 4: Generate Handoff Artifact & Vault Note

Create a structured handoff note directly in the Obsidian AgentVault at `$HOME/Documents/AgentVault/Sessions/handoff-YYYY-MM-DD-quota-wrapup.md` (and mirror to the conversation artifact directory as `handoff.md`) detailing:

1. **Frontmatter Properties**: Set `type: session-handoff`, `date: YYYY-MM-DD`, `project: <project>`, `status: pending`, `tags: [quota-wrapup, <project>]`.
2. **Quota Telemetry**: Current remaining quota percentage per group and earliest reset timestamp (`agentkit quota status`).
3. **Completed Tasks**: Tasks finished during this session with references to modified files and wikilinks.
4. **In-Flight Work**: Unfinished or partially tested modules.
5. **Remaining Backlog**: Items next in sequence for execution.
6. **Resume Command**: Recommended command or model profile to continue when quotas reset or under fallback models.

---

## Phase 5: Update Project Knowledge Base in Vault

Update `$HOME/Documents/AgentVault/Projects/<project>.md` to maintain long-term alignment across conversations:
- Update the "Current State" section with the active feature branch name, WIP commit hash, and quota reset timestamp.
- Record passing/failing test counts.
- Add unfinished DAG items to the project checklist (`- [ ]`) for offline review in Obsidian.

---

## Phase 6: Advise the User

Notify the user in the conversation summary:
- The exact reset time (from `agentkit quota status`).
- Recommended profile or model switch:
  - If Gemini/Argon is exhausted, suggest switching to Claude profile: `/model opus` or `agentkit apply --model-profile opus-flash`.
- Path to the generated Obsidian AgentVault handoff note (`$HOME/Documents/AgentVault/Sessions/handoff-YYYY-MM-DD-quota-wrapup.md`) and conversational `handoff.md`.
