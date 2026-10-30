"""
Cross-platform PyQt6 system tray application, status indicators, and flyout dashboard.

Provides real-time visualization of agentkit sleep inhibition status, active agent
sessions, power metrics, and manual stay-awake controls via Qt system tray.
"""

from __future__ import annotations

import datetime
import json
import logging
import os
from pathlib import Path
import sys
import time
from typing import Any, Dict, List, Optional
import urllib.request

try:
    import psutil
    HAS_PSUTIL = True
except ImportError:
    HAS_PSUTIL = False

try:
    from PyQt6 import QtCore, QtGui, QtWidgets
    HAS_PYQT6 = True
except ImportError:
    HAS_PYQT6 = False

from .engine import ActivityState, GuardEngine, WakeEngine, get_default_history_path
from .icons import get_status_icon, get_status_pixmap
from .inhibitor import get_system_inhibitor
from .server import GuardServer, WakeServer, fetch_status, send_override

logger = logging.getLogger("agentkit.wake.tray")


def format_duration(seconds: float) -> str:
    """Format duration in seconds to human-readable string (e.g. '01:24', '1h 05m 12s')."""
    total_secs = int(max(0, round(seconds)))
    hours = total_secs // 3600
    minutes = (total_secs % 3600) // 60
    secs = total_secs % 60
    if hours > 0:
        return f"{hours}h {minutes:02d}m {secs:02d}s"
    elif minutes > 0:
        return f"{minutes:02d}:{secs:02d}"
    else:
        return f"00:{secs:02d}"


def get_platform_lock_name(engine: Optional[WakeEngine] = None) -> str:
    """Return descriptive name of underlying platform sleep lock mechanism."""
    if engine is not None and getattr(engine, "inhibitor", None) is not None:
        try:
            return engine.inhibitor.get_inhibition_type()
        except Exception:
            pass
    if sys.platform == "win32":
        return "Windows SetThreadExecutionState"
    elif sys.platform == "darwin":
        return "macOS caffeinate"
    else:
        return "Linux systemd login1 D-Bus"


def fetch_history_entries(port: int = 18820, limit: int = 100) -> List[Dict[str, Any]]:
    """Fetch recent history journal entries from loopback server or local disk."""
    url = f"http://127.0.0.1:{port}/v1/history?limit={limit}"
    req = urllib.request.Request(url, method="GET")
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    try:
        with opener.open(req, timeout=1.0) as resp:
            if resp.status == 200:
                body = resp.read().decode("utf-8")
                return json.loads(body)
    except Exception:
        pass

    # Disk fallback
    hist_path = get_default_history_path()
    if hist_path.exists():
        entries: List[Dict[str, Any]] = []
        try:
            with open(hist_path, "r", encoding="utf-8") as f:
                for line in f:
                    line = line.strip()
                    if line:
                        try:
                            entries.append(json.loads(line))
                        except json.JSONDecodeError:
                            continue
            return entries[-limit:]
        except Exception:
            return []
    return []


def calculate_today_prevented_seconds(
    history_entries: List[Dict[str, Any]], active_sessions: List[Dict[str, Any]]
) -> float:
    """Calculate cumulative sleep inhibition duration in seconds for today."""
    now = time.time()
    midnight = datetime.datetime.now().replace(
        hour=0, minute=0, second=0, microsecond=0
    ).timestamp()
    total = 0.0

    for entry in history_entries:
        st = entry.get("start_time", 0.0)
        dur = float(entry.get("duration_seconds", 0.0))
        if st >= midnight:
            total += dur

    for s in active_sessions:
        st = float(s.get("start_time", now))
        if st >= midnight:
            total += max(0.0, now - st)
        else:
            total += max(0.0, now - midnight)

    return total


class WakeDashboardDialog(QtWidgets.QDialog):
    """
    Modern dark-mode flyout dashboard for AgentKit WakeLock.

    Displays real-time status banners, active agent session table, battery/power stats,
    cumulative prevented time, and quick toggle actions.
    """

    def __init__(
        self,
        tray_app: WakeTrayApp,
        parent: Optional[QtWidgets.QWidget] = None,
    ) -> None:
        super().__init__(parent)
        self.tray_app = tray_app
        self.setWindowTitle("AgentKit WakeLock Dashboard")
        self.setMinimumSize(580, 480)
        self.resize(620, 520)

        self._setup_style()
        self._init_ui()

        # Dialog auto-refresh timer (1000ms while visible)
        self.refresh_timer = QtCore.QTimer(self)
        self.refresh_timer.setInterval(1000)
        self.refresh_timer.timeout.connect(self._on_timer_refresh)

    def _setup_style(self) -> None:
        """Apply sleek dark-mode styling."""
        self.setStyleSheet("""
            QDialog {
                background-color: #161c26;
                color: #f0f4f8;
                font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            }
            QLabel {
                color: #f0f4f8;
            }
            QFrame#card {
                background-color: #1e2638;
                border: 1px solid #2a3850;
                border-radius: 8px;
                padding: 10px;
            }
            QTableWidget {
                background-color: #1a2234;
                color: #f0f4f8;
                border: 1px solid #2a3850;
                border-radius: 6px;
                gridline-color: #243046;
                selection-background-color: #0e7490;
                selection-color: #ffffff;
            }
            QHeaderView::section {
                background-color: #243046;
                color: #94a3b8;
                font-weight: bold;
                padding: 6px;
                border: none;
            }
            QPushButton {
                background-color: #1e293b;
                color: #f0f4f8;
                border: 1px solid #334155;
                border-radius: 6px;
                padding: 8px 16px;
                font-weight: 500;
            }
            QPushButton:hover {
                background-color: #334155;
                border-color: #00e5ff;
            }
            QPushButton:pressed {
                background-color: #0f172a;
            }
            QPushButton#accentBtn {
                background-color: #0891b2;
                border: 1px solid #00e5ff;
                color: #ffffff;
                font-weight: bold;
            }
            QPushButton#accentBtn:hover {
                background-color: #06b6d4;
            }
        """)

    def _init_ui(self) -> None:
        """Construct dashboard visual sections."""
        layout = QtWidgets.QVBoxLayout(self)
        layout.setSpacing(14)
        layout.setContentsMargins(18, 18, 18, 18)

        # ----------------------------------------------------
        # Section 1: Header Banner (State Badge & Lock Status)
        # ----------------------------------------------------
        header_card = QtWidgets.QFrame()
        header_card.setObjectName("card")
        header_layout = QtWidgets.QHBoxLayout(header_card)
        header_layout.setContentsMargins(12, 10, 12, 10)

        # Left: App Title and Icon
        title_box = QtWidgets.QHBoxLayout()
        self.icon_label = QtWidgets.QLabel()
        self.icon_label.setFixedSize(36, 36)
        self.icon_label.setScaledContents(True)
        title_box.addWidget(self.icon_label)

        title_text_box = QtWidgets.QVBoxLayout()
        title_text = QtWidgets.QLabel("AgentKit WakeLock")
        title_text.setStyleSheet("font-size: 16px; font-weight: bold; color: #f0f4f8;")
        self.lock_status_label = QtWidgets.QLabel("Lock: Detecting...")
        self.lock_status_label.setStyleSheet("font-size: 11px; color: #94a3b8;")
        title_text_box.addWidget(title_text)
        title_text_box.addWidget(self.lock_status_label)
        title_box.addLayout(title_text_box)
        header_layout.addLayout(title_box)

        header_layout.addStretch()

        # Right: State Badge Pill
        self.state_badge = QtWidgets.QLabel("IDLE")
        self.state_badge.setAlignment(QtCore.Qt.AlignmentFlag.AlignCenter)
        self.state_badge.setStyleSheet(
            "font-size: 12px; font-weight: bold; padding: 6px 14px; "
            "border-radius: 12px; background-color: #334155; color: #94a3b8;"
        )
        header_layout.addWidget(self.state_badge)
        layout.addWidget(header_card)

        # ----------------------------------------------------
        # Section 2: Active Sessions Table / List
        # ----------------------------------------------------
        sessions_box = QtWidgets.QVBoxLayout()
        sessions_header = QtWidgets.QLabel("Active Agent Sessions")
        sessions_header.setStyleSheet("font-size: 13px; font-weight: bold; color: #00e5ff;")
        sessions_box.addWidget(sessions_header)

        self.table = QtWidgets.QTableWidget(0, 4)
        self.table.setHorizontalHeaderLabels(["Agent", "Session ID", "Task", "Elapsed"])
        self.table.horizontalHeader().setStretchLastSection(False)
        self.table.horizontalHeader().setSectionResizeMode(0, QtWidgets.QHeaderView.ResizeMode.ResizeToContents)
        self.table.horizontalHeader().setSectionResizeMode(1, QtWidgets.QHeaderView.ResizeMode.ResizeToContents)
        self.table.horizontalHeader().setSectionResizeMode(2, QtWidgets.QHeaderView.ResizeMode.Stretch)
        self.table.horizontalHeader().setSectionResizeMode(3, QtWidgets.QHeaderView.ResizeMode.ResizeToContents)
        self.table.setEditTriggers(QtWidgets.QAbstractItemView.EditTrigger.NoEditTriggers)
        self.table.setSelectionBehavior(QtWidgets.QAbstractItemView.SelectionBehavior.SelectRows)
        self.table.verticalHeader().setVisible(False)
        sessions_box.addWidget(self.table)

        self.empty_label = QtWidgets.QLabel("✨ No active agent prompts executing (System sleep allowed)")
        self.empty_label.setAlignment(QtCore.Qt.AlignmentFlag.AlignCenter)
        self.empty_label.setStyleSheet(
            "font-size: 13px; color: #64748b; padding: 24px; "
            "background-color: #1a2234; border: 1px dashed #2a3850; border-radius: 6px;"
        )
        sessions_box.addWidget(self.empty_label)
        layout.addLayout(sessions_box)

        # ----------------------------------------------------
        # Section 3: System Power & Stats
        # ----------------------------------------------------
        stats_layout = QtWidgets.QHBoxLayout()

        # Power/Battery Card
        power_card = QtWidgets.QFrame()
        power_card.setObjectName("card")
        power_layout = QtWidgets.QVBoxLayout(power_card)
        power_title = QtWidgets.QLabel("POWER SOURCE")
        power_title.setStyleSheet("font-size: 10px; font-weight: bold; color: #94a3b8;")
        self.power_label = QtWidgets.QLabel("Detecting...")
        self.power_label.setStyleSheet("font-size: 13px; font-weight: bold; color: #f0f4f8;")
        power_layout.addWidget(power_title)
        power_layout.addWidget(self.power_label)
        stats_layout.addWidget(power_card)

        # Today's Prevented Sleep Time Card
        stats_card = QtWidgets.QFrame()
        stats_card.setObjectName("card")
        stats_inner_layout = QtWidgets.QVBoxLayout(stats_card)
        stats_title = QtWidgets.QLabel("TODAY'S SLEEP PREVENTED")
        stats_title.setStyleSheet("font-size: 10px; font-weight: bold; color: #94a3b8;")
        self.prevented_label = QtWidgets.QLabel("0s")
        self.prevented_label.setStyleSheet("font-size: 13px; font-weight: bold; color: #00e5ff;")
        stats_inner_layout.addWidget(stats_title)
        stats_inner_layout.addWidget(self.prevented_label)
        stats_layout.addWidget(stats_card)

        layout.addLayout(stats_layout)

        # ----------------------------------------------------
        # Section 4: Quick Action Buttons
        # ----------------------------------------------------
        btn_layout = QtWidgets.QHBoxLayout()

        self.btn_override = QtWidgets.QPushButton("⚡ Force Awake")
        self.btn_override.clicked.connect(self._on_toggle_override)
        btn_layout.addWidget(self.btn_override)

        self.btn_screen = QtWidgets.QPushButton("📺 Keep Screen On: OFF")
        self.btn_screen.clicked.connect(self._on_toggle_screen)
        btn_layout.addWidget(self.btn_screen)

        btn_layout.addStretch()

        self.btn_close = QtWidgets.QPushButton("Close")
        self.btn_close.clicked.connect(self.close)
        btn_layout.addWidget(self.btn_close)

        layout.addLayout(btn_layout)

    def showEvent(self, event: QtGui.QShowEvent) -> None:
        """Start refresh timer when dialog becomes visible."""
        super().showEvent(event)
        self.refresh()
        self.refresh_timer.start()

    def hideEvent(self, event: QtGui.QHideEvent) -> None:
        """Stop refresh timer when dialog is closed/hidden."""
        super().hideEvent(event)
        self.refresh_timer.stop()

    def _on_timer_refresh(self) -> None:
        """Auto-refresh slot called every second."""
        if self.isVisible():
            self.refresh()

    def refresh(
        self,
        status: Optional[Dict[str, Any]] = None,
        engine: Optional[WakeEngine] = None,
    ) -> None:
        """Update all visual dashboard components with fresh data."""
        if status is None:
            if self.tray_app.engine is not None:
                status = self.tray_app.engine.get_status()
            else:
                status = fetch_status(port=self.tray_app.port)

        status = status or {}
        st = status.get("state", "idle").lower()
        inhibited = bool(status.get("inhibited", False))
        sessions: List[Dict[str, Any]] = status.get("sessions", [])

        # Update Icon
        self.icon_label.setPixmap(get_status_pixmap(st, size=36))

        # Update State Badge
        if st == "active":
            self.state_badge.setText("⚡ ACTIVE INHIBIT")
            self.state_badge.setStyleSheet(
                "font-size: 12px; font-weight: bold; padding: 6px 14px; "
                "border-radius: 12px; background-color: #083344; color: #00e5ff; "
                "border: 1px solid #00e5ff;"
            )
        elif st == "cooldown":
            cooldown_rem = int(round(status.get("cooldown_remaining", 0)))
            self.state_badge.setText(f"⏳ COOLDOWN ({cooldown_rem}s)")
            self.state_badge.setStyleSheet(
                "font-size: 12px; font-weight: bold; padding: 6px 14px; "
                "border-radius: 12px; background-color: #451a03; color: #f59e0b; "
                "border: 1px solid #f59e0b;"
            )
        elif st == "override":
            self.state_badge.setText("🔒 MANUAL OVERRIDE")
            self.state_badge.setStyleSheet(
                "font-size: 12px; font-weight: bold; padding: 6px 14px; "
                "border-radius: 12px; background-color: #1e1b4b; color: #60a5fa; "
                "border: 1px solid #60a5fa;"
            )
        else:
            self.state_badge.setText("IDLE")
            self.state_badge.setStyleSheet(
                "font-size: 12px; font-weight: bold; padding: 6px 14px; "
                "border-radius: 12px; background-color: #334155; color: #94a3b8;"
            )

        # Update Lock Status
        lock_name = get_platform_lock_name(engine or self.tray_app.engine)
        if inhibited:
            self.lock_status_label.setText(f"Lock: {lock_name} • ACQUIRED")
            self.lock_status_label.setStyleSheet("font-size: 11px; font-weight: bold; color: #00e5ff;")
        else:
            self.lock_status_label.setText(f"Lock: {lock_name} • RELEASED")
            self.lock_status_label.setStyleSheet("font-size: 11px; color: #94a3b8;")

        # Update Active Sessions Table
        if sessions:
            self.empty_label.setVisible(False)
            self.table.setVisible(True)
            self.table.setRowCount(len(sessions))
            now = time.time()
            for row, s in enumerate(sessions):
                agent = s.get("agent_name", "agent")
                sess_id = s.get("session_id", "")
                task = s.get("task", "")
                st_time = float(s.get("start_time", now))
                elapsed = format_duration(now - st_time)

                item_agent = QtWidgets.QTableWidgetItem(str(agent))
                item_id = QtWidgets.QTableWidgetItem(str(sess_id)[:14])
                item_task = QtWidgets.QTableWidgetItem(str(task))
                item_elapsed = QtWidgets.QTableWidgetItem(elapsed)

                item_agent.setForeground(QtGui.QColor("#00e5ff"))
                item_elapsed.setForeground(QtGui.QColor("#fbbf24"))

                self.table.setItem(row, 0, item_agent)
                self.table.setItem(row, 1, item_id)
                self.table.setItem(row, 2, item_task)
                self.table.setItem(row, 3, item_elapsed)
        else:
            self.table.setRowCount(0)
            self.table.setVisible(False)
            self.empty_label.setVisible(True)

        # Update Power / Battery
        if HAS_PSUTIL:
            try:
                battery = psutil.sensors_battery()
                if battery is not None:
                    pct = int(round(battery.percent))
                    if battery.power_plugged:
                        self.power_label.setText(f"⚡ AC Power: {pct}% (Plugged in)")
                    else:
                        time_left = ""
                        if battery.secsleft > 0 and battery.secsleft != psutil.POWER_TIME_UNLIMITED:
                            rem_m = int(battery.secsleft // 60)
                            time_left = f", ~{rem_m // 60}h {rem_m % 60}m left"
                        self.power_label.setText(f"🔋 Battery: {pct}% (On Battery{time_left})")
                else:
                    self.power_label.setText("🔌 AC Power: Desktop / Server")
            except Exception:
                self.power_label.setText("🔌 AC Connected")
        else:
            self.power_label.setText("🔌 Power: Online")

        # Update Today's Cumulative Sleep Prevented
        history = fetch_history_entries(port=self.tray_app.port, limit=100)
        today_secs = calculate_today_prevented_seconds(history, sessions)
        self.prevented_label.setText(format_duration(today_secs))

        # Update Action Button States
        if st == "override":
            self.btn_override.setText("↩️ Disable Override")
            self.btn_override.setObjectName("accentBtn")
        else:
            self.btn_override.setText("⚡ Force Awake")
            self.btn_override.setObjectName("")
        self.btn_override.setStyle(self.btn_override.style())

        if self.tray_app.keep_display_on:
            self.btn_screen.setText("📺 Keep Screen On: ON")
            self.btn_screen.setStyleSheet("border-color: #00e5ff; color: #00e5ff;")
        else:
            self.btn_screen.setText("📺 Keep Screen On: OFF")
            self.btn_screen.setStyleSheet("")

    def _on_toggle_override(self) -> None:
        """Toggle manual override via tray app."""
        current_st = self.tray_app.current_state
        if current_st == "override":
            self.tray_app.set_force_awake(False, 0)
        else:
            self.tray_app.set_force_awake(True, 0)
        self.refresh()

    def _on_toggle_screen(self) -> None:
        """Toggle screen on state via tray app."""
        self.tray_app.toggle_keep_display_on()
        self.refresh()


class WakeTrayApp(QtCore.QObject):
    """
    Cross-platform PyQt6 system tray application for AgentKit WakeLock.

    Features:
    - Dynamic vector status icons (idle, active, cooldown, override).
    - Informative rich tooltips reflecting running agent tasks and countdowns.
    - Right-click context menu with Force Awake durations, screen toggle, and dashboard.
    - Left-click flyout dashboard dialog.
    """

    def __init__(
        self,
        port: int = 18820,
        embed_engine: bool = False,
        engine: Optional[WakeEngine] = None,
        parent: Optional[QtCore.QObject] = None,
    ) -> None:
        super().__init__(parent)
        self.port = port
        self.embed_engine = embed_engine
        self.engine: Optional[WakeEngine] = engine
        self.server: Optional[WakeServer] = None
        self.keep_display_on: bool = False
        self.current_state: str = "idle"
        self.current_status: Dict[str, Any] = {}
        self.dashboard: Optional[WakeDashboardDialog] = None

        if self.embed_engine and self.engine is None:
            self.engine = WakeEngine()

        if self.embed_engine and self.engine is not None:
            try:
                self.server = WakeServer(
                    engine=self.engine, host="127.0.0.1", port=self.port
                )
                self.server.start_background()
                self.port = self.server.port
            except OSError as e:
                logger.warning(
                    "Could not bind embedded WakeServer on port %s: %s", self.port, e
                )
                self.server = None

        self.is_tray_available = (
            HAS_PYQT6
            and QtWidgets.QApplication.instance() is not None
            and QtWidgets.QSystemTrayIcon.isSystemTrayAvailable()
        )

        if not self.is_tray_available:
            logger.warning(
                "System tray is not available on this display or desktop environment."
            )
            return

        self._init_tray()

        # Periodic timer (1000ms)
        self.timer = QtCore.QTimer(self)
        self.timer.setInterval(1000)
        self.timer.timeout.connect(self.update_status)
        self.timer.start()

        # Initial tick
        self.update_status()

    def _init_tray(self) -> None:
        """Initialize QSystemTrayIcon and context menu."""
        self.tray_icon = QtWidgets.QSystemTrayIcon(self)
        self.tray_icon.setIcon(get_status_icon("idle"))
        self.tray_icon.setToolTip("AgentKit WakeLock: Initializing...")

        # Build context menu
        self.menu = QtWidgets.QMenu()
        self.menu.setStyleSheet("""
            QMenu {
                background-color: #1e2638;
                color: #f0f4f8;
                border: 1px solid #2a3850;
                border-radius: 6px;
                padding: 4px;
            }
            QMenu::item {
                padding: 6px 20px;
                border-radius: 4px;
            }
            QMenu::item:selected {
                background-color: #0e7490;
                color: #ffffff;
            }
            QMenu::separator {
                height: 1px;
                background-color: #2a3850;
                margin: 4px 8px;
            }
        """)

        # Title header
        title_action = QtGui.QAction("🛡️ AgentKit WakeLock", self.menu)
        title_font = title_action.font()
        title_font.setBold(True)
        title_action.setFont(title_font)
        title_action.setEnabled(False)
        self.menu.addAction(title_action)

        # Status text
        self.status_action = QtGui.QAction("Status: Idle", self.menu)
        self.status_action.setEnabled(False)
        self.menu.addAction(self.status_action)

        self.menu.addSeparator()

        # Force Awake Submenu
        self.force_menu = self.menu.addMenu("⚡ Force Awake")
        self.force_group = QtGui.QActionGroup(self)
        self.force_group.setExclusive(True)

        self.act_15m = QtGui.QAction("15 Minutes", self.force_menu, checkable=True)
        self.act_15m.triggered.connect(lambda: self.set_force_awake(True, 15 * 60))
        self.force_group.addAction(self.act_15m)
        self.force_menu.addAction(self.act_15m)

        self.act_1h = QtGui.QAction("1 Hour", self.force_menu, checkable=True)
        self.act_1h.triggered.connect(lambda: self.set_force_awake(True, 60 * 60))
        self.force_group.addAction(self.act_1h)
        self.force_menu.addAction(self.act_1h)

        self.act_indefinite = QtGui.QAction("Indefinite", self.force_menu, checkable=True)
        self.act_indefinite.triggered.connect(lambda: self.set_force_awake(True, 0))
        self.force_group.addAction(self.act_indefinite)
        self.force_menu.addAction(self.act_indefinite)

        self.act_disabled = QtGui.QAction("Disabled", self.force_menu, checkable=True)
        self.act_disabled.setChecked(True)
        self.act_disabled.triggered.connect(lambda: self.set_force_awake(False, 0))
        self.force_group.addAction(self.act_disabled)
        self.force_menu.addAction(self.act_disabled)

        # Keep Display On toggle
        self.display_action = QtGui.QAction("📺 Keep Display On", self.menu, checkable=True)
        self.display_action.setChecked(self.keep_display_on)
        self.display_action.toggled.connect(self._on_display_action_toggled)
        self.menu.addAction(self.display_action)

        # View Dashboard
        dashboard_action = QtGui.QAction("📊 View Dashboard...", self.menu)
        dashboard_action.triggered.connect(self.open_dashboard)
        self.menu.addAction(dashboard_action)

        self.menu.addSeparator()

        # Exit action
        exit_action = QtGui.QAction("❌ Exit", self.menu)
        exit_action.triggered.connect(self.exit_app)
        self.menu.addAction(exit_action)

        self.tray_icon.setContextMenu(self.menu)
        self.tray_icon.activated.connect(self._on_tray_activated)

    def show(self) -> None:
        """Display the tray icon."""
        if hasattr(self, "tray_icon"):
            self.tray_icon.show()

    def _on_tray_activated(self, reason: QtWidgets.QSystemTrayIcon.ActivationReason) -> None:
        """Handle tray icon click activation."""
        if reason == QtWidgets.QSystemTrayIcon.ActivationReason.Trigger:
            self.open_dashboard()

    def _on_display_action_toggled(self, checked: bool) -> None:
        """Handle Keep Display On context menu action."""
        self.keep_display_on = checked
        if self.engine is not None and getattr(self.engine, "inhibitor", None) is not None:
            self.engine.inhibitor._keep_screen_on = self.keep_display_on
            if self.engine.inhibitor.is_active():
                self.engine.inhibitor.acquire(
                    reason=self.engine.inhibitor._reason,
                    keep_screen_on=self.keep_display_on,
                )
        if self.dashboard is not None and self.dashboard.isVisible():
            self.dashboard.refresh(self.current_status, self.engine)

    def toggle_keep_display_on(self) -> None:
        """Toggle keep display on state programmatically."""
        self.keep_display_on = not self.keep_display_on
        if hasattr(self, "display_action"):
            self.display_action.setChecked(self.keep_display_on)
        self._on_display_action_toggled(self.keep_display_on)

    def set_force_awake(self, enabled: bool, duration_seconds: int = 0) -> None:
        """Send manual override command to engine or remote server."""
        if self.engine is not None:
            self.engine.set_override(enabled=enabled, duration_seconds=duration_seconds)
        else:
            send_override(
                enabled=enabled,
                port=self.port,
                duration_seconds=duration_seconds,
            )
        self.update_status()

    def update_status(self) -> None:
        """Periodic status update fetching state, updating icon, tooltip, and dashboard."""
        if self.engine is not None:
            status = self.engine.get_status()
        else:
            status = fetch_status(port=self.port)

        if status is None:
            self.current_state = "offline"
            self.current_status = {}
            if hasattr(self, "tray_icon"):
                self.tray_icon.setIcon(get_status_icon("idle"))
                self.tray_icon.setToolTip(
                    f"AgentKit WakeLock: Offline (connecting to 127.0.0.1:{self.port}...)"
                )
            if hasattr(self, "status_action"):
                self.status_action.setText(f"Status: Offline (127.0.0.1:{self.port})")
            if hasattr(self, "act_disabled"):
                self.act_disabled.setChecked(True)
            return

        self.current_status = status
        st = status.get("state", "idle").lower()
        self.current_state = st

        # Update Icon
        if hasattr(self, "tray_icon"):
            self.tray_icon.setIcon(get_status_icon(st))

        # Format Tooltip based on specifications
        if st == "idle":
            tooltip = "AgentKit WakeLock: Idle (System sleep allowed)"
            status_text = "Status: Idle (Sleep allowed)"
        elif st == "active":
            count = status.get("active_count", 0)
            sessions: List[Dict[str, Any]] = status.get("sessions", [])
            lines = [f"⚡ Active WakeLock: {count} Agent(s) running"]
            now = time.time()
            for s in sessions[:3]:  # Show top 3 in tooltip
                agent = s.get("agent_name", "agent")
                task = s.get("task", "Prompt executing")
                elapsed = format_duration(now - float(s.get("start_time", now)))
                lines.append(f"{agent}: {task} ({elapsed})")
            if len(sessions) > 3:
                lines.append(f"+{len(sessions) - 3} more...")
            tooltip = "\n".join(lines)
            status_text = f"Status: Active WakeLock ({count} running)"
        elif st == "cooldown":
            cooldown_rem = int(round(status.get("cooldown_remaining", 0)))
            tooltip = f"⏳ Cooldown: {cooldown_rem}s remaining before release"
            status_text = f"Status: Cooldown ({cooldown_rem}s)"
        elif st == "override":
            tooltip = "🔒 Manual Awake Override Active"
            status_text = "Status: Manual Override (Awake)"
        else:
            tooltip = f"AgentKit WakeLock: {st.title()}"
            status_text = f"Status: {st.title()}"

        if hasattr(self, "tray_icon"):
            self.tray_icon.setToolTip(tooltip)
        if hasattr(self, "status_action"):
            self.status_action.setText(status_text)

        # Update force awake checkmark
        if hasattr(self, "act_disabled") and hasattr(self, "act_indefinite"):
            if st != "override":
                self.act_disabled.setChecked(True)
            elif not self.act_15m.isChecked() and not self.act_1h.isChecked():
                self.act_indefinite.setChecked(True)

        # Update Flyout Dashboard if open
        if self.dashboard is not None and self.dashboard.isVisible():
            self.dashboard.refresh(self.current_status, self.engine)

    def open_dashboard(self) -> None:
        """Open or raise the Flyout Dashboard dialog."""
        if self.dashboard is None:
            self.dashboard = WakeDashboardDialog(tray_app=self)

        # Position near tray icon if available
        if hasattr(self, "tray_icon"):
            geo = self.tray_icon.geometry()
            if geo.isValid() and not geo.isNull():
                x = max(20, geo.x() - self.dashboard.width() // 2)
                y = max(20, geo.y() - self.dashboard.height() - 20)
                self.dashboard.move(x, y)

        self.dashboard.show()
        self.dashboard.raise_()
        self.dashboard.activateWindow()
        self.dashboard.refresh(self.current_status, self.engine)

    def stop(self) -> None:
        """Safely stop embedded server, engine, timers, and dialogs."""
        if hasattr(self, "timer") and self.timer is not None:
            try:
                self.timer.stop()
            except Exception:
                pass
        if self.dashboard is not None:
            try:
                self.dashboard.close()
            except Exception:
                pass
            self.dashboard = None
        if hasattr(self, "tray_icon") and self.tray_icon is not None:
            try:
                self.tray_icon.hide()
            except Exception:
                pass
        if self.server is not None:
            try:
                self.server.stop()
            except Exception:
                pass
            self.server = None
        if self.engine is not None:
            try:
                self.engine.stop()
            except Exception:
                pass

    def exit_app(self) -> None:
        """Cleanly terminate the tray application, embedded server, and timers."""
        self.stop()
        if HAS_PYQT6 and QtWidgets.QApplication.instance():
            QtWidgets.QApplication.quit()


# Backward compatibility aliases
GuardDashboardDialog = WakeDashboardDialog
GuardTrayApp = WakeTrayApp


def run_tray(port: int = 18820, embed_engine: bool = False) -> int:
    """
    Launch the agentkit wake-lock system tray application.

    Returns exit code (0 for clean termination or graceful headless exit).
    """
    if not HAS_PYQT6:
        print(
            "PyQt6 is required to run the agentkit wake system tray GUI.",
            file=sys.stderr,
        )
        return 1

    # In Linux environments without active X11 or Wayland display, fallback to offscreen
    if "QT_QPA_PLATFORM" not in os.environ:
        has_wayland = bool(os.environ.get("WAYLAND_DISPLAY"))
        has_x11 = bool(os.environ.get("DISPLAY"))
        if not has_wayland and not has_x11:
            os.environ["QT_QPA_PLATFORM"] = "offscreen"

    app = QtWidgets.QApplication.instance()
    if app is None:
        app = QtWidgets.QApplication(sys.argv if sys.argv else ["agentkit-wake-tray"])

    app.setQuitOnLastWindowClosed(False)

    if not QtWidgets.QSystemTrayIcon.isSystemTrayAvailable():
        logger.warning(
            "System tray is not available on this desktop environment or display. "
            "Exiting gracefully."
        )
        print(
            "System tray is not available on this desktop environment or display. "
            "Exiting gracefully.",
            file=sys.stderr,
        )
        return 0

    tray_app = WakeTrayApp(port=port, embed_engine=embed_engine)
    tray_app.show()

    return app.exec()
