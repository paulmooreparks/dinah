# Building the shared language services

The checked-in browser artifacts require no package installation at runtime. Editor and Diagrams load the same CodeMirror client and browser worker through `yavcMdEditor.languageServices()`. The worker uses Microsoft's JSON, CSS and HTML language services behind a JSON-RPC LSP adapter. CSS provides diagnostics, completion, hover and formatting. HTML provides completion, hover and formatting, with no general validation API. No filesystem provider or network loader is configured.

`package-lock.json` pins the build inputs. The existing CodeMirror and Lezer versions remain pinned while `@codemirror/lsp-client` 6.3.0 is added. The worker uses `vscode-json-languageservice` 5.6.4 and `vscode-languageserver-textdocument` 1.0.12. esbuild 0.28.2 produces classic, self-contained browser bundles without runtime imports.

Install dependencies outside `apps/`. The server fingerprints that directory, so putting `node_modules` there would also make dependencies part of the served asset tree. From the repository root in PowerShell, run:

```powershell
$lspBuild = Join-Path $env:TEMP 'yavc-lsp-build'
New-Item -ItemType Directory -Force $lspBuild | Out-Null
Copy-Item apps/md-editor/language-service/package*.json $lspBuild
npm ci --ignore-scripts --prefix $lspBuild
$env:YAVC_LSP_BUILD_ROOT = $lspBuild
node apps/md-editor/language-service/build.mjs
node apps/md-editor/language-service/build.mjs --check
node --test apps/md-editor/test/language-service.test.mjs
```

The build writes the shared CodeMirror bundle, `worker.js`, their complete license inventories and `manifest.json`. The manifest records source and output SHA-256 hashes and the packages actually included by esbuild. The module test checks those hashes without npm dependencies; `--check` also reproduces both bundles and license inventories byte for byte.

The npm archive for `@vscode/l10n` 0.0.18 omits its license file. `licenses/vscode-l10n-LICENSE.txt` is the MIT license from Microsoft's matching `l10n/v0.0.18` release tag, Git blob `9e841e7a26e4eb057b24511e7b92d42b257a80e5`. Its upstream location is [microsoft/vscode-l10n LICENSE](https://github.com/microsoft/vscode-l10n/blob/l10n/v0.0.18/LICENSE).

The public contracts used here are documented by [CodeMirror's LSP client](https://code.haverbeke.berlin/codemirror/lsp-client), [Microsoft's JSON language service](https://github.com/microsoft/vscode-json-languageservice) and [LSP 3.17](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/). The schema adds the [yavc extension](../../../docs/canvas-extension.md) to JSON Canvas. The canvas reader still owns reference and semantic validation.

Run `scripts/test-language-services.cjs chromium firefox webkit` with Playwright available through `NODE_PATH` against a disposable local server on port 8412. Set `YAVC_LSP_TEST_PORT` to use another disposable server. `scripts/test-web-languages.cjs` checks CSS and HTML completion after the context goes offline, real completion application and undoable formatting. Markdown catalog assistance has its own fixture in `scripts/test-shortcode-catalog.cjs` and does not use an admin endpoint.

The CSS and HTML service versions are pinned to 6.3.10 and 5.6.2. Their documented APIs are described in the upstream [CSS service](https://github.com/microsoft/vscode-css-languageservice) and [HTML service](https://github.com/microsoft/vscode-html-languageservice) repositories. These services run on the opened source only. Embedded JavaScript inside HTML and project-wide symbol resolution remain outside their scope.
