import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { renderMarkdown } from "../assets/markdown.mjs";

const root = new URL("../", import.meta.url);
const data = JSON.parse(readFileSync(new URL("assets/prs.json", root), "utf8"));

test("each project article explains every listed PR with its original source", () => {
  for (const project of data.projects) {
    const markdown = readFileSync(new URL(project.article, root), "utf8");
    const sections = markdown.split(/^## /m).slice(1);
    const prs = data.prs.filter((pr) => pr.org === project.org && pr.repo === project.repo && pr.state !== "closed");
    assert.equal(sections.length, prs.length, project.repo);
    for (const pr of prs) {
      const section = sections.find((text) => text.startsWith(`PR #${pr.number}：`));
      assert.ok(section, `${project.repo} #${pr.number}`);
      for (const title of ["背景", "问题", "解决方式", "成果"]) {
        assert.ok(section.includes(`### ${title}\n\n`), `${project.repo} #${pr.number}: ${title}`);
      }
      assert.ok(section.includes(`](${pr.url})`), `${project.repo} #${pr.number}: source`);
      assert.ok(section.includes(`](${pr.url}/files)`), `${project.repo} #${pr.number}: diff`);
    }
    const html = renderMarkdown(markdown, new URL(project.article, "https://example.com/blog/").href);
    assert.equal((html.match(/<h2>/g) || []).length, prs.length, project.repo);
  }
});

test("project articles and brand assets are available in a subdirectory deployment", () => {
  for (const project of data.projects) {
    assert.ok(project.article.startsWith("./assets/contributions/"));
    assert.ok(project.logos.length > 0, project.repo);
    for (const logo of project.logos) {
      assert.ok(logo.name.trim());
      const bytes = readFileSync(new URL(`assets/images/logos/${logo.file}`, root));
      assert.ok(bytes.length > 0, logo.file);
    }
  }
});
