#!/usr/bin/env python3
import argparse
import json
import os
import socket
import sys
import textwrap


def default_socket_path() -> str:
    runtime_dir = os.environ.get("XDG_RUNTIME_DIR")
    if runtime_dir:
        return os.path.join(runtime_dir, "streamdeck-go.sock")
    return os.path.join("/tmp", f"streamdeck-go-{os.getuid()}.sock")


def clean_text(value: str) -> str:
    return " ".join((value or "").split())


def split_message(text: str) -> list[str]:
    lines = textwrap.wrap(clean_text(text), width=14, max_lines=2, placeholder="...")
    if not lines:
        lines = ["Atencion requerida"]
    while len(lines) < 2:
        lines.append("")
    return lines[:2]


def text_layer(text: str, font_size: int) -> dict:
    return {
        "type": "text",
        "text": text,
        "font_size": font_size,
        "color": "#ffffff",
        "outline_color": "#111827",
    }


def color_layer(color: str, effect: dict | None = None) -> dict:
    layer = {
        "type": "color",
        "color": color,
    }
    if effect:
        layer["effect"] = effect
    return layer


def button(color: str, text: str, font_size: int = 14, effect: dict | None = None) -> dict:
    return {
        "layers": [
            color_layer(color, effect),
            text_layer(text, font_size),
        ],
        "press": {"type": "page", "page": "main"},
    }


def blink_button(color: str, text: str = "", font_size: int = 17) -> dict:
    effect = {"type": "blink", "color": "#0f172a", "blink_ms": 450}
    layers = [color_layer(color, effect)]
    if text:
        layers.append(text_layer(text, font_size))
    return {"layers": layers, "press": {"type": "page", "page": "main"}}


def page(agent: str, title: str, message: str, color: str, timeout_seconds: int = 0) -> dict:
    dark = "#0f172a"
    payload = {
        "background": {"type": "solid", "color": dark},
        "buttons": {
            "0": blink_button(color),
            "1": blink_button(color, agent.upper(), 17),
            "2": blink_button(color),
            "3": blink_button(color),
            "4": blink_button(color),
            "5": blink_button(color),
        },
    }
    if timeout_seconds > 0:
        payload["timeout_seconds"] = timeout_seconds
    return payload


def send_request(socket_path: str, method: str, path: str, payload: dict | None = None) -> None:
    body = b""
    if payload is not None:
        body = json.dumps(payload, separators=(",", ":")).encode("utf-8")
    request = (
        f"{method} {path} HTTP/1.1\r\n"
        "Host: streamdeck\r\n"
        f"Content-Length: {len(body)}\r\n"
        "Connection: close\r\n"
    )
    if payload is not None:
        request += "Content-Type: application/json\r\n"
    request = (request + "\r\n").encode("utf-8") + body

    with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as client:
        client.settimeout(1.5)
        client.connect(socket_path)
        client.sendall(request)
        response = client.recv(128)
    if b" 200 " not in response:
        first_line = response.decode("utf-8", errors="replace").splitlines()[0]
        raise RuntimeError(first_line)


def send_page(socket_path: str, page_id: str, payload: dict) -> None:
    send_request(socket_path, "PUT", f"/pages/{page_id}", payload)


def clear_page(socket_path: str, page_id: str) -> None:
    send_request(socket_path, "POST", f"/pages/{page_id}/clear")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("title", nargs="?")
    parser.add_argument("message", nargs="?", default="")
    parser.add_argument("--agent", default="Agent")
    parser.add_argument("--page", default="")
    parser.add_argument("--color", default="#dc2626")
    parser.add_argument("--clear", action="store_true")
    parser.add_argument("--timeout-seconds", type=int, default=0)
    parser.add_argument("--socket", default=default_socket_path())
    args = parser.parse_args()

    agent = clean_text(args.agent) or "Agent"
    page_id = args.page or f"{agent.lower().replace(' ', '_')}_attention"
    try:
        if args.clear:
            clear_page(args.socket, page_id)
        else:
            title = clean_text(args.title or "") or "Atencion requerida"
            send_page(
                args.socket,
                page_id,
                page(agent, title, args.message, args.color, args.timeout_seconds),
            )
    except Exception as exc:
        print(f"streamdeck-agent-notify: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
