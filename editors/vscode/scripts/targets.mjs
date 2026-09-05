// The one artifact this extension publishes.

/**
 * The universal artifact carries no binary, and it is the only artifact there
 * is. The extension is a companion to the dinah command-line tool rather than
 * a way of shipping it, so a user installs dinah themselves and the extension
 * finds it on PATH or wherever `dinah.path` names.
 */
export const UNIVERSAL = "universal";
