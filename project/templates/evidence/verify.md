# Verification Evidence: {{PROJECT_NAME}}

> **Primary Verify Command**: `{{TEST_COMMAND}}`

## 1. Checks Run & Results

| Check | Command / Target | Scope Covered | Result | Notes |
| :--- | :--- | :--- | :--- | :--- |
| Automated Test Suite | `{{TEST_COMMAND}}` | Unit, guarded-failure, integration | _Pending / Pass_ | Run after final edit |
| Linter / Type Check | _(e.g., `ruff check`, `mypy`, `tsc`)_ | Static types & style | _Pending / Pass_ | Zero ignored errors |
| CLI Dry-Run / Smoke | _(e.g., `--dry-run`)_ | Idempotent mutation preview | _Pending / Pass_ | Verify no side effects |

## 2. Property-Based & Independent Oracle Coverage

- **Invariants Verified**: Document key algebraic, conservation, monotonicity, or zero-leakage properties tested (e.g., via `hypothesis`).
- **Independent Oracle**: Document any reference SQL/baseline implementation used to cross-check complex pipelines on deterministic fixtures.
- **Guarded Failure & Safe Degrade**: Confirm explicit tests exist for malformed input, boundary violations, and safe-abstain / fail-fast paths.

## 3. Known Gaps

| Gap ID | Description | Risk / Impact | Mitigation / Follow-Up |
| :--- | :--- | :--- | :--- |
| `GAP-01` | _(e.g., Cloud target integration tested via local mock seam only)_ | Low-Medium | Verify against staging credentials before release |

## 4. Check Matrix (Local vs. PR vs. Release)

| Check | Local Pre-Commit | PR CI Gate | Release Gate |
| :--- | :--- | :--- | :--- |
| `{{TEST_COMMAND}}` (unit + property) | Required | Required | Required |
| Static lint / type check | Required | Required | Required |
| Adversarial diff & `AC-*` audit | Recommended | Required | Required |
| End-to-end / Cloud parity smoke | Optional | Optional / Mocked | Required |
