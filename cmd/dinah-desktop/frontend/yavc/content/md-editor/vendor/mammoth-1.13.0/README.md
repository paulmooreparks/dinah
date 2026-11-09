# Word importer dependencies

The `office.js` bundle contains Mammoth 1.13.0, JSZip 3.10.1 and their pinned runtime dependencies. The bundle is built from the lockfile in `../../import`, using esbuild 0.28.2 and the documented browser entry mappings. It does not use Mammoth's prebuilt browser bundle, whose transitive dependencies differ from the current lockfile. XML parsing uses @xmldom/xmldom 0.8.15. The licenses directory retains the runtime packages' notices.

Set `YAVC_OFFICE_BUILD_ROOT` to an isolated absolute directory and run `node apps/md-editor/import/build.mjs`. Adding `--check` compares the rebuilt bytes to the committed bundle. The build requires npm access, while the resulting importer runs offline. The worker disables external file access and embedded style maps.

Mammoth's documented conversion API and its sanitation warning are described at https://github.com/mwilliamson/mammoth.js. The importer sanitizes the resulting HTML before it reaches the page schema or a visible document.
