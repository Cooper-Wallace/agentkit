# ADR 0002: Dual Naming Convention (`antigravity-standardise` and `agy-standardise`)

## Context
The Google Antigravity ecosystem features two common naming paradigms:
- The full product name: **Google Antigravity** (matching sibling projects like `gh-standardise`).
- The official CLI executable name: **`agy`**.

In terminal workflows, developers prefer concise commands (e.g. `agy` rather than `antigravity`), but for GitHub repository discovery and project spaces, full descriptive names are preferred.

## Decision
1. The repository space is named **`antigravity-standardise`**, directly aligning with **`gh-standardise`**.
2. The primary CLI binary is named **`agy-standardise`**, mirroring the official `agy` command prefix.
3. An alias symlink **`antigravity-standardise`** is created in `bin/` and installed to `~/.local/bin` to support both command preferences.

## Consequences
- **Positive**: High ergonomics in shell autocomplete (`agy-std<tab>` vs `antigravity-std<tab>`).
- **Positive**: Clear, descriptive repository naming in developer workspaces and GitHub portfolios.
