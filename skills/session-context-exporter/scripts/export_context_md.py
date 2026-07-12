#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import re
import sys
from datetime import datetime
from pathlib import Path
from typing import Any, Iterable


DEFAULT_TITLE = "Session Context Handoff"
DEFAULT_SECTIONS = [
    ("Current Status", "TODO: Summarize where the task stands."),
    ("Key Decisions", "TODO: List important decisions and constraints."),
    ("Files And Artifacts", "TODO: List relevant files, directories, URLs, or generated artifacts."),
    ("Verification", "TODO: List commands run and their outcomes."),
    ("Open Questions", "TODO: List blockers, risks, or unknowns."),
    ("Next Steps", "TODO: Describe exactly what the next conversation should do."),
]


def slugify(value: str) -> str:
    value = value.strip().lower()
    value = re.sub(r"[^a-z0-9]+", "-", value)
    value = value.strip("-")
    return value[:60] or "session-context"


def now_stamp() -> str:
    return datetime.now().astimezone().strftime("%Y-%m-%d %H:%M:%S %z")


def filename_stamp() -> str:
    return datetime.now().astimezone().strftime("%Y%m%d-%H%M%S")


def read_json(path: str | None) -> dict[str, Any]:
    if not path:
        return {}
    if path == "-":
        raw = sys.stdin.read()
    else:
        raw = Path(path).read_text(encoding="utf-8")
    if not raw.strip():
        return {}
    data = json.loads(raw)
    if not isinstance(data, dict):
        raise SystemExit("JSON payload must be an object.")
    return data


def merge_cli_args(data: dict[str, Any], args: argparse.Namespace) -> dict[str, Any]:
    merged = dict(data)
    for key in ("title", "workspace", "objective", "status", "next_prompt"):
        value = getattr(args, key)
        if value:
            merged[key] = value
    if args.section:
        sections = list(normalize_sections(merged.get("sections")))
        for raw in args.section:
            if "=" not in raw:
                raise SystemExit("--section values must use Heading=Body format.")
            heading, body = raw.split("=", 1)
            sections.append((heading.strip(), body.strip()))
        merged["sections"] = [{"heading": h, "body": b} for h, b in sections]
    return merged


def normalize_sections(value: Any) -> list[tuple[str, Any]]:
    if not value:
        return []
    if isinstance(value, dict):
        return [(str(k), v) for k, v in value.items()]
    if isinstance(value, list):
        normalized: list[tuple[str, Any]] = []
        for index, item in enumerate(value, start=1):
            if isinstance(item, dict):
                heading = item.get("heading") or item.get("title") or f"Section {index}"
                body = item.get("body", item.get("items", item.get("content", "")))
                normalized.append((str(heading), body))
            else:
                normalized.append((f"Section {index}", item))
        return normalized
    return [("Notes", value)]


def render_value(value: Any) -> list[str]:
    if value is None:
        return []
    if isinstance(value, str):
        text = value.strip()
        return text.splitlines() if text else []
    if isinstance(value, list):
        lines: list[str] = []
        for item in value:
            if isinstance(item, (dict, list)):
                nested = " ".join(line.strip() for line in render_value(item) if line.strip())
                if nested:
                    lines.append(f"- {nested}")
            else:
                text = str(item).strip()
                if text:
                    lines.append(f"- {text}")
        return lines
    if isinstance(value, dict):
        lines = []
        for key, item in value.items():
            rendered = render_value(item)
            if len(rendered) == 1 and not rendered[0].startswith("- "):
                lines.append(f"- {key}: {rendered[0]}")
            elif rendered:
                lines.append(f"- {key}:")
                lines.extend(f"  {line}" if line.startswith("- ") else f"  - {line}" for line in rendered)
            else:
                lines.append(f"- {key}")
        return lines
    return [str(value).strip()]


def append_block(lines: list[str], heading: str, value: Any) -> None:
    rendered = render_value(value)
    if not rendered:
        return
    lines.extend(["", f"## {heading}", ""])
    lines.extend(rendered)


def render_markdown(data: dict[str, Any]) -> str:
    title = str(data.get("title") or DEFAULT_TITLE).strip()
    captured_at = str(data.get("captured_at") or now_stamp())
    workspace = str(data.get("workspace") or "").strip()

    lines = [
        f"# {title}",
        "",
        "> Markdown handoff for continuing a Codex task in a future conversation.",
        "",
        "## Metadata",
        "",
        f"- Captured at: {captured_at}",
    ]
    if workspace:
        lines.append(f"- Workspace: {workspace}")

    append_block(lines, "Objective", data.get("objective"))
    append_block(lines, "Status", data.get("status"))

    sections = normalize_sections(data.get("sections")) or DEFAULT_SECTIONS
    for heading, body in sections:
        append_block(lines, heading, body)

    append_block(lines, "Next Prompt", data.get("next_prompt"))
    lines.append("")
    return "\n".join(lines)


def resolve_output(output: str | None, title: str) -> Path:
    if output:
        path = Path(output)
        if path.suffix.lower() == ".md":
            return path
        return path / f"{filename_stamp()}-{slugify(title)}.md"
    return Path.cwd() / "context-handoffs" / f"{filename_stamp()}-{slugify(title)}.md"


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Export Codex task context as a Markdown handoff.")
    parser.add_argument("--json", help="Path to a JSON payload, or '-' to read JSON from stdin.")
    parser.add_argument("--output", help="Output .md file or directory. Defaults to context-handoffs/.")
    parser.add_argument("--title", help="Markdown title.")
    parser.add_argument("--workspace", help="Workspace path to include in metadata.")
    parser.add_argument("--objective", help="Task objective.")
    parser.add_argument("--status", help="Current task status.")
    parser.add_argument("--next-prompt", dest="next_prompt", help="Prompt to use in the next conversation.")
    parser.add_argument("--section", action="append", help="Add a section using Heading=Body. May be repeated.")
    return parser


def main(argv: Iterable[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    data = merge_cli_args(read_json(args.json), args)
    title = str(data.get("title") or DEFAULT_TITLE)
    output = resolve_output(args.output, title)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(render_markdown(data), encoding="utf-8", newline="\n")
    print(output.resolve())
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
