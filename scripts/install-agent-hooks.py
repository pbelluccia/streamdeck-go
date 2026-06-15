#!/usr/bin/env python3
import argparse
import json
import os
from pathlib import Path
from typing import Any


def load_json(path: Path) -> dict[str, Any]:
    if not path.exists():
        return {}
    with path.open("r", encoding="utf-8") as handle:
        data = json.load(handle)
    if not isinstance(data, dict):
        raise ValueError(f"{path} must contain a JSON object")
    return data


def write_json(path: Path, data: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_suffix(path.suffix + ".tmp")
    with tmp.open("w", encoding="utf-8") as handle:
        json.dump(data, handle, indent=2)
        handle.write("\n")
    os.replace(tmp, path)


def command_hook(command: str, status: str | None = None) -> dict[str, Any]:
    hook: dict[str, Any] = {
        "type": "command",
        "command": command,
        "timeout": 5,
    }
    if status:
        hook["statusMessage"] = status
    return hook


def remove_matching_command(groups: list[Any], command: str) -> list[Any]:
    cleaned = []
    for group in groups:
        if not isinstance(group, dict):
            cleaned.append(group)
            continue
        hooks = group.get("hooks")
        if not isinstance(hooks, list):
            cleaned.append(group)
            continue
        next_hooks = [
            hook
            for hook in hooks
            if not (isinstance(hook, dict) and hook.get("command") == command)
        ]
        if next_hooks:
            next_group = dict(group)
            next_group["hooks"] = next_hooks
            cleaned.append(next_group)
    return cleaned


def add_event_hook(data: dict[str, Any], event: str, group: dict[str, Any], command: str) -> None:
    hooks = data.setdefault("hooks", {})
    if not isinstance(hooks, dict):
        raise ValueError("hooks must be a JSON object")
    groups = hooks.get(event, [])
    if not isinstance(groups, list):
        raise ValueError(f"hooks.{event} must be a JSON array")
    groups = remove_matching_command(groups, command)
    groups.append(group)
    hooks[event] = groups


def remove_command_from_all_events(data: dict[str, Any], command: str) -> None:
    hooks = data.get("hooks")
    if hooks is None:
        return
    if not isinstance(hooks, dict):
        raise ValueError("hooks must be a JSON object")
    for event in list(hooks):
        groups = hooks[event]
        if not isinstance(groups, list):
            raise ValueError(f"hooks.{event} must be a JSON array")
        groups = remove_matching_command(groups, command)
        if groups:
            hooks[event] = groups
        else:
            del hooks[event]


def install_codex(path: Path, command: str) -> None:
    data = load_json(path)
    remove_command_from_all_events(data, command)
    add_event_hook(
        data,
        "PermissionRequest",
        {
            "hooks": [
                command_hook(command, "Notificando aprobacion en Stream Deck"),
            ],
        },
        command,
    )
    add_event_hook(
        data,
        "Stop",
        {
            "hooks": [
                command_hook(command, "Notificando fin en Stream Deck"),
            ],
        },
        command,
    )
    add_event_hook(
        data,
        "PostToolUse",
        {
            "hooks": [
                command_hook(command),
            ],
        },
        command,
    )
    add_event_hook(
        data,
        "UserPromptSubmit",
        {
            "hooks": [
                command_hook(command),
            ],
        },
        command,
    )
    write_json(path, data)


def install_claude(path: Path, command: str) -> None:
    data = load_json(path)
    remove_command_from_all_events(data, command)
    add_event_hook(
        data,
        "PermissionRequest",
        {
            "hooks": [
                command_hook(command, "Notificando aprobacion en Stream Deck"),
            ],
        },
        command,
    )
    add_event_hook(
        data,
        "Stop",
        {
            "hooks": [
                command_hook(command),
            ],
        },
        command,
    )
    add_event_hook(
        data,
        "StopFailure",
        {
            "hooks": [
                command_hook(command),
            ],
        },
        command,
    )
    for event in ("PostToolUse", "PostToolUseFailure", "PermissionDenied"):
        add_event_hook(
            data,
            event,
            {
                "hooks": [
                    command_hook(command),
                ],
            },
            command,
        )
    add_event_hook(
        data,
        "UserPromptSubmit",
        {
            "hooks": [
                command_hook(command),
            ],
        },
        command,
    )
    write_json(path, data)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--agent", choices=("codex", "claude"), required=True)
    parser.add_argument("--config", required=True)
    parser.add_argument("--command", required=True)
    args = parser.parse_args()

    path = Path(args.config).expanduser()
    if args.agent == "codex":
        install_codex(path, args.command)
    else:
        install_claude(path, args.command)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
