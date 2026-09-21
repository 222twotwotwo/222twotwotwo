> **归档说明**：三个参考站的子链接清单（分类导航页 + 作品详情页）与截图索引，说明「从哪里找风格样本」，以及详情页的结构规律。

---

# 设计参考包 · 索引

采集日期 2026-09-20 · 工具 `agent-browser`（Chromium，视口 1440×900，逐屏滚动截图）

## 一、提取到的子链接（源站 → 可复用的设计样本）

### 1. `https://www.awwwards.com/`
**分类子链接（设计分类导航，可用于按风格找参考）**
- `/websites/sites_of_the_day/`（每日最佳，本次采集源）
- `/websites/sites_of_the_month/`、`/websites/nominees/`
- 按类型：`/websites/single-page/`（单页）· `/websites/business-corporate/`（企业）
  · `/websites/design-agencies/` · `/websites/portfolio/` · `/websites/header-design/`（导航设计）
  · `/websites/scrolling/`（滚动手法）· `/websites/typography/` · `/websites/interaction-design/`

**作品详情子页面（本次采集 30 条，挑与本项目调性相符的）**
```
/sites/the-tuscan-journey-begins   ← 暖色旅行绘本风（已截图）
/sites/aardvark-book-club          ← 暖调社论型
/sites/era-residence               ← 建筑/住宅 极简
/sites/white-desert                ← 大地色 奢华旅居
/sites/oimachi                     ← 东方克制留白
/sites/miu-miu-a-house-that-we-shaped
/sites/l-i-s-a                     ← 社论排版
/sites/kononenko-architectural-bureau
/sites/warm-fuzzy                  ← 暖色品牌站
/sites/leo-parpeix-portfolio-2026
```
> 详情页结构：居中超大标题 → 圆形 SOTD 徽章 + 右上三个圆形图标 → 大尺寸圆角预览图（带浏览器 mock）。
> 页面内「Visit Site」按钮指向真实站点，本次已跳转到 Tuscan Journey 实站截图。

### 2. `https://onepagelove.com/`
**分类子链接（按题材，与本项目最相关的是 service / informational / event）**
```
/genre/service          ← 服务型（本次解析源）
/genre/informational    ← 信息型  ★与夏令营官网最贴合
/genre/event            ← 活动型  ★
/genre/landing-page     ← 落地页
/genre/startup · /genre/saas · /genre/product · /genre/restaurant
/inspiration  · /sections（区块级灵感库）· /typefaces（字体样本）
```

**作品详情子页面（每条 = 一份可直接核对的设计规格）**
```
/kontu            ← 数字产品落地页：大留白 / 单色 / 结构卡片（已截图）
/spatial          ← 活动官网：单页 + 时间轴 + 节目单密集排版（已截图）
/colosse          ← 字体产品站，展示型排版
/weeksync         ← 团队协作 App 落地页
/lofi · /letters · /typefaces · /locker-land · /poetry-camera · /shawn-golden
```
> 详情页结构：左对齐标题 + 作者 → 2–3 行描述（限宽 ~62ch）→ FEATURES 描边 chips
> → 元数据两栏表（CATEGORY / TYPEFACE / TECH / STYLE / COLOR / PUBLISHED）→ 全屏长图预览。
> **这是一个天然的「设计提示词模板」**：STYLE + COLOR + TYPEFACE 三行就是风格关键词。

### 3. `https://evolveprivatewealth.com/`
单页站，无子链接；本次按区块逐屏采集 5 屏（首屏 / 陈述 / 数据 / 服务 / 页脚）。

---

## 二、截图清单

### `design-refs/`（evolve）
| 文件 | 区块 |
|---|---|
| `evolve-1-hero.png` | 首屏：巨型 logotype + 胶囊导航 CTA |
| `evolve-2.png` | 左标签 / 右大段 两栏陈述区 |
| `evolve-3.png` | 媒体 logo 条 + 数据三栏 |
| `evolve-4.png` | 服务详情 + 胶囊按钮 |
| `evolve-5.png` | 页脚三栏 |

### `design-refs/awwwards/`
| 文件 | 区块 |
|---|---|
| `home-top.png` | 首页首屏：居中超大字标题 |
| `sotd-list-top.png` | 列表页：筛选胶囊条 + 卡片网格 |
| `sotd-list-grid.png` | 卡片网格细节（行距 > 列距） |
| `sotd-tuscan-top.png` | 作品详情页版式 |
| `sotd-tuscan-site-hero.png` `-s2.png` `-s3.png` | 获奖实站：暖色旅行绘本质感 |

### `design-refs/onepagelove/`
| 文件 | 区块 |
|---|---|
| `home-top.png` | 首页 hero：暖底居中 + 邮箱订阅 |
| `home-grid.png` | 卡片网格（白卡 8px 圆角 + 极轻阴影） |
| `kontu-top.png` | 详情页头部：标题 / 描述 / 标签 chips / 元数据表 |
| `kontu-s2.png` | 单列限宽大图预览 |
| `spatial-top.png` | FEATURES chips 与元数据间距规范 |
| `spatial-s2.png` | 长内容分段留白 |

---

## 三、配套文档
- `设计风格分析.md` —— 三站 × 五维度（配色/字体/间距/组件/布局）逐项拆解 + 迁移结论
- `风格迭代提示词.md` —— 通用版提示词 + 本项目专用版提示词 + 截图用途映射表
