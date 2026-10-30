#!/usr/bin/env python3
"""
Notify Wake Hook Script for agentkit wake-lock plugin.
Pings local WakeServer with active/idle status.
"""

import argparse
import json
import os
from pathlib import Path
import sys

# Resolve agentkit repository root dynamically and add to sys.path
repo_candidates = [
    Path(__file__).resolve().parent.parent.parent.parent.parent,
    Path.home() / "Projects" / "agentkit",
]
cur = Path(__file__).resolve().parent
while cur != cur.parent:
    if (cur / "project" / "tools" / "wake").exists():
        repo_candidates.insert(0, cur)
        break
    cur = cur.parent

for candidate in repo_candidates:
    if candidate.exists() and str(candidate) not in sys.path:
        sys.path.insert(0, str(candidate))


def _post_notification(session_id: str, state: str, agent: str, task: str, port: int) -> None:
    """Send non-blocking HTTP notification to WakeServer via send_notification or urllib."""
    try:
        from project.tools.wake.server import send_notification
        send_notification(
            session_id=session_id,
            state=state,
            agent=agent,
            task=task,
            port=port,
            timeout=0.5,
        )
        return
    except Exception:
        pass

    try:
        import urllib.request
        url = f"http://127.0.0.1:{port}/v1/notify"
        data = json.dumps(
            {
                "session_id": session_id,
                "state": state,
                "agent": agent,
                "task": task,
                "pid": os.getpid(),
            }
        ).encode("utf-8")
        req = urllib.request.Request(
            url,
            data=data,
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        with opener.open(req, timeout=0.5):
            pass
    except Exception:
        pass


def main() -> int:
    if "--dry-run" in sys.argv:
        print(json.dumps({"status": "dry-run", "message": "notify-wake.py is operational"}))
        sys.stdout.flush()
        return 0

    parser = argparse.ArgumentParser(description="Notify AgentKit WakeLock server")
    parser.add_argument("state", nargs="?", default=None, choices=["active", "idle", None], help="Agent execution state")
    parser.add_argument("--session", default=None, help="Session ID")
    parser.add_argument("--agent", default=os.environ.get("AGENTKIT_HARNESS", "agent"), help="Agent name")
    parser.add_argument("--task", default="", help="Current task description")
    parser.add_argument("--port", type=int, default=int(os.environ.get("AGENTKIT_WAKE_PORT", os.environ.get("WAKE_PORT", 18820))), help="Wake server port")
    parser.add_argument("--test", action="store_true", help="Test mode flag")
    parser.add_argument("--dry-run", action="store_true", help="Dry run check")

    args = parser.parse_args()

    payload = {}
    if not args.test and not sys.stdin.isatty():
        try:
            raw = sys.stdin.read().strip()
            if raw:
                loaded = json.loads(raw)
                if isinstance(loaded, dict):
                    payload = loaded
        except Exception:
            payload = {}

    # State resolution
    if args.state:
        state = args.state
    elif "terminationReason" in payload:
        state = "idle"
    else:
        state = "active"

    # Session ID resolution
    session = args.session
    if not session:
        session = (
            payload.get("conversationId")
            or os.environ.get("AGY_SESSION_ID")
            or os.environ.get("TERM_SESSION_ID")
            or os.environ.get("ANTIGRAVITY_SESSION_ID")
            or f"ppid-{os.getppid()}"
        )

    # Agent name resolution
    agent = args.agent
    if not agent or agent == "agent":
        agent = payload.get("agent", os.environ.get("AGENTKIT_HARNESS", "agent"))

    # Task description resolution
    task = args.task
    if not task:
        if "toolCall" in payload and isinstance(payload["toolCall"], dict):
            task = payload["toolCall"].get("name", "")
        elif "task" in payload:
            task = str(payload["task"])

    _post_notification(session_id=session, state=state, agent=agent, task=task, port=args.port)

    # Print Antigravity hook decision response to stdout
    if "toolCall" in payload:
        print(json.dumps({"decision": "allow"}))
    elif "terminationReason" in payload:
        print(json.dumps({"decision": "stop"}))
    else:
        print(json.dumps({}))
    sys.stdout.flush()
    return 0


if __name__ == "__main__":
    sys.exit(main())
