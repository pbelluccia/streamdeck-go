#!/usr/bin/env python3
import fcntl
import json
import os
import shutil
import subprocess
import sys
import time
from pathlib import Path


PAGE_ID = "claude_attention"


def notifier_path() -> str:
    configured = os.environ.get("STREAMDECK_AGENT_NOTIFY")
    if configured:
        return configured
    resolved = shutil.which("streamdeck-agent-notify")
    if resolved:
        return resolved
    return str(Path.home() / ".local/bin/streamdeck-agent-notify")


def first_line(text: str, limit: int = 80) -> str:
    text = " ".join((text or "").split())
    if len(text) <= limit:
        return text
    return text[: limit - 3] + "..."


def state_path() -> str:
    runtime_dir = os.environ.get("XDG_RUNTIME_DIR")
    if runtime_dir:
        return os.path.join(runtime_dir, "streamdeck-claude-state.json")
    return os.path.join("/tmp", f"streamdeck-claude-state-{os.getuid()}.json")


def load_state(path: str) -> dict:
    try:
        with open(path, "r", encoding="utf-8") as handle:
            data = json.load(handle)
    except FileNotFoundError:
        return {}
    except Exception:
        return {}
    if not isinstance(data, dict):
        return {}
    if not isinstance(data.get("sessions"), dict):
        data["sessions"] = {}
    return data


def write_state(path: str, data: dict) -> None:
    directory = os.path.dirname(path)
    if directory:
        os.makedirs(directory, exist_ok=True)
    tmp_path = f"{path}.tmp"
    with open(tmp_path, "w", encoding="utf-8") as handle:
        json.dump(data, handle, separators=(",", ":"))
        handle.write("\n")
    os.replace(tmp_path, path)


def update_session(session_id: str, updater):
    path = state_path()
    lock_path = f"{path}.lock"
    directory = os.path.dirname(path)
    if directory:
        os.makedirs(directory, exist_ok=True)
    with open(lock_path, "w", encoding="utf-8") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        data = load_state(path)
        sessions = data.setdefault("sessions", {})
        current = sessions.get(session_id, {})
        if not isinstance(current, dict):
            current = {}
        result, next_state = updater(current)
        if next_state:
            sessions[session_id] = next_state
        else:
            sessions.pop(session_id, None)
        write_state(path, data)
        return result


def remember(session_id: str, state: str, event: dict) -> None:
    def updater(_current: dict):
        return None, {
            "state": state,
            "page": PAGE_ID,
            "turn_id": event.get("turn_id") or "",
            "updated_at": time.time(),
        }

    update_session(session_id, updater)


def clear_if_waiting(session_id: str) -> bool:
    def updater(current: dict):
        if current.get("state") != "waiting":
            return False, current
        return True, {}

    return bool(update_session(session_id, updater))


def forget(session_id: str) -> None:
    update_session(session_id, lambda _current: (None, {}))


def notifier(args: list[str]) -> None:
    subprocess.run(
        args,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        timeout=3,
        check=False,
    )


def notify(title: str, message: str, color: str) -> None:
    notifier([
        notifier_path(),
        title,
        message,
        "--agent", "Claude",
        "--page", PAGE_ID,
        "--color", color,
    ])


def clear_notification() -> None:
    notifier([
        notifier_path(),
        "--clear",
        "--agent", "Claude",
        "--page", PAGE_ID,
    ])


def clear_waiting(session_id: str) -> None:
    if clear_if_waiting(session_id):
        clear_notification()


def main() -> int:
    try:
        event = json.load(sys.stdin)
    except Exception:
        event = {}

    session_id = event.get("session_id") or "global"
    name = event.get("hook_event_name", "")

    # PermissionRequest is the authoritative "needs permission" signal. It fires
    # when the approval dialog appears and is matchable by tool name. We do not
    # rely on the Notification event for this: its stdin payload does not carry
    # a notification_type field, so permission and idle notifications are
    # indistinguishable there.
    if name == "PermissionRequest":
        remember(session_id, "waiting", event)
        notify("Claude", "", "#f59e0b")
        return 0

    # Any of these means the gated tool resolved (ran, errored, or was denied),
    # so the pending permission alert must come down.
    if name in ("PostToolUse", "PostToolUseFailure", "PermissionDenied"):
        clear_waiting(session_id)
        return 0

    if name == "Stop":
        remember(session_id, "done", event)
        notify("Claude", "", "#16a34a")
        return 0

    if name == "StopFailure":
        notify("Claude", "", "#dc2626")
        return 0

    if name == "UserPromptSubmit":
        forget(session_id)
        clear_notification()
        return 0

    # Notification (idle_prompt, auth_success, elicitation_*) is intentionally
    # ignored. Reacting to idle_prompt used to repaint the deck with a
    # permission-looking alert after the turn had already finished.
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
