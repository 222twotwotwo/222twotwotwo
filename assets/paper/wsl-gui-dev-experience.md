---
title: WSL 也能有完整的 GUI 开发体验：DSH 工作台 + VS Code Remote
date: 2026-09-02
category: 开发环境
tags: WSL, Windows, VS Code, DSH
readTime: 6 分钟阅读
summary: 之前开发在 Windows、测试在 WSL，因为 Windows 上偶尔会出现 UDP 测试超时这类奇怪问题，而 WSL 又只有痛苦的命令行。现在把 DSH 部署进 WSL，配好 dsh-better-sidebar 工作台，再用 VS Code Remote-WSL 连进去：GUI 体验和 Windows 完全一样，环境却是真正的 Linux。
cover: ../images/logos/dsh-vscode-wsl-cover.svg
---

我之前的开发环境是割裂的：开发、修 bug 在 Windows 上做，测试却要跑到 WSL 里跑。原因是 Windows 环境偶尔会出现一些奇怪的问题，比如此前就遇到 UDP 相关的测试超时，同样的代码和用例放到 Linux 上就完全正常。为了不被平台差异干扰，我只好把测试固定到 WSL。

环境是切过去了，体验却回不来了。WSL 默认只有那个痛苦的命令行窗口，日常开发要用的东西几乎都不顺手。所以很长一段时间里，我仍然是先在 Windows 里写代码，再复制到 WSL 里验证，两边来回倒。

现在这个问题被解决了：把 DSH 部署进 WSL，配好 `dsh-better-sidebar` 插件，再用 VS Code Remote-WSL 连进去，我获得了和 Windows 完全相同的 GUI 开发体验，而代码运行的环境是 Linux。

## 为什么测试要放到 WSL

Windows 本身是很好的日常开发系统，但有些问题只在它上面出现。

我遇到的最典型的一种是网络相关测试超时。UDP 用例在 Windows 下跑会超时，同一份代码、同一个用例在 Linux 上不会。这类问题不一定每次都出现，但一旦出现就很折磨人：你分不清是代码的问题、测试的问题，还是平台的问题。最有效的排除办法，就是把测试放到一个更接近生产的环境里跑。

WSL 正好补上这一块：不需要额外虚拟机，就能得到一个完整的 Linux 环境，Docker、Go、Node 这些工具链在里面的行为和生产环境一致。

## WSL 真正劝退我的地方：只有命令行

WSL 的 Linux 环境没问题，问题是它的界面。

日常开发不只是敲命令。我要看文件树、要编辑代码、要对比 diff、要预览 Markdown、要在多个终端之间切换、偶尔还要开个浏览器调试。这些事在 Windows 的图形界面里是顺手的，到了 WSL 就只剩一个终端窗口，感觉像退回了十年前。

我也试过在 WSL 里装各种 TUI 工具，但总体的体验仍然零散：装一堆东西，记住一堆快捷键，效率还不如回到 Windows 图形界面。于是很长一段时间，我的选择是：能在 Windows 写就在 Windows 写，测试必须 Linux 时再切过去。开发体验和测试环境始终无法兼得。

## 现在的布局：界面留在 Windows，环境换成 Linux

转折点是最近把整套开发环境搬进了 WSL。布局变成了这样：

- Windows 侧只保留图形界面：VS Code 窗口、浏览器、DSH 的 Web 界面。
- WSL 侧承担真实工作：代码、构建、测试、以及 DSH 本身。
- 我不再需要“写完复制过去跑”，开发测试都在同一套 Linux 环境里。

下面是我用的三个关键部分。

### VS Code Remote-WSL：把编辑窗口留在 Windows

VS Code 的 Remote-WSL（WSL 扩展）让编辑器窗口跑在 Windows 上，但文件、终端、调试器都运行在 WSL 里。

连上之后感觉不到“远程”的存在：左侧文件树直接打开 WSL 里的目录，集成终端默认就是 WSL 的 shell，Go 等语言的扩展也装在 WSL 侧，用的都是 Linux 的工具链。对使用者来说，这就是一个普通的 VS Code，只是下面的终端和文件都来自 Linux。

这一步解决了“代码编辑”的 GUI 问题，但我的开发流程里还有另一块：AI 工作台。

### DSH：在 WSL 里跑起自己的 Agent 工作台

DSH（DeepSeek Harness，`@deepseek-ai/dsh`）是 DeepSeek 开源的 agent harness，采用插件化架构。它通过 `dsh web` 启动一个 Web 界面，默认监听 `http://127.0.0.1:3080`。在 WSL 里启动后，因为 WSL2 默认转发 localhost，我直接在 Windows 浏览器里打开就能用。

DSH 本身解决的是 AI 会话的问题，但它默认的界面离“工作台”还差一些东西。这时候就轮到插件出场。

### dsh-better-sidebar：给 DSH 补齐工作台能力

[dsh-better-sidebar](https://github.com/omdsh-dev/DSH-better-sidebar) 是 DSH 的侧边栏工作台插件，核心思路是“右侧栏 + 底部面板”双工作台，并把 `ctx.betterSidebar` 服务开放给其他插件。装上之后，DSH 的 Web 界面里直接多了这些能力：

- 文件工作台：资源管理器 + 内置编辑器，可以直接浏览和编辑 WSL 里的文件。
- 真实终端：xterm.js + node-pty 的真实 shell，断线还能回放。
- 内嵌浏览器：在侧边栏里开网页，HTTP 链接默认在侧边栏打开。
- 文件变动：Git 视角和模型读写视角合在一起看 diff，谁改了什么一目了然。
- 后台任务：subagent 拓扑和后台任务状态、退出码、实时输出都能看到。
- 侧边对话：开一条不进入主会话的线程，继承完整上下文继续追问。

这些能力让 DSH 从“聊天的网页”变成了“能干活的工作台”，而且运行在 WSL 的真实环境里。

## 从零配置一次

如果想把同样的一套搭起来，步骤大致是这样。

### 1. WSL 里安装并启动 DSH

```bash
npm install -g @deepseek-ai/dsh
dsh web
```

启动后打开浏览器访问 `http://localhost:3080`。WSL2 默认会把 localhost 转发到 Windows，所以直接在 Windows 的浏览器里访问即可。不想让命令自动打开浏览器时，可以加 `--no-open`。

### 2. 安装 dsh-better-sidebar 插件

```bash
dsh plugin --profile web add dsh-better-sidebar@latest
```

首次执行大概率会因为 pnpm 11 拦截 `node-pty` 的构建脚本而失败，这属正常，依赖其实已经写入。接着放行构建脚本并重跑一次：

```bash
cd ~/.dsh/profiles/web && pnpm approve-builds --all
dsh plugin --profile web add dsh-better-sidebar@latest
```

完成后硬刷新浏览器（Ctrl+Shift+R）就能看到侧边栏。DSH 对 client 改动是热加载的，不需要重启进程。如果 `dsh` 命令不在 PATH 里，也可以用 `npx -y --package @deepseek-ai/dsh dsh plugin --profile web add dsh-better-sidebar@latest`。

### 3. VS Code 连接 WSL

安装 VS Code 的 WSL 扩展，然后打开命令面板执行 `WSL: Connect to WSL`，或者直接 `code ~/你的项目目录`。首次连接会自动在 WSL 侧安装 VS Code Server，之后打开项目、装扩展都跟在 Windows 里一样，只是都发生在 Linux 侧。

## 一些容易踩的坑

- 插件装完看不到侧边栏：先硬刷新浏览器，client 改动不需要重启 DSH。
- 终端提示 `node-pty 加载失败`：构建脚本被 pnpm 拦截了。在 `~/.dsh/profiles/web` 下执行 `pnpm approve-builds --all && pnpm rebuild node-pty`，重启 DSH 再试。
- 页面出现两个侧边栏：通常是同时启用了 npm 和 plugin-registry 两个安装通道导致双挂载，移除其中一个即可。
- 版本通道要对上：DSH 还在快速迭代（developer preview），装插件时按 DSH 版本选择通道，stable 用 `@latest`，alpha 通道的 DSH 用 `@alpha`，具体以插件 README 的说明为准。

## 小结

这套组合解决的是我长期的割裂感：测试必须 Linux，开发又离不开 GUI。

现在开发、测试、AI 工作台全部跑在 WSL 的同一套 Linux 环境里，Windows 只负责我喜欢的那些图形界面。UDP 超时这类“只在 Windows 出现”的问题不再需要特殊对待，GUI 体验也没有任何妥协。

另外我比较看好的一点是，这类插件把 DSH 的 Web 界面做成了可扩展的工作台：文件、终端、浏览器、diff 都可以变成模型能调用的工具。AI 会话不再停留在对话框里，而是真的住在开发环境里——这大概是我后续会持续折腾的方向。

