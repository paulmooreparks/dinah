# Taskbar workspace labels

The overnight assignment specifies workspace-name (applet) for taskbar group labels. Workbench will keep the existing stable applet/workspace tuple keys and use the resolved workspace label before the applet title. Workspace-free applets retain their applet title. Renames and duplicate workspace names continue to use the existing identity and disambiguation rules.

The taskbar ownership suite will verify initial labels, renamed labels, restoration and same-named workspace separation. This change is independent of blocked state glyph adoption and the skins contrast decision. The taskbar-stacks suite passed initial labels, rename, restoration and same-named workspace separation. Its taskbar image was inspected in skins-spatial-20261031-run2. Independent Quality acceptance and merged release gates remain pending.
