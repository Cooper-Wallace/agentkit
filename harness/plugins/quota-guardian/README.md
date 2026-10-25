# Quota Guardian Plugin (`quota-guardian`)

The **Quota Guardian** plugin provides automated quota-aware model routing, complexity-to-model tier mapping, and a graceful wrap-up gate for autonomous AI agent workflows.

---

## Features

- **Real-Time Quota Telemetry**: Queries the local Antigravity Language Server (`RetrieveUserQuotaSummary`) to extract remaining capacity across model groups (`gemini`, `claude`, `argon`).
- **Fail-Open Architecture**: If the language server is unreachable or offline, the plugin safely falls back to cached data or synthetic healthy payloads, ensuring developer workflows are never blocked by network failure.
- **Complexity-Aware Routing**: Inspects task complexity tags (`[complexity: low|med|high]`) to dynamically select cost-effective worker models (`flash_lite`, `flash`) or escalate to reasoning models (`pro`).
- **Safety Guardrail**: Automatically upgrades tasks marked `low` to `flash` if prompt content involves tests, refactoring, debugging, or security.
- **PreToolUse Interception**: Intercepts `invoke_subagent` calls, rewriting degraded model requests to healthy fallbacks or halting delegation when quotas are critically depleted.
- **Graceful Quota Wrap-Up**: Triggers the `quota-wrapup` skill when all model quotas dip below critical thresholds (`<= 5%`), saving WIP commits and generating `handoff.md` artifacts.

---

## Directory Structure

```text
harness/plugins/quota-guardian/
├── README.md               # User and developer documentation
├── plugin.json             # Antigravity plugin manifest
├── hooks.json              # PreToolUse hook configuration for invoke_subagent
├── rules/
│   └── AGENTS.md           # Universal quota rules and directives for agents
├── scripts/
│   ├── quota_status.py     # Quota summary querying and route calculation CLI
│   └── guard-quota.py      # PreToolUse hook script for invoke_subagent
└── skills/
    └── quota-wrapup/
        └── SKILL.md        # 6-phase graceful task conclusion runbook
```

---

## CLI Usage

### Check Quota Status
```bash
agentkit quota status
# or with JSON output:
agentkit quota status --json
```

### Route Task Complexity
```bash
agentkit quota route --complexity low
agentkit quota route --complexity med
agentkit quota route --complexity high --json
```

---

## Environment Variables

- `AGENTKIT_QUOTA_GUARD=off`: Disables `guard-quota.py` hook interception.
- `AGENTKIT_QUOTA_FIXTURE=/path/to/fixture.json`: Injects an offline JSON payload for testing and verification.
- `AGENTKIT_QUOTA_CACHE=/path/to/cache.json`: Overrides the default cache path (`~/.cache/agentkit/quota.json`).
- `AGENTKIT_QUOTA_CONFIG=/path/to/quota.json`: Overrides the configuration file path.

---

## Testing

Run unit tests:
```bash
python3 -m unittest tests/test_quota_guardian.py -v
```
