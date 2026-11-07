# Group chat Add menu

Group chat uses the regular chat Add menu, with the paperclip glyph and dropdown. Both composers share the menu definitions and reference catalogue.

The menu accepts computer uploads, workspace files, open documents, chat references, windows and applets. Upload limits apply to uploaded files. Reaching that limit disables the computer-file item while leaving references available.

Selected references appear as removable attachment chips. References chosen through the picker are sent only while their token remains in the message. Selecting a chat reference does not notify that chat as a group member. The existing member picker and typed member mentions still control recipients.

The backend resolves workspace files through the existing resource validator and resolves document, chat, window and applet references through the same helper used by regular chat. Groups carry that resolved context in the message body; uploaded files retain their existing storage and display behavior. Selection grants no new permissions and invokes no applet action.

Attachment contents remain in memory and are cleared after a successful send. A failed send retains them. As before, attachments are not restored from draft text or message history.
