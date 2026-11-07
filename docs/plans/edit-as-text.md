# Explicit text editing and file actions

Files and Workspace offer Edit as Text for regular local files, regardless of extension. The action opens the existing resource-backed draft in Text view. It does not change double-click defaults or file-type associations.

The existing text reader validates UTF-8, rejects NUL-containing files and enforces its 1 MiB limit when opening. The same resource identity and revision checks protect saving back to the original file. Listing a directory does not read every file to detect its contents.

Both applets use shared item commands for their context menus. Files supports right-click and the keyboard menu shortcut on file rows and its tree. Regular local files can be renamed independently of their extension. Source-specific actions in Workspace remain available.

Successful saves update the Editor status without posting routine success notifications. Save errors, conflicts and other warnings retain their existing reporting.

Remembered file-type preferences remain a separate follow-up.
