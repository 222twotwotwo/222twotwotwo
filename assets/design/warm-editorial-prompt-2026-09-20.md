> **归档说明**：两段可直接投喂的风格迭代提示词（A 通用版 / B 项目专用版），外加「截图 → 用途」映射表，用来把参考截图交给任意会话执行。

---

# 「逐梦之夏」UI 风格迭代 — 提示词包

本文件包含两段可直接投喂的提示词：
- **A. 通用版**：把参考截图交给任意 AI 会话时使用（结构与你给的范例一致）
- **B. 项目专用版**：已把三站参考提炼成本项目可落地的设计 token，可直接交给执行会话

参考截图目录：`design-refs/`（三站原始采集）与 `design-refs/awwwards/`、`design-refs/onepagelove/`
分析依据：`design-refs/设计风格分析.md`

---

## A. 通用版提示词（直接复制）

```
我需要对当前业务的整体 UI 设计做一次风格迭代，我会给你几张参考设计的截图
请你仔细分析它的视觉风格，然后把这种风格应用到我当前的页面上

请重点还原这几个方面：
1. 配色：主色、辅助色、背景色、文字颜色
2. 字体：标题和正文的字号层级、粗细、对比
3. 间距与留白：模块之间、元素之间的呼吸感
4. 组件样式：按钮、卡片、导航栏的圆角、阴影、边框
5. 整体布局：参考它的排版结构来组织页面

约束：
- 不要改动任何文案、数据结构、组件拆分和交互逻辑，只改视觉层
- 不要引入新的 UI 框架或组件库
- 改完后必须保证移动端断点（≤960px / ≤600px）同样成立
- 统一用 CSS 变量收口颜色、圆角、间距，不要散落魔法数字
```

---

## B. 项目专用版提示词（直接复制，交给执行会话）

```
【任务】对「逐梦之夏」夏令营官网（Vue 3 + Quasar + Three.js）做一次整体 UI 风格迭代。
只改视觉层，不改文案 / 组件拆分 / 交互逻辑 / 路由。

【参考素材】我已采集好 3 个参考站的关键区块截图，请先逐张读图：
- 深色高级社论风：design-refs/evolve-1-hero.png、evolve-2.png、evolve-3.png、evolve-4.png、evolve-5.png
- 白底网格 + 超大字：design-refs/awwwards/home-top.png、sotd-list-top.png、sotd-list-grid.png、sotd-tuscan-top.png
- 暖调单页排版规范：design-refs/onepagelove/home-top.png、home-grid.png、kontu-top.png、kontu-s2.png、spatial-top.png
- 暖色旅行绘本质感：design-refs/awwwards/sotd-tuscan-site-hero.png

我需要的目标风格（从上面截图提炼）：
- 暖白社论风：米白底 + 深棕主色，层级靠「字重 + 字号 + 明暗」拉开，不新增彩色
- 参考 One Page Love 的单列限宽排版规范 + Evolve 的「左小标签 / 右大段内容」两栏制

请重点还原这 5 个方面：

1) 配色 —— 保留暖白基调，重构色阶
- 背景 #F6F2E9，卡片 #FFFDF8，描边 #E6DFCE（1px，低对比）
- 主文字 #241E14，次级文字 #7C7361，禁用纯黑纯灰
- 强调色 sun 从 #d4b878 提升为唯一高饱和 accent：金黄 #E0A93C
  仅用于：主按钮底、数字高亮、下划线、当前态圆点；出现次数严格克制
- 深色反转区保留 #2C2117（用于 statement / join 区块，形成深-浅-深节奏）

2) 字体 —— 建立强反差层级
- 中文标题改用衬线：font-family: "Noto Serif SC", "Source Han Serif SC", "Songti SC", serif
- 中文正文保留无衬线（PingFang SC / Microsoft YaHei），行高 1.8
- 字号层级（桌面 → 移动）：
  - 首屏主标题 clamp(56px, 10vw, 108px) / 900，字距从 12px 收紧到 2px
  - 区块标题 clamp(30px, 3.6vw, 44px) / 700 / 衬线
  - 大段陈述 clamp(24px, 3vw, 34px) / 400 / 衬线 / 行高 1.55
  - 正文 15px / 400 / 行高 1.8
  - 英文小标签 11–12px / 600 / 大写 / 字距 2.5–3px（保留现有宽字距）
- 去掉当前全站 4–12px 的字距，中文一律收窄到 0.5–1px（宽字距只留给英文小标签）

3) 间距与留白 —— 统一到 4 档变量
- 定义 --sp-1: 8px / --sp-2: 16px / --sp-3: 28px / --sp-4: 48px / --sp-5: 80px
- 区块纵向 padding 保持 130–160px（参考 Evolve 的呼吸感）
- 正文段落最大宽度锁 ~72ch（参考 OPL 的 62ch），避免超宽行
- 卡片网格：列距 20px、行距 40px（行距 > 列距）
- 首屏主标题上下的留白不低于 48px

4) 组件样式 —— 换掉「直角 + hover 阴影」的混合感
统一取「直角骨架 + 描边变化」方案（保留当前直角识别度，但去掉影子）：
- 按钮：直角，主按钮改为 sun 金黄底 + 深棕文字，padding 14px 36px；
  次按钮 1px 描边、透明底；hover 只做描边变色 + 位移 -2px，去掉 box-shadow
- 卡片：border-radius 0，1px 描边，去掉 hover 的 --shadow，改为 hover 时描边变深 + 顶部 3px 强调线展开
- 导航：滚动后背景 rgba(255,253,248,.92) + 1px 底描边，滚动前完全透明；
  CTA 按钮改为 sun 金黄底；导航项 14px，间距 26px
- 分隔线：全站统一 1px 低对比描边，作为主要结构语言（替代阴影）
- 表格 / 时间轴：表头改米色底 #F0EAD9，去掉 #eef0f3 冷灰

5) 整体布局 —— 引入两个参考站的排版范式
- 长文本区块（statement / goals / assess 说明区）改为「左 12px 宽字距标签 + 右大段内容」两栏制
  （参考 evolve-2.png：左列为小节名，右列为衬线大段陈述）
- 数据/统计区抄 evolve-3.png：三栏，细竖线分隔，大号衬线数字 + 小号说明
- 保留现有的 showcase 吸顶逐屏结构、S 形流程图、QTimeline，只调色和留白
- 首屏参考 evolve-1-hero.png 的「非居中」思路：主标题压在视觉中下部，导航浮在上方

【硬约束 / 验收】
- 所有颜色、圆角、间距必须收口到 src/style.css 顶部的 :root 变量，禁止散落魔法值
- 不引入新的依赖（不加 tailwind、不加 UI 库）
- 必须保持 ≤960px / ≤720px / ≤600px 三个断点全部可用，尤其
  showcase 吸顶降级、卡片网格降级、表格横向滚动不能破
- 改完执行 `npm run build` 必须零报错；再用 `npm run dev` 起服务截首屏验证
- 只改 src/style.css 与 src/components/*.vue 中 <style> 段落，避免触碰 <script> 逻辑
```

---

## C. 截图 → 用途映射（给执行会话的索引）

| 截图 | 用途 |
|---|---|
| `evolve-1-hero.png` | 首屏节奏、巨型 logotype、非居中构图、胶囊 CTA |
| `evolve-2.png` | 左标签 / 右大段 两栏制、细分隔线 |
| `evolve-3.png` | 数据三栏、大号数字层级 |
| `evolve-4.png` | 服务详情版式、胶囊按钮、细线插画 |
| `evolve-5.png` | 页脚三栏结构、深-浅交替收尾 |
| `awwwards/home-top.png` | 居中超大字标题 + 居中副文 |
| `awwwards/sotd-list-top.png` | 全圆角描边筛选胶囊条 |
| `awwwards/sotd-list-grid.png` | 3 列卡片网格（行距 > 列距） |
| `awwwards/sotd-tuscan-top.png` | 详情页版式：标题 + 圆形徽章 + 大预览图 |
| `awwwards/sotd-tuscan-site-hero.png` | 暖色大地系 + 复古质感 + 米白胶囊按钮 |
| `onepagelove/home-top.png` | 暖底居中 hero、无大图、单一玫红 accent |
| `onepagelove/home-grid.png` | 卡片 = 白底 8px 圆角 + 浅描边 + 极轻阴影 |
| `onepagelove/kontu-top.png` | 左对齐标题 + 描述限宽 + 标签 chips + 元数据两栏表 |
| `onepagelove/kontu-s2.png` | 单列限宽大图预览的排版密度 |
| `onepagelove/spatial-top.png` | FEATURES 标签 chips 与元数据表的间距规范 |
| `onepagelove/spatial-s2.png` | 单页长内容的分段留白 |
