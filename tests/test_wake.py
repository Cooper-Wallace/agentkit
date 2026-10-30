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

from project.tools.wake.engine import (
    ActivityState,
    GuardEngine,
    SessionInfo,
    WakeEngine,
    get_default_history_path,
)
from project.tools.wake.inhibitor import (
    BaseSleepInhibitor,
    LinuxSleepInhibitor,
    MacOSSleepInhibitor,
    MockSleepInhibitor,
    WindowsSleepInhibitor,
    get_system_inhibitor,
)
from project.tools.wake.server import (
    GuardServer,
    WakeServer,
    fetch_status,
    send_notification,
    send_override,
)
from project.tools.wake.tray import GuardTrayApp, WakeTrayApp


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
                    "idle:sleep", "agentkit-wake", "Linux Task", "block"
                )

                self.assertTrue(lin_inh.release())
                mock_close.assert_called_once_with(42)
                self.assertFalse(lin_inh.is_active())

                # Test with keep_screen_on=True
                self.assertTrue(lin_inh.acquire(reason="Screen Task", keep_screen_on=True))
                mock_manager.Inhibit.assert_called_with(
                    "idle:sleep:handle-lid-switch", "agentkit-wake", "Screen Task", "block"
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
                self.assertIn("--who=agentkit-wake", cmd)
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


class TestWakeEngine(unittest.TestCase):
    """Test state engine transitions, cooldown handling, journal persistence, and override."""

    def setUp(self) -> None:
        self.tmp_dir = tempfile.TemporaryDirectory()
        self.history_path = Path(self.tmp_dir.name) / "test_history.jsonl"
        self.mock_inhibitor = MockSleepInhibitor()
        self.engine = WakeEngine(
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
        fast_engine = WakeEngine(
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


class TestWakeServer(unittest.TestCase):
    """Test HTTP server endpoints and client helper functions on ephemeral port."""

    def setUp(self) -> None:
        self.tmp_dir = tempfile.TemporaryDirectory()
        self.history_path = Path(self.tmp_dir.name) / "test_history.jsonl"
        self.mock_inhibitor = MockSleepInhibitor()
        self.engine = WakeEngine(
            inhibitor=self.mock_inhibitor,
            cooldown_seconds=10.0,
            history_path=self.history_path,
        )
        self.server = WakeServer(engine=self.engine, host="127.0.0.1", port=0)
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
            self.assertIn("wake_history.jsonl", str(p))

        with mock.patch("sys.platform", "linux"), mock.patch.dict("os.environ", {"XDG_STATE_HOME": "/tmp/state"}):
            p = get_default_history_path()
            self.assertEqual(str(p), "/tmp/state/agentkit/wake_history.jsonl")

    def test_fetch_status_offline_server_returns_none(self) -> None:
        # Connecting to unused port returns None gracefully
        res = fetch_status(port=1)
        self.assertIsNone(res)

        res_send = send_notification("s", "active", port=1)
        self.assertFalse(res_send)


class TestWakeAutostart(unittest.TestCase):
    """Test platform autostart configuration for Linux and Windows."""

    def test_autostart_path_linux(self) -> None:
        from project.tools.wake.autostart import get_autostart_path
        with mock.patch("sys.platform", "linux"), mock.patch.dict("os.environ", {"XDG_CONFIG_HOME": "/custom/config"}):
            p = get_autostart_path()
            self.assertEqual(str(p), "/custom/config/autostart/agentkit-wake.desktop")

    def test_autostart_path_windows(self) -> None:
        from project.tools.wake.autostart import get_autostart_path
        with mock.patch("sys.platform", "win32"), mock.patch.dict("os.environ", {"APPDATA": "C:\\Users\\User\\AppData\\Roaming"}):
            p = get_autostart_path()
            self.assertIn("agentkit-wake.cmd", str(p))

    def test_enable_and_disable_autostart_linux(self) -> None:
        from project.tools.wake.autostart import enable_autostart, disable_autostart, is_autostart_enabled
        with tempfile.TemporaryDirectory() as tmp_dir:
            with mock.patch("sys.platform", "linux"), mock.patch.dict("os.environ", {"XDG_CONFIG_HOME": tmp_dir}):
                self.assertFalse(is_autostart_enabled())
                ok, msg = enable_autostart(agentkit_bin="/usr/bin/agentkit")
                self.assertTrue(ok)
                self.assertTrue(is_autostart_enabled())

                # Verify desktop file content
                desktop_path = Path(tmp_dir) / "autostart" / "agentkit-wake.desktop"
                self.assertTrue(desktop_path.exists())
                content = desktop_path.read_text(encoding="utf-8")
                self.assertIn("Exec=/usr/bin/agentkit wake tray", content)
                self.assertIn("Name=AgentKit WakeLock", content)

                # Disable
                ok, msg = disable_autostart()
                self.assertTrue(ok)
                self.assertFalse(is_autostart_enabled())


class TestWakeCLI(unittest.TestCase):
    """Test agentkit wake command line invocations."""

    def test_wake_status_offline(self) -> None:
        agentkit_bin = REPO_ROOT / "bin" / "agentkit"
        for subcmd in ("wake", "wakelock"):
            res = subprocess.run(
                [sys.executable, str(agentkit_bin), subcmd, "status", "--json"],
                capture_output=True,
                text=True,
            )
            self.assertEqual(res.returncode, 1)
            data = json.loads(res.stdout)
            self.assertFalse(data.get("active"))
            self.assertIn("error", data)

    def test_wake_notify_offline_does_not_fail(self) -> None:
        agentkit_bin = REPO_ROOT / "bin" / "agentkit"
        res = subprocess.run(
            [sys.executable, str(agentkit_bin), "wake", "notify", "active", "--quiet"],
            capture_output=True,
            text=True,
        )
        self.assertEqual(res.returncode, 0)

    def test_wake_autostart_status(self) -> None:
        agentkit_bin = REPO_ROOT / "bin" / "agentkit"
        res = subprocess.run(
            [sys.executable, str(agentkit_bin), "wake", "autostart", "--status"],
            capture_output=True,
            text=True,
        )
        self.assertEqual(res.returncode, 0)
        self.assertIn("AgentKit Wake Autostart:", res.stdout)

    def test_agentkit_apply_deploys_wake_lock_and_autostart(self) -> None:
        agentkit_bin = REPO_ROOT / "bin" / "agentkit"
        with tempfile.TemporaryDirectory() as tmp_dir:
            tmp_path = Path(tmp_dir)
            settings_path = tmp_path / "settings.json"
            plugins_dir = tmp_path / "plugins"
            gemini_dir = tmp_path / ".gemini"
            config_home = tmp_path / "config"

            res = subprocess.run(
                [
                    sys.executable,
                    str(agentkit_bin),
                    "apply",
                    "--settings-path",
                    str(settings_path),
                    "--plugins-dir",
                    str(plugins_dir),
                    "--gemini-dir",
                    str(gemini_dir),
                    "--no-mcp",
                ],
                capture_output=True,
                text=True,
                env={**os.environ, "XDG_CONFIG_HOME": str(config_home)},
            )
            self.assertEqual(res.returncode, 0, f"agentkit apply failed: {res.stderr}")

            # Verify wake-lock plugin was deployed
            wake_plugin_dir = plugins_dir / "wake-lock"
            self.assertTrue(wake_plugin_dir.exists(), "wake-lock plugin directory was not deployed")
            self.assertTrue((wake_plugin_dir / "plugin.json").exists(), "wake-lock plugin.json missing")
            self.assertTrue((wake_plugin_dir / "hooks.json").exists(), "wake-lock hooks.json missing")
            self.assertTrue(
                (wake_plugin_dir / "scripts" / "notify-wake.py").exists(),
                "wake-lock notify-wake.py missing",
            )

            # Verify autostart desktop entry was created
            autostart_entry = config_home / "autostart" / "agentkit-wake.desktop"
            self.assertTrue(autostart_entry.exists(), "agentkit-wake.desktop autostart was not created")
            content = autostart_entry.read_text(encoding="utf-8")
            self.assertIn("agentkit wake tray", content)


class TestWakeLockPluginAndTray(unittest.TestCase):
    """Test wake-lock plugin schema, notify-wake hook responses, and WakeTrayApp embedded server lifecycle."""

    def test_wake_lock_hooks_schema(self) -> None:
        """Verify harness/plugins/wake-lock/hooks.json conforms to the Antigravity hook schema."""
        hooks_path = REPO_ROOT / "harness" / "plugins" / "wake-lock" / "hooks.json"
        self.assertTrue(hooks_path.exists(), "wake-lock hooks.json does not exist")
        data = json.loads(hooks_path.read_text(encoding="utf-8"))
        self.assertIn("wake-lock-activity", data)
        self.assertNotIn("UserPrompt", data)
        activity = data["wake-lock-activity"]
        self.assertTrue(activity.get("enabled"))
        self.assertIn("PreInvocation", activity)
        self.assertIsInstance(activity["PreInvocation"], list)
        self.assertIn("PreToolUse", activity)
        self.assertIsInstance(activity["PreToolUse"], list)
        self.assertIn("Stop", activity)
        self.assertIsInstance(activity["Stop"], list)
        self.assertNotIn("UserPrompt", activity)

    def test_notify_wake_dry_run(self) -> None:
        """Verify notify-wake.py --dry-run outputs valid JSON with status: dry-run."""
        script_path = REPO_ROOT / "harness" / "plugins" / "wake-lock" / "scripts" / "notify-wake.py"
        res = subprocess.run(
            [sys.executable, str(script_path), "--dry-run"],
            capture_output=True,
            text=True,
        )
        self.assertEqual(res.returncode, 0)
        data = json.loads(res.stdout)
        self.assertEqual(data.get("status"), "dry-run")
        self.assertIn("notify-wake.py is operational", data.get("message", ""))

    def test_notify_wake_pre_tool_use(self) -> None:
        """Verify notify-wake.py reading PreToolUse payload posts active state and returns allow decision."""
        script_path = REPO_ROOT / "harness" / "plugins" / "wake-lock" / "scripts" / "notify-wake.py"
        mock_inh = MockSleepInhibitor()
        engine = WakeEngine(inhibitor=mock_inh, cooldown_seconds=10.0)
        server = WakeServer(engine=engine, host="127.0.0.1", port=0)
        server.start_background()
        port = server.port

        try:
            pre_tool_payload = {
                "conversationId": "conv-abc",
                "toolCall": {"name": "run_command"},
            }
            res_pre = subprocess.run(
                [sys.executable, str(script_path), "active", "--port", str(port)],
                input=json.dumps(pre_tool_payload),
                capture_output=True,
                text=True,
            )
            self.assertEqual(res_pre.returncode, 0)
            decision_pre = json.loads(res_pre.stdout)
            self.assertEqual(decision_pre.get("decision"), "allow")

            status_active = fetch_status(port=port)
            self.assertIsNotNone(status_active)
            self.assertEqual(status_active["state"], "active")
            self.assertTrue(status_active["inhibited"])
            self.assertEqual(status_active["active_count"], 1)
            sessions = status_active.get("sessions", [])
            self.assertTrue(any(s.get("session_id") == "conv-abc" for s in sessions))
        finally:
            server.stop()

    def test_notify_wake_stop(self) -> None:
        """Verify notify-wake.py reading Stop payload posts idle state and returns stop decision."""
        script_path = REPO_ROOT / "harness" / "plugins" / "wake-lock" / "scripts" / "notify-wake.py"
        mock_inh = MockSleepInhibitor()
        engine = WakeEngine(inhibitor=mock_inh, cooldown_seconds=10.0)
        server = WakeServer(engine=engine, host="127.0.0.1", port=0)
        server.start_background()
        port = server.port

        try:
            # First activate session
            engine.notify("conv-abc", "active")
            st_init = fetch_status(port=port)
            self.assertEqual(st_init["state"], "active")

            stop_payload = {
                "conversationId": "conv-abc",
                "terminationReason": "COMPLETE",
            }
            res_stop = subprocess.run(
                [sys.executable, str(script_path), "idle", "--port", str(port)],
                input=json.dumps(stop_payload),
                capture_output=True,
                text=True,
            )
            self.assertEqual(res_stop.returncode, 0)
            decision_stop = json.loads(res_stop.stdout)
            self.assertEqual(decision_stop.get("decision"), "stop")

            status_stop = fetch_status(port=port)
            self.assertIsNotNone(status_stop)
            self.assertEqual(status_stop["state"], "cooldown")
            self.assertTrue(status_stop["inhibited"])
            self.assertEqual(status_stop["active_count"], 0)
        finally:
            server.stop()

    def test_wake_tray_app_embedded_server_lifecycle(self) -> None:
        """Verify WakeTrayApp(embed_engine=True, port=0) starts embedded server and stops cleanly."""
        app = WakeTrayApp(embed_engine=True, port=0)
        try:
            self.assertIsNotNone(app.server)
            self.assertGreater(app.port, 0)
            status = fetch_status(port=app.port)
            self.assertIsNotNone(status)
            self.assertEqual(status.get("state"), "idle")
            self.assertFalse(status.get("inhibited"))
        finally:
            app.stop()
            time.sleep(0.1)
            self.assertIsNone(fetch_status(port=app.port))


# Backward compatibility aliases
TestGuardEngine = TestWakeEngine
TestGuardServer = TestWakeServer
TestGuardAutostart = TestWakeAutostart
TestGuardCLI = TestWakeCLI


if __name__ == "__main__":
    unittest.main()
