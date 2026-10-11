import { Marked, Renderer } from "./vendor/marked.mjs";
import hljs from "./highlight.mjs";
import { escapeHtml, resolveAssetPath } from "./utils.mjs";

export function renderMarkdown(markdown, sourcePath, { allowHtml = false } = {}) {
  const renderer = new Renderer();

  // Only repository-owned README files opt in to their embedded HTML.
  renderer.html = ({ text }) => allowHtml ? text : escapeHtml(text);

  renderer.link = function ({ href, title, tokens }) {
    const label = this.parser.parseInline(tokens);
    const url = resolveAssetPath(href, sourcePath);
    if (!url) return label;
    return `<a href="${escapeHtml(url)}"${title ? ` title="${escapeHtml(title)}"` : ""}>${label}</a>`;
  };

  renderer.image = ({ href, title, text }) =>
    `<img src="${escapeHtml(resolveAssetPath(href, sourcePath))}" alt="${escapeHtml(text)}"${
      title ? ` title="${escapeHtml(title)}"` : ""
    } loading="lazy" />`;

  renderer.paragraph = function (token) {
    const media = token.tokens.length === 1 && token.tokens[0];
    if (!allowHtml && media?.type === "image") {
      const video = /^video:/i.test(media.text);
      const caption = video ? media.text.slice(6).trim() : media.text;
      const content = video
        ? `<video controls preload="metadata" src="${escapeHtml(resolveAssetPath(media.href, sourcePath))}"></video>`
        : this.image(media);
      return `<figure class="media-frame">${content}${caption ? `<figcaption>${escapeHtml(caption)}</figcaption>` : ""}</figure>\n`;
    }
    return Renderer.prototype.paragraph.call(this, token);
  };

  renderer.code = ({ text, lang }) => {
    const language = lang?.trim().split(/\s+/)[0].toLowerCase() || "text";
    const supported = hljs.getLanguage(language);
    const code = supported
      ? hljs.highlight(text, { language, ignoreIllegals: true }).value
      : escapeHtml(text);
    return `<pre class="code-block" data-language="${escapeHtml(language)}"><code${supported ? ' class="hljs"' : ""}>${code}</code></pre>\n`;
  };

  renderer.table = function (token) {
    return `<div class="table-scroll" tabindex="0" role="region" aria-label="表格（可横向滚动）">${Renderer.prototype.table.call(this, token)}</div>\n`;
  };

  return new Marked({ renderer }).parse(markdown);
}
