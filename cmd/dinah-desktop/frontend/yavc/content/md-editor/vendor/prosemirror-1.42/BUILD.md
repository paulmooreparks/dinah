# ProseMirror for the Editor's Page View

The Page view's bundle is `prosemirror.js`, with the style sheets `prosemirror.css`, `tables.css` and `gapcursor.css` copied unmodified from `prosemirror-view`, `prosemirror-tables` and `prosemirror-gapcursor`. It is ProseMirror (MIT), bundled from npm and vendored unmodified, and it defines the global `PM` with the exports listed in `entry.js`. The Editor loads it only when Page view first opens. `LICENSE.txt` beside the bundle lists every package in it with its version.

It was built on 2026-10-12 with esbuild 0.28.2, in a scratch directory holding `package.json` and `entry.js` from here. `package.json` pins every package, the indirect ones through `overrides`, so `npm install` followed by this line reproduces the bundle:

```
npx esbuild entry.js --bundle --format=iife --global-name=PM --minify \
  --legal-comments=eof --outfile=prosemirror.js
```

It was rebuilt on 13 October 2026 from the same package versions with `mergeCells`, `splitCell`, `toggleHeaderCell`, `toggleHeaderRow`, `toggleHeaderColumn`, `CellSelection` and `findTable` added to `entry.js`, for building tables with merged and heading cells on the page.

It was rebuilt on 2026-11-05 from the same pinned package versions with `wrapIn` and `selectedRect` exported for quote editing and whole-column GFM alignment. Dependencies are installed in an external scratch directory, outside the served applet folders. The browser bundle URL now uses `?v=2`.

The main version is `prosemirror-view` 1.42.6. The bundle is about 250 KB.

To upgrade, repeat the build in a scratch directory with the new versions, put the bundle, the three style sheets and a new `LICENSE.txt` in a new directory named for the `prosemirror-view` version, and update the paths in `editor-applet.js`.
