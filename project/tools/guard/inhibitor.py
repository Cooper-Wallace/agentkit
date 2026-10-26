"""
Cross-platform system sleep inhibitor implementations.

Supports Windows (SetThreadExecutionState via ctypes),
Linux (systemd login1 Manager via D-Bus with systemd-inhibit CLI fallback),
macOS (caffeinate via subprocess), and in-memory mock for testing.
"""

from abc import ABC, abstractmethod
import os
import shutil
import subprocess
import sys
from typing import Optional


class BaseSleepInhibitor(ABC):
    """Abstract base class for system sleep inhibitors."""

    def __init__(self) -> None:
        self._is_active: bool = False
        self._reason: str = ""
        self._keep_screen_on: bool = False

    @abstractmethod
    def acquire(self, reason: str = "Agent active", keep_screen_on: bool = False) -> bool:
        """Acquire sleep inhibition lock."""
        pass

    @abstractmethod
    def release(self) -> bool:
        """Release sleep inhibition lock."""
        pass

    def is_active(self) -> bool:
        """Return True if sleep inhibition is currently active."""
        return self._is_active

    @abstractmethod
    def get_inhibition_type(self) -> str:
        """Return the platform inhibition type identifier."""
        pass


class WindowsSleepInhibitor(BaseSleepInhibitor):
    """Windows sleep inhibitor using SetThreadExecutionState via ctypes."""

    ES_CONTINUOUS: int = 0x80000000
    ES_SYSTEM_REQUIRED: int = 0x00000001
    ES_AWAYMODE_REQUIRED: int = 0x00000040
    ES_DISPLAY_REQUIRED: int = 0x00000002

    def __init__(self) -> None:
        super().__init__()

    def acquire(self, reason: str = "Agent active", keep_screen_on: bool = False) -> bool:
        flags = self.ES_CONTINUOUS | self.ES_SYSTEM_REQUIRED | self.ES_AWAYMODE_REQUIRED
        if keep_screen_on:
            flags |= self.ES_DISPLAY_REQUIRED

        try:
            import ctypes

            windll = getattr(ctypes, "windll", None)
            if windll is not None:
                res = windll.kernel32.SetThreadExecutionState(flags)
                if res == 0:
                    return False
            elif sys.platform == "win32":
                return False
            else:
                # Non-Windows without windll mock
                return False

            self._is_active = True
            self._reason = reason
            self._keep_screen_on = keep_screen_on
            return True
        except Exception:
            return False

    def release(self) -> bool:
        if not self._is_active:
            return True
        try:
            import ctypes

            windll = getattr(ctypes, "windll", None)
            if windll is not None:
                windll.kernel32.SetThreadExecutionState(self.ES_CONTINUOUS)
            self._is_active = False
            return True
        except Exception:
            return False

    def get_inhibition_type(self) -> str:
        return "windows"


class LinuxSleepInhibitor(BaseSleepInhibitor):
    """Linux sleep inhibitor using systemd login1 Manager D-Bus or systemd-inhibit."""

    def __init__(self) -> None:
        super().__init__()
        self._fd: Optional[int] = None
        self._proc: Optional[subprocess.Popen] = None
        self._mechanism: str = "none"

    def acquire(self, reason: str = "Agent active", keep_screen_on: bool = False) -> bool:
        if self._is_active:
            return True

        what = "idle:sleep:handle-lid-switch" if keep_screen_on else "idle:sleep"
        who = "agentkit-guard"
        why = reason
        mode = "block"

        # Primary mechanism: D-Bus org.freedesktop.login1.Manager Inhibit
        try:
            import dbus

            bus = dbus.SystemBus()
            proxy = bus.get_object("org.freedesktop.login1", "/org/freedesktop/login1")
            manager = dbus.Interface(proxy, "org.freedesktop.login1.Manager")
            res_fd = manager.Inhibit(what, who, why, mode)
            fd = res_fd.take() if hasattr(res_fd, "take") else int(res_fd)
            if fd >= 0:
                self._fd = fd
                self._mechanism = "dbus"
                self._is_active = True
                self._reason = reason
                self._keep_screen_on = keep_screen_on
                return True
        except Exception:
            self._fd = None

        # Fallback mechanism: systemd-inhibit command line wrapper
        try:
            inhibit_bin = shutil.which("systemd-inhibit") or "/usr/bin/systemd-inhibit"
            cmd = [
                inhibit_bin,
                f"--what={what}",
                f"--who={who}",
                f"--why={why}",
                "sleep",
                "infinity",
            ]
            self._proc = subprocess.Popen(
                cmd,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )
            if self._proc.poll() is None:
                self._mechanism = "systemd-inhibit"
                self._is_active = True
                self._reason = reason
                self._keep_screen_on = keep_screen_on
                return True
            else:
                self._proc = None
        except Exception:
            self._proc = None

        return False

    def release(self) -> bool:
        if not self._is_active:
            return True

        success = True
        if self._fd is not None:
            try:
                os.close(self._fd)
            except OSError:
                success = False
            self._fd = None

        if self._proc is not None:
            try:
                self._proc.terminate()
                self._proc.wait(timeout=2.0)
            except Exception:
                try:
                    self._proc.kill()
                except Exception:
                    pass
            self._proc = None

        self._is_active = False
        self._mechanism = "none"
        return success

    def get_inhibition_type(self) -> str:
        return "linux"

    def __del__(self) -> None:
        try:
            self.release()
        except Exception:
            pass


class MacOSSleepInhibitor(BaseSleepInhibitor):
    """macOS sleep inhibitor using /usr/bin/caffeinate."""

    def __init__(self) -> None:
        super().__init__()
        self._proc: Optional[subprocess.Popen] = None

    def acquire(self, reason: str = "Agent active", keep_screen_on: bool = False) -> bool:
        if self._is_active:
            return True

        flags = ["-i", "-s"]
        if keep_screen_on:
            flags.append("-d")

        caffeinate_bin = shutil.which("caffeinate") or "/usr/bin/caffeinate"
        cmd = [caffeinate_bin, *flags]

        try:
            self._proc = subprocess.Popen(
                cmd,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )
            if self._proc.poll() is None:
                self._is_active = True
                self._reason = reason
                self._keep_screen_on = keep_screen_on
                return True
            else:
                self._proc = None
                return False
        except Exception:
            self._proc = None
            return False

    def release(self) -> bool:
        if not self._is_active:
            return True

        if self._proc is not None:
            try:
                self._proc.terminate()
                self._proc.wait(timeout=2.0)
            except Exception:
                try:
                    self._proc.kill()
                except Exception:
                    pass
            self._proc = None

        self._is_active = False
        return True

    def get_inhibition_type(self) -> str:
        return "macos"

    def __del__(self) -> None:
        try:
            self.release()
        except Exception:
            pass


class MockSleepInhibitor(BaseSleepInhibitor):
    """Clean in-memory mock sleep inhibitor for testing and non-production environments."""

    def __init__(self) -> None:
        super().__init__()

    def acquire(self, reason: str = "Agent active", keep_screen_on: bool = False) -> bool:
        self._is_active = True
        self._reason = reason
        self._keep_screen_on = keep_screen_on
        return True

    def release(self) -> bool:
        self._is_active = False
        return True

    def get_inhibition_type(self) -> str:
        return "mock"


def get_system_inhibitor() -> BaseSleepInhibitor:
    """Factory returning the appropriate sleep inhibitor for current platform."""
    if sys.platform == "win32":
        return WindowsSleepInhibitor()
    elif sys.platform == "linux":
        return LinuxSleepInhibitor()
    elif sys.platform == "darwin":
        return MacOSSleepInhibitor()
    return MockSleepInhibitor()
