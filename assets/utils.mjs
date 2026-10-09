export function escapeHtml(value) {
  return String(value)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;");
}

export function resolveAssetPath(path, sourcePath) {
  if (!path) return "";

  const url = new URL(path, new URL(sourcePath, globalThis.location?.href));
  if (!["http:", "https:", "mailto:"].includes(url.protocol)) return "";
  return path.startsWith("#") ? path : url.href;
}
