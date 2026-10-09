import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { renderMarkdown } from "../assets/markdown.mjs";

const source = "https://example.com/blog/assets/paper/example.md";

test("README HTML, linked badges, and tables render together", () => {
  for (const file of ["README.md", "README.zh-CN.md"]) {
    const markdown = readFileSync(new URL(`../${file}`, import.meta.url), "utf8");
    const html = renderMarkdown(markdown, `https://example.com/blog/${file}`, { allowHtml: true });
    assert.match(html, /<div align="center">/);
    assert.equal((html.match(/<a href="https:\/\/github.com\/syvixor\/skills-icons"><img /g) || []).length, 2);
    assert.equal((html.match(/class="table-scroll"/g) || []).length, 2);
    assert.doesNotMatch(html, /\[!\[Skills\]/);
  }
});

test("relative links and images preserve the deployment subdirectory and query parameters", () => {
  const html = renderMarkdown('[docs](./notes.md?a=1&b=2)\n\n![Go](../images/go.svg)', source);
  assert.match(html, /href="https:\/\/example.com\/blog\/assets\/paper\/notes.md\?a=1&amp;b=2"/);
  assert.doesNotMatch(html, /&amp;amp;/);
  assert.match(html, /src="https:\/\/example.com\/blog\/assets\/images\/go.svg"/);
  assert.match(html, /<figure class="media-frame">/);
  assert.match(html, /<figcaption>Go<\/figcaption>/);
});

test("article videos and fenced code retain their existing presentation", () => {
  const html = renderMarkdown('![video:演示](../demo.mp4)\n\n```js\nconst x = "<tag>";\n```', source);
  assert.match(html, /<video controls preload="metadata" src="https:\/\/example.com\/blog\/assets\/demo.mp4"><\/video>/);
  assert.match(html, /<figcaption>演示<\/figcaption>/);
  assert.match(html, /class="code-block" data-language="js"/);
  assert.match(html, /&lt;tag&gt;/);
});

test("code stays literal and HTML is opt-in", () => {
  const html = renderMarkdown('`[link](https://example.com)`\n\n<script>alert(1)</script>', source);
  assert.match(html, /<code>\[link\]\(https:\/\/example.com\)<\/code>/);
  assert.doesNotMatch(html, /<script>/);
  assert.match(html, /&lt;script&gt;/);
  assert.doesNotMatch(renderMarkdown('[bad](javascript:alert%281%29)', source), /href=/);
});

test("tables, ordered lists, and incomplete Markdown do not lose content or hang", () => {
  const html = renderMarkdown('| A | B |\n| --- | --- |\n| x | y |\n\n1. first\n2. second\n\n| plain line\n\n```js incomplete\nconst x = 1;', source);
  assert.match(html, /<table>/);
  assert.match(html, /<ol>/);
  assert.match(html, /plain line/);
  assert.match(html, /const x = 1;/);
});
