# ADR 0001: Non-Destructive Set-Union Merging Strategy for Permissions

## Context
When standardizing developer settings across multiple workstations, overwriting `~/.gemini/antigravity-cli/settings.json` completely would erase machine-specific customizations. Specifically:
- Different machines may have distinct `trustedWorkspaces` (e.g. desktop RAID storage paths vs laptop SSD directories).
- The user may have manually approved unique local scripts or third-party tools during active pair-programming sessions.

## Decision
`agy-standardise apply` employs a **non-destructive set-union merge strategy**:
1. Reads existing `settings.json` into memory.
2. Identifies all standard permissions from `config/permissions.json` not currently present in `permissions.allow`.
3. Appends only missing permissions to `permissions.allow`, preserving existing order and local entries.
4. Leaves `trustedWorkspaces`, custom models, and user preferences intact.
5. Generates a timestamped backup (`settings.json.bak.<timestamp>`) prior to writing.

## Consequences
- **Positive**: Zero risk of wiping machine-specific workspaces or existing custom rules.
- **Positive**: Idempotent execution—running `apply` repeatedly causes no drift or duplication.
- **Tradeoff**: Deprecating or revoking a standard permission in the repository will not automatically delete it from local machines without an explicit removal mechanism.
