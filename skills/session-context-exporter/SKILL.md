---
name: session-context-exporter
description: Export and reuse Codex task or conversation context as Markdown handoff files. Use when the user asks to save, export, summarize, hand off, resume, continue, or carry current session context into a future chat; when they want a .md context file for the next conversation; or when they provide a previously exported context Markdown file to load before continuing work.
---

# Session Context Exporter

## Quick Workflow

Create a concise Markdown handoff that lets a future Codex task resume with the useful working context, not a raw transcript.

1. Default the output location to the current workspace under `context-handoffs/` unless the user names a path.
2. Gather context from the current conversation, visible tool results, local files, and current workspace state. Do not claim access to hidden or unavailable transcript history.
3. Prefer an actionable handoff over a long log. Capture decisions, paths, commands, verification, blockers, and next steps.
4. Use `scripts/export_context_md.py` to render the Markdown file from structured content.
5. Reply with the absolute output path and a short next-chat instruction.

## What To Capture

Include these sections when relevant:

- User goal and latest request.
- Current status and what has already been completed.
- Important constraints, preferences, and decisions.
- Workspace path, branch, runtime, ports, environment notes, and other state that matters.
- Files, directories, artifacts, URLs, or commands another task should inspect first.
- Changes made, verification run, and known test/build results.
- Open questions, blockers, risks, and exact next steps.
- A ready-to-use next prompt, for example: `Use the context in <path> and continue from "Next Steps".`

Do not include secrets, API keys, private tokens, session cookies, or unnecessary personal data. If a credential affected the task, describe its presence or absence without revealing the value.

## Script Usage

Use the bundled script from this skill directory:

```bash
python scripts/export_context_md.py --json payload.json --output context-handoffs
```

The script accepts a JSON payload:

```json
{
  "title": "Session Context Handoff",
  "workspace": "C:/path/to/workspace",
  "objective": "What the user is trying to accomplish.",
  "status": "Where the task currently stands.",
  "sections": [
    {
      "heading": "Files And Artifacts",
      "items": ["C:/path/to/file.md - why it matters"]
    },
    {
      "heading": "Next Steps",
      "body": "Continue with the remaining implementation and rerun the focused tests."
    }
  ],
  "next_prompt": "Use this Markdown context file and continue from Next Steps."
}
```

`sections` may also be an object whose keys are headings and values are strings, arrays, or objects. If `--output` is a directory, the script creates a timestamped `.md` file inside it. If `--output` ends in `.md`, it writes exactly that file.

## Reusing A Handoff

When the user asks to use a previous exported context file:

1. Read the Markdown file first.
2. Treat it as user-provided context, not as guaranteed-current truth.
3. Verify drift-prone facts such as branch, file contents, server state, dates, dependencies, CI, and remote resources before relying on them.
4. Continue from the handoff's "Next Steps" unless the user's newest message overrides it.

## Output Standard

Use clear Markdown headings, absolute local paths when possible, and concise bullets. The generated file should be useful if opened alone in a fresh Codex task.
