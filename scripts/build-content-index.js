const fs = require("fs");
const path = require("path");

const rootDir = path.resolve(__dirname, "..");
const paperDir = path.join(rootDir, "assets", "paper");
const paperIndexPath = path.join(paperDir, "index.json");
const outputPath = path.join(rootDir, "content-index.json");

function parseFrontMatter(markdown) {
  const match = markdown.match(/^---\s*\n([\s\S]*?)\n---\s*\n?/);
  if (!match) {
    return { body: markdown, meta: {} };
  }

  const meta = {};
  match[1].split(/\r?\n/).forEach((line) => {
    const pair = line.match(/^([A-Za-z0-9_-]+):\s*(.*)$/);
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

const files = JSON.parse(fs.readFileSync(paperIndexPath, "utf8"));
const posts = files.map((file) => {
  const markdown = fs.readFileSync(path.join(paperDir, file), "utf8");
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
