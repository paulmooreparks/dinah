# Release notes rendering

Updates and the update dialog use `dinahReleaseNotes.set(root, text)` to render release notes. This helper uses the existing chat Markdown parser and HTML sanitizer. It does not resolve local files. Relative and file links become their label text because release notes have no local workspace directory. Absolute HTTP and HTTPS links use the existing browser action.

Code examples and tables have their own keyboard-accessible horizontal scroll regions. Their width stays within the notes panel. The Updates notes panel retains its fixed height, so changing notes does not move the surrounding controls.

The helper retains the last source text for each root. Repeated status refreshes with unchanged notes preserve the rendered nodes, focus and reading position.

This change is for the next build. It does not change the frozen numbered release archive.
