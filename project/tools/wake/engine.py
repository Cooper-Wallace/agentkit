"""
Activity state arbiter and sleep inhibition engine.

Tracks active agent sessions, manages cooldown transitions to prevent inhibition flapping,
and persists session metrics to JSONL history.
"""

from dataclasses import asdict, dataclass
from enum import Enum
import json
import os
from pathlib import Path
import sys
import threading
import time
from typing import Optional, Union

from .inhibitor import BaseSleepInhibitor, get_system_inhibitor


class ActivityState(Enum):
    """Activity states for the wake engine."""

    IDLE = "idle"
    ACTIVE = "active"
    COOLDOWN = "cooldown"
    OVERRIDE = "override"


@dataclass
class SessionInfo:
    """Represents an active agent execution session."""

    session_id: str
    agent_name: str
    task: str
    start_time: float
    last_active_time: float
    pid: int = 0


def get_default_history_path() -> Path:
    """Return platform-standard path for wake history journal."""
    if sys.platform == "win32":
        local_app_data = os.environ.get("LOCALAPPDATA")
        base = Path(local_app_data) if local_app_data else Path.home() / "AppData" / "Local"
        return base / "agentkit" / "wake_history.jsonl"
    else:
        state_home = os.environ.get("XDG_STATE_HOME")
        base = Path(state_home) if state_home else Path.home() / ".local" / "state"
        return base / "agentkit" / "wake_history.jsonl"


class WakeEngine:
    """State arbiter controlling sleep inhibition based on active agent sessions."""

    def __init__(
        self,
        inhibitor: Optional[BaseSleepInhibitor] = None,
        cooldown_seconds: float = 45.0,
        history_path: Optional[Union[str, Path]] = None,
    ) -> None:
        self.inhibitor: BaseSleepInhibitor = (
            inhibitor if inhibitor is not None else get_system_inhibitor()
        )
        self.cooldown_seconds: float = float(cooldown_seconds)
        self.history_path: Path = (
            Path(history_path) if history_path is not None else get_default_history_path()
        )
        self.state: ActivityState = ActivityState.IDLE
        self.sessions: dict[str, SessionInfo] = {}

        self._lock: threading.RLock = threading.RLock()
        self._cooldown_timer: Optional[threading.Timer] = None
        self._cooldown_end: float = 0.0
        self._override_timer: Optional[threading.Timer] = None
        self._pending_history: list[dict] = []

    def _flush_history(self) -> None:
        """Write pending session records to JSONL history file."""
        if not self._pending_history:
            return
        try:
            self.history_path.parent.mkdir(parents=True, exist_ok=True)
            with open(self.history_path, "a", encoding="utf-8") as f:
                for entry in self._pending_history:
                    f.write(json.dumps(entry) + "\n")
        except Exception:
            pass
        finally:
            self._pending_history.clear()

    def _on_cooldown_expired(self) -> None:
        """Handle cooldown expiration when no new sessions became active."""
        with self._lock:
            if self.state == ActivityState.COOLDOWN and not self.sessions:
                self.state = ActivityState.IDLE
                self._cooldown_timer = None
                self._cooldown_end = 0.0
                self.inhibitor.release()
                self._flush_history()

    def notify(
        self,
        session_id: str,
        state: str,
        agent: str = "agent",
        task: str = "",
        pid: int = 0,
    ) -> dict:
        """Process session state transition notification."""
        with self._lock:
            state_norm = state.strip().lower()
            now = time.time()

            if state_norm == "active":
                # Cancel any pending cooldown timer
                if self._cooldown_timer is not None:
                    self._cooldown_timer.cancel()
                    self._cooldown_timer = None
                    self._cooldown_end = 0.0

                if session_id in self.sessions:
                    s = self.sessions[session_id]
                    s.last_active_time = now
                    if agent:
                        s.agent_name = agent
                    if task:
                        s.task = task
                    if pid:
                        s.pid = pid
                else:
                    self.sessions[session_id] = SessionInfo(
                        session_id=session_id,
                        agent_name=agent,
                        task=task,
                        start_time=now,
                        last_active_time=now,
                        pid=pid,
                    )

                if self.state != ActivityState.OVERRIDE:
                    self.state = ActivityState.ACTIVE
                    if not self.inhibitor.is_active():
                        self.inhibitor.acquire(reason=f"Active agent session: {session_id}")

            elif state_norm == "idle":
                if session_id in self.sessions:
                    s = self.sessions.pop(session_id)
                    duration = now - s.start_time
                    self._pending_history.append(
                        {
                            "session_id": s.session_id,
                            "agent_name": s.agent_name,
                            "task": s.task,
                            "start_time": s.start_time,
                            "end_time": now,
                            "duration_seconds": round(duration, 2),
                            "pid": s.pid,
                        }
                    )

                if not self.sessions and self.state != ActivityState.OVERRIDE:
                    if self.cooldown_seconds > 0:
                        self.state = ActivityState.COOLDOWN
                        self._cooldown_end = now + self.cooldown_seconds
                        if self._cooldown_timer is not None:
                            self._cooldown_timer.cancel()
                        self._cooldown_timer = threading.Timer(
                            self.cooldown_seconds, self._on_cooldown_expired
                        )
                        self._cooldown_timer.daemon = True
                        self._cooldown_timer.start()
                    else:
                        self.state = ActivityState.IDLE
                        self._cooldown_timer = None
                        self._cooldown_end = 0.0
                        self.inhibitor.release()
                        self._flush_history()

            else:
                raise ValueError(
                    f"Invalid state: '{state}'. Allowed states are 'active' or 'idle'."
                )

            return self.get_status()

    def set_override(self, enabled: bool, duration_seconds: int = 0) -> dict:
        """Set or clear manual sleep inhibition override."""
        with self._lock:
            if self._override_timer is not None:
                self._override_timer.cancel()
                self._override_timer = None

            if enabled:
                if self._cooldown_timer is not None:
                    self._cooldown_timer.cancel()
                    self._cooldown_timer = None
                    self._cooldown_end = 0.0

                self.state = ActivityState.OVERRIDE
                if not self.inhibitor.is_active():
                    self.inhibitor.acquire(reason="Manual override enabled")

                if duration_seconds > 0:
                    self._override_timer = threading.Timer(
                        float(duration_seconds),
                        lambda: self.set_override(False),
                    )
                    self._override_timer.daemon = True
                    self._override_timer.start()
            else:
                if self.sessions:
                    self.state = ActivityState.ACTIVE
                    if not self.inhibitor.is_active():
                        self.inhibitor.acquire(
                            reason=f"Active agent sessions ({len(self.sessions)})"
                        )
                elif self._cooldown_timer is not None and self._cooldown_end > time.time():
                    self.state = ActivityState.COOLDOWN
                else:
                    self.state = ActivityState.IDLE
                    self.inhibitor.release()
                    self._flush_history()

            return self.get_status()

    def get_status(self) -> dict:
        """Return current activity and inhibition status dictionary."""
        with self._lock:
            now = time.time()
            cooldown_remaining = 0.0
            if self.state == ActivityState.COOLDOWN and self._cooldown_end > now:
                cooldown_remaining = round(max(0.0, self._cooldown_end - now), 2)

            sessions_list = [asdict(s) for s in self.sessions.values()]

            return {
                "state": self.state.value,
                "inhibited": self.inhibitor.is_active(),
                "active_count": len(self.sessions),
                "sessions": sessions_list,
                "cooldown_remaining": cooldown_remaining,
            }

    def get_history(self, limit: int = 50) -> list[dict]:
        """Read recent session journal entries from disk."""
        with self._lock:
            if limit <= 0:
                return []
            entries: list[dict] = []
            if self.history_path.exists():
                try:
                    with open(self.history_path, "r", encoding="utf-8") as f:
                        for line in f:
                            line = line.strip()
                            if line:
                                try:
                                    entries.append(json.loads(line))
                                except json.JSONDecodeError:
                                    continue
                except Exception:
                    pass
            return entries[-limit:]

    def stop(self) -> None:
        """Cancel background timers, flush pending history, and release inhibitor."""
        with self._lock:
            if self._cooldown_timer is not None:
                self._cooldown_timer.cancel()
                self._cooldown_timer = None
            if self._override_timer is not None:
                self._override_timer.cancel()
                self._override_timer = None
            self._flush_history()
            if self.inhibitor.is_active():
                self.inhibitor.release()


# Backward compatibility alias
GuardEngine = WakeEngine
