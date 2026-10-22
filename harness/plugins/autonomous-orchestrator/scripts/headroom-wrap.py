#!/usr/bin/env python3
"""Transparent subprocess execution, content-aware token compression, and Reversible CCR caching."""

import hashlib
import json
import os
import re
import subprocess
import sys
from pathlib import Path


def compress_payload(raw_output: str) -> str:
    """Perform content-aware compression using headroom or deterministic fallback."""
    # 1. Try importing headroom library if available
    try:
        from headroom import compress

        res = compress([{"role": "user", "content": raw_output}])
        candidate = ""
        if isinstance(res, list) and len(res) > 0 and isinstance(res[0], dict):
            candidate = res[0].get("content", "")
        elif isinstance(res, str):
            candidate = res
        if candidate and len(candidate) < len(raw_output):
            return candidate
    except Exception:
        pass

    # 2. JSON SmartCrusher
    stripped = raw_output.lstrip()
    if stripped.startswith(("{", "[")):
        try:
            data = json.loads(stripped)
            modified = False
            if isinstance(data, list):
                if len(data) > 6:
                    omitted = len(data) - 5
                    data = (
                        data[:3]
                        + [{"_headroom_omitted": f"... {omitted} items omitted ..."}]
                        + data[-2:]
                    )
                    modified = True
            elif isinstance(data, dict):
                for k, v in list(data.items()):
                    if isinstance(v, list) and len(v) > 6:
                        omitted = len(v) - 5
                        data[k] = (
                            v[:3]
                            + [{"_headroom_omitted": f"... {omitted} items omitted ..."}]
                            + v[-2:]
                        )
                        modified = True
            if modified:
                compacted = json.dumps(data, indent=2)
                if len(compacted) < len(raw_output):
                    return compacted
        except Exception:
            pass

    # 3. Git Diff Structural Compressor (DiffCrusher)
    diff_lines = raw_output.splitlines()
    if "diff --git " in raw_output and len(diff_lines) > 60:
        header_prefixes = (
            "diff --git ",
            "index ",
            "--- ",
            "+++ ",
            "@@ ",
            "new file mode",
            "deleted file mode",
            "rename ",
        )
        compressed_lines = []
        current_hunk_body = []

        def flush_hunk(body: list) -> list:
            if len(body) > 16:
                omitted = len(body) - 16
                return (
                    body[:12]
                    + [f"... [{omitted} hunk lines compressed by Headroom CCR] ..."]
                    + body[-4:]
                )
            return body

        for line in diff_lines:
            if line.startswith(header_prefixes):
                if current_hunk_body:
                    compressed_lines.extend(flush_hunk(current_hunk_body))
                    current_hunk_body = []
                compressed_lines.append(line)
            else:
                current_hunk_body.append(line)

        if current_hunk_body:
            compressed_lines.extend(flush_hunk(current_hunk_body))

        compressed_diff = "\n".join(compressed_lines)
        if len(compressed_diff) < len(raw_output):
            return compressed_diff

    # 4. Error-Preserving Log & Output Crusher
    threshold = int(os.environ.get("AGY_COMPRESSION_THRESHOLD", "1500"))
    lines = raw_output.splitlines()
    if len(lines) > 60:
        head = lines[:25]
        tail = lines[-25:]
        middle = lines[25:-25]
        critical_re = re.compile(
            r"(?i)(error|fail|failed|traceback|exception|assert|panic:|fatal|warning:)"
        )
        selected_indices = set()
        for idx, line in enumerate(middle):
            if critical_re.search(line):
                for ctx_idx in range(max(0, idx - 2), min(len(middle), idx + 3)):
                    selected_indices.add(ctx_idx)
                    if len(selected_indices) >= 40:
                        break
            if len(selected_indices) >= 40:
                break

        sorted_indices = sorted(selected_indices)
        omitted_count = len(middle) - len(sorted_indices)
        divider = f"\n... [{omitted_count} routine lines compressed by Headroom CCR] ...\n"
        if sorted_indices:
            middle_lines = [middle[i] for i in sorted_indices]
            result = (
                "\n".join(head)
                + divider
                + "\n".join(middle_lines)
                + "\n"
                + "\n".join(tail)
            )
        else:
            result = "\n".join(head) + divider + "\n".join(tail)
        return result
    elif len(raw_output) >= threshold:
        first_part = raw_output[:700]
        last_part = raw_output[-500:]
        omitted_chars = len(raw_output) - 1200
        divider = f"\n... [{omitted_chars} chars compressed by Headroom CCR] ...\n"
        return first_part + divider + last_part

    return raw_output


def main() -> None:
    # 1. Handle --dry-run
    if sys.argv[1:] == ["--dry-run"]:
        print(json.dumps({"status": "dry-run", "message": "headroom-wrap.py is operational"}))
        sys.exit(0)

    # 2. Strip leading '--' separator
    args = list(sys.argv[1:])
    if args and args[0] == "--":
        args = args[1:]
    if not args:
        sys.exit(0)

    cmd = args

    # 3. Execute target command
    try:
        if len(cmd) == 1:
            proc = subprocess.run(
                cmd[0],
                shell=True,
                stdout=subprocess.PIPE,
                stderr=subprocess.STDOUT,
                text=True,
            )
        else:
            try:
                proc = subprocess.run(
                    cmd,
                    shell=False,
                    stdout=subprocess.PIPE,
                    stderr=subprocess.STDOUT,
                    text=True,
                )
            except FileNotFoundError:
                proc = subprocess.run(
                    " ".join(cmd),
                    shell=True,
                    stdout=subprocess.PIPE,
                    stderr=subprocess.STDOUT,
                    text=True,
                )
    except Exception as e:
        sys.stderr.write(f"headroom-wrap execution error: {e}\n")
        sys.exit(1)

    raw_output = proc.stdout if proc.stdout is not None else ""

    # 4. Check compression threshold and disable toggle
    threshold = int(os.environ.get("AGY_COMPRESSION_THRESHOLD", "1500"))
    if len(raw_output) < threshold or os.environ.get("AGY_DISABLE_TOKEN_COMPRESSION") == "1":
        sys.stdout.write(raw_output)
        sys.stdout.flush()
        sys.exit(proc.returncode)

    # 5. Reversible CCR Cache storage
    content_hash = hashlib.sha256(raw_output.encode("utf-8", errors="replace")).hexdigest()[:12]
    cache_dir = Path.home() / ".cache" / "antigravity-headroom"
    cache_file = cache_dir / f"{content_hash}.txt"
    try:
        cache_dir.mkdir(parents=True, exist_ok=True)
        cache_file.write_text(raw_output, encoding="utf-8", errors="replace")
        txt_files = sorted(cache_dir.glob("*.txt"), key=lambda p: p.stat().st_mtime)
        if len(txt_files) > 100:
            for old_file in txt_files[:-100]:
                try:
                    old_file.unlink()
                except OSError:
                    pass
    except Exception:
        pass

    # 6. Compress payload
    compressed = compress_payload(raw_output)
    orig_len = len(raw_output)
    comp_len = len(compressed)
    saved_pct = round((1.0 - (comp_len / orig_len)) * 100.0, 1) if orig_len > 0 else 0.0

    breadcrumb = (
        f"\n[Headroom CCR: compressed {orig_len} -> {comp_len} chars "
        f"({saved_pct}% saved). Full output cached at {cache_file}]\n"
    )

    sys.stdout.write(compressed.rstrip("\n") + breadcrumb)
    sys.stdout.flush()
    sys.exit(proc.returncode)


if __name__ == "__main__":
    main()
