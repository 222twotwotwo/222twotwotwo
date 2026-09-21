const fs = require("fs");
const path = require("path");

const rootDir = path.resolve(__dirname, "..");
const paperDir = path.join(rootDir, "assets", "paper");
const paperIndexPath = path.join(paperDir, "index.json");
const outputPath = path.join(rootDir, "content-index.json");
const designDir = path.join(rootDir, "assets", "design");
const designIndexPath = path.join(designDir, "index.json");

const DESIGN_CONVENTION =
  "assets/design/<风格代号>-<文档类型>-<YYYY-MM-DD>.md。新增一份设计文档时按命名规则丢进该目录，" +
  "再跑一次 scripts/build-content-index.js，本文件会自动重建。日期后缀用于版本并存，不覆盖旧版。";

function parseFrontMatter(markdown) {
  const match = markdown.match(/^---\s*\n([\s\S]*?)\n---\s*\n?/);
  if (!match) {
    return { body: markdown, meta: {} };
  }

  const meta = {};
  match[1].split(/\r?\n/).forEach((line) => {
    const pair = line.trim().match(/^([A-Za-z0-9_-]+):\s*(.*)$/);
    if (!pair) return;
    const key = pair[1];
    const value = pair[2].trim();
    meta[key] = key === "tags" ? value.split(",").map((tag) => tag.trim()).filter(Boolean) : value;
  });

  return {
    body: markdown.slice(match[0].length),
    meta
  };
}

function normalizeLineEndings(markdown) {
  return markdown.replace(/\r+\n/g, "\n").replace(/\r/g, "\n");
}

function slugFromFile(file) {
  return file.replace(/\\/g, "/").split("/").pop().replace(/\.md$/i, "");
}

function estimateReadingTime(markdown) {
  const text = markdown.replace(/```[\s\S]*?```/g, "").replace(/[^\w\u4e00-\u9fa5]+/g, " ").trim();
  const cjk = (text.match(/[\u4e00-\u9fa5]/g) || []).length;
  const latin = text.split(/\s+/).filter(Boolean).length;
  return Math.max(1, Math.ceil((cjk + latin) / 420));
}

function makeExcerpt(body, fallback) {
  if (fallback) return fallback;
  return body
    .split(/\n\n/)
    .find(Boolean)
    ?.replace(/[#>*`-]/g, "")
    .trim() || "";
}

// ---------- 设计文档库：assets/design/ ----------
// 目录里的每份 .md 都是一份可复用的设计资产，文件名即元数据来源。
function firstLineMatching(body, predicate) {
  return body.split("\n").map((line) => line.trim()).find(predicate) || "";
}

function stripMarkdown(value) {
  return value
    .replace(/^[>#*\-\s]+/, "")
    .replace(/[*`]/g, "")
    .replace(/^归档说明[：:]\s*/, "")
    .trim();
}

function truncate(value, max) {
  return value.length > max ? `${value.slice(0, max - 1)}…` : value;
}

function buildDesignDocs() {
  if (!fs.existsSync(designDir)) {
    return [];
  }

  return fs
    .readdirSync(designDir)
    .filter((file) => /\.md$/i.test(file))
    .map((file) => {
      const markdown = normalizeLineEndings(fs.readFileSync(path.join(designDir, file), "utf8"));
      const name = file.replace(/\.md$/i, "");
      const dateMatch = name.match(/-(\d{4}-\d{2}-\d{2})$/);
      const titleLine = firstLineMatching(markdown, (line) => /^#\s+/.test(line));
      const summaryLine =
        firstLineMatching(markdown, (line) => /^>\s+\S/.test(line)) ||
        firstLineMatching(markdown, (line) => Boolean(line) && !/^[#>|`]/.test(line));

      return {
        name: file,
        path: `assets/design/${file}`,
        sourcePath: `./assets/design/${file}`,
        slug: dateMatch ? name.slice(0, -dateMatch[0].length) : name,
        date: dateMatch ? dateMatch[1] : "",
        title: stripMarkdown(titleLine) || name,
        summary: truncate(stripMarkdown(summaryLine), 160),
        lines: markdown.split("\n").length
      };
    })
    .sort((a, b) => String(b.date).localeCompare(String(a.date)) || a.slug.localeCompare(b.slug));
}

function writeDesignIndex() {
  if (!fs.existsSync(designDir)) {
    return 0;
  }

  const docs = buildDesignDocs();
  const designIndex = {
    convention: DESIGN_CONVENTION,
    count: docs.length,
    docs
  };

  fs.writeFileSync(designIndexPath, `${JSON.stringify(designIndex, null, 2)}\n`, "utf8");
  return docs.length;
}

const files = JSON.parse(fs.readFileSync(paperIndexPath, "utf8"));
const posts = files.map((file) => {
  const markdown = normalizeLineEndings(fs.readFileSync(path.join(paperDir, file), "utf8"));
  const parsed = parseFrontMatter(markdown);
  const meta = parsed.meta;
  const readingTime = Number.parseInt(meta.readTime, 10) || estimateReadingTime(parsed.body);
  const slug = meta.slug || slugFromFile(file);

  return {
    name: file,
    path: `assets/paper/${file}`,
    sourcePath: `./assets/paper/${file}`,
    slug,
    type: "file",
    title: meta.title || slug,
    description: meta.summary || "",
    summary: meta.summary || "",
    date: meta.date || "",
    category: meta.category || "笔记",
    tags: Array.isArray(meta.tags) ? meta.tags : [],
    readTime: meta.readTime || `${readingTime} 分钟阅读`,
    readingTime,
    excerpt: makeExcerpt(parsed.body, meta.summary),
    cover: meta.cover || "",
    contentType: "post",
    hidden: false,
    markdown
  };
});

const tree = posts.map(({ markdown, ...post }) => post);
const contentIndex = {
  posts,
  tree,
  imageMap: {}
};

fs.writeFileSync(outputPath, `${JSON.stringify(contentIndex, null, 2)}\n`, "utf8");
console.log(`Generated ${path.relative(rootDir, outputPath)} with ${posts.length} posts.`);

const designCount = writeDesignIndex();
console.log(
  designCount
    ? `Generated ${path.relative(rootDir, designIndexPath)} with ${designCount} design docs.`
    : "Skipped assets/design (directory not found)."
);
