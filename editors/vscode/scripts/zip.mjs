// Reads what is inside a zip archive: the entry names, and one entry's bytes.
//
// A vsix is a zip, and the packaging check needs to know which files ended up
// inside one and what one of them says. Entry names live in the central
// directory as plain bytes, so reading them needs no decompression and no
// dependency: the format is fixed by PKWARE's APPNOTE, and the constants below
// are its own. Reading an entry's content needs one step more, because the
// content sits behind a local file header and is usually deflated, which
// node:zlib undoes.

import { readFileSync } from "node:fs";
import { inflateRawSync } from "node:zlib";

const END_OF_CENTRAL_DIRECTORY = 0x06054b50;
const CENTRAL_FILE_HEADER = 0x02014b50;
const LOCAL_FILE_HEADER = 0x04034b50;
const END_OF_CENTRAL_DIRECTORY_SIZE = 22;
const METHOD_STORED = 0;
const METHOD_DEFLATE = 8;

/**
 * Returns one record per archive entry, in central-directory order, carrying
 * everything either reader below needs.
 *
 * The walk is shared so that the two exported functions cannot disagree about
 * which entries an archive holds. Each record has `name` (forward slashes, as
 * the format itself uses), `method`, `compressedSize` and `localHeaderOffset`.
 * The compressed size comes from the central directory rather than from the
 * local header, because a local header written by a streaming producer is
 * allowed to carry zero there and defer the real size to a trailing
 * descriptor.
 */
function readCentralDirectory(path, buffer) {
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

	const entries = [];
	for (let entry = 0; entry < count; entry++) {
		if (buffer.readUInt32LE(at) !== CENTRAL_FILE_HEADER) {
			throw new Error(
				`${path}: central directory entry ${String(entry)} does not start with the expected signature`,
			);
		}
		const method = buffer.readUInt16LE(at + 10);
		const compressedSize = buffer.readUInt32LE(at + 20);
		const nameLength = buffer.readUInt16LE(at + 28);
		const extraLength = buffer.readUInt16LE(at + 30);
		const commentLength = buffer.readUInt16LE(at + 32);
		const localHeaderOffset = buffer.readUInt32LE(at + 42);
		entries.push({
			name: buffer.toString("utf8", at + 46, at + 46 + nameLength),
			method,
			compressedSize,
			localHeaderOffset,
		});
		at += 46 + nameLength + extraLength + commentLength;
	}
	return entries;
}

/**
 * Returns every entry name in the archive at `path`, in central-directory
 * order, with forward slashes as the separator the format itself uses.
 */
export function listZipEntries(path) {
	const buffer = readFileSync(path);
	return readCentralDirectory(path, buffer).map((entry) => entry.name);
}

/**
 * Returns the decompressed bytes of the entry named `name` in the archive at
 * `path`.
 *
 * Entry names alone cannot answer a question about what an entry says, and the
 * pre-release property this repository checks for lives inside the content of
 * `extension.vsixmanifest` rather than in its name, since that entry is present
 * in every archive either way.
 */
export function readZipEntry(path, name) {
	const buffer = readFileSync(path);
	const entry = readCentralDirectory(path, buffer).find((candidate) => candidate.name === name);
	if (entry === undefined) {
		throw new Error(`${path} carries no entry named ${name}`);
	}

	const header = entry.localHeaderOffset;
	if (buffer.readUInt32LE(header) !== LOCAL_FILE_HEADER) {
		throw new Error(`${path}: the local header for ${name} does not start with the expected signature`);
	}
	const nameLength = buffer.readUInt16LE(header + 26);
	const extraLength = buffer.readUInt16LE(header + 28);
	const start = header + 30 + nameLength + extraLength;
	const data = buffer.subarray(start, start + entry.compressedSize);

	if (entry.method === METHOD_STORED) {
		return data;
	}
	if (entry.method === METHOD_DEFLATE) {
		return inflateRawSync(data);
	}
	throw new Error(`${path}: ${name} uses compression method ${String(entry.method)}, which this reader does not decode`);
}
