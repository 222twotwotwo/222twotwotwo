import { escapeHtml } from "./utils.mjs";

const list = document.querySelector("#prList");

async function loadPullRequests() {
  try {
    const response = await fetch("./assets/prs.json");
    if (!response.ok) throw new Error("PR 数据加载失败 " + response.status);
    const data = await response.json();
    const stateLabel = { merged: "已合并", open: "进行中", closed: "已关闭" };
    list.innerHTML = data.prs
      .filter((pr) => pr.state !== "closed")
      .map(
        (pr) => `
        <a class="pr-item" href="${escapeHtml(pr.url)}" target="_blank" rel="noopener">
          <span class="pr-state pr-state-${escapeHtml(pr.state)}">${escapeHtml(stateLabel[pr.state] || pr.state)}</span>
          <span class="pr-repo">${escapeHtml(pr.repo)}</span>
          <span class="pr-title">${escapeHtml(pr.title)}</span>
          <time class="pr-date" datetime="${escapeHtml(pr.createdAt)}">${escapeHtml(pr.createdAt)}</time>
        </a>`
      )
      .join("");
  } catch (error) {
    console.error(error);
    list.innerHTML = `<p class="empty-state">PR 记录加载失败，请访问 GitHub 主页查看。</p>`;
  }
}

loadPullRequests();
