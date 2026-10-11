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
  assert.match(html.replace(/<\/?span\b[^>]*>/g, ""), /const x = 1;/);
});

test("article code languages and aliases receive syntax highlighting", () => {
  for (const [language, code, token] of [
    ["go", 'package main\nfunc main() { println("hello") }', "keyword"],
    ["golang", "var answer int = 42", "keyword"],
    ["js", 'const message = "hello";', "keyword"],
    ["JavaScript title=example", 'const message = "hello";', "keyword"],
    ["bash", 'echo "$HOME" # home directory', "built_in"],
    ["sh", 'echo "$HOME"', "variable"],
    ["powershell", 'Get-Process | Where-Object { $_.Name -eq "node" }', "built_in"],
    ["json", '{"enabled": true, "count": 42}', "attr"],
    ["ini", '[wsl2]\nnetworkingMode=mirrored', "section"],
    ["api", 'service user-api {\n    @handler Detail\n    get /user/:id (Request) returns (Response)\n}', "meta"]
  ]) {
    const html = renderMarkdown(`\`\`\`${language}\n${code}\n\`\`\``, source);
    assert.match(html, /<code class="hljs">/, language);
    assert.match(html, new RegExp(`class="hljs-${token}"`), language);
  }
});

test("plain text, unspecified languages, and unknown languages stay literal", () => {
  for (const language of ["text", "plaintext", "", "unknown-language", 'unknown" onclick="alert(1)']) {
    const code = '<script>alert("hello")</script> & text';
    const html = renderMarkdown(`\`\`\`${language}\n${code}\n\`\`\``, source);
    assert.doesNotMatch(html, /<span|<script| onclick=/);
    assert.match(html, /<code>&lt;script&gt;alert\(&quot;hello&quot;\)&lt;\/script&gt; &amp; text<\/code>/);
  }
});

test("highlighted code escapes HTML and preserves code text", () => {
  const html = renderMarkdown('```js\nconst html = "<img src=x onerror=alert(1)>"; // & comment\n```', source);
  assert.match(html, /class="hljs-string"/);
  assert.doesNotMatch(html, /<img/);
  assert.match(html.replace(/<\/?span\b[^>]*>/g, ""), /const html = &quot;&lt;img src=x onerror=alert\(1\)&gt;&quot;; \/\/ &amp; comment/);
});
