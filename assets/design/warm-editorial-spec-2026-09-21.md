> **归档说明**：暖白社论风的完整设计规范与复现手册：色板、字体层级、间距与网格、三个排版范式、组件规格、断点降级、Quasar 适配坑、实现约定与验收清单。原属「逐梦之夏」官网项目，作为可复用的 UI 风格资产归档于此。参考截图未一并归档（体积大且为第三方站点截图）。

---

# design.md · 暖白社论风（Warm Editorial）

> 本文是「逐梦之夏」官网的视觉规范与复现手册。
> 目标：任何人（包括 AI）读完本文 + 打开 `src/style.css`，就能把这套风格完整复现到别的页面上。
>
> 风格代号：**暖白社论风** —— 暖白底 + 深棕主色 + 唯一高饱和金黄强调色 + 全站直角 + 无阴影描边结构语言。

---

## 0. 复现速查（TL;DR）

只要抓住这 8 条，风格就不会跑偏：

1. **配色只有两个色相**：暖白系（`#F6F2E9`/`#FFFDF8`）+ 深棕系（`#241E14`/`#2C2117`），
   再加**唯一一个**高饱和金黄 `#E0A93C` 做强调。不引入任何其他色相。
2. **层级靠字重 + 字号 + 明暗**，不靠颜色。正文只有两级文字色（`--text` / `--dim`）。
3. **中文标题用衬线**（Noto Serif SC 栈），中文正文用无衬线。形成"小号宽字距大写标签 ↔ 大号衬线标题"的强反差。
4. **中文一律窄字距**（0.5–2px）；宽字距（2.5–3px）**只留给英文小标签**。
5. **全站直角** `--radius: 0`，圆角一律不用。
6. **全站零阴影**。结构感靠 1px 低对比描边 + 留白 + 一根金色强调线。
7. **区块留白极大**（上下 150px），但**段内元素间距只有 4 档**（8/16/28/48）。
8. **深色反转区**（深棕底）用来切分节奏：深 → 浅 → 深，不要让全站一直浅底。

---

## 1. 设计依据

本风格由三个参考站提炼而成（详见 `design-refs/设计风格分析.md`）：

| 参考站 | 取用了什么 |
| --- | --- |
| **Evolve Private Wealth** | 唯一强调色 + 大留白；「左小标签 / 右大段内容」的社论两栏制；细竖线分隔的数据区 |
| **Awwwards** | 靠字重字号（而非颜色）拉层级；卡片网格"行距 > 列距"；全圆角描边筛选胶囊的秩序感 |
| **One Page Love** | 暖米底 + 单一高饱和 accent 的组合；正文限宽 ~62ch；低密度单列排版 |

明确的取舍：**Round 与直角二选一**——参考站主流是圆角/胶囊，本站保持直角识别度，
因此**放弃了阴影**（避免"直角 + 投影"的混合感），改用描边变化表达交互。

---

## 2. Design Tokens（直接复制到 `:root`）

```css
:root {
  /* ---- 色板：暖白 + 深棕 + 唯一高饱和金黄 ---- */
  --bg: #F6F2E9;               /* 页面底色（暖白） */
  --card: #FFFDF8;             /* 卡片/浮层底色（比底色更亮一点） */
  --line: #E6DFCE;             /* 主描边（1px 结构语言） */
  --line-soft: #EFE9DB;        /* 更弱的描边（表格行线、列表分隔） */
  --line-strong: #C9BFA6;      /* hover 描边加深 */
  --text: #241E14;             /* 主文字（近黑暖棕，禁用纯黑） */
  --dim: #7C7361;              /* 次级文字（暖灰，禁用冷灰） */
  --accent: #2C2117;           /* 深色反转区底色（statement / join / 徽章） */
  --on-dark: #F6F2E9;          /* 深色区上的主文字 */
  --on-dark-dim: rgba(246, 242, 233, 0.62);
  --on-dark-line: rgba(246, 242, 233, 0.16);
  --accent-soft: #F0EAD9;      /* 米色填充（表头、高亮标签） */
  --sun: #E0A93C;              /* ★ 唯一强调色 */
  --sun-deep: #C8922E;         /* 强调色 hover */
  --on-sun: #241E14;           /* 金黄底上的文字 */

  /* ---- 间距四档 + 网格 ---- */
  --sp-1: 8px;  --sp-2: 16px;  --sp-3: 28px;  --sp-4: 48px;  --sp-5: 80px;
  --section-y: 150px;          /* 区块纵向留白 */
  --grid-col-gap: 20px;        /* 网格列距 */
  --grid-row-gap: 40px;        /* 网格行距（刻意 > 列距） */

  /* ---- 字体 ---- */
  --font-serif: "Noto Serif SC", "Source Han Serif SC", "Songti SC", "SimSun", serif;
  --font-sans: "PingFang SC", "Microsoft YaHei", "Segoe UI", system-ui, sans-serif;

  /* ---- 形状 ---- */
  --radius: 0px;               /* 全站直角 */
}
```

**`--sun` 的使用红线**：只允许出现在「主按钮底色 / 强调线上的数字 / 标题下的短横线 / 当前态圆点 /
进度条填充 / 链接 hover 下划线 / 区块编号」这 **7 类**语义用途。
当前实现共 13 处引用都落在这 7 类之内。**任何大面积填充都不许用它**，
否则唯一性被稀释，整套配色就塌了。

**Alpha 变体（唯一允许的硬编码例外）**：色板没有穷举半透明色，实现中有 8 处
`rgba(...)` 是既有 token 的透明度变体，属于有意为之、可直接沿用：

| 写法 | 等价于 | 用途 |
| --- | --- | --- |
| `rgba(255,253,248, 0 / .72 / .92)` | `--card` + α | 导航透明态 / 首屏 eyebrow 底 / 导航滚动后底 |
| `rgba(246,242,233, .5 / .72)` | `--bg` + α | 首屏图片遮罩 / 角条说明文字 |
| `rgba(44,33,23, .84 / .86)` | `--accent` + α | 陈述区遮罩 / Join 区遮罩 |
| `rgba(224,169,60, .28)` | `--sun` + α | 文字选中态 |


---

## 3. 字体层级表

英文小标签 = 无衬线 + 大写 + 宽字距；中文标题 = 衬线；中文正文 = 无衬线。

| 用途 | 字体 | 字号 | 行高 | 字重 | 字距 |
| --- | --- | --- | --- | --- | --- |
| 首屏主标题 `.hero-title` | serif | `clamp(56px, 10vw, 108px)` | 1.08 | **900** | 2px（+ `text-indent: 2px` 补偿末字空隙） |
| 区块标题 `.sec-title` | serif | `clamp(30px, 3.6vw, 44px)` | 1.25 | 700 | 0.5px |
| 产品秀大标题 `.slide-title` | serif | `clamp(40px, 5.2vw, 60px)` | 1.2 | 700 | 1px |
| 深色陈述 `.st-text` | serif | `clamp(24px, 3vw, 34px)` | 1.55 | **400** | 0.5px |
| 目标编号 `.goal-item span` | serif | 26px | 1 | 700 | 0.5px |
| 数据 `.stat b` / 页码 `.showcase-counter b` | serif | `clamp(16px,1.4vw,19px)` / 20px | — | 700 | 0.5px |
| Logo 中文 `.logo-text` | serif | 17px | — | 700 | 0.5px |
| 卡片标题 `.mod-name` / `.snake-card h3` | serif | 16px / 18px | — | 700 | 0.5px |
| **英文小标签** `.sec-eyebrow` `.st-en` `.slide-en` | sans | 12px | — | 600 | **2.5–3px + uppercase** |
| Logo 英文 `.logo-text em` | sans | 9px | — | 600 | 3px |
| 首屏副标 `.hero-sub-en` | sans | `clamp(11px,1.5vw,12px)` | — | 600 | 3px + uppercase |
| 首屏 slogan `.hero-slogan` | sans | `clamp(16px,2.2vw,19px)` | — | 600 | 1px |
| **正文** `.hero-desc` `.sec-desc` | sans | 15px | 1.8 | 400 | 0 |
| 表格表头 `th` | sans | 12px | — | 600 | 1px |
| 表格单元 `td` / 列表项 | sans | 14px | — | 400 | 0 |
| 小号说明 `.stat i` `.footer-en` | sans | 11px / 10px | — | 500 | 1px / 3px |

**正文限宽**：`.hero-desc` / `.sec-desc` / `.st-sub` 统一 `max-width: 72ch`——这是防止长行失焦的关键，别省。

中文标题的字距**从宽改窄**是这次迭代最容易被忽略的一步：
旧版中文标题用 4–12px 宽字距，看起来像"拼音化"，收窄到 0.5–2px 后立刻有了社论的克制感。

---

## 4. 间距与网格

```css
/* 区块容器 */
.section {
  max-width: 1760px;
  margin: 0 auto;
  padding: var(--section-y) clamp(24px, 5vw, 88px) 30px;  /* 上下 150px 大留白 */
}

/* 卡片网格：行距刻意大于列距，让每张卡有"标题区"呼吸 */
.grid { display: grid; gap: var(--grid-row-gap) var(--grid-col-gap); }  /* 40px 20px */
```

网格列数约定：

| 区块 | 列数 | 备注 |
| --- | --- | --- |
| 网站模块 `.modules` | 3 列 | ≤960 → 2 列；≤600 → 1 列 |
| 活动目标 `.goal-row` | 2 列 | ≤600 → 1 列 |
| 考核区 `.assess-grid` | `1.25fr 1fr` | ≤960 → 1 列 |
| S 形流程 `.snake-wrap` | 绝对定位 4 卡 | ≤720 → 纵向 |

**段内间距只用 4 档**（`--sp-1..5`），不要出现 12px、22px、35px 这类随手值。
现有代码里的 20px / 22px / 26px / 44px 是历史值，新写样式请优先用档位变量。

---

## 5. 三个可复用的排版范式

这套风格的骨架其实就是三个模式在反复出现，换页面时照着套即可。

### 范式 A · 社论两栏（最重要）

小标签在左、大段内容在右。用于区块头部和深色陈述区。

```css
.head {
  display: grid;
  grid-template-columns: minmax(180px, 260px) minmax(0, 1fr);
  column-gap: var(--sp-4);        /* 48px */
  align-items: start;
  border-top: 1px solid var(--line);   /* 顶部 1px 描边当作"段落起点" */
  padding-top: var(--sp-3);
}
.head .eyebrow { grid-column: 1; grid-row: 1; }   /* 英文小标签 12px/600/2.5px */
.head .title   { grid-column: 2; grid-row: 1; }   /* 衬线大标题 */
.head .rule    { grid-column: 2; grid-row: 2; }   /* 44×3px 金黄短横线 */
.head .desc    { grid-column: 2; grid-row: 3; }   /* 15px 正文，max-width 72ch */
```

≤960px 时降级为单列，并显式重排 `grid-row`（1 标签 / 2 标题 / 3 横线 / 4 正文）。

### 范式 B · 细竖线数据栏

```css
.stats { display: flex; white-space: nowrap; }
.stat { display: flex; flex-direction: column; align-items: center; gap: 2px; padding: 0 44px; }
.stat + .stat { border-left: 1px solid var(--line); }   /* 用描边代替分隔块 */
.stat i { font-size: 11px; color: var(--dim); }          /* 小号无衬线说明 */
.stat b { font-family: var(--font-serif); font-size: 19px; font-weight: 700; }  /* 大号衬线数字 */
```

### 范式 C · 深色反转区

用背景图 + 深色遮罩做出质感，靠明暗反转切分页面节奏。

```css
.dark {
  background: url('...') center / cover no-repeat;
  display: grid;
  grid-template-columns: minmax(180px, 260px) minmax(0, 1fr);  /* 同样是范式 A */
  column-gap: var(--sp-4);
  padding: var(--section-y) clamp(24px, 5vw, 88px);
  position: relative;
}
.dark::before {                      /* 遮罩层 */
  content: ""; position: absolute; inset: 0;
  background: rgba(44, 33, 23, 0.84);
  backdrop-filter: grayscale(0.5) sepia(0.1);
}
.dark > * { position: relative; z-index: 1; }
```

深色区文字色换成 `--on-dark` / `--on-dark-dim`，描边换 `--on-dark-line`。

---

## 6. 组件规格

### 导航 `.nav`
- `position: fixed`，高度 **68px**，左右 `clamp(24px, 4vw, 64px)`
- 顶部完全透明 → 滚动超过 40px 加 `.scrolled`：底 `rgba(255,253,248,.92)` + `border-bottom: 1px var(--line)`
- `backdrop-filter: blur(14px)`
- 导航项 14px `--dim`，hover/active 变 `--text`；**当前态用 `inset 0 -2px 0 var(--sun)` 金色下划线**
- 主 CTA 走 Quasar `color="accent"`（被覆写成金黄底深棕字）

### 汉堡按钮 `.nav-burger`（**极易踩坑**）
```css
.nav-burger {
  width: 38px; height: 38px;
  min-width: 38px; min-height: 38px;   /* ★ 必须！见第 8 节 */
  color: var(--text);                  /* ★ 必须！见第 8 节 */
  background: transparent;
  border: 1px solid var(--line);
  padding: 0;
}
.nav-burger .q-icon { font-size: 20px; }
.nav-burger:hover { border-color: var(--line-strong); }
```

### 按钮
```css
.btn      { padding: 14px 36px; border-radius: 0; font-size: 15px; font-weight: 600; }
.btn-ghost{ border: 1px solid var(--line); background: transparent; }        /* 次按钮 */
.btn-ghost:hover { border-color: var(--text); transform: translateY(-2px); } /* 只描边变深，不加阴影 */
```
主按钮不要自己写背景色，交给 Quasar 的 `color="accent"` + 全局覆写（见第 8 节），这样主题只有一处开关。

### 卡片
```css
.card {
  border: 1px solid var(--line);
  border-radius: 0;
  background: var(--card);
  /* 无 box-shadow —— 永远不加 */
  transition: border-color .3s;
}
.card:hover { border-color: var(--line-strong); }   /* hover 只改描边 */
```

### 表格
```css
.tbl th { background: var(--accent-soft); color: var(--dim);   /* 米色表头，禁用冷灰 */
          font-size: 12px; letter-spacing: 1px; font-weight: 600; padding: 15px 20px; text-align: left; }
.tbl td { padding: 14px 20px; color: var(--dim); font-size: 14px;
          border-top: 1px solid var(--line-soft); }
.tbl td:first-child { color: var(--text); font-weight: 600; }   /* 首列提权 */
```

### 标签 chip / 徽章
```css
.chip { padding: 5px 13px; border: 1px solid var(--line); border-radius: 0; color: var(--dim); font-size: 13.5px; }
.chip.hl { background: var(--accent-soft); color: var(--text); font-weight: 700; border-color: transparent; }

/* 燕尾形序号徽章 */
.badge { width: 30px; height: 38px; background: var(--accent); color: var(--on-dark);
         display: grid; place-items: center; padding-bottom: 10px;
         clip-path: polygon(0 0, 100% 0, 100% 68%, 50% 100%, 0 68%); }
```

### 强调线与圆点
```css
.rule { width: 44px; height: 3px; background: var(--sun); }   /* 标题下的金色短横线 */
.dot  { width: 8px; height: 8px; border-radius: 50%; background: var(--line); }
.dot.active { background: var(--sun); transform: scale(1.45); }

/* 文字链接：hover 出金色下划线 */
.link:hover { text-decoration: underline; text-decoration-color: var(--sun);
              text-decoration-thickness: 2px; text-underline-offset: 5px; }
```

### 图片去色处理
站内实景图统一做"降饱和 + 轻微暖化"，避免照片原色破坏暖白基调：
```css
img { filter: grayscale(0.55) sepia(0.12) contrast(1.04) saturate(1.15); }
/* 单张图片卡：1px 深描边 + 卡片底色（不是白边、不是投影），并轻微旋转制造手工感 */
.photo { border: 1px solid var(--line-strong); background: var(--card); transform: rotate(1.5deg); }
```

---

## 7. 断点与降级

| 断点 | 降级内容 |
| --- | --- |
| **≤960px** | 社论两栏（`.sec-head` / `.statement`）→ 单列；产品秀**关闭吸顶劫持**改纵向堆叠；导航收进 `q-drawer`；`.modules` 2 列；`.assess-grid` 1 列；首屏信息条回文档流；区块留白 150 → 96px；表格允许横向滚动（`min-width: 520px`） |
| **≤720px** | S 形流程图 → 纵向排列（隐藏 SVG 连线，徽章由绝对定位改静态） |
| **≤600px** | `.goal-row` / `.modules` 单列；首屏单列、按钮满宽、标题降到 `clamp(44px,13vw,60px)` |
| **桌面短视口** `(min-width:961px) and (max-height:900px)` | 首屏整体收紧：`padding 108px 24px 48px`、标题降到 `clamp(48px,6.6vw,84px)`、隐藏滚动提示 |

**产品秀降级是三处联动**，少改一处就会整页重叠：
```css
.showcase-track { height: auto; }
.showcase { position: static; height: auto; overflow: visible; }
.showcase-inner { position: static; }   /* ★ 容器必须回文档流，否则高度塌陷 */
```

---

## 8. Quasar 适配要点与坑

本站 UI 组件库是 Quasar，**主题只有一处开关：把 Quasar 的 `accent` 品牌色转接成金黄**。

```css
/* 主按钮：color="accent" → 金黄底 + 深棕字 */
.q-btn.bg-accent { background: var(--sun) !important; color: var(--on-sun) !important;
                   box-shadow: none; transition: background .2s, transform .2s; }
.q-btn.bg-accent:hover { background: var(--sun-deep) !important; transform: translateY(-2px); }

/* 强调色文字 / 时间轴节点 */
.text-accent, .q-timeline .text-accent { color: var(--sun) !important; }

/* 卡片 */
.q-card { background: var(--card); color: var(--text); border-radius: 0; }
.q-card--bordered { border: 1px solid var(--line) !important; box-shadow: none; }
```

### 四个必须记住的坑

1. **`q-header` 默认继承 `text-white`**
   任何放进 `q-header` 的图标/文字若没显式给 `color`，在暖白底上就是**不可见的白色**。
   处置：`.nav { color: var(--text); }` 兜底 + 具体元素再显式声明。

2. **`.q-btn--round` 自带 `2.572em` 的 `min-width`/`min-height`**
   只写 `width: 38px` 会被压回去（实测渲染成 42px）。必须同时写 `min-width`/`min-height`。

3. **`q-btn` 带 `href` 会渲染成 `<a>`，不带则是 `<button>`**
   所以 `.nav-links a` 这类选择器对带 href 的导航项生效，对汉堡按钮不生效——给汉堡写样式时别照抄 `.nav-links a`。

4. **`v-reveal` 入场动画用 IntersectionObserver**（`src/directives/reveal.ts`，threshold 0.12）
   无头浏览器程序化滚动时 observer 不回调，截图会整片空白。
   审查截图时带上 `--init-script design-refs/audit/audit-init.js`。

---

## 9. 实现约定

- **所有样式集中在 `src/style.css`，组件内不写 `<style>`**。改视觉只动这一个文件。
- 颜色、间距、字体、圆角**必须收口到 `:root`**，禁止组件里散落魔法值。
- **不引入新依赖**（没有 Tailwind、没有额外 UI 库）。
- 交互反馈一律**不用阴影**，改用：描边变深 / 位移 -2px / 强调线展开。
- 改样式不得触碰 `<script>` 逻辑（滚劫持、observer、Three.js 初始化）与文案。

---

## 10. 从零复现这份风格的步骤

给一个全新的 Vue/React 项目套用本风格时，按顺序做：

1. **铺底色**：把第 2 节的 `:root` 整段复制进全局样式文件。
2. **定字体**：`body` 用 `--font-sans` / 15px / 行高 1.8；引入 Noto Serif SC（或系统中文字体兜底）。
3. **建社论两栏头**：实现范式 A，作为所有区块的统一头部。**这一步做完，风格就立住一半了。**
4. **建按钮与卡片**：直角 + 1px 描边 + 零阴影；主按钮金黄底深棕字，次按钮透明描边。
5. **建网格**：`gap: 40px 20px`，列数按第 4 节表。
6. **处理长文本**：所有正文段落加 `max-width: 72ch`。
7. **插入反转区**：挑 1–2 个区块做成深棕底（范式 C），形成深—浅—深的节奏。
8. **收窄中文字距**：检查全站，中文标题字距压到 0.5–2px，宽字距只留英文标签。
9. **扫一遍金色**：确认 `--sun` 只出现在第 2 节列举的那几处。
10. **写响应式**：按第 7 节逐断点降级，特别注意吸顶/粘性布局的降级要三处联动。
11. **`grep -n "box-shadow"`**：除 `inset`（下划线）与 `none` 外，其余全部删掉。

---

## 11. 验收清单

- [ ] `:root` 之外的 CSS 里没有硬编码色值（**例外**：第 2 节列的 8 处 Alpha 变体）
- [ ] 全站 `border-radius` 只有 `0`，外加 `50%`（当前仅 2 处圆形圆点：滚动提示点、产品秀当前态点）
- [ ] 除 inset 下划线与 `none` 外，无任何 `box-shadow`（当前仅 3 条声明：1 条 inset + 2 条 none）
- [ ] 中文字距 ≤ 2px；宽字距（≥2.5px）只出现在英文小标签上
- [ ] 正文段落都有 `max-width: 72ch`
- [ ] 衬线只用于标题/数字，正文是无衬线
- [ ] 金色 `--sun` 只落在第 2 节的 7 类用途内
- [ ] 至少一个深棕反转区，页面有明暗节奏
- [ ] ≤960 / ≤720 / ≤600 三档都过一遍，产品秀与流程图降级正常
- [ ] `npm run build` 零报错

---

## 12. 参考素材

| 文件 | 内容 |
| --- | --- |
| `design-refs/设计风格分析.md` | 三站 × 五维度（配色/字体/间距/组件/布局）逐项拆解 + 迁移结论 |
| `design-refs/风格迭代提示词.md` | 通用版 + 项目专用版提示词，可直接投喂新会话 |
| `design-refs/README-参考包.md` | 参考站子链接清单与截图索引 |
| `design-refs/audit/scan_dead_css.py` | 死代码扫描（找出 style.css 中零引用的类） |
| `design-refs/audit/audit-init.js` | 审查截图辅助：强制 `.reveal` 可见，否则无头截图全空白 |
| `src/style.css` | 本风格的**唯一实现源**，891 行，全部 token 在文件头 |

> `design-refs/` 下的 PNG 截图未入库（已在 `.gitignore` 排除）：体积大且为第三方站点截图。
