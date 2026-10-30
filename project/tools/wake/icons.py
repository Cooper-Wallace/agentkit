"""
Dynamic vector status icon generation for agentkit sleep inhibition wake lock.

Generates high-resolution vector icons across all 4 wake states:
- idle: Slate/grey shield with subtle sleeping moon/dot.
- active: Vibrant cyan shield with bright lightning bolt and glowing ring.
- cooldown: Warm amber shield with hourglass timer.
- override: Royal blue shield with padlock stay-awake indicator.

Supports PyQt6 QPainter rendering with PIL fallback and raw SVG/PNG export.
"""

from __future__ import annotations

import io
import os
import sys
from typing import Any, Dict, Optional, Tuple

try:
    from PyQt6 import QtCore, QtGui, QtWidgets
    HAS_PYQT6 = True
except ImportError:
    HAS_PYQT6 = False

try:
    from PIL import Image, ImageDraw
    HAS_PIL = True
except ImportError:
    HAS_PIL = False

# Cached QIcon instances: state -> QIcon
_ICON_CACHE: Dict[str, QtGui.QIcon] = {}
_HEADLESS_APP: Optional[Any] = None

# Raw SVG template definitions for all 4 states
_SVG_TEMPLATES: Dict[str, str] = {
    "idle": """<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 128 128" width="128" height="128">
  <defs>
    <linearGradient id="shieldGrad" x1="0%" y1="0%" x2="0%" y2="100%">
      <stop offset="0%" stop-color="#334155"/>
      <stop offset="100%" stop-color="#0f172a"/>
    </linearGradient>
  </defs>
  <path d="M 24 24 L 104 24 L 104 66 Q 104 96 64 114 Q 24 96 24 66 Z" fill="url(#shieldGrad)" stroke="#64748b" stroke-width="4" stroke-linejoin="round"/>
  <path d="M 64 42 A 18 18 0 1 0 78 70 A 15 15 0 1 1 64 42 Z" fill="#94a3b8"/>
  <circle cx="86" cy="46" r="3" fill="#cbd5e1"/>
</svg>""",
    "active": """<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 128 128" width="128" height="128">
  <defs>
    <linearGradient id="shieldGradActive" x1="0%" y1="0%" x2="0%" y2="100%">
      <stop offset="0%" stop-color="#0e7490"/>
      <stop offset="100%" stop-color="#083344"/>
    </linearGradient>
    <filter id="glow" x="-20%" y="-20%" width="140%" height="140%">
      <feGaussianBlur stdDeviation="6" result="blur"/>
      <feComposite in="SourceGraphic" in2="blur" operator="over"/>
    </filter>
  </defs>
  <path d="M 20 20 L 108 20 L 108 68 Q 108 100 64 120 Q 20 100 20 68 Z" fill="none" stroke="#00e5ff" stroke-width="4" opacity="0.3" filter="url(#glow)"/>
  <path d="M 24 24 L 104 24 L 104 66 Q 104 96 64 114 Q 24 96 24 66 Z" fill="url(#shieldGradActive)" stroke="#00e5ff" stroke-width="4" stroke-linejoin="round"/>
  <polygon points="68,34 46,66 62,66 58,96 84,60 68,60 76,34" fill="#fbbf24" stroke="#ffffff" stroke-width="1.5" stroke-linejoin="round"/>
</svg>""",
    "cooldown": """<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 128 128" width="128" height="128">
  <defs>
    <linearGradient id="shieldGradCooldown" x1="0%" y1="0%" x2="0%" y2="100%">
      <stop offset="0%" stop-color="#b45309"/>
      <stop offset="100%" stop-color="#451a03"/>
    </linearGradient>
  </defs>
  <path d="M 24 24 L 104 24 L 104 66 Q 104 96 64 114 Q 24 96 24 66 Z" fill="url(#shieldGradCooldown)" stroke="#f59e0b" stroke-width="4" stroke-linejoin="round"/>
  <polygon points="48,44 80,44 66,64 80,84 48,84 62,64" fill="#fde68a" stroke="#ffffff" stroke-width="1.5" stroke-linejoin="round"/>
  <line x1="44" y1="43" x2="84" y2="43" stroke="#fbbf24" stroke-width="3" stroke-linecap="round"/>
  <line x1="44" y1="85" x2="84" y2="85" stroke="#fbbf24" stroke-width="3" stroke-linecap="round"/>
</svg>""",
    "override": """<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 128 128" width="128" height="128">
  <defs>
    <linearGradient id="shieldGradOverride" x1="0%" y1="0%" x2="0%" y2="100%">
      <stop offset="0%" stop-color="#2563eb"/>
      <stop offset="100%" stop-color="#1e1b4b"/>
    </linearGradient>
  </defs>
  <path d="M 24 24 L 104 24 L 104 66 Q 104 96 64 114 Q 24 96 24 66 Z" fill="url(#shieldGradOverride)" stroke="#60a5fa" stroke-width="4" stroke-linejoin="round"/>
  <path d="M 52 62 L 52 48 A 12 12 0 0 1 76 48 L 76 62" fill="none" stroke="#e2e8f0" stroke-width="5" stroke-linecap="round"/>
  <rect x="46" y="58" width="36" height="30" rx="6" fill="#60a5fa" stroke="#ffffff" stroke-width="2"/>
  <circle cx="64" cy="70" r="3.5" fill="#1e293b"/>
  <line x1="64" y1="70" x2="64" y2="78" stroke="#1e293b" stroke-width="2.5" stroke-linecap="round"/>
</svg>""",
}


def _normalize_state(state: str) -> str:
    """Normalize state string to one of the 4 recognized states."""
    if not state:
        return "idle"
    st = str(state).strip().lower()
    if st in ("active", "running", "inhibited"):
        return "active"
    if st in ("cooldown", "cooling"):
        return "cooldown"
    if st in ("override", "forced", "manual"):
        return "override"
    return "idle"


def _ensure_qt_gui_app() -> bool:
    """Ensure a QGuiApplication exists for QPixmap/QPainter operations."""
    global _HEADLESS_APP
    if not HAS_PYQT6:
        return False
    app = QtCore.QCoreApplication.instance()
    if app is None:
        if "QT_QPA_PLATFORM" not in os.environ:
            has_wayland = bool(os.environ.get("WAYLAND_DISPLAY"))
            has_x11 = bool(os.environ.get("DISPLAY"))
            if not has_wayland and not has_x11:
                os.environ["QT_QPA_PLATFORM"] = "offscreen"
        try:
            _HEADLESS_APP = QtGui.QGuiApplication(["agentkit-wake", "-platform", os.environ.get("QT_QPA_PLATFORM", "offscreen")])
            return True
        except Exception:
            return False
    return isinstance(app, QtGui.QGuiApplication)


def _render_qt_image(norm_state: str, size: int = 128) -> QtGui.QImage:
    """Draw crisp vector status shield and emblem using PyQt6 QPainter."""
    image = QtGui.QImage(size, size, QtGui.QImage.Format.Format_ARGB32_Premultiplied)
    image.fill(QtCore.Qt.GlobalColor.transparent)

    painter = QtGui.QPainter(image)
    try:
        painter.setRenderHint(QtGui.QPainter.RenderHint.Antialiasing, True)
        painter.setRenderHint(QtGui.QPainter.RenderHint.SmoothPixmapTransform, True)

        # Scale factor relative to 128x128 reference grid
        scale = size / 128.0
        painter.scale(scale, scale)

        # Glowing halo for active state
        if norm_state == "active":
            glow_path = QtGui.QPainterPath()
            glow_path.moveTo(20, 20)
            glow_path.lineTo(108, 20)
            glow_path.lineTo(108, 68)
            glow_path.quadTo(108, 100, 64, 120)
            glow_path.quadTo(20, 100, 20, 68)
            glow_path.closeSubpath()
            painter.strokePath(glow_path, QtGui.QPen(QtGui.QColor(0, 229, 255, 75), 8))

        # Main shield boundary path
        shield_path = QtGui.QPainterPath()
        shield_path.moveTo(24, 24)
        shield_path.lineTo(104, 24)
        shield_path.lineTo(104, 66)
        shield_path.quadTo(104, 96, 64, 114)
        shield_path.quadTo(24, 96, 24, 66)
        shield_path.closeSubpath()

        # State-specific gradient and pen
        grad = QtGui.QLinearGradient(0, 24, 0, 114)
        if norm_state == "active":
            grad.setColorAt(0.0, QtGui.QColor("#0e7490"))  # cyan-700
            grad.setColorAt(1.0, QtGui.QColor("#083344"))  # cyan-950
            border_pen = QtGui.QPen(QtGui.QColor("#00e5ff"), 4)  # vibrant cyan
        elif norm_state == "cooldown":
            grad.setColorAt(0.0, QtGui.QColor("#b45309"))  # amber-700
            grad.setColorAt(1.0, QtGui.QColor("#451a03"))  # amber-950
            border_pen = QtGui.QPen(QtGui.QColor("#f59e0b"), 4)  # amber-500
        elif norm_state == "override":
            grad.setColorAt(0.0, QtGui.QColor("#2563eb"))  # blue-600
            grad.setColorAt(1.0, QtGui.QColor("#1e1b4b"))  # indigo-950
            border_pen = QtGui.QPen(QtGui.QColor("#60a5fa"), 4)  # blue-400
        else:  # idle
            grad.setColorAt(0.0, QtGui.QColor("#334155"))  # slate-700
            grad.setColorAt(1.0, QtGui.QColor("#0f172a"))  # slate-900
            border_pen = QtGui.QPen(QtGui.QColor("#64748b"), 4)  # slate-500

        border_pen.setJoinStyle(QtCore.Qt.PenJoinStyle.RoundJoin)
        painter.setPen(border_pen)
        painter.setBrush(QtGui.QBrush(grad))
        painter.drawPath(shield_path)

        # Draw State Emblem
        if norm_state == "active":
            # Vibrant amber/yellow lightning bolt
            bolt = QtGui.QPolygonF([
                QtCore.QPointF(68, 34),
                QtCore.QPointF(46, 66),
                QtCore.QPointF(62, 66),
                QtCore.QPointF(58, 96),
                QtCore.QPointF(84, 60),
                QtCore.QPointF(68, 60),
                QtCore.QPointF(76, 34),
            ])
            painter.setBrush(QtGui.QColor("#fbbf24"))
            painter.setPen(QtGui.QPen(QtGui.QColor("#ffffff"), 1.5, join=QtCore.Qt.PenJoinStyle.RoundJoin))
            painter.drawPolygon(bolt)

        elif norm_state == "cooldown":
            # Warm amber hourglass
            hg = QtGui.QPolygonF([
                QtCore.QPointF(48, 44),
                QtCore.QPointF(80, 44),
                QtCore.QPointF(66, 64),
                QtCore.QPointF(80, 84),
                QtCore.QPointF(48, 84),
                QtCore.QPointF(62, 64),
            ])
            painter.setBrush(QtGui.QColor("#fde68a"))
            painter.setPen(QtGui.QPen(QtGui.QColor("#ffffff"), 1.5, join=QtCore.Qt.PenJoinStyle.RoundJoin))
            painter.drawPolygon(hg)
            # Top and bottom caps
            cap_pen = QtGui.QPen(QtGui.QColor("#fbbf24"), 3, cap=QtCore.Qt.PenCapStyle.RoundCap)
            painter.setPen(cap_pen)
            painter.drawLine(44, 43, 84, 43)
            painter.drawLine(44, 85, 84, 85)

        elif norm_state == "override":
            # Royal blue padlock
            # Shackle
            shackle_path = QtGui.QPainterPath()
            shackle_path.moveTo(52, 62)
            shackle_path.lineTo(52, 48)
            shackle_path.arcTo(QtCore.QRectF(52, 36, 24, 24), 180, -180)
            shackle_path.lineTo(76, 62)
            shackle_pen = QtGui.QPen(QtGui.QColor("#e2e8f0"), 5, cap=QtCore.Qt.PenCapStyle.RoundCap)
            painter.strokePath(shackle_path, shackle_pen)

            # Padlock body
            body_rect = QtCore.QRectF(46, 58, 36, 30)
            painter.setBrush(QtGui.QColor("#60a5fa"))
            painter.setPen(QtGui.QPen(QtGui.QColor("#ffffff"), 2))
            painter.drawRoundedRect(body_rect, 6, 6)

            # Keyhole
            painter.setBrush(QtGui.QColor("#1e293b"))
            painter.setPen(QtCore.Qt.PenStyle.NoPen)
            painter.drawEllipse(QtCore.QPointF(64, 70), 3.5, 3.5)
            key_pen = QtGui.QPen(QtGui.QColor("#1e293b"), 2.5, cap=QtCore.Qt.PenCapStyle.RoundCap)
            painter.setPen(key_pen)
            painter.drawLine(64, 70, 64, 78)

        else:  # idle
            # Subtle crescent moon and sleep dot
            moon_path = QtGui.QPainterPath()
            moon_path.addEllipse(QtCore.QPointF(60, 60), 18, 18)
            sub_path = QtGui.QPainterPath()
            sub_path.addEllipse(QtCore.QPointF(67, 54), 16, 16)
            crescent = moon_path.subtracted(sub_path)
            painter.setBrush(QtGui.QColor("#94a3b8"))
            painter.setPen(QtCore.Qt.PenStyle.NoPen)
            painter.drawPath(crescent)
            # Sleep star / dot
            painter.drawEllipse(QtCore.QPointF(86, 46), 3, 3)

    finally:
        painter.end()

    return image


def _render_pil_bytes(norm_state: str, size: int = 128) -> bytes:
    """Fallback PNG byte generator using PIL."""
    if not HAS_PIL:
        # Return empty bytes if PIL missing
        return b""

    img = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    draw = ImageDraw.Draw(img)

    scale = size / 128.0

    def pt(x: float, y: float) -> Tuple[int, int]:
        return (int(x * scale), int(y * scale))

    shield_pts = [pt(24, 24), pt(104, 24), pt(104, 66), pt(64, 114), pt(24, 66)]

    if norm_state == "active":
        fill = (14, 116, 144, 255)
        outline = (0, 229, 255, 255)
    elif norm_state == "cooldown":
        fill = (180, 83, 9, 255)
        outline = (245, 158, 11, 255)
    elif norm_state == "override":
        fill = (37, 99, 235, 255)
        outline = (96, 165, 250, 255)
    else:
        fill = (51, 65, 85, 255)
        outline = (100, 116, 139, 255)

    draw.polygon(shield_pts, fill=fill, outline=outline, width=max(2, int(4 * scale)))

    if norm_state == "active":
        bolt_pts = [
            pt(68, 34), pt(46, 66), pt(62, 66), pt(58, 96),
            pt(84, 60), pt(68, 60), pt(76, 34),
        ]
        draw.polygon(bolt_pts, fill=(251, 191, 36, 255), outline=(255, 255, 255, 255))
    elif norm_state == "cooldown":
        hg_pts = [pt(48, 44), pt(80, 44), pt(66, 64), pt(80, 84), pt(48, 84), pt(62, 64)]
        draw.polygon(hg_pts, fill=(253, 230, 138, 255), outline=(255, 255, 255, 255))
    elif norm_state == "override":
        draw.rectangle([pt(46, 58), pt(82, 88)], fill=(96, 165, 250, 255), outline=(255, 255, 255, 255), width=2)
    else:
        draw.ellipse([pt(52, 52), pt(76, 76)], fill=(148, 163, 184, 255))

    buf = io.BytesIO()
    img.save(buf, format="PNG")
    return buf.getvalue()


def get_status_svg(state: str) -> str:
    """Return raw SVG markup string for the requested status."""
    norm = _normalize_state(state)
    return _SVG_TEMPLATES.get(norm, _SVG_TEMPLATES["idle"])


def get_status_png(state: str, size: int = 128) -> bytes:
    """Return PNG bytes for requested status, using PyQt6 or PIL fallback."""
    norm = _normalize_state(state)
    if HAS_PYQT6 and _ensure_qt_gui_app():
        image = _render_qt_image(norm, size=size)
        buffer = QtCore.QBuffer()
        buffer.open(QtCore.QIODevice.OpenModeFlag.WriteOnly)
        image.save(buffer, "PNG")
        return bytes(buffer.data())
    return _render_pil_bytes(norm, size=size)


def get_status_pixmap(state: str, size: int = 128) -> QtGui.QPixmap:
    """Return QPixmap for requested status (requires PyQt6 and initialized QGuiApplication)."""
    if not HAS_PYQT6:
        raise RuntimeError("PyQt6 is required to generate QPixmap")
    _ensure_qt_gui_app()
    norm = _normalize_state(state)
    image = _render_qt_image(norm, size=size)
    return QtGui.QPixmap.fromImage(image)


def get_status_icon(state: str, size: int = 128) -> QtGui.QIcon:
    """
    Return a cached high-resolution QIcon for the given guard status.

    Recognized states:
    - 'idle'
    - 'active'
    - 'cooldown'
    - 'override'
    """
    if not HAS_PYQT6:
        raise RuntimeError("PyQt6 is required to generate QIcon")

    norm = _normalize_state(state)
    cache_key = f"{norm}_{size}"

    if cache_key in _ICON_CACHE:
        return _ICON_CACHE[cache_key]

    _ensure_qt_gui_app()
    pixmap = get_status_pixmap(norm, size=size)
    icon = QtGui.QIcon(pixmap)

    # Also add standard tray sizes for crisp high-DPI downscaling
    for s in (16, 22, 24, 32, 48, 64):
        if s != size:
            sub_pix = get_status_pixmap(norm, size=s)
            icon.addPixmap(sub_pix)

    _ICON_CACHE[cache_key] = icon
    return icon


def clear_icon_cache() -> None:
    """Flush the in-memory QIcon cache."""
    _ICON_CACHE.clear()
