// Lists the entry names inside a zip archive.
//
// A vsix is a zip, and the packaging check needs to know which files ended up
// inside one. Entry names live in the central directory as plain bytes, so
// reading them needs no decompression and no dependency: the format is fixed
// by PKWARE's APPNOTE, and the three constants below are its own.

import { readFileSync } from "node:fs";

const END_OF_CENTRAL_DIRECTORY = 0x06054b50;
const CENTRAL_FILE_HEADER = 0x02014b50;
const END_OF_CENTRAL_DIRECTORY_SIZE = 22;

/**
 * Returns every entry name in the archive at `path`, in central-directory
 * order, with forward slashes as the separator the format itself uses.
 */
export function listZipEntries(path) {
	const buffer = readFileSync(path);

	// The end-of-central-directory record is last, but a zip comment may sit
	// behind it, so the signature is searched for from the end.
	let end = -1;
	for (let at = buffer.length - END_OF_CENTRAL_DIRECTORY_SIZE; at >= 0; at--) {
		if (buffer.readUInt32LE(at) === END_OF_CENTRAL_DIRECTORY) {
			end = at;
			break;
		}
	}
	if (end < 0) {
		throw new Error(`${path} does not end in a zip central directory record`);
	}

	const count = buffer.readUInt16LE(end + 10);
	let at = buffer.readUInt32LE(end + 16);

	const names = [];
	for (let entry = 0; entry < count; entry++) {
		if (buffer.readUInt32LE(at) !== CENTRAL_FILE_HEADER) {
			throw new Error(
				`${path}: central directory entry ${String(entry)} does not start with the expected signature`,
			);
		}
		const nameLength = buffer.readUInt16LE(at + 28);
		const extraLength = buffer.readUInt16LE(at + 30);
		const commentLength = buffer.readUInt16LE(at + 32);
		names.push(buffer.toString("utf8", at + 46, at + 46 + nameLength));
		at += 46 + nameLength + extraLength + commentLength;
	}
	return names;
}
