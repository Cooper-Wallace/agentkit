# Intent & Acceptance Criteria: {{PROJECT_NAME}}

> **Summary**: {{PROJECT_DESCRIPTION}}  
> **Primary Stack**: {{PRIMARY_LANGUAGE}}

## 1. Problem Statement

Describe the concrete user or system pain point {{PROJECT_NAME}} solves, why existing tooling is insufficient, and the measurable outcome expected.

## 2. Target Users & Workflows

- **Primary Users**: (e.g., local developers, automated agent workflows, analysts)
- **Core Workflows**:
  1. Workflow A — trigger, action, observable outcome.
  2. Workflow B — failure/degrade path and recovery behavior.

## 3. Scope Boundaries

### In Scope
- Core capability 1
- Deterministic validation and structured diagnostics

### Out of Scope
- Features explicitly deferred (record rationale in `evidence/decisions.md`)

## 4. Assumptions & Constraints

- Runs deterministically in local environment using `$HOME`-relative or repo-relative paths.
- Must degrade safely on invalid input or missing optional dependencies without silent corruption.

## 5. Acceptance Criteria (`AC-*`)

| ID | Category | Given / When / Then | Verification Method |
| :--- | :--- | :--- | :--- |
| `AC-01` | Functional | **Given** valid input, **When** primary workflow executes, **Then** expected output matches contract. | `{{TEST_COMMAND}}` (unit + oracle test) |
| `AC-02` | Functional | **Given** malformed input or missing dependency, **When** startup/execution runs, **Then** system fails fast with actionable non-zero exit/error. | `{{TEST_COMMAND}}` (guarded failure test) |
| `AC-03` | Non-Functional | **Given** repeated execution, **When** run with identical state, **Then** operation is idempotent with zero duplicate side effects. | `{{TEST_COMMAND}}` (idempotency test) |
| `AC-04` | Non-Functional | **Given** runtime execution, **When** logs are emitted, **Then** only allow-listed structured JSON fields appear (zero secrets/PII). | Log schema assertion |
