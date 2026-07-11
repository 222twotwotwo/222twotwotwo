---
title: 项目生命周期脚本 Skill：让启动和停止变成可复用能力
date: 2026-07-11
category: Codex Skill
tags: 我的skill, Codex, 自动化
readTime: 5 分钟阅读
summary: 这个 Skill 会先总览项目，再根据当前系统生成匹配的启动和停止脚本，把临时摸索沉淀成可重复执行的项目生命周期入口。
cover: ../images/avatar.png
---

我最近做了一个名为 `project-lifecycle-scripts` 的 Codex Skill。它解决的是一个很常见但容易被忽略的问题：每接手一个项目，都要重新判断怎么启动、怎么停止、脚本应该写成 PowerShell 还是 shell。

这个 Skill 的目标不是替代项目文档，而是把“读项目、找入口、生成生命周期脚本”这件事流程化。下一次进入一个陌生仓库时，可以让 Codex 先做项目总览，再生成可以直接执行的 `start-project` 和 `stop-project` 脚本。

## 它做什么

这个 Skill 分成两部分：

- `SKILL.md`：告诉 Codex 什么时候应该使用它，以及使用时应该先检查哪些项目文件。
- `generate_lifecycle_scripts.py`：真正负责扫描项目、判断系统、生成脚本。

默认情况下，它会把生成物写入目标项目的 `scripts` 目录。Windows 下会生成：

```text
start-project.ps1
stop-project.ps1
start-project.cmd
stop-project.cmd
project-lifecycle.json
```

Linux 或 macOS 下会生成：

```text
start-project.sh
stop-project.sh
project-lifecycle.json
```

其中 `project-lifecycle.json` 是总览结果，记录项目路径、当前系统、识别到的技术栈、启动命令、停止命令和生成的文件。

## 判断项目入口

它的检测顺序是保守的：先尊重用户传入的命令，再看项目自己的 manifest，最后才使用框架约定。

比如：

- 看到 `package.json`，会优先寻找 `dev`、`start`、`serve`、`preview` 这些脚本。
- 看到 `manage.py`，会按 Django 项目处理。
- 看到 FastAPI 或 Uvicorn 依赖，会尝试生成 `python -m uvicorn ... --reload`。
- 看到 `go.mod`，会生成 `go run .` 或 `go run ./cmd/<name>`。
- 看到 Docker Compose 文件，会生成 `docker compose up` 和 `docker compose down`。
- 只有 `index.html` 时，会把它当作静态站点，用 `python -m http.server 8000` 启动。

如果自动判断不够准确，可以显式传入命令：

```powershell
python .\skills\project-lifecycle-scripts\scripts\generate_lifecycle_scripts.py --project . --command "npm run dev"
```

## 跟随系统生成脚本

这个 Skill 的一个重点是“跟随系统”。也就是说，默认使用 `--target auto`：

```powershell
python .\skills\project-lifecycle-scripts\scripts\generate_lifecycle_scripts.py --project . --target auto
```

在 Windows 上，它生成 PowerShell 脚本和 `.cmd` 包装脚本；在 Linux 或 macOS 上，它生成 shell 脚本。如果需要同时生成两套，可以使用：

```powershell
python .\skills\project-lifecycle-scripts\scripts\generate_lifecycle_scripts.py --project . --target all
```

这个设计让脚本更贴近当前机器，而不是每个项目都手写一份容易漂移的启动说明。

## 启动和停止如何工作

启动脚本会在项目根目录运行选定的启动命令，并把进程 PID 写入 `.project-lifecycle.pid`。日志会写到 `project-lifecycle.log` 和 `project-lifecycle.err.log`。

停止脚本会读取 PID 文件，结束对应进程。Windows 版本会递归停止子进程树，避免只停掉外层 `cmd.exe` 而留下真正的服务进程。Unix 版本会优先发送 `TERM`，如果进程仍然存在，再发送 `KILL`。

这让它适合本地开发项目：能启动，也能尽量干净地收尾。

## 为什么要做成 Skill

把这件事做成 Skill 的价值在于复用。

如果只是某个项目里的一个脚本，它只能服务当前仓库；做成 Skill 后，Codex 在以后遇到类似需求时，会知道应该先总览项目，再调用生成器，而不是每次重新写一套临时脚本。

对我来说，这类 Skill 的意义是把个人工作流沉淀下来：不是让工具变复杂，而是让重复判断变少，让项目入口更明确。

当前这个 Skill 已经复制到了本站仓库的 `skills/project-lifecycle-scripts` 目录，后续可以继续扩展更多项目类型，比如前后端组合项目、多服务项目、带数据库依赖的项目，或者自动检测端口占用并选择备用端口。
