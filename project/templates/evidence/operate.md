# Operations & Runbook: {{PROJECT_NAME}}

> **Summary**: {{PROJECT_DESCRIPTION}}  
> **Primary Stack**: {{PRIMARY_LANGUAGE}} | **Verify**: `{{TEST_COMMAND}}`

## 1. Setup & Run

```bash
# 1. Configure environment (never commit .env)
cp .env.example .env

# 2. Verify installation and test suite
{{TEST_COMMAND}}

# 3. Preview mutating operations safely
# <cli-entrypoint> --dry-run
```

## 2. Fail-Fast Startup Checks

On startup, {{PROJECT_NAME}} validates all prerequisites before accepting work:
- Required environment variables and config schema types/ranges.
- Read/write permissions on `$HOME`-relative or configured data directories.
- Schema/version compatibility of downstream stores (abort immediately with clear diagnostic on mismatch).

## 3. Structured Allow-Listed JSON Logs

Emit single-line JSON logs to `stdout`/`stderr` containing **only explicitly allow-listed keys**:
- **Allow-listed fields**: `timestamp`, `level`, `event`, `component`, `correlation_id`, `duration_ms`, `status`, `error_code`.
- **Forbidden fields**: Raw tokens, `.env` values, credentials, full file payloads, or unredacted user PII.

## 4. Configuration & Secrets

- Non-secret defaults live in versioned config; secrets live strictly in untracked `.env` or cloud secret managers.
- Resolve all filesystem paths dynamically via `$HOME` or project root—never hardcode `/home/<user>`.

## 5. Failure Modes & Recovery

| Failure Mode | Detection Signal | Impact | Recovery / Safe Degrade |
| :--- | :--- | :--- | :--- |
| Missing/invalid config at boot | Non-zero exit + `CONFIG_INVALID` log | Startup aborted cleanly | Fix `.env`/config and restart |
| Upstream data/API unavailable | Timeout or schema error | Batch/request aborted | Return safe-abstain / retry with backoff |
| Partial write interrupted | Checksum / atomic rename check | Zero corrupted state | Re-run idempotent command |

## 6. Rollback & Safe Degrade

- **Dry-Run Mode**: Run mutating scripts/CLIs with `--dry-run` before applying changes.
- **Rollback**: Revert git commit or restore previous atomic artifact snapshot; idempotent re-runs reconcile state safely.

## 7. Local-to-Cloud Operational Parity

| Concern | Local Behavior | Cloud Behavior | Divergence Risk |
| :--- | :--- | :--- | :--- |
| Secrets | `.env` file | Cloud Secret Manager / env vars | Ensure identical env var names |
| Storage Paths | `$HOME`-relative / local dir | Object bucket / managed volume | Abstract via repository interface |
| Scheduling / Triggers | Manual / local cron | CI runner / cloud scheduler | Keep CLI idempotent & stateless |
