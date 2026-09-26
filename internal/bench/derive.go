package bench

import (
	"path/filepath"
)

// This file holds the derive functions of the entity kinds and the readers
// that go through them. Each derive function is a pure function of the
// anchor's path and bytes, so a resident snapshot may build it once per
// content and hand the same value to every request, and each reader clones
// what Derive answered before it hands it out.

// deriveItem is DeriveItem's derive function.
func deriveItem(path, text, _ string) (any, error) {
	return itemFromText(filepath.Dir(path), text), nil
}

// deriveComment is DeriveComment's derive function.
func deriveComment(path, text, _ string) (any, error) {
	dir := filepath.Dir(path)
	return commentFromText(dir, filepath.Base(dir), text), nil
}

// deriveAttachment is DeriveAttachment's derive function. It builds what the
// attachment's anchor says and leaves Path empty, because the payload's name
// is read from the payload directory and not from the anchor, and a value
// memoised on the anchor's bytes must not depend on another file.
func deriveAttachment(path, text, _ string) (any, error) {
	dir := filepath.Dir(path)
	return attachmentFromText(dir, filepath.Base(dir), text), nil
}

// itemAt reads the item whose directory is dir through src. Its error is
// the anchor's read error.
func itemAt(src Source, dir string) (*Item, error) {
	anchor := filepath.Join(dir, ItemAnchor)
	observeAnchor(anchor)
	value, err := src.Derive(anchor, DeriveItem, deriveItem)
	if err != nil {
		return nil, err
	}
	return value.(*Item).Clone(), nil
}

// commentAt reads the comment whose directory is dir through src.
func commentAt(src Source, dir string) (*Comment, error) {
	anchor := filepath.Join(dir, CommentAnchor)
	observeAnchor(anchor)
	value, err := src.Derive(anchor, DeriveComment, deriveComment)
	if err != nil {
		return nil, err
	}
	return value.(*Comment).Clone(), nil
}

// attachmentAt reads the attachment whose directory is dir through src,
// with the path of the file its payload directory holds.
func attachmentAt(src Source, dir string) (*Attachment, error) {
	anchor := filepath.Join(dir, AttachmentAnchor)
	observeAnchor(anchor)
	value, err := src.Derive(anchor, DeriveAttachment, deriveAttachment)
	if err != nil {
		return nil, err
	}
	attachment := value.(*Attachment).Clone()
	withPayload(src, attachment)
	return attachment, nil
}

// withPayload fills an attachment's Path from its payload directory, empty
// when the directory holds no file.
func withPayload(src Source, attachment *Attachment) {
	payload, err := payloadOf(src, attachment.Dir)
	if err != nil {
		payload = ""
	}
	attachment.Path = payload
}

// rereader is the mark of a source that reads the file again on every call
// and memoises nothing, which Disk carries. A composition over such a source
// keeps each anchor's text itself, so it reads each anchor at most once; over
// a source that memoises, it asks the source, whose memo outlives the
// composition.
type rereader interface {
	rereads()
}

// rereads marks Disk as a source that memoises nothing.
func (Disk) rereads() {}
