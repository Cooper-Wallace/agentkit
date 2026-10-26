"""
Loopback HTTP server and client utilities for agentkit sleep inhibition guard.

Provides standard library ThreadingHTTPServer binding to 127.0.0.1:18820,
exposing status, notify, override, and history endpoints.
"""

from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import threading
from typing import Any, Optional
import urllib.parse
import urllib.request

from .engine import GuardEngine


class _GuardRequestHandler(BaseHTTPRequestHandler):
    """HTTP request handler for agentkit sleep inhibition guard service."""

    def log_message(self, format: str, *args: Any) -> None:
        """Suppress default request console logging."""
        pass

    @property
    def engine(self) -> GuardEngine:
        return self.server.engine  # type: ignore

    def _send_json(self, status_code: int, data: Any) -> None:
        body = json.dumps(data).encode("utf-8")
        self.send_response(status_code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _read_body_json(self) -> dict:
        content_length = int(self.headers.get("Content-Length", 0))
        if content_length > 0:
            raw_body = self.rfile.read(content_length).decode("utf-8")
            try:
                return json.loads(raw_body)
            except json.JSONDecodeError:
                return {}
        return {}

    def do_GET(self) -> None:
        parsed = urllib.parse.urlparse(self.path)
        path = parsed.path.rstrip("/")
        query = urllib.parse.parse_qs(parsed.query)

        if path in ("/v1/status", "/status"):
            self._send_json(200, self.engine.get_status())
        elif path in ("/v1/notify", "/notify"):
            session_id = query.get("session", query.get("session_id", [""]))[0]
            state = query.get("state", [""])[0]
            agent = query.get("agent", ["agent"])[0]
            task = query.get("task", [""])[0]
            try:
                pid = int(query.get("pid", ["0"])[0])
            except ValueError:
                pid = 0
            if not session_id or not state:
                self._send_json(400, {"error": "Missing 'session' or 'state' parameter"})
                return
            try:
                res = self.engine.notify(
                    session_id=session_id,
                    state=state,
                    agent=agent,
                    task=task,
                    pid=pid,
                )
                self._send_json(200, res)
            except ValueError as e:
                self._send_json(400, {"error": str(e)})
        elif path in ("/v1/override", "/override"):
            enabled_str = query.get("enabled", [""])[0].lower()
            enabled = enabled_str in ("true", "1", "yes", "on")
            try:
                duration = int(
                    query.get("duration", query.get("duration_seconds", ["0"]))[0]
                )
            except ValueError:
                duration = 0
            res = self.engine.set_override(enabled=enabled, duration_seconds=duration)
            self._send_json(200, res)
        elif path in ("/v1/history", "/history"):
            try:
                limit = int(query.get("limit", ["50"])[0])
            except ValueError:
                limit = 50
            res = self.engine.get_history(limit=limit)
            self._send_json(200, res)
        else:
            self._send_json(404, {"error": f"Unknown endpoint '{path}'"})

    def do_POST(self) -> None:
        parsed = urllib.parse.urlparse(self.path)
        path = parsed.path.rstrip("/")
        query = urllib.parse.parse_qs(parsed.query)
        body = self._read_body_json()

        if path in ("/v1/notify", "/notify"):
            session_val = body.get(
                "session_id",
                body.get("session", query.get("session", query.get("session_id", [""]))[0] if query else ""),
            )
            session_id = str(session_val)
            state = str(body.get("state", query.get("state", [""])[0] if query else ""))
            agent = str(body.get("agent", query.get("agent", ["agent"])[0] if query else "agent"))
            task = str(body.get("task", query.get("task", [""])[0] if query else ""))
            try:
                pid = int(body.get("pid", query.get("pid", [0])[0] if query else 0))
            except (ValueError, TypeError):
                pid = 0
            if not session_id or not state:
                self._send_json(400, {"error": "Missing 'session_id' or 'state'"})
                return
            try:
                res = self.engine.notify(
                    session_id=session_id,
                    state=state,
                    agent=agent,
                    task=task,
                    pid=pid,
                )
                self._send_json(200, res)
            except ValueError as e:
                self._send_json(400, {"error": str(e)})
        elif path in ("/v1/override", "/override"):
            enabled_raw = body.get(
                "enabled",
                query.get("enabled", [""])[0] if query else False,
            )
            if isinstance(enabled_raw, str):
                enabled = enabled_raw.lower() in ("true", "1", "yes", "on")
            else:
                enabled = bool(enabled_raw)
            try:
                duration = int(
                    body.get(
                        "duration_seconds",
                        body.get("duration", query.get("duration", [0])[0] if query else 0),
                    )
                )
            except (ValueError, TypeError):
                duration = 0
            res = self.engine.set_override(enabled=enabled, duration_seconds=duration)
            self._send_json(200, res)
        elif path in ("/v1/status", "/status"):
            self._send_json(200, self.engine.get_status())
        else:
            self._send_json(404, {"error": f"Unknown endpoint '{path}'"})


class GuardServer:
    """Threaded loopback HTTP server exposing guard status and control endpoints."""

    def __init__(
        self,
        engine: Optional[GuardEngine] = None,
        host: str = "127.0.0.1",
        port: int = 18820,
    ) -> None:
        self.engine: GuardEngine = engine if engine is not None else GuardEngine()
        self.host: str = host
        self.requested_port: int = port
        self._server: Optional[ThreadingHTTPServer] = None
        self._thread: Optional[threading.Thread] = None

    @property
    def port(self) -> int:
        """Return the active listening port."""
        if self._server:
            return self._server.server_address[1]
        return self.requested_port

    @property
    def server_address(self) -> tuple[str, int]:
        """Return the (host, port) tuple."""
        if self._server:
            return self._server.server_address
        return (self.host, self.requested_port)

    def start_background(self) -> None:
        """Start the HTTP server on a background daemon thread."""
        if self._server is not None:
            return
        server = ThreadingHTTPServer(
            (self.host, self.requested_port), _GuardRequestHandler
        )
        server.engine = self.engine  # type: ignore
        server.daemon_threads = True
        self._server = server
        self._thread = threading.Thread(target=self._server.serve_forever, daemon=True)
        self._thread.start()

    def stop(self) -> None:
        """Shutdown and terminate the HTTP server and engine timers."""
        if self._server is not None:
            self._server.shutdown()
            self._server.server_close()
            self._server = None
        if self._thread is not None and self._thread.is_alive():
            self._thread.join(timeout=2.0)
            self._thread = None
        if self.engine is not None:
            self.engine.stop()


def send_notification(
    session_id: str,
    state: str,
    agent: str = "agent",
    task: str = "",
    port: int = 18820,
    timeout: float = 1.0,
    pid: int = 0,
) -> bool:
    """Send activity notification to loopback guard service."""
    url = f"http://127.0.0.1:{port}/v1/notify"
    data = json.dumps(
        {
            "session_id": session_id,
            "state": state,
            "agent": agent,
            "task": task,
            "pid": pid,
        }
    ).encode("utf-8")
    req = urllib.request.Request(
        url,
        data=data,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.status == 200
    except Exception:
        return False


def fetch_status(port: int = 18820, timeout: float = 1.0) -> Optional[dict]:
    """Fetch current activity and inhibition status from guard service."""
    url = f"http://127.0.0.1:{port}/v1/status"
    req = urllib.request.Request(url, method="GET")
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            if resp.status == 200:
                body = resp.read().decode("utf-8")
                return json.loads(body)
    except Exception:
        return None
    return None


def send_override(
    enabled: bool,
    port: int = 18820,
    timeout: float = 1.0,
    duration_seconds: int = 0,
) -> bool:
    """Send manual override command to guard service."""
    url = f"http://127.0.0.1:{port}/v1/override"
    data = json.dumps(
        {
            "enabled": enabled,
            "duration_seconds": duration_seconds,
        }
    ).encode("utf-8")
    req = urllib.request.Request(
        url,
        data=data,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.status == 200
    except Exception:
        return False
