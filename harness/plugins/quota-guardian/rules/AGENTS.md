# Quota Guardian Agent Directives

These directives govern model tier routing, token quota preservation, and graceful wrap-up behavior for autonomous agent workflows.

---

## 1. Complexity Tiering & Subagent Model Routing

All subagent invocations MUST explicitly declare their task complexity in their prompt or tool parameters using the standard tag `[complexity: low|med|high]`:

- **Low Complexity (`flash_lite`)**:
  - **Use**: Mechanical find-replace, file moves, renames, formatting, markdown/doc typos, simple one-command status checks.
  - **Safety Guardrail**: Any task tagged `low` that touches test suites, security components, or non-trivial refactoring is automatically upgraded to `flash`.
- **Medium Complexity (`flash`)**:
  - **Use**: Standard feature code implementation, test writing, bug fixes, localized refactoring.
- **High Complexity (`flash` / `pro`)**:
  - **Use**: Cross-module architectural redesign, complex debugging, adversarial reviews, security audits.
  - Routes to `pro` (Argon tier) only when Argon quota is healthy; otherwise routes to `flash`.

---

## 2. Pre-Delegation Quota Verification

- Before launching parallel subagent teams or multi-worker fan-outs, check current quota health via:
  ```bash
  agentkit quota status
  ```
- If model quotas are degraded (`<= 15%`), verify that the planned delegation can proceed under fallback models (`inherit` / `flash`).
- When routing ambiguity occurs, evaluate recommended routing:
  ```bash
  agentkit quota route --complexity <low|med|high>
  ```

---

## 3. Strict Denial Adherence on Quota Wrap-Up

- When `guard-quota.py` denies `invoke_subagent` with `QUOTA WRAP-UP: All model quotas are critically low`:
  1. **Halt New Delegations**: Immediately cease invoking new subagents.
  2. **Do Not Retry Failed Invocations**: Do not attempt to bypass or re-invoke subagents with different model names.
  3. **Execute Quota Wrap-Up Runbook**: Follow the `quota-wrapup` skill to run automated tests, commit work-in-progress, generate the handoff note in the Obsidian AgentVault (`$HOME/Documents/AgentVault/Sessions/handoff-YYYY-MM-DD-quota-wrapup.md`), update `Projects/<project>.md`, and advise the user.
