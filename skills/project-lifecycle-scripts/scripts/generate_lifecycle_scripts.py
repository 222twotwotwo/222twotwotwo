#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import platform
import shlex
import stat
import sys
import textwrap
from dataclasses import asdict, dataclass, field
from pathlib import Path


@dataclass
class Plan:
    stack: str
    start_command: str
    stop_command: str = ""
    evidence: list[str] = field(default_factory=list)
    notes: list[str] = field(default_factory=list)


def read_text(path: Path) -> str:
    try:
        return path.read_text(encoding="utf-8", errors="ignore")
    except OSError:
        return ""


def load_json(path: Path) -> dict:
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return {}


def ps_quote(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


def make_executable(path: Path) -> None:
    current = path.stat().st_mode
    path.chmod(current | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)


def detect_package_manager(root: Path) -> str:
    lockfiles = [
        ("pnpm-lock.yaml", "pnpm"),
        ("yarn.lock", "yarn"),
        ("bun.lockb", "bun"),
        ("bun.lock", "bun"),
        ("package-lock.json", "npm"),
    ]
    for filename, manager in lockfiles:
        if (root / filename).exists():
            return manager
    return "npm"


def package_script_command(manager: str, script_name: str) -> str:
    if manager == "yarn":
        return f"yarn {script_name}"
    return f"{manager} run {script_name}"


def detect_node(root: Path, _target: str) -> Plan | None:
    package_json = root / "package.json"
    if not package_json.exists():
        return None
    data = load_json(package_json)
    scripts = data.get("scripts") if isinstance(data.get("scripts"), dict) else {}
    for script_name in ("dev", "start", "serve", "preview"):
        if script_name in scripts:
            manager = detect_package_manager(root)
            notes = []
            if not (root / "node_modules").exists():
                notes.append("Install dependencies before starting if node_modules is missing.")
            return Plan(
                stack="Node.js",
                start_command=package_script_command(manager, script_name),
                evidence=[f"package.json scripts.{script_name}", f"package manager: {manager}"],
                notes=notes,
            )
    return None


def dependency_blob(root: Path) -> str:
    parts = []
    for pattern in ("pyproject.toml", "requirements*.txt", "Pipfile", "setup.cfg", "setup.py"):
        for path in sorted(root.glob(pattern)):
            parts.append(read_text(path))
    return "\n".join(parts).lower()


def module_from_path(path: Path, root: Path) -> str:
    rel = path.relative_to(root).with_suffix("")
    parts = list(rel.parts)
    if parts and parts[0] == "src" and len(parts) > 1:
        parts = parts[1:]
    return ".".join(parts)


def uvicorn_command(path: Path, root: Path) -> str:
    module = module_from_path(path, root)
    rel_parts = path.relative_to(root).parts
    if rel_parts and rel_parts[0] == "src":
        return f"python -m uvicorn {module}:app --reload --app-dir src"
    return f"python -m uvicorn {module}:app --reload"


def detect_python(root: Path, _target: str) -> Plan | None:
    manage_py = root / "manage.py"
    if manage_py.exists():
        return Plan(
            stack="Python Django",
            start_command="python manage.py runserver",
            evidence=["manage.py"],
        )

    blob = dependency_blob(root)
    has_python_manifest = bool(blob) or any((root / name).exists() for name in ("app.py", "main.py"))

    if "fastapi" in blob or "uvicorn" in blob:
        for candidate in (
            root / "main.py",
            root / "app.py",
            root / "app" / "main.py",
            root / "src" / "main.py",
            root / "src" / "app" / "main.py",
        ):
            if candidate.exists():
                return Plan(
                    stack="Python ASGI",
                    start_command=uvicorn_command(candidate, root),
                    evidence=[candidate.relative_to(root).as_posix(), "FastAPI/Uvicorn dependency"],
                )
        return Plan(
            stack="Python ASGI",
            start_command="python -m uvicorn main:app --reload",
            evidence=["FastAPI/Uvicorn dependency"],
            notes=["Verify the module path if the ASGI app is not main:app."],
        )

    if "streamlit" in blob:
        for candidate in (root / "streamlit_app.py", root / "app.py", root / "main.py"):
            if candidate.exists():
                return Plan(
                    stack="Python Streamlit",
                    start_command=f"python -m streamlit run {candidate.relative_to(root).as_posix()}",
                    evidence=[candidate.relative_to(root).as_posix(), "Streamlit dependency"],
                )

    if "flask" in blob:
        for candidate in (root / "app.py", root / "main.py"):
            if candidate.exists():
                module = candidate.stem
                return Plan(
                    stack="Python Flask",
                    start_command=f"python -m flask --app {module} run --debug",
                    evidence=[candidate.name, "Flask dependency"],
                )

    if has_python_manifest:
        for candidate in (root / "app.py", root / "main.py"):
            if candidate.exists():
                return Plan(
                    stack="Python",
                    start_command=f"python {candidate.name}",
                    evidence=[candidate.name],
                    notes=["Generic Python entrypoint detected; verify it starts the intended service."],
                )
    return None


def detect_go(root: Path, _target: str) -> Plan | None:
    if not (root / "go.mod").exists():
        return None
    if (root / "main.go").exists():
        command = "go run ."
        evidence = ["go.mod", "main.go"]
    else:
        cmd_mains = sorted((root / "cmd").glob("*/main.go")) if (root / "cmd").exists() else []
        if len(cmd_mains) == 1:
            command = f"go run ./cmd/{cmd_mains[0].parent.name}"
            evidence = ["go.mod", cmd_mains[0].relative_to(root).as_posix()]
        else:
            command = "go run ."
            evidence = ["go.mod"]
    return Plan(stack="Go", start_command=command, evidence=evidence)


def detect_rust(root: Path, _target: str) -> Plan | None:
    if (root / "Cargo.toml").exists():
        return Plan(stack="Rust", start_command="cargo run", evidence=["Cargo.toml"])
    return None


def detect_java(root: Path, target: str) -> Plan | None:
    if (root / "pom.xml").exists():
        if target == "windows" and (root / "mvnw.cmd").exists():
            command = ".\\mvnw.cmd spring-boot:run"
            evidence = ["pom.xml", "mvnw.cmd"]
        elif target != "windows" and (root / "mvnw").exists():
            command = "./mvnw spring-boot:run"
            evidence = ["pom.xml", "mvnw"]
        else:
            command = "mvn spring-boot:run"
            evidence = ["pom.xml"]
        return Plan(stack="Java Maven", start_command=command, evidence=evidence)

    gradle_files = ["build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts"]
    if any((root / name).exists() for name in gradle_files):
        if target == "windows" and (root / "gradlew.bat").exists():
            command = ".\\gradlew.bat bootRun"
            evidence = ["Gradle build file", "gradlew.bat"]
        elif target != "windows" and (root / "gradlew").exists():
            command = "./gradlew bootRun"
            evidence = ["Gradle build file", "gradlew"]
        else:
            command = "gradle bootRun"
            evidence = ["Gradle build file"]
        return Plan(
            stack="Java Gradle",
            start_command=command,
            evidence=evidence,
            notes=["If this is not a Spring Boot project, replace bootRun with the project start task."],
        )
    return None


def detect_dotnet(root: Path, _target: str) -> Plan | None:
    project_files = sorted(root.glob("*.csproj"))
    if len(project_files) == 1:
        rel = project_files[0].relative_to(root).as_posix()
        return Plan(stack=".NET", start_command=f"dotnet run --project {rel}", evidence=[rel])
    if project_files:
        return Plan(
            stack=".NET",
            start_command="dotnet run",
            evidence=[path.name for path in project_files],
            notes=["Multiple project files found; verify the correct --project value."],
        )
    return None


def detect_make(root: Path, target: str) -> Plan | None:
    makefile = root / "Makefile"
    if not makefile.exists():
        return None
    text = read_text(makefile)
    for target_name in ("dev", "start", "run"):
        if any(line.startswith(f"{target_name}:") for line in text.splitlines()):
            note = "Make may need to be installed on Windows." if target == "windows" else ""
            return Plan(
                stack="Makefile",
                start_command=f"make {target_name}",
                evidence=[f"Makefile target: {target_name}"],
                notes=[note] if note else [],
            )
    return None


def detect_docker(root: Path, _target: str) -> Plan | None:
    compose_names = (
        "compose.yaml",
        "compose.yml",
        "docker-compose.yaml",
        "docker-compose.yml",
    )
    for name in compose_names:
        if (root / name).exists():
            return Plan(
                stack="Docker Compose",
                start_command="docker compose up",
                stop_command="docker compose down",
                evidence=[name],
            )
    return None


def detect_static(root: Path, _target: str) -> Plan | None:
    if (root / "index.html").exists():
        return Plan(
            stack="Static HTML",
            start_command="python -m http.server 8000",
            evidence=["index.html"],
            notes=["Change the port if 8000 is already in use."],
        )
    return None


DETECTORS = (
    detect_node,
    detect_python,
    detect_go,
    detect_rust,
    detect_java,
    detect_dotnet,
    detect_make,
    detect_docker,
    detect_static,
)


def detect_plan(root: Path, target: str, command: str | None, stop_command: str | None) -> Plan:
    if command:
        return Plan(
            stack="Custom",
            start_command=command,
            stop_command=stop_command or "",
            evidence=["--command"],
        )

    for detector in DETECTORS:
        plan = detector(root, target)
        if plan:
            if stop_command is not None:
                plan.stop_command = stop_command
            return plan

    raise SystemExit(
        "Could not detect a start command. Rerun with --command, for example: "
        "--command \"npm run dev\""
    )


def normalize_targets(value: str) -> list[str]:
    value = value.lower()
    if value == "auto":
        return ["windows"] if platform.system().lower().startswith("win") else ["unix"]
    if value in {"windows", "win"}:
        return ["windows"]
    if value in {"unix", "linux", "mac", "macos", "darwin"}:
        return ["unix"]
    if value == "all":
        return ["windows", "unix"]
    raise SystemExit(f"Unsupported target: {value}")


def root_rel_for_script(root: Path, output_dir: Path) -> str:
    rel = os.path.relpath(root, output_dir)
    return "." if rel == "." else rel


def write_text(path: Path, content: str, force: bool, executable: bool = False) -> None:
    if path.exists() and not force:
        raise SystemExit(f"{path} already exists. Rerun with --force to overwrite it.")
    path.write_text(content, encoding="utf-8", newline="\n")
    if executable:
        make_executable(path)


def powershell_start_script(plan: Plan, root_rel: str) -> str:
    template = r"""
$ErrorActionPreference = 'Stop'

$ProjectRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot @@ROOT_REL@@)).Path
$PidFile = Join-Path $PSScriptRoot '.project-lifecycle.pid'
$LogFile = Join-Path $PSScriptRoot 'project-lifecycle.log'
$ErrFile = Join-Path $PSScriptRoot 'project-lifecycle.err.log'
$Command = @@COMMAND@@

if (Test-Path -LiteralPath $PidFile) {
    $ExistingPid = (Get-Content -LiteralPath $PidFile -Raw).Trim()
    if ($ExistingPid -match '^\d+$') {
        $Existing = Get-Process -Id ([int]$ExistingPid) -ErrorAction SilentlyContinue
        if ($Existing) {
            Write-Host "Project already running with PID $ExistingPid"
            exit 0
        }
    }
}

Push-Location $ProjectRoot
try {
    Write-Host "Starting project: $Command"
    $Process = Start-Process -FilePath 'cmd.exe' -ArgumentList @('/d', '/s', '/c', $Command) -WorkingDirectory $ProjectRoot -RedirectStandardOutput $LogFile -RedirectStandardError $ErrFile -WindowStyle Hidden -PassThru
    $Process.Id | Set-Content -LiteralPath $PidFile -NoNewline
    Write-Host "Started PID $($Process.Id)"
    Write-Host "Log: $LogFile"
    Write-Host "Error log: $ErrFile"
}
finally {
    Pop-Location
}
""".lstrip()
    return template.replace("@@ROOT_REL@@", ps_quote(root_rel.replace("/", "\\"))).replace(
        "@@COMMAND@@", ps_quote(plan.start_command)
    )


def powershell_stop_script(plan: Plan, root_rel: str) -> str:
    template = r"""
$ErrorActionPreference = 'Stop'

$ProjectRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot @@ROOT_REL@@)).Path
$PidFile = Join-Path $PSScriptRoot '.project-lifecycle.pid'
$StopCommand = @@STOP_COMMAND@@

function Stop-ProcessTree {
    param([int]$ProcessId)

    $children = Get-CimInstance Win32_Process -Filter "ParentProcessId=$ProcessId" -ErrorAction SilentlyContinue
    foreach ($child in $children) {
        Stop-ProcessTree -ProcessId ([int]$child.ProcessId)
    }

    $target = Get-Process -Id $ProcessId -ErrorAction SilentlyContinue
    if ($target) {
        Stop-Process -Id $ProcessId -Force -ErrorAction SilentlyContinue
    }
}

if (-not [string]::IsNullOrWhiteSpace($StopCommand)) {
    Push-Location $ProjectRoot
    try {
        Write-Host "Running stop command: $StopCommand"
        & cmd.exe /d /s /c $StopCommand
    }
    finally {
        Pop-Location
    }
}

if (-not (Test-Path -LiteralPath $PidFile)) {
    Write-Host "No PID file found. Project is not tracked as running."
    exit 0
}

$PidText = (Get-Content -LiteralPath $PidFile -Raw).Trim()
if ($PidText -notmatch '^\d+$') {
    Remove-Item -LiteralPath $PidFile -Force
    throw "PID file did not contain a numeric PID."
}

$ProjectPid = [int]$PidText
$Process = Get-Process -Id $ProjectPid -ErrorAction SilentlyContinue
if ($Process) {
    Write-Host "Stopping PID $ProjectPid"
    Stop-ProcessTree -ProcessId $ProjectPid
}
else {
    Write-Host "PID $ProjectPid is not running."
}

Remove-Item -LiteralPath $PidFile -Force -ErrorAction SilentlyContinue
Write-Host "Stopped project."
""".lstrip()
    return template.replace("@@ROOT_REL@@", ps_quote(root_rel.replace("/", "\\"))).replace(
        "@@STOP_COMMAND@@", ps_quote(plan.stop_command)
    )


def cmd_wrapper(script_name: str) -> str:
    return textwrap.dedent(
        f"""\
        @echo off
        powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0{script_name}" %*
        exit /b %ERRORLEVEL%
        """
    )


def unix_start_script(plan: Plan, root_rel: str) -> str:
    template = r"""
#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/@@ROOT_REL@@" && pwd)"
PID_FILE="${SCRIPT_DIR}/.project-lifecycle.pid"
LOG_FILE="${SCRIPT_DIR}/project-lifecycle.log"
COMMAND=@@COMMAND@@

if [[ -f "${PID_FILE}" ]]; then
    EXISTING_PID="$(cat "${PID_FILE}")"
    if [[ "${EXISTING_PID}" =~ ^[0-9]+$ ]] && kill -0 "${EXISTING_PID}" 2>/dev/null; then
        echo "Project already running with PID ${EXISTING_PID}"
        exit 0
    fi
fi

cd "${PROJECT_ROOT}"
echo "Starting project: ${COMMAND}"
if command -v setsid >/dev/null 2>&1; then
    nohup setsid bash -lc "${COMMAND}" >"${LOG_FILE}" 2>&1 &
else
    nohup bash -lc "${COMMAND}" >"${LOG_FILE}" 2>&1 &
fi
PID="$!"
echo "${PID}" >"${PID_FILE}"
echo "Started PID ${PID}"
echo "Log: ${LOG_FILE}"
""".lstrip()
    return template.replace("@@ROOT_REL@@", root_rel.replace("\\", "/")).replace(
        "@@COMMAND@@", shlex.quote(plan.start_command)
    )


def unix_stop_script(plan: Plan, root_rel: str) -> str:
    template = r"""
#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/@@ROOT_REL@@" && pwd)"
PID_FILE="${SCRIPT_DIR}/.project-lifecycle.pid"
STOP_COMMAND=@@STOP_COMMAND@@

if [[ -n "${STOP_COMMAND}" ]]; then
    echo "Running stop command: ${STOP_COMMAND}"
    (cd "${PROJECT_ROOT}" && bash -lc "${STOP_COMMAND}")
fi

if [[ ! -f "${PID_FILE}" ]]; then
    echo "No PID file found. Project is not tracked as running."
    exit 0
fi

PID="$(cat "${PID_FILE}")"
if [[ ! "${PID}" =~ ^[0-9]+$ ]]; then
    rm -f "${PID_FILE}"
    echo "PID file did not contain a numeric PID." >&2
    exit 1
fi

if kill -0 "${PID}" 2>/dev/null; then
    echo "Stopping PID ${PID}"
    kill -TERM -- "-${PID}" 2>/dev/null || kill -TERM "${PID}" 2>/dev/null || true
    sleep 2
    if kill -0 "${PID}" 2>/dev/null; then
        kill -KILL -- "-${PID}" 2>/dev/null || kill -KILL "${PID}" 2>/dev/null || true
    fi
else
    echo "PID ${PID} is not running."
fi

rm -f "${PID_FILE}"
echo "Stopped project."
""".lstrip()
    return template.replace("@@ROOT_REL@@", root_rel.replace("\\", "/")).replace(
        "@@STOP_COMMAND@@", shlex.quote(plan.stop_command)
    )


def generate_for_target(root: Path, output_dir: Path, target: str, plan: Plan, force: bool) -> list[str]:
    output_dir.mkdir(parents=True, exist_ok=True)
    root_rel = root_rel_for_script(root, output_dir)
    generated: list[Path] = []

    if target == "windows":
        files = {
            "start-project.ps1": powershell_start_script(plan, root_rel),
            "stop-project.ps1": powershell_stop_script(plan, root_rel),
            "start-project.cmd": cmd_wrapper("start-project.ps1"),
            "stop-project.cmd": cmd_wrapper("stop-project.ps1"),
        }
        for name, content in files.items():
            path = output_dir / name
            write_text(path, content, force=force)
            generated.append(path)
    elif target == "unix":
        files = {
            "start-project.sh": unix_start_script(plan, root_rel),
            "stop-project.sh": unix_stop_script(plan, root_rel),
        }
        for name, content in files.items():
            path = output_dir / name
            write_text(path, content, force=force, executable=True)
            generated.append(path)
    else:
        raise SystemExit(f"Unsupported normalized target: {target}")

    return [str(path) for path in generated]


def manifest_overview(root: Path) -> list[str]:
    names = [
        "package.json",
        "pnpm-lock.yaml",
        "yarn.lock",
        "bun.lock",
        "bun.lockb",
        "package-lock.json",
        "pyproject.toml",
        "requirements.txt",
        "go.mod",
        "Cargo.toml",
        "pom.xml",
        "build.gradle",
        "build.gradle.kts",
        "compose.yaml",
        "compose.yml",
        "docker-compose.yaml",
        "docker-compose.yml",
        "Makefile",
    ]
    found = [name for name in names if (root / name).exists()]
    found.extend(path.name for path in sorted(root.glob("*.csproj")))
    return found


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Inspect a project and generate OS-matched start/stop scripts."
    )
    parser.add_argument("--project", default=".", help="Project root to inspect.")
    parser.add_argument("--output-dir", default=None, help="Directory for generated scripts.")
    parser.add_argument("--target", default="auto", help="auto, windows, unix, or all.")
    parser.add_argument("--command", default=None, help="Explicit start command to use.")
    parser.add_argument("--stop-command", default=None, help="Optional command run by stop script.")
    parser.add_argument("--force", action="store_true", help="Overwrite existing generated files.")
    parser.add_argument("--dry-run", action="store_true", help="Print the detected plan without writing files.")
    return parser.parse_args(argv)


def main(argv: list[str]) -> int:
    args = parse_args(argv)
    root = Path(args.project).expanduser().resolve()
    if not root.is_dir():
        raise SystemExit(f"Project root is not a directory: {root}")

    output_dir = Path(args.output_dir).expanduser().resolve() if args.output_dir else root / "scripts"
    targets = normalize_targets(args.target)

    plans = {
        target: detect_plan(root, target, command=args.command, stop_command=args.stop_command)
        for target in targets
    }

    summary = {
        "project_root": str(root),
        "host_system": platform.system(),
        "selected_targets": targets,
        "manifest_overview": manifest_overview(root),
        "plans": {target: asdict(plan) for target, plan in plans.items()},
        "output_dir": str(output_dir),
        "generated_files": [],
    }

    if not args.dry_run:
        generated: list[str] = []
        for target, plan in plans.items():
            generated.extend(generate_for_target(root, output_dir, target, plan, force=args.force))
        summary["generated_files"] = generated
        overview_path = output_dir / "project-lifecycle.json"
        write_text(overview_path, json.dumps(summary, indent=2, ensure_ascii=True) + "\n", force=args.force)
        summary["generated_files"].append(str(overview_path))

    print(json.dumps(summary, indent=2, ensure_ascii=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
