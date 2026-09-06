// Prints the version of the newest extension release in a tag list.
//
// Usage: node scripts/print-newest-version.mjs <tags file, or - for stdin>
//
// publish-extension.ps1 is the caller. It publishes an archive carrying the
// number a release already cut rather than the manifest's floor, so it has to
// ask which release is newest, and the comparison that answers that lives in
// release-version.mjs where a unit test drives it.
//
// A list carrying no extension release is not an error in the caller's data,
// it is the state this repository is in until the first release lands, so it
// gets a message that says so and a non-zero exit. The caller has already
// checked that its own lookup succeeded before it gets here, so a failure from
// this script means the list came back and carried no release, never that the
// list could not be fetched. Those two are told apart by which call failed.

import { readFileSync } from "node:fs";

import { newestReleaseVersion, parseTagList } from "./release-version.mjs";

const [tagsPath] = process.argv.slice(2);

if (tagsPath === undefined || tagsPath === "") {
	process.stderr.write("usage: print-newest-version.mjs <tags file, or - for stdin>\n");
	process.exit(1);
}

const tags = parseTagList(readFileSync(tagsPath === "-" ? 0 : tagsPath, "utf8"));
const newest = newestReleaseVersion(tags);

if (newest === undefined) {
	process.stderr.write(
		"No extension release exists yet: the release list came back and carries no vscode-v tag. Cut one first, by pushing a change under editors/vscode to main or by running the 'VS Code extension release' workflow from the Actions tab, then run this again.\n",
	);
	process.exit(1);
}

process.stdout.write(`${newest}\n`);
