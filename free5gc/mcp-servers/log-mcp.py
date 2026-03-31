#!/usr/bin/env python3
"""
Log MCP Server — structured access to free5gc shared log file.

Parses the free5gc log format:
  time="..." level="..." msg="..." CAT="..." NF="..." [extra_key="..."]

Tools:
  get_log_tail         — Last N lines from the active log
  get_errors           — Filtered errors/warnings, optionally by NF
  get_nf_activity      — Which NFs are logging and when they were last seen
  get_log_stats        — Error counts, event rates, log size, growth rate
  search_log           — Regex search with structured results
  get_log_snapshot     — Combined tail + stats + errors in one call
"""

import json
import os
import re
import time
from collections import Counter, defaultdict
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any

from mcp.server.fastmcp import FastMCP

LOG_BASE = Path("/home/henokbfg/Documents/5g-testbed-conference/free5gc/log")

# Regex to parse structured log lines
LOG_RE = re.compile(
    r'time="(?P<time>[^"]+)"\s+'
    r'level="(?P<level>[^"]+)"\s+'
    r'msg="(?P<msg>(?:[^"\\]|\\.|"(?!\s))*)"\s*'
    r'(?P<extra>.*)'
)
EXTRA_KV_RE = re.compile(r'(\w+)="([^"]*)"')

mcp = FastMCP(
    "log",
    instructions=(
        "Structured access to the free5gc shared log file. "
        "Query recent logs, filter errors by NF, search patterns, "
        "and get log statistics. The 5G core must be running for fresh data."
    ),
)


def _find_active_log() -> Path | None:
    """Find the most recent log directory and return the log file path."""
    if not LOG_BASE.exists():
        return None
    dirs = sorted(LOG_BASE.iterdir(), key=lambda d: d.name, reverse=True)
    for d in dirs:
        log_file = d / "free5gc.log"
        if log_file.exists():
            return log_file
    return None


def _parse_line(line: str) -> dict[str, Any] | None:
    """Parse a single log line into structured fields."""
    m = LOG_RE.match(line.strip())
    if not m:
        return None
    entry = {
        "time": m.group("time"),
        "level": m.group("level"),
        "msg": m.group("msg").replace('\\"', '"'),
    }
    extra = m.group("extra")
    if extra:
        for k, v in EXTRA_KV_RE.findall(extra):
            entry[k] = v
    return entry


def _read_tail(log_file: Path, lines: int) -> list[str]:
    """Read last N lines from log file efficiently."""
    try:
        with open(log_file, "rb") as f:
            f.seek(0, 2)
            size = f.tell()
            # Read from end in chunks
            chunk_size = min(size, lines * 500)  # estimate ~500 bytes/line
            f.seek(max(0, size - chunk_size))
            data = f.read().decode("utf-8", errors="replace")
            all_lines = data.splitlines()
            return all_lines[-lines:]
    except Exception:
        return []


def _fmt(data: Any) -> str:
    return json.dumps(data, indent=2, default=str)


@mcp.tool()
async def get_log_tail(lines: int = 50, nf: str = "") -> str:
    """Get the last N lines from the active log file.
    Optional: filter by NF name (e.g. 'AMF', 'SMF', 'UPF').
    Returns parsed structured log entries."""
    log_file = _find_active_log()
    if not log_file:
        return "No active log file found"

    # Read more lines than requested if filtering, to ensure enough results
    read_count = lines * 5 if nf else lines
    raw_lines = _read_tail(log_file, read_count)

    entries = []
    for line in raw_lines:
        parsed = _parse_line(line)
        if parsed:
            if nf and parsed.get("NF", "").upper() != nf.upper():
                continue
            entries.append(parsed)
            if len(entries) >= lines:
                break

    return _fmt({
        "log_file": str(log_file),
        "count": len(entries),
        "entries": entries[-lines:],
    })


@mcp.tool()
async def get_errors(nf: str = "", last_minutes: int = 0, limit: int = 100) -> str:
    """Get warning and error log entries from the active log.
    Optional: filter by NF name, limit to last N minutes, cap results.
    Returns structured error entries with counts by NF and level."""
    log_file = _find_active_log()
    if not log_file:
        return "No active log file found"

    cutoff = None
    if last_minutes > 0:
        cutoff = datetime.now(timezone(timedelta(hours=4))) - timedelta(minutes=last_minutes)

    # Read a generous chunk from the end
    raw_lines = _read_tail(log_file, 5000)

    errors = []
    nf_counts: Counter = Counter()
    level_counts: Counter = Counter()

    for line in raw_lines:
        parsed = _parse_line(line)
        if not parsed:
            continue
        if parsed["level"] not in ("warning", "error"):
            continue
        if nf and parsed.get("NF", "").upper() != nf.upper():
            continue
        if cutoff:
            try:
                line_time = datetime.fromisoformat(parsed["time"])
                if line_time < cutoff:
                    continue
            except (ValueError, TypeError):
                pass

        nf_name = parsed.get("NF", "unknown")
        nf_counts[nf_name] += 1
        level_counts[parsed["level"]] += 1
        if len(errors) < limit:
            errors.append(parsed)

    return _fmt({
        "log_file": str(log_file),
        "total_errors": sum(level_counts.values()),
        "by_level": dict(level_counts),
        "by_nf": dict(nf_counts),
        "entries": errors,
    })


@mcp.tool()
async def get_nf_activity() -> str:
    """Get activity summary for each NF — last seen timestamp, message count,
    and whether it appears active. Useful for detecting crashed or silent NFs."""
    log_file = _find_active_log()
    if not log_file:
        return "No active log file found"

    raw_lines = _read_tail(log_file, 2000)

    nf_data: dict[str, dict] = {}
    for line in raw_lines:
        parsed = _parse_line(line)
        if not parsed:
            continue
        nf_name = parsed.get("NF")
        if not nf_name:
            continue
        if nf_name not in nf_data:
            nf_data[nf_name] = {
                "nf": nf_name,
                "first_seen": parsed["time"],
                "last_seen": parsed["time"],
                "message_count": 0,
                "levels": Counter(),
            }
        nf_data[nf_name]["last_seen"] = parsed["time"]
        nf_data[nf_name]["message_count"] += 1
        nf_data[nf_name]["levels"][parsed["level"]] += 1

    # Convert counters to dicts for JSON
    result = []
    for nf_info in nf_data.values():
        nf_info["levels"] = dict(nf_info["levels"])
        result.append(nf_info)

    # Sort by last_seen descending
    result.sort(key=lambda x: x["last_seen"], reverse=True)

    return _fmt({
        "log_file": str(log_file),
        "active_nfs": len(result),
        "nfs": result,
    })


@mcp.tool()
async def get_log_stats() -> str:
    """Get log file statistics — total lines, size, growth rate,
    error/warning counts, message distribution by NF and level."""
    log_file = _find_active_log()
    if not log_file:
        return "No active log file found"

    stat = log_file.stat()
    size_bytes = stat.st_size
    modified = datetime.fromtimestamp(stat.st_mtime)

    # Count total lines
    total_lines = 0
    with open(log_file, "rb") as f:
        for _ in f:
            total_lines += 1

    # Parse last 2000 lines for stats
    raw_lines = _read_tail(log_file, 2000)
    level_counts: Counter = Counter()
    nf_counts: Counter = Counter()
    cat_counts: Counter = Counter()
    first_time = None
    last_time = None

    for line in raw_lines:
        parsed = _parse_line(line)
        if not parsed:
            continue
        level_counts[parsed["level"]] += 1
        nf = parsed.get("NF")
        if nf:
            nf_counts[nf] += 1
        cat = parsed.get("CAT")
        if cat:
            cat_counts[cat] += 1
        if not first_time:
            first_time = parsed["time"]
        last_time = parsed["time"]

    # Calculate growth rate
    growth_rate = None
    if first_time and last_time:
        try:
            t1 = datetime.fromisoformat(first_time)
            t2 = datetime.fromisoformat(last_time)
            span_secs = (t2 - t1).total_seconds()
            if span_secs > 0:
                growth_rate = f"{len(raw_lines) / span_secs:.1f} lines/sec"
        except (ValueError, TypeError):
            pass

    # Log directory name (contains start timestamp)
    log_dir_name = log_file.parent.name

    return _fmt({
        "log_file": str(log_file),
        "log_session": log_dir_name,
        "total_lines": total_lines,
        "size_bytes": size_bytes,
        "size_mb": round(size_bytes / 1024 / 1024, 2),
        "last_modified": str(modified),
        "growth_rate": growth_rate,
        "sample_window": {
            "lines_sampled": len(raw_lines),
            "from": first_time,
            "to": last_time,
        },
        "by_level": dict(level_counts),
        "by_nf": dict(nf_counts),
        "top_categories": dict(cat_counts.most_common(10)),
    })


@mcp.tool()
async def search_log(pattern: str, lines: int = 2000, limit: int = 50) -> str:
    """Search the active log for a regex pattern.
    Searches the last N lines (default 2000) and returns up to limit matches.
    Returns parsed structured entries where the raw line matches the pattern."""
    log_file = _find_active_log()
    if not log_file:
        return "No active log file found"

    try:
        regex = re.compile(pattern, re.IGNORECASE)
    except re.error as e:
        return f"Invalid regex pattern: {e}"

    raw_lines = _read_tail(log_file, lines)
    matches = []

    for line in raw_lines:
        if regex.search(line):
            parsed = _parse_line(line)
            if parsed:
                matches.append(parsed)
            else:
                matches.append({"raw": line.strip()})
            if len(matches) >= limit:
                break

    return _fmt({
        "log_file": str(log_file),
        "pattern": pattern,
        "lines_searched": len(raw_lines),
        "match_count": len(matches),
        "matches": matches,
    })


@mcp.tool()
async def get_log_snapshot() -> str:
    """Get a combined snapshot: log stats + recent errors + NF activity.
    Single call for a quick health check of the core."""
    log_file = _find_active_log()
    if not log_file:
        return "No active log file found"

    stat = log_file.stat()
    raw_lines = _read_tail(log_file, 2000)

    level_counts: Counter = Counter()
    nf_last_seen: dict[str, str] = {}
    nf_msg_counts: Counter = Counter()
    recent_errors: list[dict] = []

    for line in raw_lines:
        parsed = _parse_line(line)
        if not parsed:
            continue
        level_counts[parsed["level"]] += 1
        nf = parsed.get("NF")
        if nf:
            nf_last_seen[nf] = parsed["time"]
            nf_msg_counts[nf] += 1
        if parsed["level"] in ("warning", "error"):
            if len(recent_errors) < 20:
                recent_errors.append(parsed)

    return _fmt({
        "log_file": str(log_file),
        "log_session": log_file.parent.name,
        "size_mb": round(stat.st_size / 1024 / 1024, 2),
        "total_lines_sampled": len(raw_lines),
        "by_level": dict(level_counts),
        "nf_activity": {
            nf: {"last_seen": nf_last_seen[nf], "messages": nf_msg_counts[nf]}
            for nf in sorted(nf_last_seen.keys())
        },
        "recent_errors": recent_errors,
    })


if __name__ == "__main__":
    mcp.run()
