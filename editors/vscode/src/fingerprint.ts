// The digest a translated catalogue entry records for the English it was
// translated from.
//
// A translation that matched its English when somebody wrote it goes on
// reading as a translation after the English moves, so no check asking "is
// this string still English?" can see the drift. The entry stores a
// fingerprint of the English it was made from, and the guard in
// test/unit/l10n-staleness.test.ts recomputes the fingerprint from the
// English of the day and reports the entry when the two disagree.
//
// The algorithm is FNV-1a 64-bit over the UTF-8 bytes of the text, formatted
// as lowercase hexadecimal with no leading zeroes, which is the same
// algorithm internal/msg.Fingerprint uses in Go. This is an independent
// implementation of that algorithm and not a port of that function, and
// nothing anywhere compares a value produced here against a value produced
// there. The two catalogues are separate, each guard reads only its own side,
// and no requirement of byte parity across the two languages exists or is
// claimed.

/** The FNV-1a 64-bit offset basis, as the algorithm fixes it. */
const OFFSET_BASIS = 0xcbf29ce484222325n;

/** The FNV-1a 64-bit prime, as the algorithm fixes it. */
const PRIME = 0x100000001b3n;

/** Keeps the running hash inside 64 bits, which BigInt does not do for us. */
const MASK = 0xffffffffffffffffn;

/**
 * Returns a short, stable digest of text.
 *
 * Two calls on the same text return the same value on any machine and under
 * any Node release, because FNV-1a is a pure function of the bytes handed to
 * it and this function fixes both the encoding and the formatting.
 */
export function fingerprint(text: string): string {
	const bytes = new TextEncoder().encode(text);
	let hash = OFFSET_BASIS;
	for (const byte of bytes) {
		hash = (hash ^ BigInt(byte)) & MASK;
		hash = (hash * PRIME) & MASK;
	}
	return hash.toString(16);
}
