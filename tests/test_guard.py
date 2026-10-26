"""
Unit tests for sleep inhibition engine, state arbiter, and loopback IPC server.
"""

import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time
import unittest
from unittest import mock

# Ensure project root is in sys.path
REPO_ROOT = Path(__file__).resolve().parent.parent
if str(REPO_ROOT) not in sys.path:
    sys.path.insert(0, str(REPO_ROOT))

from project.tools.guard.engine import (
    ActivityState,
    GuardEngine,
    SessionInfo,
    get_default_history_path,
)
from project.tools.guard.inhibitor import (
    BaseSleepInhibitor,
    LinuxSleepInhibitor,
    MacOSSleepInhibitor,
    MockSleepInhibitor,
    WindowsSleepInhibitor,
    get_system_inhibitor,
)
from project.tools.guard.server import (
    GuardServer,
    fetch_status,
    send_notification,
    send_override,
)


class TestInhibitors(unittest.TestCase):
    """Test sleep inhibitor platform implementations and mocks."""

    def test_mock_sleep_inhibitor(self) -> None:
        inhibitor = MockSleepInhibitor()
        self.assertFalse(inhibitor.is_active())
        self.assertEqual(inhibitor.get_inhibition_type(), "mock")

        self.assertTrue(inhibitor.acquire(reason="Test Active", keep_screen_on=True))
        self.assertTrue(inhibitor.is_active())
        self.assertEqual(inhibitor._reason, "Test Active")
        self.assertTrue(inhibitor._keep_screen_on)

        self.assertTrue(inhibitor.release())
        self.assertFalse(inhibitor.is_active())

    def test_windows_sleep_inhibitor(self) -> None:
        with mock.patch("ctypes.windll", create=True) as mock_windll:
            mock_windll.kernel32.SetThreadExecutionState.return_value = 1

            win_inh = WindowsSleepInhibitor()
            self.assertEqual(win_inh.get_inhibition_type(), "windows")
            self.assertFalse(win_inh.is_active())

            # Normal acquire (no screen)
            self.assertTrue(win_inh.acquire(reason="Windows task", keep_screen_on=False))
            self.assertTrue(win_inh.is_active())
            expected_flags = (
                WindowsSleepInhibitor.ES_CONTINUOUS
                | WindowsSleepInhibitor.ES_SYSTEM_REQUIRED
                | WindowsSleepInhibitor.ES_AWAYMODE_REQUIRED
            )
            mock_windll.kernel32.SetThreadExecutionState.assert_called_with(expected_flags)

            # Acquire with keep_screen_on
            self.assertTrue(win_inh.acquire(reason="Screen task", keep_screen_on=True))
            mock_windll.kernel32.SetThreadExecutionState.assert_called_with(
                expected_flags | WindowsSleepInhibitor.ES_DISPLAY_REQUIRED
            )

            # Release
            self.assertTrue(win_inh.release())
            mock_windll.kernel32.SetThreadExecutionState.assert_called_with(
                WindowsSleepInhibitor.ES_CONTINUOUS
            )
            self.assertFalse(win_inh.is_active())

            # Failure when Windows API returns 0
            mock_windll.kernel32.SetThreadExecutionState.return_value = 0
            self.assertFalse(win_inh.acquire())

    def test_windows_sleep_inhibitor_non_windows_fallback(self) -> None:
        """When not on Windows and ctypes has no windll, acquire returns False gracefully."""
        with mock.patch("sys.platform", "linux"):
            win_inh = WindowsSleepInhibitor()
            self.assertFalse(win_inh.acquire())

    def test_linux_sleep_inhibitor_dbus(self) -> None:
        with mock.patch.dict("sys.modules", {"dbus": mock.MagicMock()}):
            import dbus

            mock_bus = mock.MagicMock()
            dbus.SystemBus.return_value = mock_bus
            mock_proxy = mock.MagicMock()
            mock_bus.get_object.return_value = mock_proxy
            mock_manager = mock.MagicMock()
            dbus.Interface.return_value = mock_manager

            fake_fd = mock.MagicMock()
            fake_fd.take.return_value = 42
            mock_manager.Inhibit.return_value = fake_fd

            with mock.patch("os.close") as mock_close:
                lin_inh = LinuxSleepInhibitor()
                self.assertEqual(lin_inh.get_inhibition_type(), "linux")
                self.assertFalse(lin_inh.is_active())

                self.assertTrue(lin_inh.acquire(reason="Linux Task", keep_screen_on=False))
                self.assertTrue(lin_inh.is_active())
                mock_manager.Inhibit.assert_called_with(
                    "idle:sleep", "agentkit-guard", "Linux Task", "block"
                )

                self.assertTrue(lin_inh.release())
                mock_close.assert_called_once_with(42)
                self.assertFalse(lin_inh.is_active())

                # Test with keep_screen_on=True
                self.assertTrue(lin_inh.acquire(reason="Screen Task", keep_screen_on=True))
                mock_manager.Inhibit.assert_called_with(
                    "idle:sleep:handle-lid-switch", "agentkit-guard", "Screen Task", "block"
                )
                lin_inh.release()

    def test_linux_sleep_inhibitor_systemd_fallback(self) -> None:
        with mock.patch.dict("sys.modules", {"dbus": None}):
            with mock.patch("subprocess.Popen") as mock_popen, mock.patch(
                "shutil.which", return_value="/usr/bin/systemd-inhibit"
            ):
                mock_proc = mock.MagicMock()
                mock_proc.poll.return_value = None
                mock_popen.return_value = mock_proc

                lin_inh = LinuxSleepInhibitor()
                self.assertTrue(lin_inh.acquire(reason="Fallback active"))
                self.assertTrue(lin_inh.is_active())
                mock_popen.assert_called_once()
                cmd = mock_popen.call_args[0][0]
                self.assertIn("systemd-inhibit", cmd[0])
                self.assertIn("--what=idle:sleep", cmd)
                self.assertIn("--who=agentkit-guard", cmd)
                self.assertIn("--why=Fallback active", cmd)

                self.assertTrue(lin_inh.release())
                mock_proc.terminate.assert_called_once()
                self.assertFalse(lin_inh.is_active())

    def test_macos_sleep_inhibitor(self) -> None:
        with mock.patch("subprocess.Popen") as mock_popen, mock.patch(
            "shutil.which", return_value="/usr/bin/caffeinate"
        ):
            mock_proc = mock.MagicMock()
            mock_proc.poll.return_value = None
            mock_popen.return_value = mock_proc

            mac_inh = MacOSSleepInhibitor()
            self.assertEqual(mac_inh.get_inhibition_type(), "macos")
            self.assertFalse(mac_inh.is_active())

            self.assertTrue(mac_inh.acquire(keep_screen_on=False))
            self.assertTrue(mac_inh.is_active())
            mock_popen.assert_called_once_with(
                ["/usr/bin/caffeinate", "-i", "-s"],
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )

            self.assertTrue(mac_inh.release())
            mock_proc.terminate.assert_called_once()
            self.assertFalse(mac_inh.is_active())

            # With screen on
            mock_popen.reset_mock()
            self.assertTrue(mac_inh.acquire(keep_screen_on=True))
            mock_popen.assert_called_once_with(
                ["/usr/bin/caffeinate", "-i", "-s", "-d"],
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )
            mac_inh.release()

    def test_get_system_inhibitor_platform_routing(self) -> None:
        with mock.patch("sys.platform", "win32"):
            inh = get_system_inhibitor()
            self.assertIsInstance(inh, WindowsSleepInhibitor)

        with mock.patch("sys.platform", "linux"):
            inh = get_system_inhibitor()
            self.assertIsInstance(inh, LinuxSleepInhibitor)

        with mock.patch("sys.platform", "darwin"):
            inh = get_system_inhibitor()
            self.assertIsInstance(inh, MacOSSleepInhibitor)

        with mock.patch("sys.platform", "unknown_os"):
            inh = get_system_inhibitor()
            self.assertIsInstance(inh, MockSleepInhibitor)


class TestGuardEngine(unittest.TestCase):
    """Test state engine transitions, cooldown handling, journal persistence, and override."""

    def setUp(self) -> None:
        self.tmp_dir = tempfile.TemporaryDirectory()
        self.history_path = Path(self.tmp_dir.name) / "test_history.jsonl"
        self.mock_inhibitor = MockSleepInhibitor()
        self.engine = GuardEngine(
            inhibitor=self.mock_inhibitor,
            cooldown_seconds=45.0,
            history_path=self.history_path,
        )

    def tearDown(self) -> None:
        self.engine.stop()
        self.tmp_dir.cleanup()

    def test_initial_state_idle(self) -> None:
        status = self.engine.get_status()
        self.assertEqual(status["state"], "idle")
        self.assertFalse(status["inhibited"])
        self.assertEqual(status["active_count"], 0)
        self.assertEqual(status["sessions"], [])
        self.assertEqual(status["cooldown_remaining"], 0.0)

    def test_multi_session_lifecycle_and_cooldown_cancel(self) -> None:
        # First session goes active
        s1_status = self.engine.notify("s1", "active", agent="worker-a", task="Task 1")
        self.assertEqual(s1_status["state"], "active")
        self.assertTrue(s1_status["inhibited"])
        self.assertTrue(self.mock_inhibitor.is_active())
        self.assertEqual(s1_status["active_count"], 1)

        # Second session goes active
        s2_status = self.engine.notify("s2", "active", agent="worker-b", task="Task 2")
        self.assertEqual(s2_status["state"], "active")
        self.assertTrue(s2_status["inhibited"])
        self.assertEqual(s2_status["active_count"], 2)

        # First session goes idle: stays ACTIVE because s2 is still running
        s1_idle_status = self.engine.notify("s1", "idle")
        self.assertEqual(s1_idle_status["state"], "active")
        self.assertTrue(s1_idle_status["inhibited"])
        self.assertEqual(s1_idle_status["active_count"], 1)

        # Second session goes idle: transitions to COOLDOWN
        s2_idle_status = self.engine.notify("s2", "idle")
        self.assertEqual(s2_idle_status["state"], "cooldown")
        self.assertTrue(s2_idle_status["inhibited"])  # Inhibitor NOT flapped off
        self.assertEqual(s2_idle_status["active_count"], 0)
        self.assertGreater(s2_idle_status["cooldown_remaining"], 0.0)

        # New active notification during COOLDOWN: returns to ACTIVE without flapping inhibitor
        s3_status = self.engine.notify("s3", "active", agent="worker-c", task="Task 3")
        self.assertEqual(s3_status["state"], "active")
        self.assertTrue(s3_status["inhibited"])
        self.assertEqual(s3_status["active_count"], 1)
        self.assertEqual(s3_status["cooldown_remaining"], 0.0)

    def test_cooldown_expiration_and_history_persistence(self) -> None:
        # Create fast engine with 0.05s cooldown
        fast_engine = GuardEngine(
            inhibitor=self.mock_inhibitor,
            cooldown_seconds=0.05,
            history_path=self.history_path,
        )
        try:
            fast_engine.notify("s_test", "active", agent="agent-x", task="Run task", pid=1234)
            self.assertEqual(fast_engine.state, ActivityState.ACTIVE)
            self.assertTrue(self.mock_inhibitor.is_active())

            fast_engine.notify("s_test", "idle")
            self.assertEqual(fast_engine.state, ActivityState.COOLDOWN)
            self.assertTrue(self.mock_inhibitor.is_active())

            # Wait for cooldown expiration
            time.sleep(0.12)

            self.assertEqual(fast_engine.state, ActivityState.IDLE)
            self.assertFalse(self.mock_inhibitor.is_active())

            # Verify history was flushed to JSONL journal
            history = fast_engine.get_history()
            self.assertGreaterEqual(len(history), 1)
            entry = history[0]
            self.assertEqual(entry["session_id"], "s_test")
            self.assertEqual(entry["agent_name"], "agent-x")
            self.assertEqual(entry["task"], "Run task")
            self.assertEqual(entry["pid"], 1234)
            self.assertIn("duration_seconds", entry)
        finally:
            fast_engine.stop()

    def test_override_toggle(self) -> None:
        # Override enabled
        status = self.engine.set_override(True)
        self.assertEqual(status["state"], "override")
        self.assertTrue(status["inhibited"])
        self.assertTrue(self.mock_inhibitor.is_active())

        # Override disabled with no active sessions
        status = self.engine.set_override(False)
        self.assertEqual(status["state"], "idle")
        self.assertFalse(status["inhibited"])
        self.assertFalse(self.mock_inhibitor.is_active())

        # Override enabled while active session exists, then disabled
        self.engine.notify("s_persist", "active")
        self.assertEqual(self.engine.state, ActivityState.ACTIVE)

        self.engine.set_override(True)
        self.assertEqual(self.engine.state, ActivityState.OVERRIDE)

        # Clearing override returns to ACTIVE because s_persist is still active
        status = self.engine.set_override(False)
        self.assertEqual(status["state"], "active")
        self.assertTrue(status["inhibited"])

    def test_invalid_state_raises_value_error(self) -> None:
        with self.assertRaises(ValueError):
            self.engine.notify("s1", "invalid_state")


class TestGuardServer(unittest.TestCase):
    """Test HTTP server endpoints and client helper functions on ephemeral port."""

    def setUp(self) -> None:
        self.tmp_dir = tempfile.TemporaryDirectory()
        self.history_path = Path(self.tmp_dir.name) / "test_history.jsonl"
        self.mock_inhibitor = MockSleepInhibitor()
        self.engine = GuardEngine(
            inhibitor=self.mock_inhibitor,
            cooldown_seconds=10.0,
            history_path=self.history_path,
        )
        self.server = GuardServer(engine=self.engine, host="127.0.0.1", port=0)
        self.server.start_background()
        self.port = self.server.port
        self.assertGreater(self.port, 0)

    def tearDown(self) -> None:
        self.server.stop()
        self.tmp_dir.cleanup()

    def test_client_server_lifecycle(self) -> None:
        # 1. Fetch initial status
        initial_status = fetch_status(port=self.port)
        self.assertIsNotNone(initial_status)
        self.assertEqual(initial_status["state"], "idle")
        self.assertFalse(initial_status["inhibited"])

        # 2. Send active notification
        ok = send_notification(
            session_id="srv-s1",
            state="active",
            agent="worker-test",
            task="Server test task",
            port=self.port,
        )
        self.assertTrue(ok)

        # 3. Verify status transitioned to active
        active_status = fetch_status(port=self.port)
        self.assertIsNotNone(active_status)
        self.assertEqual(active_status["state"], "active")
        self.assertTrue(active_status["inhibited"])
        self.assertEqual(active_status["active_count"], 1)

        # 4. Send idle notification
        ok = send_notification(
            session_id="srv-s1",
            state="idle",
            port=self.port,
        )
        self.assertTrue(ok)

        # 5. Verify status transitioned to cooldown
        cooldown_status = fetch_status(port=self.port)
        self.assertIsNotNone(cooldown_status)
        self.assertEqual(cooldown_status["state"], "cooldown")
        self.assertTrue(cooldown_status["inhibited"])

        # 6. Test override endpoint
        ok = send_override(enabled=True, port=self.port)
        self.assertTrue(ok)
        override_status = fetch_status(port=self.port)
        self.assertEqual(override_status["state"], "override")

        ok = send_override(enabled=False, port=self.port)
        self.assertTrue(ok)
        cleared_status = fetch_status(port=self.port)
        self.assertIn(cleared_status["state"], ("idle", "cooldown"))

    def test_server_get_endpoints_and_history(self) -> None:
        import urllib.request
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))

        # 1. Test GET /v1/notify
        url_notify = f"http://127.0.0.1:{self.port}/v1/notify?session=get-s1&state=active&agent=get-worker&task=get-task"
        with opener.open(url_notify) as resp:
            self.assertEqual(resp.status, 200)
            data = json.loads(resp.read().decode("utf-8"))
            self.assertEqual(data["state"], "active")

        # 2. Test GET /status
        url_status = f"http://127.0.0.1:{self.port}/status"
        with opener.open(url_status) as resp:
            self.assertEqual(resp.status, 200)
            data = json.loads(resp.read().decode("utf-8"))
            self.assertEqual(data["active_count"], 1)

        # 3. Test GET /v1/notify idle
        url_idle = f"http://127.0.0.1:{self.port}/notify?session=get-s1&state=idle"
        with opener.open(url_idle) as resp:
            self.assertEqual(resp.status, 200)
            data = json.loads(resp.read().decode("utf-8"))
            self.assertEqual(data["state"], "cooldown")

        # 4. Test GET /v1/history
        url_hist = f"http://127.0.0.1:{self.port}/v1/history"
        with opener.open(url_hist) as resp:
            self.assertEqual(resp.status, 200)
            data = json.loads(resp.read().decode("utf-8"))
            self.assertIsInstance(data, list)

        # 5. Test 404 for unknown endpoint
        url_404 = f"http://127.0.0.1:{self.port}/unknown"
        with self.assertRaises(urllib.error.HTTPError) as ctx:
            opener.open(url_404)
        self.assertEqual(ctx.exception.code, 404)
        ctx.exception.close()

        # 6. Test 400 for missing notify params
        url_bad = f"http://127.0.0.1:{self.port}/v1/notify"
        with self.assertRaises(urllib.error.HTTPError) as ctx:
            opener.open(url_bad)
        self.assertEqual(ctx.exception.code, 400)
        ctx.exception.close()

    def test_default_history_path_resolution(self) -> None:
        with mock.patch("sys.platform", "win32"), mock.patch.dict("os.environ", {"LOCALAPPDATA": "C:\\AppData\\Local"}):
            p = get_default_history_path()
            self.assertIn("agentkit", str(p))
            self.assertIn("guard_history.jsonl", str(p))

        with mock.patch("sys.platform", "linux"), mock.patch.dict("os.environ", {"XDG_STATE_HOME": "/tmp/state"}):
            p = get_default_history_path()
            self.assertEqual(str(p), "/tmp/state/agentkit/guard_history.jsonl")

    def test_fetch_status_offline_server_returns_none(self) -> None:
        # Connecting to unused port returns None gracefully
        res = fetch_status(port=1)
        self.assertIsNone(res)

        res_send = send_notification("s", "active", port=1)
        self.assertFalse(res_send)


class TestGuardAutostart(unittest.TestCase):
    """Test platform autostart configuration for Linux and Windows."""

    def test_autostart_path_linux(self) -> None:
        from project.tools.guard.autostart import get_autostart_path
        with mock.patch("sys.platform", "linux"), mock.patch.dict("os.environ", {"XDG_CONFIG_HOME": "/custom/config"}):
            p = get_autostart_path()
            self.assertEqual(str(p), "/custom/config/autostart/agentkit-guard.desktop")

    def test_autostart_path_windows(self) -> None:
        from project.tools.guard.autostart import get_autostart_path
        with mock.patch("sys.platform", "win32"), mock.patch.dict("os.environ", {"APPDATA": "C:\\Users\\User\\AppData\\Roaming"}):
            p = get_autostart_path()
            self.assertIn("agentkit-guard.cmd", str(p))

    def test_enable_and_disable_autostart_linux(self) -> None:
        from project.tools.guard.autostart import enable_autostart, disable_autostart, is_autostart_enabled
        with tempfile.TemporaryDirectory() as tmp_dir:
            with mock.patch("sys.platform", "linux"), mock.patch.dict("os.environ", {"XDG_CONFIG_HOME": tmp_dir}):
                self.assertFalse(is_autostart_enabled())
                ok, msg = enable_autostart(agentkit_bin="/usr/bin/agentkit")
                self.assertTrue(ok)
                self.assertTrue(is_autostart_enabled())

                # Verify desktop file content
                desktop_path = Path(tmp_dir) / "autostart" / "agentkit-guard.desktop"
                self.assertTrue(desktop_path.exists())
                content = desktop_path.read_text(encoding="utf-8")
                self.assertIn("Exec=/usr/bin/agentkit guard tray", content)

                # Disable
                ok, msg = disable_autostart()
                self.assertTrue(ok)
                self.assertFalse(is_autostart_enabled())


class TestGuardCLI(unittest.TestCase):
    """Test agentkit guard command line invocations."""

    def test_guard_status_offline(self) -> None:
        agentkit_bin = REPO_ROOT / "bin" / "agentkit"
        res = subprocess.run(
            [sys.executable, str(agentkit_bin), "guard", "status", "--json"],
            capture_output=True,
            text=True,
        )
        self.assertEqual(res.returncode, 1)
        data = json.loads(res.stdout)
        self.assertFalse(data.get("active"))
        self.assertIn("error", data)

    def test_guard_notify_offline_does_not_fail(self) -> None:
        agentkit_bin = REPO_ROOT / "bin" / "agentkit"
        res = subprocess.run(
            [sys.executable, str(agentkit_bin), "guard", "notify", "active", "--quiet"],
            capture_output=True,
            text=True,
        )
        self.assertEqual(res.returncode, 0)

    def test_guard_autostart_status(self) -> None:
        agentkit_bin = REPO_ROOT / "bin" / "agentkit"
        res = subprocess.run(
            [sys.executable, str(agentkit_bin), "guard", "autostart", "--status"],
            capture_output=True,
            text=True,
        )
        self.assertEqual(res.returncode, 0)
        self.assertIn("AgentKit Guard Autostart:", res.stdout)


if __name__ == "__main__":
    unittest.main()
