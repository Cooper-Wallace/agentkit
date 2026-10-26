"""
Cross-platform autostart manager for agentkit sleep guard.
Manages user session autostart entry for Linux (~/.config/autostart/) and Windows (%APPDATA%\\Microsoft\\Windows\\Start Menu\\Programs\\Startup\\).
"""

import os
import shutil
import sys
from pathlib import Path
from typing import Tuple


def get_autostart_path() -> Path:
    """Return the platform-specific target autostart file path."""
    if sys.platform == "win32":
        appdata = os.environ.get("APPDATA", str(Path.home() / "AppData" / "Roaming"))
        return Path(appdata) / "Microsoft" / "Windows" / "Start Menu" / "Programs" / "Startup" / "agentkit-guard.cmd"
    else:
        # Default Linux / XDG autostart path
        config_home = os.environ.get("XDG_CONFIG_HOME", str(Path.home() / ".config"))
        return Path(config_home) / "autostart" / "agentkit-guard.desktop"


def is_autostart_enabled() -> bool:
    """Return whether the platform autostart file exists."""
    return get_autostart_path().exists()


def enable_autostart(agentkit_bin: str = None, mode: str = "tray") -> Tuple[bool, str]:
    """
    Creates the autostart entry idempotently.
    Returns (success, message).
    """
    path = get_autostart_path()
    try:
        path.parent.mkdir(parents=True, exist_ok=True)
        
        if sys.platform == "win32":
            if not agentkit_bin:
                agentkit_bin = sys.executable
            # Windows cmd startup wrapper
            python_exe = sys.executable
            content = f"""@echo off
start "" "{python_exe}" "{agentkit_bin}" guard tray
"""
            path.write_text(content, encoding="utf-8")
        else:
            if not agentkit_bin:
                # Find agentkit executable in PATH or fallback to sys.argv[0] or python3 -m agentkit
                agentkit_bin = shutil.which("agentkit") or str(Path.home() / ".local" / "bin" / "agentkit")
            
            content = f"""[Desktop Entry]
Type=Application
Name=AgentKit Sleep Guard
Comment=Inhibits machine sleep while AI agent CLI prompts are actively executing
Exec={agentkit_bin} guard tray
Icon=utilities-system-monitor
Terminal=false
Categories=Utility;
X-GNOME-Autostart-enabled=true
"""
            path.write_text(content, encoding="utf-8")
            # Make desktop entry executable
            os.chmod(path, 0o755)

        return True, f"Successfully enabled autostart at {path}"
    except Exception as e:
        return False, f"Failed to enable autostart: {e}"


def disable_autostart() -> Tuple[bool, str]:
    """
    Removes the autostart entry if present.
    Returns (success, message).
    """
    path = get_autostart_path()
    try:
        if path.exists():
            path.unlink()
            return True, f"Successfully removed autostart entry at {path}"
        return True, f"Autostart entry not found at {path}"
    except Exception as e:
        return False, f"Failed to remove autostart: {e}"
