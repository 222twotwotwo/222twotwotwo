---
name: project-lifecycle-scripts
description: Inspect a software project, summarize its likely runtime entrypoint, and create operating-system-matched start and stop scripts. Use when the user asks Codex to overview a project, identify how it runs, generate startup/shutdown scripts, make Windows PowerShell or Unix shell lifecycle helpers, or create scripts that follow the current system.
---

# Project Lifecycle Scripts

## Workflow

1. Inspect the project before generating scripts.
   - Read the root file list and common manifests: `package.json`, lockfiles, `pyproject.toml`, `requirements*.txt`, `go.mod`, `Cargo.toml`, `pom.xml`, Gradle files, `.csproj`, Docker Compose files, and existing scripts.
   - Prefer the project's own documented or manifest-defined command over guessing.
   - Note the detected stack, selected start command, optional stop command, expected port if obvious, and any prerequisites.

2. Generate scripts for the current system by default.
   - Use `scripts/generate_lifecycle_scripts.py` with `--target auto`.
   - On Windows, generate `start-project.ps1`, `stop-project.ps1`, plus `.cmd` wrappers.
   - On Linux or macOS, generate `start-project.sh` and `stop-project.sh`.
   - Use `--target all` only when the user asks for cross-platform scripts.

3. Keep project changes explicit and safe.
   - Default output location is `<project>/scripts`.
   - Do not overwrite existing lifecycle scripts unless the user asked to regenerate them or you have inspected them and rerun with `--force`.
   - If automatic detection is uncertain, pass the chosen command explicitly with `--command`.
   - For stacks with a real shutdown command, pass `--stop-command`; Docker Compose commonly uses `docker compose down`.

4. Verify the generated scripts when feasible.
   - Run the start script only when starting the project is safe in the current environment.
   - Confirm a process starts, logs are written, or the expected local URL responds.
   - Run the stop script and confirm the process is no longer running.
   - If verification cannot be run, state exactly what was generated and what remains untested.

## Generator

Run from the target project root:

```bash
python C:/Users/ghs/.codex/skills/project-lifecycle-scripts/scripts/generate_lifecycle_scripts.py --project . --target auto
```

Useful options:

```bash
python C:/Users/ghs/.codex/skills/project-lifecycle-scripts/scripts/generate_lifecycle_scripts.py --project . --command "npm run dev"
python C:/Users/ghs/.codex/skills/project-lifecycle-scripts/scripts/generate_lifecycle_scripts.py --project . --target all --force
python C:/Users/ghs/.codex/skills/project-lifecycle-scripts/scripts/generate_lifecycle_scripts.py --project . --dry-run
```

The generator writes `project-lifecycle.json` beside the generated scripts. Use it as the project overview artifact when reporting the result.

## Detection Priority

Prefer explicit user instructions first, then project-owned commands, then conventional framework commands.

Common automatic choices:

- Node: package manager inferred from lockfile, script selected from `dev`, `start`, `serve`, then `preview`.
- Python: Django `manage.py`, FastAPI/Uvicorn, Streamlit, Flask, then simple `app.py` or `main.py`.
- Go: `go run .` or a single `cmd/<name>` main package.
- Rust: `cargo run`.
- Java: Maven or Gradle wrapper when present, otherwise installed `mvn` or `gradle`.
- .NET: `dotnet run`, with `--project` when one project file is found.
- Docker Compose: `docker compose up` and `docker compose down`.
- Static HTML: `python -m http.server 8000`.

When multiple viable commands exist, select the one that matches the user's goal and the repository's own scripts. If the generator picks the wrong command, rerun with `--command` rather than editing generated process-control logic by hand.
