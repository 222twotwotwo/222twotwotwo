# AGENTS.md · 6.824 目录规范

本目录是 MIT 6.824 相关项目的学习实现，每个项目会单独写成一篇博客。下面是写这些项目博客时必须遵守的约定。

## 目录结构

- 每个项目一个子目录（`gfs/`、`mapreduce/`、`raft/`），各自是独立 Go module，只依赖标准库，根目录的 `go.work` 统一管理。
- `note.md`：写这个项目时的原始笔记，**是博客正文的唯一来源**。
- `README.md`：面向仓库读者的运行与设计说明（构建、运行、测试、简化边界）。
- 两者不合并：笔记写给自己，README 写给看代码的人，博客只搬运笔记。

## 博客正文：照搬 note.md，不重写

- 一个项目对应 `assets/paper/<slug>.md`，正文与同目录 `note.md` **逐字一致**：保留原分隔线 `--------`、中文标点、`(分配、超时重派、收尾)` 这类半角括号、口语化措辞，以及原有的标题层级（`## 架构图` / `### 通信协议` / `## 实现细节` …）。
- 不改写、不扩写、不补"我的理解"、不做"成稿润色"。note 是底稿，不是素材。
- 只允许两处偏离：
  1. `mermaid` 代码块换成一张专用 SVG（站点渲染器不渲染 mermaid）；
  2. 文末在 `## 参考文献` 之后追加 `## 项目源码`。
- front matter 按站点规范写：`title` 用中文标题（与 GFS 那篇的命名风格一致，如"实现最简分布式计算框架"），`category: Go 后端`，`tags` 用 `Go, 分布式计算, …`，`summary` 取 note 首句（只补英文与中文之间的空格），`cover` 用 `../images/logos/go-logo.svg`。
- 站点渲染器是 `assets/markdown.mjs`（marked + highlight.js），**支持表格**，note 里的表格可以原样保留；图片必须独占一行才会渲染成 `<figure>`。
- 交稿前做一次逐行 diff：去掉 note 的 mermaid 块、去掉文章的图片行与「项目源码」段，其余行必须完全一致（差异应为 0）。

## 架构图：只做 mermaid 的最简等价物

- 节点、连线、边标签与 `note.md` 里的 mermaid 一一对应，不增不减；不要额外加"调度与数据分离"这类副标题，也不要加能力注释、内部字段、文件名列表。
- 首次出现的方向性错误记录在案：曾按自己的理解画了架构图 + 任务状态图两张，被判定偏离，最终只保留 mermaid 对应的一张。
- 风格沿用站内暖白社论风：底 `#F6F2E9`、卡片 `#FFFDF8`、主文字 `#241E14`、次级 `#7C7361`、唯一强调色金黄 `#E0A93C`、数据侧绿 `#5FA04E`；全直角、零阴影、1px 低对比描边。
- 文件命名 `assets/images/logos/<project>-architecture-inline.svg`，并在 `assets/images/logos/SOURCES.md` 登记一行来源（说明按本项目设计重绘、Go 标记取自 `go-logo.svg`）。

## 项目链接

- 文末「项目源码」指向 `https://github.com/222twotwotwo/222twotwotwo/tree/main/6.824/<project>`，推送后用 `curl` 确认返回 200，避免链接指向不存在的目录。

## 发布流程

- 完整流程见仓库根 `skills/blog-article-publishing/SKILL.md`，要点：把文件名登记进 `assets/paper/index.json` → `node scripts/build-content-index.js` → 本地 `python -m http.server 8012` 预览核对 → commit → `git push main main`（远端名就是 `main`）。
- 目录重构（如把 `6.824/*.go` 移入 `gfs/`）要和对应文章的项目链接在同一次提交里改完，否则线上链接会 404。
- 阅读时长按 `scripts/build-content-index.js` 的估算填（去掉代码块后中文 420 字/分钟），自己写大了会和实际不符。
