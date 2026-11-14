# Stand-alone Docs chrome

The stand-alone Markdown window and the main window mount the same bundled Docs applet. The File menu, formatting commands and editing status therefore come from the package in both hosts. This change adds the host controls that the stand-alone page lacked.

| Surface | Main window | Stand-alone window |
| --- | --- | --- |
| Title | The document name identifies the applet window. | The file name and unsaved dot identify the document. The program remains YAVC Editor, which is the installed application identity that also opens plain text. |
| File | Docs supplies Save and Save As. A publishing source supplies Publish through the host contract. | Docs supplies Save and Save As through the file service. Publish is absent because this service has no publishing source. |
| Document | The host supplies Save As and workspace-specific actions. | The host now supplies Save As by invoking the same applet command. Rename, attachments, source comments and draft disposal are absent because the file service does not implement them. |
| Status | Docs supplies the document name, language, count and position. | The same Docs status line remains intact. |
| Help | The host supplies Keyboard Shortcuts, and Docs supplies Key Bindings. | Help now supplies a document guide, a reference drawn from the current command table, and the applet's own Key Bindings editor. F1 opens the reference. |

The added menu actions follow the active mount's command availability, including while the blank Editor changes into Docs. Neither the package nor the service gains a new save or publish operation. The guide explains the file service's recovery and publishing limits. The architectural principles remain at C:\Users\paul\OneDrive\Documents\Architectural Principles.md.