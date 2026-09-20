package bench

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"

	"dinah/internal/contract"
)

// The two frontmatter keys a comment gained with dinah-525. Each is written
// here and read by name, on the terms the checklist item's own keys already
// follow, so the write side and the read side cannot drift.
const (
	// CommentDigestField holds a hex-encoded SHA-256 of the comment's body
	// exactly as ParseAnchor returns it. It is what makes a hand edit
	// detectable: a person editing comment.md changes the body and leaves
	// this key alone, and the two no longer agree.
	CommentDigestField = "digest"
	// CommentAuthorUnrecoverableField records that a comment's missing
	// author is a finding rather than an omission. The note migration
	// writes it on a comment whose settling the journal cannot attribute,
	// because an author is plain text with nothing reserved in it and a
	// word like unknown would be indistinguishable from a person of that
	// name.
	CommentAuthorUnrecoverableField = "author_unrecoverable"
)

// CommentDigest is the digest of one comment body: a hex-encoded SHA-256 over
// the body exactly as ParseAnchor returns it, which is the bytes after the
// front matter and after the newline normalisation dinah-514 applies wherever
// Dinah stores text. No part of the front matter is hashed, so the digest
// covers the value a reader sees as the comment and nothing else.
func CommentDigest(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

// StampCommentDigest records the digest of a body on a comment's header.
//
// Every verb that writes a comment's anchor calls this, not only those that
// write the body, and the distinction is what keeps the digest from crying
// wolf. Frontmatter.Render re-serialises the whole file, so a write touching
// one header key rewrites the body's bytes on the way past and can normalise a
// trailing newline; a digest recomputed only on a body write would then
// disagree with a body nobody edited.
//
// What is hashed is the body a reader will get back rather than the body
// handed in, and the two are not always the same string. Render settles the
// trailing newline, WriteText normalises every carriage return on the way to
// disk, and ParseAnchor is the door every reader comes through. So the file is
// composed here to learn what it will hold and the digest is taken over that.
// Hashing the caller's own bytes instead records a digest no reader can
// reproduce, and the first thing this feature would do is report a divergence
// nobody caused: a comment whose text carried a run of carriage returns did
// exactly that.
func StampCommentDigest(fm *Frontmatter, body string) {
	_, stored := ParseAnchor(NormalizeNewlines(fm.Render(body)))
	fm.Set(CommentDigestField, CommentDigest(stored))
}

// CommentDiverged reports whether a comment's stored digest disagrees with the
// body standing beside it, which means the body was edited by something other
// than a verb since the digest was last recorded.
//
// A comment carrying no digest is not diverged. Every comment written before
// dinah-525 is in that state, and one gains a digest the first time a verb
// writes it, so absence is silence rather than an accusation.
func CommentDiverged(fm *Frontmatter, body string) bool {
	stored := fm.Value(CommentDigestField)
	if stored == "" {
		return false
	}
	return stored != CommentDigest(body)
}

// ReadCommentAnchor opens a comment's anchor for a write, returning its whole
// header and its body rather than the fields Comments reads. It is
// ReadItemAnchor's mirror, and it exists for the same reason: a write has to
// put back every key it did not touch.
func ReadCommentAnchor(dir string) (*Frontmatter, string, error) {
	text, err := ReadText(filepath.Join(dir, CommentAnchor))
	if err != nil {
		return nil, "", contract.Refuse(contract.UnknownPath, dir)
	}
	fm, body := ParseAnchor(text)
	return fm, body, nil
}

// WriteCommentAnchor rewrites a comment's anchor from a header and a body,
// stamping the digest over the body being written.
//
// Every writer of a comment's anchor goes through this rather than calling
// WriteText itself, which is what makes "recomputed by every verb that writes
// the anchor" a property of one function instead of a rule each call site has
// to remember.
func WriteCommentAnchor(dir string, fm *Frontmatter, body string) error {
	StampCommentDigest(fm, body)
	return WriteText(filepath.Join(dir, CommentAnchor), fm.Render(body))
}
