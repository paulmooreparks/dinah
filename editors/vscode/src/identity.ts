// The extension's own identity, in one place.
//
// The marketplace identifier is `<publisher>.<name>` and it is permanent from
// the first publish, so it is spelled here and nowhere else. The integration
// tests reach the extension through `vscode.extensions.getExtension(EXTENSION_ID)`,
// the packaging script names the vsix after it, and `package.json` declares the
// two halves separately because a manifest cannot import a module. A unit test
// asserts the manifest and this file agree, which is what keeps the duplication
// from becoming a divergence.

/** The marketplace publisher this extension ships under. */
export const PUBLISHER = "paulmooreparks";

/** The extension's own name, the second half of the marketplace identifier. */
export const EXTENSION_NAME = "dinah";

/** The marketplace identifier, `<publisher>.<name>`. */
export const EXTENSION_ID = `${PUBLISHER}.${EXTENSION_NAME}`;

/** The id of the view container this extension contributes to the activity bar. */
export const VIEW_CONTAINER_ID = "dinah";

/** The id of the single view inside that container. */
export const VIEW_ID = "dinah.workbenchView";

/** The settings key holding an explicit path to the binary. */
export const SETTING_PATH = "dinah.path";

/** The settings key pinning which workbench a folder uses. */
export const SETTING_WORKBENCH = "dinah.workbench";
