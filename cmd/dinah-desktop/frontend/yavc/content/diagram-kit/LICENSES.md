# Licences of the diagram engine

The D2 engine in this folder, `yavc-d2.wasm`, is built from yavc.app's own code (Apache-2.0) and from the open-source code listed here, unmodified, at the versions in the repository's `go.mod`. The rule `engine-licences` in `scripts/check.mjs` fails the build when a module linked into the engine is missing from the first table.

## Go modules compiled into the engine

| Module | Licence |
|---|---|
| `oss.terrastruct.com/d2` | Mozilla Public License 2.0. The source of the version used is at https://github.com/terrastruct/d2 under the tag in `go.mod`. |
| `oss.terrastruct.com/util-go` | ISC |
| `github.com/PuerkitoBio/goquery` | BSD 3-Clause |
| `github.com/alecthomas/chroma/v2` | MIT |
| `github.com/andybalholm/brotli` | MIT |
| `github.com/andybalholm/cascadia` | BSD 2-Clause |
| `github.com/dlclark/regexp2` | MIT |
| `github.com/golang/freetype` | The FreeType License, chosen from its two offered licences. Portions of this software are copyright © 2006 The FreeType Project (www.freetype.org). All rights reserved. |
| `github.com/lucasb-eyer/go-colorful` | MIT |
| `github.com/mazznoer/csscolorparser` | MIT |
| `github.com/rivo/uniseg` | MIT |
| `github.com/yuin/goldmark` | MIT |
| `golang.org/x/exp` | BSD 3-Clause |
| `golang.org/x/image` | BSD 3-Clause |
| `golang.org/x/net` | BSD 3-Clause |
| `golang.org/x/text` | BSD 3-Clause |
| `golang.org/x/xerrors` | BSD 3-Clause |

## Other parts

| Part | Licence |
|---|---|
| dagre, embedded in D2 | MIT, https://github.com/dagrejs/dagre |
| Source Sans Pro, Source Code Pro and Fuzzy Bubbles, the fonts D2 embeds in a drawing | SIL Open Font License 1.1 |
| `wasm_exec.js`, the Go runtime's support for WebAssembly | BSD 3-Clause, the Go Authors |
