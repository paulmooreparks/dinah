package resource

import (
	"context"
	"time"
)

// A source that keeps comments on its documents, as a page on a site does,
// offers them through these interfaces, and the documents' comment threads
// (internal/threads, internal/documents) are brought in from it and sent to
// it (docs/design/document-comments.md). The threads layer knows nothing of
// the site; the provider turns a thread's words and messages into the
// site's own comments and back.

// Comment is one comment on a document as its source keeps it: a comment on
// the document as a whole or on a passage of it, or a reply to another.
type Comment struct {
	// ID is the comment's identity on the site, which replies refer to.
	ID string
	// ParentID is the comment this one replies to, or "" for a comment on
	// the document.
	ParentID string
	// Author is who wrote it, as the site has them.
	Author string
	// Text is the comment as Markdown.
	Text string
	// Created is when it was written.
	Created time.Time
	// Selection is the passage an inline comment was made on, as the site
	// gave it, or "" for a comment on the whole document or when the site
	// does not say. It only places the comment; nothing else depends on it.
	Selection string
	// Inline marks a comment made on a passage.
	Inline bool
	// Resolved marks a comment resolved on the site; Dangling one the site
	// resolved itself because the passage it was on is gone. ResolvedBy and
	// ResolvedAt say who last resolved or reopened it and when, both empty
	// for a comment never resolved.
	Resolved   bool
	Dangling   bool
	ResolvedBy string
	ResolvedAt time.Time
}

// CommentReader reads the comments on a document, replies included, each
// reply with its ParentID.
type CommentReader interface {
	ReadComments(ctx context.Context, path string) ([]Comment, error)
}

// NewComment is a thread's first message as a comment to make on a document.
type NewComment struct {
	// Quote is the passage the thread is about, as the document's text has
	// it; Occurrence is which occurrence of it in the text, counting from
	// 0, so that a passage the text has twice is commented where the
	// thread is. A writer that cannot place the comment on the passage
	// makes it a comment on the whole document, quoting the passage.
	Quote      string
	Occurrence int
	// Text is the message as Markdown, and FromAgent marks an agent's.
	Text      string
	FromAgent bool
	// Thread is what the writer may record with the comment about the
	// thread it came from, for a later reader: its ID, key, kind and
	// severity. A writer may ignore it.
	Thread map[string]string
}

// Made is a comment that was made.
type Made struct {
	ID string
	// Note is a line for the person when the comment is not quite what was
	// asked, such as one put on the whole document because its passage
	// could not be found; "" otherwise.
	Note string
}

// CommentWriter makes comments on a document, replies to them and resolves
// them, on the document's own site. A reply to a comment that is no longer
// on the site is refused with ErrNotFound.
type CommentWriter interface {
	AddComment(ctx context.Context, path string, c NewComment) (Made, error)
	ReplyComment(ctx context.Context, path, parentID, text string, fromAgent bool) (string, error)
	// ResolveComment resolves or reopens a comment. A site that keeps no
	// resolution for a kind of comment returns nil and does nothing.
	ResolveComment(ctx context.Context, path, id string, resolved bool) error
}
