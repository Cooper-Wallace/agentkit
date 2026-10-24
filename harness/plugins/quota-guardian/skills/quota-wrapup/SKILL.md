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

## Phase 3: Git Working Tree Hygiene & WIP Commit

1. **Inspect Working Tree**:
   ```bash
   git status
   git diff --stat
   ```
2. **Commit Uncommitted Work**:
   Stage and commit all completed and in-flight changes to the active feature branch using a Conventional Commit message:
   ```bash
   git add -A
   git commit -m "chore(wip): save progress before quota wrap-up"
   ```
   > [!IMPORTANT]
   > Do NOT push to `main` or `master`. Keep the commit on the short-lived feature branch.

---

## Phase 4: Generate Handoff Artifact (`handoff.md`)

Create a `handoff.md` artifact in the active conversation directory or project root detailing:

1. **Quota Status**: Current remaining quota percentage per group and earliest reset timestamp (`agentkit quota status`).
2. **Completed Tasks**: Tasks finished during this session with references to modified files.
3. **In-Flight Work**: Unfinished or partially tested modules.
4. **Remaining Backlog**: Items next in sequence for execution.
5. **Resume Command**: Recommended command or model profile to continue when quotas reset or under fallback models.

---

## Phase 5: Knowledge Graph Persistence (`memory` MCP)

Store session state in the persistent knowledge graph:
- Update entities: `HandoffState`, `ActiveBranch`, `QuotaResetSchedule`.
- Add observation: Current branch name, commit hash, failing/passing test count, and target model profile.

---

## Phase 6: Advise the User

Notify the user in the conversation summary:
- The exact reset time (from `agentkit quota status`).
- Recommended profile or model switch:
  - If Gemini/Argon is exhausted, suggest switching to Claude profile: `/model opus` or `agentkit apply --model-profile opus-flash`.
- Link to the generated `handoff.md`.
