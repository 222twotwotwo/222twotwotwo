---
title: First Skill：让 Codex 按细步骤推进，而不是一口气跑完
date: 2026-08-19
category: Codex Skill
tags: 我的skill, Codex, SDD, 工作流
readTime: 5 分钟阅读
summary: 这个 Skill 把 SDD 的 spec、plan、实现和 review 流程压缩成一个更适合个人使用的 Codex 工作流：每个 task 都要足够小，完成后必须停下来等我 review。
cover: ../images/logos/openai-logo.svg
---

我最近做了一个名为 `first` 的 Codex Skill。它解决的问题不是“让 AI 更快写完代码”，而是反过来：让 Codex 不要一下子把所有步骤都做完。

很多 AI 编程流程的问题不是不会写代码，而是推进得太快。一个 issue 贴进去，它可能马上定位、修改、补测试、顺手重构、最后给出一大坨 diff。看起来效率很高，但 review 成本会变得很高：你很难判断每一处改动是不是必要，也很难在中途纠正方向。

`first` 的目标就是给这个过程加一个明确的刹车。它会先把需求拆成更细的 task，每个 task 只做一个可以独立 review 的小动作。每完成一步，它必须停在 `REVIEW GATE`，等我确认之后才能继续下一步。

## 它从哪里来

这个 Skill 的思路来自 Superpowers 里的 SDD，也就是 Subagent-Driven Development。SDD 的核心很清晰：先有 spec，再有 plan；每个 task 交给独立实现者；实现之后再由独立 reviewer 检查；最后再做整体 review。

这套流程适合比较完整的工程协作，但我日常使用 Codex 时，不一定需要整套重量级机制。我更需要的是里面最关键的几件事：

- 先弄清楚需求，不直接改代码。
- 把工作拆成可 review 的小步。
- 每步有验证方式。
- 每步给出合适的 commit message。
- 每步完成后停下来，让人来判断是否继续。

所以 `first` 不是完整复刻 Superpowers，而是把 SDD 压缩成一个个人工作流：保留 spec、plan、implement、review 的骨架，同时强制加入人工 review gate。

## 它怎么工作

当我贴出一个 issue，并要求使用 `$first` 时，Codex 应该先把 issue 当作需求材料，而不是直接当作命令执行。

它要先提取这些信息：

- 问题是什么。
- 期望行为是什么。
- 当前仓库里相关代码路径在哪里。
- 哪些信息还不确定。
- 哪些行为明确不应该改。

然后它会生成 spec 或者短设计。对于非平凡任务，它需要写清楚目标、非目标、当前行为、设计方案、涉及文件、验收标准和验证命令。

接着进入 task 拆分。这里是 `first` 和普通 plan 最大的区别：它要求 step 更细，不允许一步直接把整个 issue 做完。

一个 step 应该只有一个主要目标。比如：

- 增加一个失败测试。
- 修复一个具体分支逻辑。
- 调整一个调用点。
- 更新一段文档。
- 跑一次针对性的验证。

如果一个 step 的 commit message 需要写成“修复 A 并更新 B”，那它就应该被拆成两个 step。

## Review Gate 是核心

`first` 里最重要的规则是：

```text
STOP means end the assistant turn.
```

也就是说，Codex 完成一个 step 之后，不应该继续调用工具、不应该继续改文件、不应该开始下一个 step。它必须把当前结果交出来，让我 review。

完成一步后的结尾应该像这样：

```text
REVIEW GATE: Step N complete. I am stopping here.
Please review this step. Reply "continue" to let me start Step N+1, or send changes for this step.
```

这个设计让协作节奏更接近真实工程 review。AI 可以负责推进，但每个关键边界都要交回给人判断。

## 为什么要拆得更细

细步骤不是为了增加仪式感，而是为了降低判断成本。

如果一个 task 同时改了测试、实现、文档和清理逻辑，那么 review 时很容易混在一起：测试是不是合理、实现是不是过度、文档是不是准确、清理是不是顺手改多了，这些问题会互相遮挡。

拆细之后，每一步的问题会更明确：

- 这个测试是否准确表达了 bug？
- 这个实现是否只解决了当前问题？
- 这个验证是否证明了刚才的改动？
- 这个 commit message 是否能独立描述这一步？

这也符合我对 AI 编程的偏好：不要让模型在一个回合里做太多决策。让它每次只做一小块，然后停下来接受反馈。

## 一个典型使用方式

我以后可以这样启动：

```text
Use $first。下面是 issue。
请按 SDD 流程处理：先分析 issue 和仓库上下文，生成 spec 和细粒度 task plan；
每完成一个 task 必须停在 REVIEW GATE，等我回复 continue 再继续。
```

理想输出不是“我已经全部修好了”，而是：

1. 先告诉我它对 issue 的理解。
2. 给出 spec 和 task plan。
3. 执行第一个小 task。
4. 跑对应验证。
5. 给出建议 commit message。
6. 停在 review gate。

如果我回复 `continue`，它再进入下一个 task。如果我指出问题，它只修当前 task，重新验证，再次停下。

## 它适合什么场景

`first` 适合那些需要控制节奏的开发任务：

- issue 修复。
- 小功能实现。
- 涉及多个文件但不想一次性大改的任务。
- 希望按 commit 粒度拆分的改动。
- 需要我逐步 review 的重构或文档调整。

它不适合特别简单的一句话修改。比如改一个错别字，直接改完就可以，不需要走完整 spec 流程。但只要任务开始跨文件、跨行为、跨验证方式，我就更愿意让 Codex 用 `first`。

## 这个 Skill 的意义

对我来说，`first` 的意义不是让 Codex 更“聪明”，而是让它更可控。

AI 写代码最大的风险，经常不是某一行写错，而是它替你做了太多未确认的工程判断。`first` 把这些判断拆回一个个小边界：先确认问题，再确认计划，再确认每一步结果。

它让 Codex 从“自动完成器”更接近“可以被 review 的开发伙伴”。这听起来慢一点，但实际更稳。因为真正昂贵的不是多停一次，而是一口气跑完之后发现方向错了。
