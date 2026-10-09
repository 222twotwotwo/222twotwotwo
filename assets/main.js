(function () {
  const CONTENT_INDEX = "./content-index.json";
  const PAPER_INDEX = "./assets/paper/index.json";
  const isPostPage = document.body.dataset.page === "post";
  const isOpensourcePage = document.body.dataset.page === "opensource";
  const siteTitle = "222twotwotwo";
  const state = {
    activeTag: "全部",
    tagExpanded: false,
    posts: [],
    ready: false
  };

  const els = {
    postList: document.querySelector("#postList"),
    postView: document.querySelector("#postView"),
    postCount: document.querySelector("#postCount"),
    tagFilters: document.querySelector("#tagFilters"),
    searchInput: document.querySelector("#searchInput"),
    status: document.querySelector("#contentStatus"),
    prList: document.querySelector("#prList")
  };

  function postHref(slug) {
    return `./post.html?slug=${encodeURIComponent(slug)}`;
  }

  function legacyHashSlug() {
    const match = window.location.hash.match(/^#\/post\/(.+)$/);
    return match ? decodeURIComponent(match[1]) : "";
  }

  function redirectLegacyHashRoute() {
    const slug = legacyHashSlug();
    if (!slug) {
      return false;
    }

    window.location.replace(postHref(slug));
    return true;
  }

  if (!isPostPage && redirectLegacyHashRoute()) {
    return;
  }

  function currentPostSlug() {
    const params = new URLSearchParams(window.location.search);
    return params.get("slug") || params.get("post") || legacyHashSlug();
  }

  function basename(path) {
    return path.split("/").pop().replace(/\.md$/i, "");
  }

  function escapeHtml(value) {
    return String(value)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;")
      .replace(/'/g, "&#039;");
  }

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
      const raw = pair[2].trim();
      meta[key] = key === "tags" ? raw.split(",").map((tag) => tag.trim()).filter(Boolean) : raw;
    });

    return {
      body: markdown.slice(match[0].length),
      meta
    };
  }

  function resolveAssetPath(path, sourcePath) {
    if (!path || /^(https?:|mailto:|#|\/)/i.test(path)) {
      return path;
    }

    return new URL(path, new URL(sourcePath, window.location.href)).href;
  }

  function renderInline(text, sourcePath) {
    let html = escapeHtml(text);

    html = html.replace(/`([^`]+)`/g, "<code>$1</code>");
    html = html.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
    html = html.replace(/\*([^*]+)\*/g, "<em>$1</em>");
    html = html.replace(/\[([^\]]+)\]\(([^)]+)\)/g, function (_, label, href) {
      const resolved = resolveAssetPath(href.trim(), sourcePath);
      return `<a href="${escapeHtml(resolved)}">${label}</a>`;
    });

    return html;
  }

  function renderMedia(line, sourcePath) {
    const video = line.match(/^!\[video:([^\]]*)\]\(([^)]+)\)$/i);
    if (video) {
      const caption = video[1].trim();
      const src = resolveAssetPath(video[2].trim(), sourcePath);
      return `<figure class="media-frame"><video controls preload="metadata" src="${escapeHtml(src)}"></video>${
        caption ? `<figcaption>${escapeHtml(caption)}</figcaption>` : ""
      }</figure>`;
    }

    const image = line.match(/^!\[([^\]]*)\]\(([^)]+)\)$/);
    if (image) {
      const alt = image[1].trim();
      const src = resolveAssetPath(image[2].trim(), sourcePath);
      return `<figure class="media-frame"><img src="${escapeHtml(src)}" alt="${escapeHtml(alt)}" loading="lazy" />${
        alt ? `<figcaption>${escapeHtml(alt)}</figcaption>` : ""
      }</figure>`;
    }

    return "";
  }

  function renderMarkdown(markdown, sourcePath) {
    const lines = markdown.replace(/\r\n/g, "\n").split("\n");
    const output = [];
    let index = 0;

    while (index < lines.length) {
      const line = lines[index];
      const trimmed = line.trim();

      if (!trimmed) {
        index += 1;
        continue;
      }

      const fence = trimmed.match(/^```([A-Za-z0-9_-]+)?$/);
      if (fence) {
        const language = fence[1] || "text";
        const code = [];
        index += 1;
        while (index < lines.length && !lines[index].trim().startsWith("```")) {
          code.push(lines[index]);
          index += 1;
        }
        index += 1;
        output.push(
          `<pre class="code-block" data-language="${escapeHtml(language)}"><code>${escapeHtml(code.join("\n"))}</code></pre>`
        );
        continue;
      }

      const media = renderMedia(trimmed, sourcePath);
      if (media) {
        output.push(media);
        index += 1;
        continue;
      }

      const heading = trimmed.match(/^(#{2,4})\s+(.+)$/);
      if (heading) {
        const level = heading[1].length;
        output.push(`<h${level}>${renderInline(heading[2], sourcePath)}</h${level}>`);
        index += 1;
        continue;
      }

      if (trimmed.startsWith("> ")) {
        const quote = [];
        while (index < lines.length && lines[index].trim().startsWith("> ")) {
          quote.push(lines[index].trim().replace(/^>\s?/, ""));
          index += 1;
        }
        output.push(`<blockquote>${quote.map((item) => `<p>${renderInline(item, sourcePath)}</p>`).join("")}</blockquote>`);
        continue;
      }

      if (/^[-*]\s+/.test(trimmed)) {
        const items = [];
        while (index < lines.length && /^[-*]\s+/.test(lines[index].trim())) {
          items.push(lines[index].trim().replace(/^[-*]\s+/, ""));
          index += 1;
        }
        output.push(`<ul>${items.map((item) => `<li>${renderInline(item, sourcePath)}</li>`).join("")}</ul>`);
        continue;
      }

      const paragraph = [];
      while (
        index < lines.length &&
        lines[index].trim() &&
        !/^(#{2,4})\s+/.test(lines[index].trim()) &&
        !/^[-*]\s+/.test(lines[index].trim()) &&
        !lines[index].trim().startsWith("> ") &&
        !lines[index].trim().startsWith("```") &&
        !renderMedia(lines[index].trim(), sourcePath)
      ) {
        paragraph.push(lines[index].trim());
        index += 1;
      }
      output.push(`<p>${renderInline(paragraph.join(" "), sourcePath)}</p>`);
    }

    return output.join("\n");
  }

  function estimateReadTime(markdown) {
    const words = markdown.replace(/```[\s\S]*?```/g, "").replace(/[^\w\u4e00-\u9fa5]+/g, " ").trim();
    const cjk = (words.match(/[\u4e00-\u9fa5]/g) || []).length;
    const latin = words.split(/\s+/).filter(Boolean).length;
    const minutes = Math.max(1, Math.ceil((cjk + latin) / 420));
    return `${minutes} 分钟阅读`;
  }

  function normalizePost(file, markdown, sourcePathOverride) {
    const sourcePath = sourcePathOverride || `./assets/paper/${file}`;
    const parsed = parseFrontMatter(markdown);
    const meta = parsed.meta;
    const slug = meta.slug || basename(file);
    const tags = Array.isArray(meta.tags) ? meta.tags : [];

    return {
      slug,
      file,
      sourcePath,
      title: meta.title || slug,
      date: meta.date || "",
      category: meta.category || "笔记",
      tags,
      readTime: meta.readTime || estimateReadTime(parsed.body),
      summary: meta.summary || parsed.body.split(/\n\n/)[0].replace(/[#>*`-]/g, "").trim(),
      cover: meta.cover ? resolveAssetPath(meta.cover, sourcePath) : "",
      markdown: parsed.body,
      html: renderMarkdown(parsed.body, sourcePath)
    };
  }

  function flattenContentTree(items) {
    return items.flatMap((item) => {
      if (item.children) {
        return flattenContentTree(item.children);
      }
      return item.type === "file" ? [item] : [];
    });
  }

  function normalizePath(value) {
    return String(value || "").replace(/\\/g, "/");
  }

  function sourcePathFromRecord(record) {
    if (record.sourcePath) {
      return normalizePath(record.sourcePath);
    }

    const path = normalizePath(record.path || record.file || record.name || `${record.slug}.md`);
    return path.startsWith(".") || path.startsWith("/") ? path : `./${path}`;
  }

  function fileNameFromRecord(record) {
    const path = normalizePath(record.file || record.path || record.name || `${record.slug}.md`);
    return path.split("/").pop();
  }

  async function normalizeIndexRecord(record) {
    if (typeof record === "string") {
      const response = await fetch(`./assets/paper/${record}`);
      if (!response.ok) {
        throw new Error(`Unable to load ${record}`);
      }
      return normalizePost(record, await response.text());
    }

    const sourcePath = sourcePathFromRecord(record);
    const file = fileNameFromRecord(record);
    let markdown = record.markdown || record.content || record.body || "";

    if (!markdown) {
      const response = await fetch(sourcePath);
      if (!response.ok) {
        throw new Error(`Unable to load ${sourcePath}`);
      }
      markdown = await response.text();
    }

    const post = normalizePost(file, markdown, sourcePath);
    return {
      ...post,
      slug: record.slug || post.slug,
      title: record.title || post.title,
      date: record.date || post.date,
      category: record.category || post.category,
      tags: Array.isArray(record.tags) ? record.tags : post.tags,
      readTime: record.readTime || (record.readingTime ? `${record.readingTime} 分钟阅读` : post.readTime),
      summary: record.summary || record.description || record.excerpt || post.summary,
      cover: record.cover ? resolveAssetPath(record.cover, sourcePath) : post.cover
    };
  }

  function recordsFromContentIndex(index) {
    if (Array.isArray(index)) {
      return index;
    }

    if (Array.isArray(index.posts)) {
      return index.posts;
    }

    if (Array.isArray(index.tree)) {
      return flattenContentTree(index.tree).filter((item) => !item.hidden);
    }

    return [];
  }

  function uniqueTags() {
    return ["全部", ...new Set(state.posts.flatMap((post) => post.tags))];
  }

  function matchesQuery(post, query) {
    const haystack = `${post.title} ${post.summary} ${post.category} ${post.tags.join(" ")} ${post.markdown}`.toLowerCase();
    return haystack.includes(query.trim().toLowerCase());
  }

  function filteredPosts() {
    const query = els.searchInput ? els.searchInput.value || "" : "";
    return state.posts.filter((post) => {
      const tagMatch = state.activeTag === "全部" || post.tags.includes(state.activeTag);
      return tagMatch && matchesQuery(post, query);
    });
  }

  function setStatus(message, variant) {
    if (!els.status) return;
    els.status.hidden = !message;
    els.status.textContent = message || "";
    els.status.dataset.variant = variant || "";
  }

  const TAG_COLLAPSED_COUNT = 8; // 折叠时展示「全部」+ 前 7 个标签，其余收进「更多」

  function renderTagFilters() {
    if (!els.tagFilters) return;

    const tags = uniqueTags();
    const hasMore = tags.length > TAG_COLLAPSED_COUNT + 1;
    const hiddenTags = hasMore ? tags.slice(TAG_COLLAPSED_COUNT) : [];
    // 折叠状态下若选中的标签恰好被收起，自动展开
    const expanded = !hasMore || state.tagExpanded || hiddenTags.includes(state.activeTag);

    els.tagFilters.innerHTML = "";
    (expanded ? tags : tags.slice(0, TAG_COLLAPSED_COUNT)).forEach((tag) => {
      const button = document.createElement("button");
      button.type = "button";
      button.textContent = tag;
      button.className = tag === state.activeTag ? "active" : "";
      button.addEventListener("click", () => {
        state.activeTag = tag;
        renderTagFilters();
        renderPostList();
      });
      els.tagFilters.appendChild(button);
    });

    if (hasMore) {
      const more = document.createElement("button");
      more.type = "button";
      more.className = "tag-more";
      more.textContent = expanded ? "收起" : "··· 更多";
      more.setAttribute("aria-expanded", expanded ? "true" : "false");
      more.addEventListener("click", () => {
        state.tagExpanded = !state.tagExpanded;
        renderTagFilters();
      });
      els.tagFilters.appendChild(more);
    }
  }

  function renderPostList() {
    if (!els.postList) return;

    const visiblePosts = filteredPosts();
    if (els.postView) {
      els.postView.hidden = true;
    }
    els.postList.hidden = false;
    els.postList.innerHTML = "";
    if (els.postCount) {
      els.postCount.textContent = `${visiblePosts.length} 篇文章`;
    }

    if (!visiblePosts.length) {
      els.postList.innerHTML = `<p class="empty-state">没有找到匹配的文章。换一个关键词或标签试试。</p>`;
      return;
    }

    const ICONS = {
      calendar: '<svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M8 2v4M16 2v4M3 10h18M5 4h14a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2z"/></svg>',
      book: '<svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20V2H6.5A2.5 2.5 0 0 0 4 4.5v15zM4 19.5A2.5 2.5 0 0 0 6.5 22H20v-5"/></svg>',
      lines: '<svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 6h16M4 12h10M4 18h13"/></svg>',
      clock: '<svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 3"/></svg>',
      eye: '<svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7-10-7-10-7z"/><circle cx="12" cy="12" r="3"/></svg>'
    };

    visiblePosts.forEach((post, index) => {
      const card = document.createElement("article");
      card.className = "post-card";
      card.style.setProperty("--node-index", `"${String(index + 1).padStart(2, "0")}"`);
      const wordCount = String(post.markdown || "").replace(/\s/g, "").length;
      card.innerHTML = `
        <a class="post-hitbox" href="${postHref(post.slug)}" aria-label="阅读《${escapeHtml(post.title)}》"></a>
        <div class="post-card-main">
          <h3><a href="${postHref(post.slug)}">${escapeHtml(post.title)}</a></h3>
          <div class="post-meta-row">
            <span class="meta-chip">${ICONS.calendar}${escapeHtml(post.date)}</span>
            <span class="meta-chip">${ICONS.book}${escapeHtml(post.category)}</span>
          </div>
          <div class="card-tags-line">
            <span class="meta-chip hash-chip">#</span>
            <span class="tags-text">${post.tags.map((tag) => escapeHtml(tag)).join('<i class="tag-sep">/</i>')}</span>
          </div>
          <p class="post-summary">${escapeHtml(post.summary)}</p>
          <div class="post-stats">
            <span>${ICONS.lines}${wordCount} 字</span>
            <span>${ICONS.clock}${escapeHtml(post.readTime)}</span>
            <span class="post-views">${ICONS.eye}<img class="hits-badge" src="https://hits.sh/222twotwotwo.github.io/post/${escapeHtml(post.slug)}.svg" alt="浏览次数" loading="lazy" /></span>
          </div>
        </div>
        ${post.cover ? `<img class="post-cover" src="${escapeHtml(post.cover)}" alt="" loading="lazy" />` : ""}
      `;
      els.postList.appendChild(card);
    });
  }

  function renderPostNotFound(slug) {
    if (els.postCount) {
      els.postCount.textContent = "未找到";
    }
    if (!els.postView) return;

    document.title = `文章未找到 | ${siteTitle}`;
    els.postView.hidden = false;
    els.postView.innerHTML = `
      <div class="article-shell empty-article">
        <header class="article-header">
          <p class="eyebrow">文章未找到</p>
          <h1>没有找到这篇文章</h1>
          <p class="post-lead">${
            slug
              ? `当前链接中的 slug 是 “${escapeHtml(slug)}”，请回到文章列表重新打开。`
              : "当前链接没有带文章 slug，请回到文章列表重新打开。"
          }</p>
        </header>
      </div>
    `;
  }

  function renderPost(slug) {
    const post = state.posts.find((item) => item.slug === slug);
    if (!post) {
      renderPostNotFound(slug);
      return;
    }

    if (els.postList) {
      els.postList.hidden = true;
    }
    if (!els.postView) return;

    els.postView.hidden = false;
    if (els.postCount) {
      els.postCount.textContent = "阅读中";
    }
    document.title = `${post.title} | ${siteTitle}`;
    els.postView.innerHTML = `
      <div class="article-shell">
        <header class="article-header">
          <div class="post-meta">
            <span>${escapeHtml(post.category)}</span>
            <time datetime="${escapeHtml(post.date)}">${escapeHtml(post.date)}</time>
            <span>${escapeHtml(post.readTime)}</span>
          </div>
          <h1>${escapeHtml(post.title)}</h1>
          <p class="post-lead">${escapeHtml(post.summary)}</p>
          <div class="post-tags">
            ${post.tags.map((tag) => `<span>${escapeHtml(tag)}</span>`).join("")}
          </div>
        </header>
        ${post.cover ? `<img class="article-cover" src="${escapeHtml(post.cover)}" alt="" />` : ""}
        <div class="post-body">${post.html}</div>
      </div>
    `;
    window.scrollTo({ top: 0, behavior: "smooth" });
  }

  function renderCurrentPostPage() {
    if (!state.ready) return;
    renderPost(currentPostSlug());
  }

  async function loadFromContentIndex() {
    const response = await fetch(CONTENT_INDEX, { cache: "no-cache" });
    if (!response.ok) {
      throw new Error(`Unable to load ${CONTENT_INDEX}`);
    }

    const index = await response.json();
    const records = recordsFromContentIndex(index);
    if (!records.length) {
      throw new Error(`${CONTENT_INDEX} has no posts`);
    }

    return Promise.all(records.map(normalizeIndexRecord));
  }

  async function loadFromPaperIndex() {
    const indexResponse = await fetch(PAPER_INDEX);
    if (!indexResponse.ok) {
      throw new Error(`Unable to load ${PAPER_INDEX}`);
    }

    const files = await indexResponse.json();
    return Promise.all(files.map(normalizeIndexRecord));
  }

  async function loadPosts() {
    setStatus("正在加载文章索引...", "loading");

    let loaded;
    try {
      loaded = await loadFromContentIndex();
    } catch (error) {
      console.warn(error);
      loaded = await loadFromPaperIndex();
    }

    state.posts = loaded.sort((a, b) => String(b.date).localeCompare(String(a.date)));
    state.ready = true;
    setStatus("", "");
    if (isPostPage) {
      renderCurrentPostPage();
    } else {
      renderTagFilters();
      renderPostList();
    }
  }

  if (els.searchInput) {
    els.searchInput.addEventListener("input", () => {
      if (redirectLegacyHashRoute()) return;

      renderPostList();
    });
  }

  window.addEventListener("hashchange", () => {
    if (isPostPage) {
      renderCurrentPostPage();
      return;
    }

    if (redirectLegacyHashRoute()) return;
    renderPostList();
  });

  async function loadPullRequests() {
    if (!els.prList) return;
    try {
      const response = await fetch("./assets/prs.json");
      if (!response.ok) throw new Error("PR 数据加载失败 " + response.status);
      const data = await response.json();
      const stateLabel = { merged: "已合并", open: "进行中", closed: "已关闭" };
      els.prList.innerHTML = data.prs
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
      els.prList.innerHTML = `<p class="empty-state">PR 记录加载失败，请访问 GitHub 主页查看。</p>`;
    }
  }

  const backTopButton = document.querySelector("#backTop");
  if (backTopButton) {
    const toggleBackTop = () => {
      backTopButton.classList.toggle("visible", window.scrollY > 480);
    };
    window.addEventListener("scroll", toggleBackTop, { passive: true });
    toggleBackTop();
    backTopButton.addEventListener("click", () => {
      window.scrollTo({ top: 0, behavior: "smooth" });
    });
  }

  if (isOpensourcePage) {
    loadPullRequests();
  } else {
    loadPosts().catch((error) => {
      console.error(error);
      state.ready = false;
      if (els.postCount) {
        els.postCount.textContent = "离线";
      }
      if (els.postList) {
        els.postList.innerHTML = "";
      }
      setStatus(
        "文章加载失败。请通过本地静态服务器或 GitHub Pages 打开页面，而不是直接双击 file://。",
        "error"
      );
    });
  }
})();
