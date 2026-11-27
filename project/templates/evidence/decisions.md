# Architectural Decisions & Non-Goals: {{PROJECT_NAME}}

> **Summary**: {{PROJECT_DESCRIPTION}}

## 1. Key Trade-Off Decisions

| ID | Context | Decision | Alternatives Rejected | Consequences |
| :--- | :--- | :--- | :--- | :--- |
| `DEC-01` | Persistence & portability | Isolate storage behind a typed repository interface (`Protocol`) | Direct SQL/filesystem calls in domain functions | Easy local-to-cloud swap; slight boilerplate at seam |
| `DEC-02` | Startup reliability | Fail fast on missing config or invalid schemas | Lazy runtime defaults or silent fallbacks | Catches misconfiguration immediately; requires explicit config |
| `DEC-03` | Verification rigor | Combine `{{TEST_COMMAND}}` unit tests with property invariants | Happy-path-only example tests | Higher confidence on edge cases and zero-leakage rules |

## 2. What Was Intentionally Not Built

Keep scope tight and document deferred capabilities explicitly so future contributors do not treat omissions as bugs:

- **Item 1 (e.g., Distributed multi-node orchestration)**: Not needed for current single-host throughput; revisit only if latency/volume exceeds local bounds.
- **Item 2 (e.g., Custom UI / dashboard)**: CLI + structured JSON logs + Markdown evidence artifacts satisfy current agentic and human workflows.
- **Item 3 (e.g., Speculative plugin framework)**: Direct modular imports in `src/` are simpler until at least 3 concrete adapters exist.
