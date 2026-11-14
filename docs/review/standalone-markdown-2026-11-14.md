# Stand-alone Markdown routing

The candidate on `engine/standalone-markdown` selects Docs for `.md` and `.markdown`, ignoring case, while plain text stays in Editor. Its source is `1401f45c` followed by `1e607d46efaaa1ed92d9592c87570d2c5e37866b`. The installed program and Open With registration remain YAVC Editor. No registration change was needed.

The service chooses the applet within the existing application identity. Permissions, recovery keys, save revision checks and process selection are shared. Open reuses an untouched blank window, otherwise it allocates or raises another window. The host drains and retires old recovery handles before replacing a blank mount. New from Docs retains Docs in the receiving window. The design is recorded in `docs/design/standalone-documents.md`.

The compound `.slides.md` name is an exception. The bundled Docs explicitly refuses slide decks in `text-workspace/workspace.js`, so the candidate keeps their previous Editor source behavior. This adds no stand-alone Slides offering and changes no vendor files.

## Checks

The complete document-service suite passed in 6.378 seconds. New tests cover case-insensitive selection, compound-suffix precedence, unchanged application identity, granted-file requirements, refusal to replace a used or modified blank window, recovery transfer, existing-file reuse and Docs selection on a New ticket. The final desktop registration and window-loop tests passed in 8.406 seconds with the desktop tag. They include `TestAStandaloneApplicationIsOfferedInOpenWithAndTakesNoKind`, `TestAnInstalledNameIsItsApplication` and `TestDocumentWindowOutlivesAnOpenDialog`.

The production-tagged window program built successfully with version `standalone-markdown-trial`. Its SHA256 is `3526b589aa0d3c757e2148c445bdf10bc8845f954e713213ade8fdc6ff45b5cc`. The hidden script `scripts/test-standalone-markdown.cjs` exited zero against that binary. It starts the actual headless document service on an isolated home and launches two actual window processes with `DINAH_DOCUMENT_ENTRY_FILE`, one using `--document editor FILE` and one from a copied executable named `yavc-editor.exe`. Their page addresses are opened in headless Edge.

The assertions cover the following behavior:

- Markdown mounts Docs and displays its rich document view, while plain text and slide-deck source mount Editor.
- Save writes the Markdown file, a stale save preserves the draft and external file, and Save As writes the chosen alternative.
- Docs recovers its draft after close, and it also recovers a Markdown draft produced by the text Editor after Save As.
- Agent actions require the document's permission dialog, including refusal and one-time approval.
- Open replaces an untouched blank Editor mount with Docs, preserves the current document when another window is needed, and routes plain text selected from Docs into Editor.
- New from Docs retains Docs, and plain-text Open in a blank Editor stays in that window.
- A recovery record written after replacement belongs to the incoming file, and no page error occurs.

The first browser attempt issued a view command before the applet's ready promise settled. The fixture now waits for PUDL's `data-applet-busy` state to clear. Two later runs passed all assertions but exited nonzero when native launch artifacts could not be deleted (`EPERM`). The final harness intentionally retains those invented artifacts and reports their location. No assertion was removed to obtain the final pass.

## Limits and artifacts

All builds and checks ran through FIFO. No visible window, shared mouse or keyboard action, real-home write, credential-file read, push, merge or installation occurred. The Windows registration test uses an isolated test registry key. Explorer double-click and the actual Open With chooser were not driven; their unchanged registered executable command was tested through the named executable launch. Native Open/Save dialogs were not displayed. The native test windows loaded no document page; rich rendering and the permission dialog were checked in headless Edge.

No owned test window process remained after the final run. Native files remain in `%TEMP%/standalone-markdown-EabAI4`, `standalone-markdown-ZMLzoE`, `standalone-markdown-WWfIjt` and the final `standalone-markdown-IiR64q`. The compiled candidate is `%TEMP%/standalone-markdown-candidate/host.exe`. These contain invented documents and isolated browser profiles only. This task makes no cleanup-pass claim.

The branch is ready for Lead's review. Engine is returning to the separately committed Decisions-origin contract after this handoff.
