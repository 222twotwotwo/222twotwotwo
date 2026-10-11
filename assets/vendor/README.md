Marked 18.1.0 is vendored from the npm package's `lib/marked.esm.js` as
`marked.mjs`, so Markdown rendering does not depend on a CDN at runtime.

- Upstream: https://github.com/markedjs/marked
- Package: https://registry.npmjs.org/marked/-/marked-18.1.0.tgz
- License: MIT; see `marked.LICENSE.md`.

Highlight.js 11.11.1 is vendored from `@highlightjs/cdn-assets` in `highlight/`.
Its minified ES modules are copied unchanged and renamed from `.min.js` to
`.mjs`: the core plus Bash, Go, INI, JavaScript, JSON, and PowerShell grammars.
Only the languages used by the articles are included (about 39 KB total).

- Upstream: https://github.com/highlightjs/highlight.js
- Package: https://registry.npmjs.org/@highlightjs/cdn-assets/-/cdn-assets-11.11.1.tgz
- License: BSD-3-Clause; see `highlight/LICENSE`.

`../highlight.mjs` registers these grammars and a Go-based grammar for go-zero
`api` fences. Highlighting runs locally during Markdown rendering, with no CDN
requests. Unspecified, plain-text, and unknown languages remain escaped text.
The token colors in `../styles.css` match the existing dark code blocks.

The shared renderer lives in `../markdown.mjs`. Raw HTML is enabled only for
the repository-owned README files, which must remain trusted content.

Run renderer regression tests from the repository root with:

```sh
node --test tests/markdown.test.mjs
```
