import { escapeHtml } from "./utils.mjs";

const list = document.querySelector("#contributionList");

async function loadContributions() {
  try {
    const response = await fetch("./assets/prs.json");
    if (!response.ok) throw new Error("开源贡献加载失败 " + response.status);
    const data = await response.json();
    list.innerHTML = data.projects
      .map(
        (project) => `
        <article class="contribution-item">
          <h3 class="contribution-project">
            <span class="contribution-owner">${escapeHtml(project.org)}</span>
            <a href="https://github.com/${encodeURIComponent(project.org)}/${encodeURIComponent(project.repo)}" target="_blank" rel="noopener">${escapeHtml(project.repo)}</a>
          </h3>
          <p class="contribution-summary">${escapeHtml(project.summary)}</p>
        </article>`
      )
      .join("");
  } catch (error) {
    console.error(error);
    list.innerHTML = `<p class="empty-state">开源贡献加载失败，请访问 GitHub 主页查看。</p>`;
  }
}

loadContributions();
