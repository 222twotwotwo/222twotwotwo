Marked 18.1.0 is vendored from the npm package's `lib/marked.esm.js` as
`marked.mjs`, so Markdown rendering does not depend on a CDN at runtime.

- Upstream: https://github.com/markedjs/marked
- Package: https://registry.npmjs.org/marked/-/marked-18.1.0.tgz
- License: MIT; see `marked.LICENSE.md`.

The shared renderer lives in `../markdown.mjs`. Raw HTML is enabled only for
the repository-owned README files, which must remain trusted content.

Run renderer regression tests from the repository root with:

```sh
node --test tests/markdown.test.mjs
```
