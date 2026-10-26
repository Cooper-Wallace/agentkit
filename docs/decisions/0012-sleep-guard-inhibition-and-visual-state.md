# 12. Cross-Platform Agent Sleep Guard and Visual State

Date: 2026-10-26
Status: Accepted

## Context
During long-running automated agent tasks, multi-hour refactors, or overnight executions, host operating systems (Linux, Windows, macOS) frequently enter sleep or power-saving states when there is no active physical keyboard or mouse input. This causes AI agent CLIs and background tasks to get suspended mid-prompt.

## Decision
We implement `agentkit guard`, a cross-platform sleep inhibition and visual status monitoring system comprising:
1. **Decoupled Inhibitor Engine (`project/tools/guard/engine.py`)**: Platform-specific backends supporting Linux D-Bus `login1` / `Inhibit`, Windows `SetThreadExecutionState`, and macOS `caffeinate`.
2. **Loopback IPC Server (`project/tools/guard/server.py`)**: Lightweight FastAPI/WSGI HTTP server on `127.0.0.1:18820` managing session states, 45-second anti-flapping cooldowns, and audit journaling.
3. **Cross-Platform System Tray & Icons (`project/tools/guard/tray.py`, `icons.py`)**: Ambient tray feedback reflecting sleep inhibition state (active vs idle) with menu controls for manual override and status inspection.
4. **Autostart Manager (`project/tools/guard/autostart.py`)**: Standardized startup persistence via XDG autostart `.desktop` entries on Linux and Startup shortcuts on Windows.
5. **CLI Subcommands (`bin/agentkit guard`)**: Full management suite for status inspection, daemon control, tray execution, event notification, live watch loop, autostart configuration, manual overrides, and session log history.

## Consequences
- **Zero Sleep Interruptions**: Agent CLI prompts executing actively prevent host sleep without requiring user wake locks.
- **Zero KDE/OS Lock-In**: Decoupled standard backends work across Linux desktop environments (KDE, GNOME, headless), Windows, and macOS.
- **Ambient Feedback**: System tray indicator provides instant visual confirmation of agent activity.
- **Resilient Fallback**: Hook notification failures silently timeout without disrupting agent prompt workflows.
