# 12. Cross-Platform Agent Wake Lock and Visual State

Date: 2026-10-26
Status: Accepted

## Context
During long-running automated agent tasks, multi-hour refactors, or overnight executions, host operating systems (Linux, Windows, macOS) frequently enter sleep or power-saving states when there is no active physical keyboard or mouse input. This causes AI agent CLIs and background tasks to get suspended mid-prompt. To avoid semantic confusion with AI safety guardrails, the sleep prevention functionality is standardized under the "wake" domain (with "wakelock" alias).

## Decision
We implement `agentkit wake` (with `agentkit wakelock` alias), a cross-platform sleep inhibition and ambient wake-lock monitoring system comprising:
1. **Decoupled Inhibitor Engine (`project/tools/wake/engine.py`)**: Platform-specific backends supporting Linux D-Bus `login1` / `Inhibit`, Windows `SetThreadExecutionState`, and macOS `caffeinate`.
2. **Loopback IPC Server (`project/tools/wake/server.py`)**: Lightweight ThreadingHTTPServer on `127.0.0.1:18820` managing session states, 45-second anti-flapping cooldowns, and audit journaling.
3. **Cross-Platform System Tray & Icons (`project/tools/wake/tray.py`, `icons.py`)**: Ambient tray feedback reflecting wake-lock state (active vs idle) with menu controls for manual override and status inspection.
4. **Autostart Manager (`project/tools/wake/autostart.py`)**: Standardized startup persistence via XDG autostart `.desktop` entries on Linux and Startup shortcuts on Windows, integrated directly into `agentkit apply`.
5. **CLI Subcommands (`bin/agentkit wake` / `bin/agentkit wakelock`)**: Full management suite for status inspection, daemon control, tray execution, event notification, live watch loop, autostart configuration, manual overrides, and session log history.

## Consequences
- **Zero Sleep Interruptions**: Agent CLI prompts executing actively prevent host sleep without requiring user wake locks.
- **AI Safety Clarity**: Renaming from "guard" to "wake" prevents confusion with agent safety rails (e.g., quota-guardian, guard-git-operations).
- **Zero KDE/OS Lock-In**: Decoupled standard backends work across Linux desktop environments (KDE, GNOME, headless), Windows, and macOS.
- **Ambient Feedback**: System tray indicator provides instant visual confirmation of agent activity.
- **Automated Lifecycle Deployment**: `agentkit apply` automatically installs the `wake-lock` plugin and registers desktop autostart.
- **Resilient Fallback**: Hook notification failures silently timeout without disrupting agent prompt workflows.
