# Fonts and themes

## Implemented behavior

Dinah Desktop uses Mineral by default and includes Linen as an alternative. Settings > Appearance selects the theme and retains PUDL's Light, Dark and System controls. Preferences apply across workspaces. This design follows Paul's cross-project Architectural Principles.

Theme Editor is an addressable workbench applet. It edits surfaces, actions, status colors, topbar, syntax, lighting and fonts, with isolated light and dark component previews. Save persists a draft; Apply validates and activates it. Built-in themes are immutable. Saving an edited active theme without applying creates a separate draft copy, so Save alone cannot change the running appearance. Apply updates an existing personal theme in place. Global font changes use Apply and therefore retain its identity and name; editing a built-in theme creates a personal copy once. Deleting an active personal theme restores Mineral.

Global font roles cover interface, heading, reading and monospace text. Chat, Editor source, document content and Terminal have personal content-font and size overrides. Shared chrome retains global fonts and colors. Content sizes can range from 6 to 32 pixels, or 6 to 22 pixels for terminals. These are configurable bounds; the defaults remain unchanged. Checkbox labels use PUDL's inline check control. Font suggestions use PUDL combo boxes, and content sizes use PUDL number fields. Content-size changes save and apply when the field commits, without a separate Apply action. An explicit terminal window size takes precedence over its applet default until Reset Text Size removes it. Font families are local names with generic fallbacks; no fonts are downloaded or embedded.

## Storage and interchange

The supported portable format is UTF-8 JSON, `format: dinah-theme`, `version: 1`, with the `.dinah-theme.json` suffix. PUDL 0.83.0 supplies no interchange contract, so this is a Dinah contract. Version 1 remains supported if a PUDL format is adopted later; that would require an explicit importer rather than reinterpreting existing files.

Documents contain a name, complete light/dark palettes and interface, heading, reading and monospace typography objects. Each font has an array of literal family names and a validated generic fallback. Opaque extension data round-trips without becoming CSS. Exports exclude personal applet overrides, window state, workspace paths and mode preferences. Imports create drafts and allocate independent identities; name collisions receive a suffix.

The native `appearance.json` record stores the active identity, personal theme library and overrides together. Shared statefile locking and a content-hash revision prevent stale writes. A validated browser cache provides the last applied appearance before native reconciliation. A corrupt native record is reported and is never silently overwritten. Reload allows retrying after an external correction.

Documents are limited to 256 KiB and libraries to 8 MiB, with 32 personal themes. Names are limited to 100 UTF-8 bytes. Font lists contain at most eight names of at most 128 bytes each. Unknown core fields, unsupported versions, arbitrary CSS, URLs and incomplete palettes are rejected. Palette colors are opaque six-digit hex values; lighting values are bounded percentages. Native import and export use Wails file dialogs.

## Validation and application

Normal text, muted text, status, syntax, topbar and action text are checked at 4.5:1 against their tested backgrounds in both modes. Input boundaries are checked at 3:1. Derived input fills and destructive hover gradients are included. Invalid contrast can be saved in an inactive draft but cannot become active. Automated checks do not establish distinguishability of every status or replace visual review. Linen's light warning and destructive-hover colors were adjusted slightly from the study to pass these checks.

Every palette also sets PUDL's eight category colors (PUDL 0.91.0), which tell things apart without saying anything about state. PUDL requires them of any theme that changes the surfaces or the lighting, and every Dinah theme does. The checks follow chapter 3 of the PUDL specification: each color reaches 3:1 on the workspace, the content surface and the derived input fill; each is at least 12 from the warning, error and success colors in OKLab (multiplied by 100) with a hue at least 20 degrees from theirs, and at least 8 from the accent; and neighboring slots are at least 15 apart, and 8 apart under simulated protanopia and deuteranopia (Machado, Oliveira and Fernandes, 2009). A failure blocks Apply in the same way as a contrast failure. The Theme Editor shows the eight as fields in their own section, and its previews show them as swatches.

The built-in colors were chosen per palette by a search that starts from PUDL's default colors for the mode, scales their chroma to the theme's syntax colors, and keeps the nearest colors that pass every rule with a small margin. A Go test and the browser theme suite check all 22 palettes against every rule.

A theme saved or exported before category colors still loads. A palette with none of the eight takes those of the built-in palette of the same mode whose workspace, surface and input fill are nearest its own, so a copy of a built-in theme gets that theme's colors; a slot that then breaks a rule takes the nearest built-in category color that keeps them. The library on disk changes when it is next saved. If the theme in use cannot be given passing colors this way, Mineral is used in its place and Settings says which theme and why, so its colors can be set in the Theme Editor. A palette with some but not all eight is refused like any incomplete palette. In forced colors every category color is the reader's text color, as PUDL sets it; the window does not apply the theme's values while forced colors are on.

The Editor's diff view colours its lines with PUDL's diff tokens (PUDL 0.93.0). PUDL requires every syntax color to keep 4.5:1 on `--diff-add-bg` and `--diff-del-bg` laid over the input fill, and each sign to keep 3:1, and it lets a theme with other syntax colors set its own tint. Applying a theme therefore sets the two tints to the strongest amount, up to PUDL's 10 percent in half-percent steps, at which both rules hold for that theme and mode.

The dark modes of Solarized, Nord, One Dark Pro and Dracula had almost no tint, and those of GitHub and Gruvbox a faint one, because their syntax colors sat close to 4.5:1 on the input fill. Paul approved lightening those syntax colors slightly on 28 October 2026. Only the colors that limited the tint were raised, in OKLCH lightness, with their hue kept and as much of their chroma as sRGB allows; every other color, the category colors among them, is unchanged, and every contrast rule still holds. Every light palette reaches 10 percent, and so does every dark palette except two: Tokyo Night's added lines get 9 percent, as before, and Dracula's 8, since at 10 its pink keyword and purple names would have lost about a quarter of their chroma and turned pastel.

| Dark palette | Tint before (added, removed) | Tint after (added, removed) | Syntax colors raised |
| --- | --- | --- | --- |
| Solarized | 0, 0.5 | 10, 10 | keyword, string, number and attribute, comment, name and link, tag, error |
| Nord | 0, 0 | 10, 10 | keyword, string, name and link, tag, error |
| One Dark Pro | 0, 0.5 | 10, 10 | keyword, number, comment, name and link, tag, error |
| Dracula | 0, 0 | 8, 10 | keyword, number with name and link, comment, error |
| GitHub | 3, 4.5 | 10, 10 | name and link, error |
| Gruvbox | 2, 2.5 | 10, 10 | keyword, name and link, error |

The largest raise in OKLCH lightness is about 0.05, for Nord, One Dark Pro and Dracula, and about 0.04 for Solarized and Gruvbox, and GitHub's colors moved by less than 0.03. Chroma changed by no more than 0.002 except where a color met the edge of sRGB. GitHub's error red lost 4 percent of its chroma and Gruvbox's 12 percent, and Dracula's purple lost 12 percent, its pink 18 and its red 22.

Application updates CSS variables without remounting applets. CodeMirror requests measurement and xterm updates documented font options and refits without restarting a process. Font loading completion is revision-checked, so a late completion cannot restore an older appearance. Hidden measurements use the shared scheduler. Chats retain a content anchor during scrollback and continue following when already at the end. Drafts and selections remain intact.

## Verification and limits

The native tests cover schema validation, contrast, durable preferences, stale writes, unsafe imports, extensions and corrupt records. The browser theme suite covers selection, fonts, draft isolation, contrast rejection, import/export, failed saves, reload, fallback and narrow settings under the shell's content security policy. Editor and terminal regressions check live updates without recreation. Native file dialogs and platform-specific font rasterization require separate manual verification. The bus page retains its Mineral stylesheet; this editor manages Desktop appearance.

## References

- [Wails dialogs](https://wails.io/docs/reference/runtime/dialog/) document native file selection.
- [xterm options](https://xtermjs.org/docs/api/terminal/interfaces/iterminaloptions/) document font updates.
- [CodeMirror EditorView](https://codemirror.net/docs/ref/#view.EditorView.requestMeasure) documents measurement requests.
- [CSS Font Loading](https://developer.mozilla.org/en-US/docs/Web/API/CSS_Font_Loading_API) documents font readiness.
- [WCAG 2.2](https://www.w3.org/TR/WCAG22/) defines contrast requirements.

View > Color Mode selects Light, Dark or Follow the System independently of the named theme. View > Themes lists built-in and personal themes, marks the active theme, and offers Edit Current Theme and Appearance Settings. The separate Use Default Theme menu command was removed; selecting Mineral provides that operation explicitly.
