# PUDL Skins in Dinah

Paul asked on 30 October 2026 for Dinah and PUDL to plan PUDL's skins in Dinah Desktop, and PUDL Architect sent its proposed host integration the same day. This is Dinah's plan in answer. Nothing here is built, and Dinah takes no skin until PUDL releases it.

PUDL's contract for skins is `docs/SKINS.md` in the PUDL repository. A skin is a structured definition compiled to a stylesheet that sets tokens only. Standard is the default; Bevel and Gel are candidates. Every skin keeps PUDL's fonts, target sizes, keyboard behaviour, mounted content and chrome order, and the light or dark palette is chosen apart from the skin.

## Who Owns the Preference

Dinah's engine owns the skin preference, in the same appearance store that already holds the colour theme, its custom themes and their overrides (`ReadAppearance`). The skin is one choice for the whole application. It is the person's own setting, so under workgroups (`workgroups.md`) it is kept once in Dinah's home folder and every workgroup reads it. It is not set per workspace or per window.

PUDL's runtime must not keep a second copy, since two stores for one choice can disagree and the one that wins depends on load order. PUDL's release candidate of 30 October provides for this (message 1389). With `data-pudl-persistence="host"` on the root element before `pudl-theme.js` and `pudl-skins.js` load, neither script reads or writes local storage or follows storage events. They take the initial theme from `data-theme-pref` or `data-theme` and the skin from `data-skin`, and their selection calls and change events work as before. Dinah sets the persistence attribute in the markup of `index.html` and of the bus's web page, which also ends Dinah's mirroring of `pudl-theme`.

The page keeps a cache of the last applied skin in local storage under a key of Dinah's, beside the existing appearance cache, used only to draw the first frame. The engine's value replaces it as soon as it is read, as the colour theme's cache works now.

## Loading Without a Wrong-Skin Flash

The page's Content Security Policy runs no inline script, so the order is set in `index.html` by files.

1. PUDL's base stylesheets (`pudl.css`, `pudl-windows.css`).
2. Every admitted skin's stylesheet, bundled locally with the release, as static links in the head, each marked with its catalogue ID as `data-pudl-skin` (for example `data-pudl-skin="bevel"`), so `pudl-skins.js` registers the loaded links itself before it first reconciles the selection. They are small and local, so loading all of them costs little, and static links in the head block the first paint until they have loaded.
3. A small `appearance-init.js`, placed after those links and before PUDL's scripts, which reads Dinah's cache and writes `data-skin` and `data-theme-pref` on the root. A classic script after a stylesheet waits for that stylesheet, so the skin is loaded by then. It replaces today's `theme-init.js`.
4. PUDL's `pudl-theme.js` and `pudl-skins.js`, which then find the choices on the root. They move from their present place above the stylesheets to here.
5. Dinah's own stylesheets (`brand.css`, `dinah.css`, `app.css` and the applets' sheets), and then `appearance.js`'s palette overrides, so the host's theme stays last, as PUDL's proposal requires.

PUDL registers a skin only once its stylesheet has loaded, and it is not adding a pending state in this release. The static links above cover the first paint. If a skin's stylesheet fails later, Dinah handles the link's error, keeps the preference and shows Standard, and PUDL never reports an unloaded skin as in effect.

Changing the skin later sets it on the root only. No applet is mounted again, and the windows, their layout and the address stay as they are.

The bus's web page (`internal/web`), which the command line serves on the loopback address, is a second PUDL host. It is drawn on the server, so it reads the preference there and writes the skin into the page's root element, with no script and no flash. It takes the same released skins as the window.

## Skins and Dinah's Colour Themes

Dinah layers its own colour themes over PUDL's palettes: the built-in themes such as Mineral, the person's custom themes from the theme editor, overrides by surface, and forced colours. PUDL set the division on 30 October (message 1389). A skin may give default palette values, and a host's palette overrides win over them. Shape and the paints derived from the palette belong to PUDL's skin compiler, and a host does not set them. Dinah's themes and the theme editor therefore set palette values only (with type, which skins leave alone), and Dinah's theme validation refuses a theme that sets anything else. Every combination of theme and skin that Dinah offers is tested.

The theme editor's contrast checks run against every admitted skin, because a skin's gradients or bevels change what a colour sits on.

## What a Skin Reaches

A skin reaches every PUDL surface in the page: the menu bar, taskbar, windows and their chrome, dialogs, controls and applets. It also reaches the theme editor's preview, which is a separate document built in `theme-editor.js`, so that document links the same skin and selects it.

Content keeps its own look: the terminal's palette, the code editor, Markdown and Mermaid diagrams, pictures, and anything an applet draws as content. So does whatever the operating system draws: the native title bar, the tray, the system's file dialogs and its notifications. Each running Dinah is one native window, so once workgroups arrive, several instances read the one preference, and an instance takes a change when its appearance is next read or when the engine announces the change.

## When Dinah Takes a Skin

Dinah takes skins only in a tagged PUDL release, copied into both PUDL folders (`cmd/dinah-desktop/frontend/pudl` and `internal/web/static/pudl`), as with 0.94.0. Only a skin that PUDL marks as admitted in that release appears in Settings. Bevel arrives when it has passed PUDL's full release gates, and Gel after its independent rendered review and the same gates. A preference that names a skin missing from the build shows Standard and keeps the preference, as PUDL's runtime already does.

Settings lists the skins from the release's data, never from names written into Dinah, which follows the same rule as providers.

Dinah's own gates for the release that brings skins are listed below.

- Screenshots of the main window, a chat, the group chat, Settings and the theme editor in every admitted skin, in light and dark, and at a narrow width, reviewed by Dinah UI.
- A suite that changes the skin while applets are open and checks that no applet element was replaced, that window positions, focus and the address are unchanged, and that no request was made.
- A suite that starts with each skin cached and checks the first drawn frame's computed styles, so a wrong-skin flash fails the run.
- A suite with a preference for a skin that is not in the build.
- Every built-in Dinah theme and a custom theme, with each skin, through the contrast checks.
- The bus's web page in each skin.

## Who Builds It

Engine adds the skin to the appearance store and its validation, the event that announces a change, and the web page's server-side rendering. Front End adds the persistence attribute, the stylesheets and `appearance-init.js`, the Skin choice beside the colour theme in Appearance, the theme editor's preview, and the suites. Dinah UI reviews the rendered skins. Dinah Release takes the PUDL release and runs the gates, and Architecture reviews and decides what merges. The work starts when PUDL publishes a release in which at least one skin is admitted.

## What PUDL Provides

Dinah asked PUDL on 30 October for five things, and PUDL answered in its release candidate the same day (message 1389).

1. **Host-owned persistence** is provided by `data-pudl-persistence="host"`, for theme mode and skin alike.
2. **A pending state for a loading skin** is not added in this release. Static links before the initializing script cover the first paint, and Dinah handles a later failure itself, as described above.
3. **Change events** remain available alongside the selection calls.
4. **The token division** is that a skin may give palette defaults which a host's palette overrides win over, while shape and derived paints belong to PUDL's compiler.
5. **The release catalogue** is `dist/pudl-skins.json`, with each skin's ID, label, stylesheet, SHA-256 and admitted status. Settings lists skins from it, and Dinah checks each bundled stylesheet against its SHA-256 when it takes the release. Every skin stays unadmitted until PUDL's final review.
