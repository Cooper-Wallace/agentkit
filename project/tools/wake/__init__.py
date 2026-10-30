"""
Cross-platform sleep inhibition engine, state arbiter, and loopback IPC server.
"""

from .engine import (
    ActivityState,
    GuardEngine,
    SessionInfo,
    WakeEngine,
    get_default_history_path,
)
from .inhibitor import (
    BaseSleepInhibitor,
    LinuxSleepInhibitor,
    MacOSSleepInhibitor,
    MockSleepInhibitor,
    WindowsSleepInhibitor,
    get_system_inhibitor,
)
from .server import (
    GuardServer,
    WakeServer,
    fetch_status,
    send_notification,
    send_override,
)

__all__ = [
    "BaseSleepInhibitor",
    "WindowsSleepInhibitor",
    "LinuxSleepInhibitor",
    "MacOSSleepInhibitor",
    "MockSleepInhibitor",
    "get_system_inhibitor",
    "ActivityState",
    "SessionInfo",
    "WakeEngine",
    "GuardEngine",
    "get_default_history_path",
    "WakeServer",
    "GuardServer",
    "send_notification",
    "fetch_status",
    "send_override",
]
