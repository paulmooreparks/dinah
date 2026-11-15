# CodeMirror 6 for the site's editor

On 2026-11-14 the shared bundle added the documented `LanguageDescription` and `LanguageSupport` exports, for code colouring (DESIGN.md decision 188). Each coloured language has its own small script in `lang/`, built by the same `language-service/build.mjs` from the pinned `@codemirror/legacy-modes` and listed in the same manifest; the kit loads the bundle with `?v=10`.

On 2026-11-09 the shared bundle added the documented EditorSelection, ChangeSet and Facet exports for reversible source history. Package versions remain pinned. The kit loads the regenerated bundle with `?v=9`. Source history uses its own versioned serializer and does not access CodeMirror history internals.

On 2026-11-09 the shared bundle added the existing pinned HTML language export for Editor's HTML source assistance. The language worker adds pinned Microsoft CSS 6.3.10 and HTML 5.6.2 services. The reproducible build and package manifest cover both outputs; the kit loads CodeMirror with `?v=8`.

On 2026-11-07 the shared editing commands added the documented `undoDepth`, `redoDepth` and `isolateHistory` exports. The pinned build and manifest were regenerated without changing package versions. The kit loads this bundle with `?v=7`.

`codemirror.js` is CodeMirror 6 (MIT), bundled once from npm and vendored unmodified. It defines the global `CM` with the exports listed in `entry.js`. The editor applet (`js/editor.js`) loads it only when the editor opens.

On 2026-11-05 the shared stylesheet editor added the `css` export from pinned `@codemirror/lang-css` 6.3.1, which was already an indirect dependency of Markdown's HTML support. The reproducible build is now `apps/md-editor/language-service/build.mjs` with its adjacent lockfile and output manifest. Build dependencies stay outside `apps`, as that folder's README describes. The kit loads this bundle with `?v=6`. No dependency version changed in this rebuild.

Built on 2026-09-29 with esbuild 0.28.2:

```
npm install esbuild @codemirror/state @codemirror/view @codemirror/commands \
  @codemirror/language @codemirror/search @codemirror/lint @codemirror/autocomplete \
  @codemirror/lang-markdown @codemirror/lang-json @codemirror/legacy-modes @lezer/highlight
npx esbuild entry.js --bundle --format=iife --global-name=CM --minify \
  --legal-comments=eof --outfile=codemirror.js
```

It was rebuilt on 2026-10-07 from the same package versions, with `Transaction`, `historyField`, `searchPanelOpen`, `getSearchQuery`, `setSearchQuery` and `SearchQuery` added to the exports, so the editor can keep its undo history and its search across a reload. Before the change, a rebuild from those versions matched the old bundle byte for byte. Installing exact versions needs `--save-exact` and npm `overrides` for the indirect packages, at the versions `LICENSE.txt` lists.

The main versions are `@codemirror/view` 6.43.13 and `@codemirror/state` 6.7.6; `LICENSE.txt` lists every package in the bundle with its version and licence. The bundle is about 600 KB, or 200 KB compressed, most of it the Markdown language, which brings in the HTML, CSS and JavaScript highlighters for HTML inside Markdown.

To upgrade, repeat the build in a scratch directory with the new versions, replace the three files here in a new directory named for the `@codemirror/view` version, and update the path in `js/editor.js`.

It was rebuilt again on 2026-10-18 from the same package versions, with `Decoration`, `ViewPlugin`, `MatchDecorator`, `WidgetType` and `hoverTooltip` from `@codemirror/view` and `autocompletion`, `completionKeymap`, `startCompletion` and `acceptCompletion` from `@codemirror/autocomplete` added to the exports, for the shortcode highlighting, completion and hover in the editor's text view. A rebuild of the previous `entry.js` first matched the old bundle byte for byte. The bundle grew to about 630 KB. The editor loads it with `?v=3`.

It was rebuilt again on 2026-10-21 from the same package versions, with `closeSearchPanel`, `findNext`, `findPrevious`, `replaceNext` and `replaceAll` from `@codemirror/search` added to the exports, for the editor's own find bar, which CodeMirror's `search({ createPanel })` puts in its panel. A rebuild of the previous `entry.js` first matched the old bundle byte for byte. The editor loads it with `?v=4`.
