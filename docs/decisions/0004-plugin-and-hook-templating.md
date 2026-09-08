# ADR 0004: Templating Strategy for Lifecycle Hooks and Plugins

## Context
Antigravity CLI provides extensible runtime points through lifecycle hooks (`hooks.json`) and plugin bundles (`plugins/<name>/plugin.json`).

While defining standard pre-approved permissions and universal developer directives is immediately applicable to all workspaces, activating active hooks (such as forced linters or pre-execution command interceptors) can introduce unwanted latency or unexpected side-effects if imposed globally before a project specifies its desired toolchain.

## Decision
1. **Provide Canonical Archetype Templates**:
   Deliver clean, documented templates for hooks ([`templates/hooks.json`](../../templates/hooks.json)), plugin bundles ([`templates/plugin/`](../../templates/plugin/)), and plugin search registries ([`templates/plugins.json`](../../templates/plugins.json)).
2. **Opt-In / Disabled by Default**:
   All template hooks and plugin definitions declare `"enabled": false` or `"disabled": true` so that inspecting or copying them causes zero unintended runtime side-effects.
3. **Document Potential Standard Candidates**:
   Document recommendations for safety-gate hooks, auto-formatting hooks, and ecosystem plugins (`rko-workstation-companion`, `gh-devops-governance`) in [`docs/plugins-and-hooks-guide.md`](../plugins-and-hooks-guide.md) without enforcing live options prematurely.

## Consequences
- **Positive**: Clean separation between immediate baseline standards (permissions, settings, GEMINI.md) and optional project-level runtime automations.
- **Positive**: Developers have high-quality archetypes ready to copy and activate on demand.
- **Positive**: Zero latency or unexpected command blocking in existing workspaces.
