---
title: 会话上下文导出 Skill：把一次协作变成下一次可继续的起点
date: 2026-07-12
category: Codex Skill
tags: 我的skill, Codex, 上下文, 自动化
readTime: 5 分钟阅读
summary: 这个 Skill 会把当前 Codex 会话整理成 Markdown 交接文件，让下一次对话可以直接读取目标、状态、文件、验证结果和下一步。
cover: ../images/avatar.png
---

我最近做了一个名为 `session-context-exporter` 的 Codex Skill。它解决的是一个非常日常的问题：一次任务做了一半，或者已经做完但还想沉淀过程，下次对话怎样快速接上上下文。

很多时候，真正重要的不是完整聊天记录，而是任务交接需要的那几类信息：目标是什么，已经完成到哪里，改过哪些文件，跑过哪些验证，还有下一步应该从哪里开始。这个 Skill 就是把这些内容整理成一个 Markdown 文件，让它可以被下一次 Codex 对话直接读取。

## 它做什么

这个 Skill 会生成一个面向继续工作的 handoff 文件，而不是简单保存原始 transcript。默认导出的内容包括：

- 当前目标和最新请求。
- 任务状态和已经完成的工作。
- 关键决策、约束和偏好。
- 相关文件、目录、产物、链接或命令。
- 已经执行过的测试、构建或验证结果。
- 仍然存在的问题、风险和下一步。
- 一段可以在下一次对话中直接使用的启动提示。

默认情况下，它把文件写到当前工作区的 `context-handoffs/` 目录。这样做的原因很简单：上下文交接文件通常属于当前项目的工作记录，放在项目里更容易被下一次任务找到。

## 为什么不是原始聊天记录

原始聊天记录看起来完整，但真正继续任务时反而不一定高效。下一次对话最需要的是可执行信息，而不是把每一次探索都重新读一遍。

所以这个 Skill 的取舍是保留“能帮助继续工作”的内容：

- 哪些结论已经确认。
- 哪些路径应该优先查看。
- 哪些命令已经跑过，结果如何。
- 哪些事实可能会漂移，需要重新验证。
- 用户最新的要求是否覆盖了旧计划。

这让交接文件更像工程笔记，而不是聊天备份。

## 脚本怎么用

Skill 里包含一个脚本：

```powershell
python .\skills\session-context-exporter\scripts\export_context_md.py --json payload.json --output context-handoffs
```

`payload.json` 可以传入标题、工作区、目标、状态、章节和下一次提示。例如：

```json
{
  "title": "Session Context Handoff",
  "workspace": "C:/path/to/workspace",
  "objective": "Continue a partially completed task.",
  "status": "Implementation is done, verification remains.",
  "sections": [
    {
      "heading": "Files And Artifacts",
      "items": ["C:/path/to/file.md - article draft"]
    },
    {
      "heading": "Next Steps",
      "body": "Run the focused tests and publish the final changes."
    }
  ],
  "next_prompt": "Read this Markdown context file first, then continue from Next Steps."
}
```

如果 `--output` 是目录，脚本会自动生成带时间戳的 `.md` 文件。如果 `--output` 直接指向某个 `.md` 文件，它就会覆盖写入那个文件。

## 复用时的规则

下一次对话使用这个上下文文件时，最重要的是先读文件，再继续工作。但这个文件也不是绝对真相。

例如分支、远端状态、依赖版本、服务端口、CI 结果、网页部署状态，都可能在导出之后发生变化。Skill 的说明里明确要求 Codex 对这些容易漂移的信息重新验证，再继续执行。

这能避免一个常见问题：拿着旧上下文继续做事，却忘了现实环境已经变了。

## 为什么要放进项目

这次我把 `session-context-exporter` 同步到了本站仓库的 `skills/session-context-exporter` 目录。这样它不只是安装在本机 Codex 里，也成为这个项目的一部分。

对我来说，这类 Skill 的意义是把个人工作流沉淀下来：每次任务结束时，不只是“做完了”，还可以留下一个足够清晰的继续入口。下一次再打开项目时，不需要从零回忆发生过什么，只要读取 Markdown handoff，就能接着往前走。

后续可以继续扩展它，比如加入更固定的文件命名规则、生成更详细的命令日志，或者把多个 handoff 合并成项目级工作记录。但目前这个版本已经覆盖了最核心的一步：把一次协作变成下一次可继续的起点。
