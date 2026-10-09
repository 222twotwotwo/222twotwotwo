import { renderMarkdown } from "./markdown.mjs";

const target = document.querySelector("#aboutBody");
const status = document.querySelector("#contentStatus");
const language = new URLSearchParams(window.location.search).get("lang");
const file = language === "zh-CN" ? "README.zh-CN.md" : "README.md";
const sourcePath = `./${file}`;

document.querySelector("#readmeSource").href =
  `https://github.com/222twotwotwo/222twotwotwo/blob/main/${file}`;
target.lang = language === "zh-CN" ? "zh-CN" : "en";

async function loadAbout() {
  target.setAttribute("aria-busy", "true");
  status.hidden = false;
  status.textContent = "正在加载关于内容...";
  try {
    const response = await fetch(sourcePath, { cache: "no-cache" });
    if (!response.ok) throw new Error(`README 加载失败 ${response.status}`);
    target.innerHTML = renderMarkdown(await response.text(), sourcePath, { allowHtml: true });

    // Keep the README's language links inside the rendered about page.
    const languages = new Map([
      [new URL("./README.md", location.href).href, "./about.html"],
      [new URL("./README.zh-CN.md", location.href).href, "./about.html?lang=zh-CN"]
    ]);
    target.querySelectorAll("a[href]").forEach((link) => {
      const destination = languages.get(link.href);
      if (destination) link.href = destination;
    });

    const intro = target.querySelector(":scope > div[align='center']");
    if (intro) {
      intro.classList.add("readme-intro");
      const title = intro.querySelector("h1");
      if (title) {
        const name = document.createElement("p");
        name.className = "readme-name";
        name.textContent = title.textContent;
        title.replaceWith(name);
      }
    }

    const navigation = document.querySelector("#aboutNavLinks");
    target.querySelectorAll(":scope > h2").forEach((heading, index) => {
      heading.id = `about-section-${index + 1}`;
      const link = document.createElement("a");
      link.href = `#${heading.id}`;
      link.textContent = heading.textContent;
      navigation.appendChild(link);
    });
    document.querySelector("#aboutNav").hidden = !navigation.childElementCount;

    target.querySelectorAll('img[src^="https://github-profile-summary-cards.vercel.app/"]').forEach((image) => {
      const url = new URL(image.src);
      url.searchParams.set("theme", "github");
      image.src = url.href;
    });
    status.hidden = true;
    status.textContent = "";
  } catch (error) {
    console.error(error);
    status.dataset.variant = "error";
    status.textContent = "关于内容加载失败，请刷新重试，或点击「在 GitHub 查看」。";
  } finally {
    target.setAttribute("aria-busy", "false");
  }
}

loadAbout();
