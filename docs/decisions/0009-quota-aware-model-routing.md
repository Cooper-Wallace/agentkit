# ADR 0009: Quota-Aware Model Routing, Subagent Complexity Tiers, and Graceful Wrap-Up Protocol

## Context

Autonomous multi-model workflows under `agentkit` coordinate high-tier root orchestrators with concurrent subagents across short-lived feature branches. However, production operations across developer workstations revealed four operational failure modes under heavy development loads:

1. **Catastrophic Quota Exhaustion & Hung Subagents**:
   - Subagents dispatched during long-running feature DAGs or large fan-outs previously had zero runtime awareness of remaining token quotas.
   - When a quota family (such as Google Gemini daily/weekly allocations) exhausted mid-execution, subagent invocations stalled or failed abruptly. Parent orchestrators hung waiting for responses that would never arrive, leaving uncommitted changes and unmerged worktrees.
2. **Retirement of Gemini 3.1 Pro / 3.0 Pro from Antigravity CLI**:
   - Google Antigravity CLI retired Gemini 3.1 Pro and 3.0 Pro endpoints. Previous model profiles (e.g. `pro-flash`) and references to `Gemini 3.0 Pro` as a root orchestrator or subagent tier became obsolete.
   - Workflows required an updated model tiering strategy reconciling Gemini 4 Argon, Gemini 3.8 Flash (with High reasoning), Claude Opus 5.5, and Claude Sonnet 3.7.
3. **Uniform Subagent Over-Provisioning**:
   - Subagents were uniformly dispatched as `flash` or `inherit` regardless of whether the task was a 3-line doc typo fix or a deep cross-module security refactoring.
   - Simple mechanical tasks (file renames, formatting, regex find-and-replace, single-command checks) unnecessarily consumed standard Flash quota, while complex architectural subtasks lacked an escalation path to deep reasoning models.
4. **Ungraceful Termination on Final Token Depletion**:
   - When token exhaustion occurred near the end of a sprint or daily limit, agents were terminated without capturing state, running final test suites, or documenting unfinished task DAGs for resuming in the next quota window.

---

## Decision

1. **Updated Cognitive Architecture & Model Tiering**:
   - **Retire Gemini 3.1 Pro / 3.0 Pro**: Deprecate `pro-flash` in favor of `flash-high`. Pro endpoints are removed from root default configurations.
   - **Root Orchestrator Hierarchy**:
     - **Primary Tier (`argon-flash`)**: Gemini 4 Argon (`Model: "pro"` / `"inherit"`) when Argon quota is available AND the task involves complex architecture, multi-repo task DAG generation, or adversarial diff auditing.
     - **Standard Tier (`flash-high`)**: Gemini 3.8 Flash with High reasoning (`Model: "flash"` / `"inherit"`) for standard feature and lifecycle workflows, delivering rapid execution with zero Argon quota consumption.
     - **Fallback Tiers (`opus-flash` / `sonnet-flash`)**: Claude Opus 5.5 or Claude Sonnet 3.7 (`Model: "opus"` / `"claude-opus-5.5"` / `"inherit"`) when Gemini quota groups are exhausted or rate-limited.
   - **Subagent Complexity Tiers**:
     - `low`: `flash_lite` (file moves/renames, mechanical find-replace, formatting, doc typos, simple one-command inspection).
     - `med`: `flash` (feature code, tests, localized refactors).
     - `high`: `flash` (cross-module refactors, complex debugging, security), with escalation to `pro` (Gemini 4 Argon) for architecture, multi-repo design, or adversarial review when Argon quota is healthy.
     - **Tag Convention**: Orchestrators must place `[complexity: low|med|high]` at the start of subagent prompts.

2. **Language Server Quota Summary Polling**:
   - Interface deterministically with the Antigravity Language Server RPC endpoint `RetrieveUserQuotaSummary` (`http://$ANTIGRAVITY_LS_ADDRESS/exa.language_server_pb.LanguageServerService/RetrieveUserQuotaSummary` using header `x-codeium-csrf-token: $ANTIGRAVITY_CSRF_TOKEN`).
   - Group quotas into 3 distinct operational families:
     - **Gemini Family**: Gemini 3.8 Flash and Flash Lite.
     - **Claude + GPT Family**: Claude Opus 5.5 and Claude Sonnet 3.7.
     - **Gemini 4 Argon Family**: Dedicated deep reasoning quota.
   - Cache responses locally (`~/.cache/antigravity-quota/summary.json` or configurable via `AGENTKIT_QUOTA_CACHE`) with a 60-second TTL (`cache_ttl_seconds: 60`) to avoid RPC overhead during parallel subagent fan-outs.
   - Provide testing fixtures and emergency overrides via `AGENTKIT_QUOTA_FIXTURE` and `AGENTKIT_QUOTA_GUARD=0`.

3. **Autonomous Quota Guardian Plugin (`quota-guardian`)**:
   - Introduce `harness/plugins/quota-guardian/` equipped with a `PreToolUse` hook intercepting `invoke_subagent` calls (`scripts/guard-quota.py`).
   - **Automated Fallback Rerouting (`switch_threshold: 0.15`)**: When a target model's quota group falls below 15% remaining capacity, the hook automatically inspects `config/quota.json` fallback chains and mutates the `Model` argument in-flight to route the subagent to an available healthy model family without interrupting execution.
   - **Complexity-Aware Routing**: Evaluates `[complexity: low|med|high]` prompt tags to map tasks to `flash_lite`, `flash`, or `pro` according to live quota headroom.

4. **Graceful Wrap-Up Circuit Breaker (`wrapup_threshold: 0.05`)**:
   - When all eligible quota groups fall below 5% remaining capacity, `guard-quota.py` denies further subagent delegation with a `QUOTA WRAP-UP:` decision.
   - Enforces the `quota-wrapup` protocol:
     - Immediately cease spawning new subagents.
     - Run existing test suites locally to verify current stability.
     - Create a clean WIP commit on the active feature branch (`git commit -m "chore(wip): save state prior to quota wrap-up"`).
     - Generate a structured `handoff.md` detailing completed work, in-flight DAG tasks, and exact commands to resume.
     - Inform the user of the exact reset timestamp and recommend an advisory root model switch.

5. **Advisory Root Model Switching**:
   - Because the root session model cannot be altered programmatically via API during an active session, root switching remains advisory.
   - When root quota is depleted, the agent explicitly instructs the user to switch the session model using the UI model picker dropdown or the `/model` slash command.

6. **CLI Quota Diagnostics**:
   - Provide first-class CLI commands via `agentkit quota`:
     - `agentkit quota status [--json]`: Displays formatted table of remaining percentages, request counts, and reset times across all 3 quota groups.
     - `agentkit quota route --complexity <low|med|high> [--json]`: Recommends optimal model allocation based on real-time quota telemetry.

---

## Consequences

- **Positive (Zero Hung Subagents)**: Proactive pre-tool checks and automated fallback rerouting prevent subagents from failing silently or stalling orchestrator loops when quotas lapse.
- **Positive (Resource Efficiency)**: Complexity tiering routes mechanical and lightweight tasks to `flash_lite`, conserving Flash and Argon quotas for heavy engineering tasks.
- **Positive (Predictable Session Handoffs)**: The 5% wrap-up threshold ensures that work is never lost, guaranteeing clean git commits, passing tests, and actionable handoff markdown before complete shutdown.
- **Positive (Modernized Model Profiles)**: Replaces retired Gemini 3.0/3.1 Pro references with `flash-high`, aligning with current Antigravity CLI capabilities.
- **Tradeoff**: Fallback routing across model providers (e.g. Gemini to Claude) may slightly alter token generation characteristics or output phrasing during high-load periods.
