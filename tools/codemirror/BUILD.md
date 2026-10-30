# CodeMirror 6 for the Editor and the Diff View

Dinah's copy of the bundle is `cmd/dinah-desktop/frontend/vendor/codemirror-6.43/codemirror.js`. It is CodeMirror 6 (MIT), bundled once from npm with esbuild and vendored unmodified, and it defines the global `CM` with the exports listed in `entry.js`. The Editor (`editor-applet.js`) loads it only when an Editor window opens, and the diff view (`diff-applet.js`) loads the same file.

The recipe came from the Parks Computing site. `package.json` here pins every package in the bundle, the indirect ones through npm `overrides`, so a rebuild from it reproduces the bundle byte for byte; that was checked on 25 October 2026 against the bundle of 10 October before anything was added.

## Building

In a scratch directory holding copies of `package.json` and `entry.js`:

```
npm install
npx esbuild entry.js --bundle --format=iife --global-name=CM --minify \
  --legal-comments=eof --outfile=codemirror.js
```

Copy the result over the vendored `codemirror.js`, record its SHA-256 and the date in the folder's `NOTICE.txt`, and list any added package with its version and licence in `LICENSE.txt`. Adding `--metafile=meta.json` to the esbuild line lists every input file, which is how the package list in `LICENSE.txt` is checked.

## History

The bundle was built on 29 September 2026 with esbuild 0.28.2 and rebuilt on 7 October (`Transaction`, `historyField` and the search query exports) and on 10 October (`Decoration` and `WidgetType`, for the Editor's suggestions). On 25 October 2026 it was rebuilt for the diff view with `@codemirror/merge` 6.12.3 added, together with `gutter`, `GutterMarker`, `ViewPlugin`, `Prec`, `Text`, the HTML, CSS and JavaScript languages that Markdown already brought in, and the small stream languages listed in `entry.js`. The main versions are `@codemirror/view` 6.43.13 and `@codemirror/state` 6.7.6; the bundle is about 715 KB, most of it the Markdown language and the HTML, CSS and JavaScript highlighters it brings in.

To upgrade, repeat the build with the new versions, replace the three files in a new vendor directory named for the `@codemirror/view` version, and update the path in `editor-applet.js` and `diff-applet.js`.
