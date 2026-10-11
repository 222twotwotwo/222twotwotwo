import hljs from "./vendor/highlight/core.mjs";
import bash from "./vendor/highlight/bash.mjs";
import go from "./vendor/highlight/go.mjs";
import ini from "./vendor/highlight/ini.mjs";
import javascript from "./vendor/highlight/javascript.mjs";
import json from "./vendor/highlight/json.mjs";
import powershell from "./vendor/highlight/powershell.mjs";

for (const [name, grammar] of Object.entries({ bash, go, ini, javascript, json, powershell })) {
  hljs.registerLanguage(name, grammar);
}

// go-zero API definitions share Go types, strings, and comments.
hljs.registerLanguage("api", (highlighter) => {
  const grammar = go(highlighter);
  return {
    ...grammar,
    name: "go-zero API",
    aliases: [],
    keywords: {
      ...grammar.keywords,
      keyword: [...grammar.keywords.keyword, "syntax", "info", "service", "returns", "get", "post", "put", "patch", "delete", "head", "options"]
    },
    contains: [...grammar.contains, { scope: "meta", match: /@[A-Za-z]+\b/ }]
  };
});

export default hljs;
