(function () {
  const CONTENT_INDEX = "./content-index.json";
  const PAPER_INDEX = "./assets/paper/index.json";
  const state = {
    activeTag: "全部",
    posts: [],
    ready: false
  };

  const els = {
    postList: document.querySelector("#postList"),
    postView: document.querySelector("#postView"),
    postCount: document.querySelector("#postCount"),
    tagFilters: document.querySelector("#tagFilters"),
    searchInput: document.querySelector("#searchInput"),
    status: document.querySelector("#contentStatus")
  };

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
    const query = els.searchInput.value || "";
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

  function renderTagFilters() {
    els.tagFilters.innerHTML = "";
    uniqueTags().forEach((tag) => {
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
  }

  function renderPostList() {
    const visiblePosts = filteredPosts();
    els.postView.hidden = true;
    els.postList.hidden = false;
    els.postList.innerHTML = "";
    els.postCount.textContent = `${visiblePosts.length} 篇文章`;

    if (!visiblePosts.length) {
      els.postList.innerHTML = `<p class="empty-state">没有找到匹配的文章。换一个关键词或标签试试。</p>`;
      return;
    }

    visiblePosts.forEach((post, index) => {
      const card = document.createElement("article");
      card.className = "post-card";
      card.style.setProperty("--node-index", `"${String(index + 1).padStart(2, "0")}"`);
      card.innerHTML = `
        <a class="post-hitbox" href="#/post/${post.slug}" aria-label="阅读《${escapeHtml(post.title)}》"></a>
        <div class="post-card-main">
          <div class="post-meta">
            <span>${escapeHtml(post.category)}</span>
            <time datetime="${escapeHtml(post.date)}">${escapeHtml(post.date)}</time>
            <span>${escapeHtml(post.readTime)}</span>
          </div>
          <h3><a href="#/post/${post.slug}">${escapeHtml(post.title)}</a></h3>
          <p>${escapeHtml(post.summary)}</p>
          <div class="post-tags">
            ${post.tags.map((tag) => `<span>${escapeHtml(tag)}</span>`).join("")}
          </div>
        </div>
        ${post.cover ? `<img class="post-cover" src="${escapeHtml(post.cover)}" alt="" loading="lazy" />` : ""}
      `;
      els.postList.appendChild(card);
    });
  }

  function renderPost(slug) {
    const post = state.posts.find((item) => item.slug === slug);
    if (!post) {
      window.location.hash = "#/";
      return;
    }

    els.postList.hidden = true;
    els.postView.hidden = false;
    els.postCount.textContent = "阅读中";
    els.postView.innerHTML = `
      <a class="back-link" href="#/">返回文章列表</a>
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

  function handleRoute() {
    if (!state.ready) return;
    const match = window.location.hash.match(/^#\/post\/(.+)$/);
    if (match) {
      renderPost(decodeURIComponent(match[1]));
    } else {
      renderPostList();
    }
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
    renderTagFilters();
    handleRoute();
  }

  els.searchInput.addEventListener("input", () => {
    if (window.location.hash !== "#/") {
      window.location.hash = "#/";
      return;
    }
    renderPostList();
  });

  window.addEventListener("hashchange", handleRoute);

  loadPosts().catch((error) => {
    console.error(error);
    state.ready = false;
    els.postCount.textContent = "离线";
    els.postList.innerHTML = "";
    setStatus(
      "文章加载失败。请通过本地静态服务器或 GitHub Pages 打开页面，而不是直接双击 file://。",
      "error"
    );
  });
})();
