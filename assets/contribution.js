import { escapeHtml } from "./utils.mjs";
import { renderMarkdown } from "./markdown.mjs";

const view = document.querySelector("#contributionView");
const status = document.querySelector("#contentStatus");
const projectId = new URLSearchParams(location.search).get("project");

async function loadContribution() {
  view.setAttribute("aria-busy", "true");
  try {
    const response = await fetch("./assets/prs.json");
    if (!response.ok) throw new Error(`贡献索引加载失败 ${response.status}`);
    const data = await response.json();
    const project = data.projects.find((item) => `${item.org}/${item.repo}` === projectId);
    if (!project) {
      document.title = "项目未找到 | 222twotwotwo";
      view.innerHTML = '<div class="article-header empty-article"><h1>没有找到这个项目</h1><p>请返回开源概览，重新选择项目。</p></div>';
      status.hidden = true;
      return;
    }

    const article = await fetch(project.article);
    if (!article.ok) throw new Error(`贡献详情加载失败 ${article.status}`);
    const markdown = await article.text();
    const repoUrl = `https://github.com/${encodeURIComponent(project.org)}/${encodeURIComponent(project.repo)}`;
    document.title = `${project.repo} 贡献详情 | 222twotwotwo`;
    document.querySelector('meta[name="description"]').content = project.summary;
    view.innerHTML = `
      <div class="article-shell">
        <header class="article-header">
          <p class="post-meta">开源贡献 · ${escapeHtml(project.org)}</p>
          <h1>${escapeHtml(project.repo)}</h1>
          <p class="post-lead">${escapeHtml(project.summary)}</p>
          <div class="contribution-brands">${project.logos.map((logo) =>
            `<img src="./assets/images/logos/${escapeHtml(logo.file)}" alt="${escapeHtml(logo.name)}" title="${escapeHtml(logo.name)}" />`
          ).join("")}</div>
          <a class="opensource-link" href="${repoUrl}" target="_blank" rel="noopener">查看项目仓库 ↗</a>
        </header>
        <nav class="contribution-toc" aria-label="PR 目录">
          <h2>贡献记录</h2>
          <ul></ul>
        </nav>
        <div class="post-body contribution-body">${renderMarkdown(markdown, project.article)}</div>
        <a class="opensource-link" href="./opensource.html">← 返回开源概览</a>
      </div>`;

    const directory = view.querySelector(".contribution-toc ul");
    view.querySelectorAll(".contribution-body > h2").forEach((heading) => {
      const number = heading.textContent.match(/^PR #(\d+)/)[1];
      heading.id = `pr-${number}`;
      const item = document.createElement("li");
      const link = document.createElement("a");
      link.href = `#${heading.id}`;
      link.textContent = heading.textContent;
      item.appendChild(link);
      directory.appendChild(item);
    });
    status.hidden = true;
    document.getElementById(location.hash.slice(1))?.scrollIntoView();
  } catch (error) {
    console.error(error);
    status.dataset.variant = "error";
    status.textContent = "贡献详情加载失败，请刷新重试，或返回开源概览选择项目。";
  } finally {
    view.setAttribute("aria-busy", "false");
  }
}

const backTop = document.querySelector("#backTop");
window.addEventListener("scroll", () => {
  backTop.classList.toggle("visible", window.scrollY > 480);
}, { passive: true });
backTop.addEventListener("click", () => window.scrollTo({ top: 0, behavior: "smooth" }));

loadContribution();
