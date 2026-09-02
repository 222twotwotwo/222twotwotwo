---
name: blog-article-publishing
description: "Publish an article to the 222twotwotwo GitHub Pages blog. Use when the user asks to write, draft, publish, add, update, or delete an article/note/post for this blog repo (写文章、发博客、发布笔记、更新文章、删文章), when they want a Markdown file added to assets/paper, when they need the cover/logo rule applied, or when they need to rebuild the site content index and verify a post online. This skill documents the repo-specific flow: front matter conventions, the Markdown subset the in-browser renderer actually supports, logo/cover bookkeeping in assets/images/logos/SOURCES.md, registering posts in assets/paper/index.json, regenerating content-index.json with scripts/build-content-index.js, local preview via a static server, and the git push that GitHub Pages deploys."
---

# Blog Article Publishing

这个仓库（`222twotwotwo/222twotwotwo`）是一个纯静态的 GitHub Pages 博客，线上地址为 `https://222twotwotwo.github.io/222twotwotwo/`。文章是 `assets/paper/` 下的 Markdown 文件；浏览器端 `assets/main.js` 读取 JSON 索引后在 `index.html` / `post.html` 中渲染。站点不做 Jekyll 构建（仓库根目录的 `.nojekyll` 用于关闭它），也没有 CI 工作流——**发布 = 改 Markdown + 更新索引 + 推送到 `main` 分支**，GitHub Pages 自动部署。

## 目录与文件角色

| 路径 | 角色 |
| --- | --- |
| `assets/paper/<slug>.md` | 文章正文，Markdown + front matter |
| `assets/paper/index.json` | 文章源索引：一个 JSON 字符串数组，每行一个文件名（手动维护） |
| `content-index.json` | 运行时索引：由构建脚本生成（含全文与摘要），**提交到仓库** |
| `scripts/build-content-index.js` | Node.js 构建脚本：读取 `index.json` + 各文章 front matter，生成 `content-index.json` |
| `index.html` / `post.html` / `assets/main.js` / `assets/styles.css` | 站点外壳与渲染逻辑，发文一般不需要改动 |
| `assets/images/logos/` + `SOURCES.md` | Logo/封面素材库及来源记录 |
| `AGENTS.md` | 项目规则：文章涉及的主题/产品/工具/框架若有 logo，必须在文章或封面中使用 |

关键机制：`assets/main.js` 优先 `fetch("./content-index.json")`，**只有它加载失败时才回退**到 `assets/paper/index.json`。因此新增文章后若只改了源索引而不重新生成 `content-index.json`，线上不会出现新文章。

## Front Matter 规范

文章文件必须以 `---` 开头，字段参考（均来自既有文章的真实写法）：

```markdown
---
title: 文章标题
date: 2026-08-24
category: Go 后端
tags: Go, 微服务, 实践
readTime: 5 分钟阅读
summary: 一句话摘要，会出现在列表卡片与文章头部。
cover: ../images/logos/go-logo.svg
---
```

| 字段 | 必填 | 规则与惯例 |
| --- | --- | --- |
| `title` | 是 | 中文标题；列表与详情页共用 |
| `date` | 是 | 固定 `YYYY-MM-DD`；列表按它降序排列（字符串比较），与文件顺序无关 |
| `category` | 否 | 默认 `笔记`；现网已用分类：`Go 后端`、`网络排障`、`Codex Skill`、`产品思考`、`AI 工程`、`开源阅读`、`Git 协作`、`工具折腾`，优先复用 |
| `tags` | 否 | 英文/中文标签，逗号分隔（如 `Git, GitHub, gh, CLA`），驱动首页标签筛选 |
| `readTime` | 否 | 格式 `N 分钟阅读`；缺省时按正文自动估算（中文 420 字/分钟） |
| `summary` | 否 | 建议填写一句话摘要；缺省时取正文第一段并剥离符号，可能不美观 |
| `cover` | 否 | 封面资源，**相对文章文件解析**，本地 logo 写作 `../images/logos/<file>`；用于列表卡片与详情页顶部 |
| `slug` | 否 | 缺省 = 文件名去 `.md`；决定文章 URL `post.html?slug=<slug>`，改名需同步索引并放弃旧链接 |

## 渲染引擎只支持一个 Markdown 子集

正文最终由 `assets/main.js` 的 `renderMarkdown`/`renderInline`/`renderMedia` 渲染，只支持：

- 标题：`##`、`###`、`####`（`#` 和 `#####` 级别不渲染，会掉进段落）；
- 无序列表：`- ` 或 `* ` 开头的连续行（单层，不支持嵌套与有序列表）；
- 引用：连续 `> ` 行；
- 代码块：```` ```lang ```` 围栏（语言名会显示在代码块头部）；
- 行内格式：`` `code` ``、`**粗体**`、`*斜体*`、`[链接文字](url)`；
- 图片：**必须独占一行**的 `![alt](path)`，会包成 `<figure>`，alt 同时充当图注；
- 视频：独占一行的 `![video:标题](path)`（渲染为 `<video controls>`）；
- 连续的非空文本行会被合并成同一段落。

不支持且会渲染出错/丢失的内容：`#` 一级标题、`#####`+ 标题、表格、有序列表、嵌套列表、原始 HTML（会被转义成可见文本）。`<video>` 等 HTML 片段不要直接写进正文，用 `![video:...](...)`。相对路径一律按文章所在目录（`assets/paper/`）解析，所以正文插图写作 `../images/logos/xxx.svg`；`https://`、`mailto:`、`#`、`/` 开头的地址原样保留。

## Logo 与封面规范（AGENTS.md 强制项）

1. 文章凡提及有官方 logo 的产品/工具/框架/服务（如 Go、go-zero、Git、Sunshine、Moonlight、OpenAI、MCP、Dubbo、Seata 等），必须把对应 logo 用于封面或正文插图。
2. 优先复用 `assets/images/logos/` 里已收集的资源；新增 logo 命名 `<名称>-logo.<svg|png|webp>`，并把官方来源记录到 `assets/images/logos/SOURCES.md`（一行一个，注明出处 URL；本地合成的封面也需注明由哪些素材合成）。
3. 封面与列表卡片视觉由 `cover` 决定；正文插图引用同一目录的相对路径。
4. 遵循版权：使用官方品牌资源或注明来源的素材，不要随意从第三方站点热链。

## 发布流程（一次完整发布）

1. **写草稿**：新建 `assets/paper/<kebab-case-slug>.md`，按上文的 front matter + Markdown 子集撰写。标题用 `##` 分节，每段保持简短；风格参考现有文章（中文、第一人称项目笔记、先结论后细节）。
2. **处理 Logo/封面**：按 Logo 规范决定 `cover`，必要时新增 logo 素材并更新 `SOURCES.md`。
3. **登记源索引**：把文件名追加到 `assets/paper/index.json`（保持每行一个文件名的 JSON 数组格式）。
4. **重建运行时索引**：在仓库根目录执行

   ```bash
   node scripts/build-content-index.js
   ```

   成功输出 `Generated content-index.json with N posts.`。检查 `content-index.json` 已包含新文章（含 `title/date/tags/summary/readTime`）。若环境没有 Node，删除旧的 `content-index.json` 可让前端回退读 `assets/paper/index.json`，但正常应重建并提交。
5. **本地预览验证**：静态服务器（`file://` 双击打开会按设计报错），例如：

   ```bash
   python -m http.server 8000
   ```

   然后打开 `http://localhost:8000/` 与 `http://localhost:8000/post.html?slug=<slug>` 逐项检查：
   - 首页出现新卡片：标题、分类、日期、阅读时长、摘要、标签正确，且按日期降序排在正确位置；
   - 详情页正文渲染正确：标题层级、代码块、列表、引用、图片/视频、行内格式；
   - 封面图正常显示（相对路径解析正确）；
   - 搜索关键词与标签筛选能命中新文章；
   - 无表格/有序列表/`#` 标题等渲染陷阱（对照渲染子集）。
6. **提交**：`git add` 本次涉及的 `assets/paper/<slug>.md`、`assets/paper/index.json`、`content-index.json`、logo 素材与 `SOURCES.md`；commit message 先看 `git log --oneline -n 10` 对齐仓库风格。未获用户确认前不擅自 push。
7. **上线**：`git push origin main`。GitHub Pages 从 `main` 分支根目录自动部署（仓库 Settings → Pages 配置），无需手动触发。等待 1–2 分钟，访问线上 `https://222twotwotwo.github.io/222twotwotwo/` 确认列表与详情页；看不到新内容先强制刷新（浏览器缓存）。
8. **收尾**：如文章配套新增了 skill/工具，参照既有惯例（见 `assets/paper/first-skill.md`、`session-context-exporter-skill.md`、`project-lifecycle-scripts-skill.md`）：技能本体放 `skills/<name>/`（SKILL.md + agents/openai.yaml + 可选 scripts/），介绍文章用分类 `Codex Skill`、标签含 `我的skill`，封面用 `../images/logos/openai-logo.svg`。

## 常见错误速查

| 现象 | 原因/修法 |
| --- | --- |
| 首页不出现新文章，但文件存在 | `content-index.json` 未重建（前端优先读它）；执行第 4 步 |
| 列表里文章顺序不对 | `date` 不是 `YYYY-MM-DD` 或写错；列表按 date 字符串降序 |
| 正文里的 `#` 大标题变成普通段落 | 渲染器只支持 `##`–`####` |
| 表格/编号列表错乱 | 渲染器不支持表格与有序列表；改用 `- ` 列表 + 文字 |
| 图片不显示、显示成链接文本 | 图片必须独占一行；确认相对路径（`../images/logos/...`）无误 |
| 详情页封面 404 | `cover` 是相对文章文件的路径，不是相对仓库根目录 |
| 文章 URL 变化 | slug = 文件名（去 `.md`）或 front matter 的 `slug`；改名后旧链接失效 |
| 主题该有 logo 却没有 | 违反 AGENTS.md；补 `cover`/插图并登记 `SOURCES.md` |
| 线上和本地不一致 | 推送到 `main` 后等部署完成再强刷验证 |
