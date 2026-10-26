#!/usr/bin/env python3
"""
Notify Guard Hook Script for agentkit sleep guard plugin.
Pings local GuardServer with active/idle status.
"""

import argparse
import os
import sys
from pathlib import Path

# Add project root to path so we can import project.tools.guard
REPO_ROOT = Path(__file__).resolve().parent.parent.parent.parent
if str(REPO_ROOT) not in sys.path:
    sys.path.insert(0, str(REPO_ROOT))


def main():
    parser = argparse.ArgumentParser(description="Notify AgentKit Sleep Guard server")
    parser.add_argument("state", choices=["active", "idle"], help="Agent execution state")
    parser.add_argument("--session", default=os.environ.get("ANTIGRAVITY_SESSION_ID", "default"), help="Session ID")
    parser.add_argument("--agent", default=os.environ.get("AGENTKIT_HARNESS", "agent"), help="Agent name")
    parser.add_argument("--task", default="", help="Current task description")
    
    args = parser.parse_args()

    try:
        from project.tools.guard.server import send_notification
        send_notification(
            session_id=args.session,
            state=args.state,
            agent=args.agent,
            task=args.task,
            timeout=0.5
        )
    except Exception:
        # Silently exit 0 so hook failures never block or crash the agent
        pass


if __name__ == "__main__":
    main()
