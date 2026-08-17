package bench

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dinah/internal/contract"
)

// Comment is one comment of a card: an entity like every other, ordered by
// the timestamp its anchor carries rather than by its directory name.
type Comment struct {
	// ID is the comment's 12-hex identifier.
	ID string
	// Dir is the comment's directory.
	Dir string
	// TS is when the comment was written.
	TS string
	// Author is who wrote it.
	Author string
	// Body is the comment itself.
	Body string
}

// AddComment writes a comment entity under a card and returns it.
func AddComment(cardDir, author, ts, body string) (*Comment, error) {
	collection := filepath.Join(cardDir, CommentsDir)
	id, err := ClaimID(collection, nil)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(collection, id)
	fm := NewFrontmatter()
	fm.Set("ts", ts)
	fm.Set("author", author)
	if err := WriteText(filepath.Join(dir, CommentAnchor), fm.Render(body)); err != nil {
		return nil, err
	}
	return &Comment{ID: id, Dir: dir, TS: ts, Author: author, Body: body}, nil
}

// Comments reads a card's comments in timestamp order.
func Comments(cardDir string) ([]*Comment, error) {
	collection := filepath.Join(cardDir, CommentsDir)
	var comments []*Comment
	for _, id := range ListIDs(collection) {
		dir := filepath.Join(collection, id)
		text, err := ReadText(filepath.Join(dir, CommentAnchor))
		if err != nil {
			continue
		}
		fm, body := ParseAnchor(text)
		comment := &Comment{
			ID:     id,
			Dir:    dir,
			TS:     fm.Value("ts"),
			Author: fm.Value("author"),
			Body:   body,
		}
		comments = append(comments, comment)
	}
	sort.SliceStable(comments, func(i, j int) bool {
		return comments[i].TS < comments[j].TS
	})
	return comments, nil
}

// Attachment is one attachment: the entity wrapping bytes the format never
// inspects, carrying the original filename, a description and provenance.
type Attachment struct {
	// ID is the attachment's 12-hex identifier.
	ID string
	// Dir is the attachment's directory.
	Dir string
	// Filename is the payload's original name.
	Filename string
	// Description is the optional prose describing the attachment.
	Description string
	// Provenance says where the bytes came from.
	Provenance string
}

// AddAttachment copies a file into a new attachment entity of the collection
// belonging to any entity directory: the bench, a state, a card or a comment.
func AddAttachment(ownerDir, source, description, provenance string) (*Attachment, error) {
	collection := filepath.Join(ownerDir, AttachmentsDir)
	id, err := ClaimID(collection, nil)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(collection, id)
	filename := filepath.Base(source)
	fm := NewFrontmatter()
	fm.Set("filename", filename)
	if description != "" {
		fm.Set("description", description)
	}
	fm.Set("provenance", provenance)
	if err := WriteText(filepath.Join(dir, AttachmentAnchor), fm.Render("")); err != nil {
		return nil, err
	}
	if err := copyFile(source, filepath.Join(dir, PayloadDir, filename)); err != nil {
		return nil, err
	}
	attachment := &Attachment{
		ID:          id,
		Dir:         dir,
		Filename:    filename,
		Description: description,
		Provenance:  provenance,
	}
	return attachment, nil
}

// ReplaceAttachment swaps an attachment's payload for the bytes of another
// file, which is a journaled act rather than a quiet overwrite.
func ReplaceAttachment(dir, source string) (*Attachment, error) {
	attachment, err := LoadAttachment(dir)
	if err != nil {
		return nil, err
	}
	payload := filepath.Join(dir, PayloadDir)
	if err := os.RemoveAll(payload); err != nil {
		return nil, err
	}
	filename := filepath.Base(source)
	if err := copyFile(source, filepath.Join(payload, filename)); err != nil {
		return nil, err
	}
	fm, body := loadAnchor(filepath.Join(dir, AttachmentAnchor))
	fm.Set("filename", filename)
	if err := WriteText(filepath.Join(dir, AttachmentAnchor), fm.Render(body)); err != nil {
		return nil, err
	}
	attachment.Filename = filename
	return attachment, nil
}

// LoadAttachment reads an attachment entity from its directory.
func LoadAttachment(dir string) (*Attachment, error) {
	text, err := ReadText(filepath.Join(dir, AttachmentAnchor))
	if err != nil {
		return nil, contract.Refuse(contract.UnknownPath, dir)
	}
	fm, _ := ParseAnchor(text)
	attachment := &Attachment{
		ID:          filepath.Base(dir),
		Dir:         dir,
		Filename:    fm.Value("filename"),
		Description: fm.Value("description"),
		Provenance:  fm.Value("provenance"),
	}
	return attachment, nil
}

// loadAnchor reads an anchor file, returning an empty header when it will not
// read, which is what keeps a caller mid-write from having to decide.
func loadAnchor(path string) (*Frontmatter, string) {
	text, err := ReadText(path)
	if err != nil {
		return NewFrontmatter(), ""
	}
	return ParseAnchor(text)
}

// copyFile copies bytes into a new file, creating the directories above it.
func copyFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// ArchiveTarget is where an entity directory goes when it is archived: the
// archive mirror at its own level, which is one pattern serving every kind at
// every depth.
func ArchiveTarget(dir string) string {
	collection := filepath.Dir(dir)
	parent := filepath.Dir(collection)
	return filepath.Join(parent, ArchiveDir, filepath.Base(collection), filepath.Base(dir))
}

// ArchiveEntity moves an entity's whole directory into the archive mirror,
// history and all.
func ArchiveEntity(dir string) (string, error) {
	target := ArchiveTarget(dir)
	if Exists(target) {
		return "", contract.Refuse(contract.Exists, target)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(dir, target); err != nil {
		return "", err
	}
	return target, nil
}

// DeleteEntity removes an entity's directory and the history inside it.
func DeleteEntity(dir string) error {
	return os.RemoveAll(dir)
}

// StateOccupied reports whether any live card sits in a state, which is what
// keeps a state from being archived or deleted underneath its cards.
func (b *Bench) StateOccupied(id string) bool {
	cards, err := b.Cards()
	if err != nil {
		return true
	}
	for _, card := range cards {
		if card.State == id {
			return true
		}
	}
	return false
}

// EntityRef is a reference resolved to an entity directory and the kind of
// thing that directory holds.
type EntityRef struct {
	// Kind is one of bench, state, card, comment and attachment.
	Kind string
	// Dir is the entity's directory.
	Dir string
	// ID is the entity's identifier, empty for the bench itself.
	ID string
	// Card is the card the entity belongs to, when one does.
	Card *Card
}

// ResolveEntity resolves the reference the entity-shaped commands take: the
// bench itself, a state, a card, or a comment or attachment below one.
func (b *Bench) ResolveEntity(ref string) (*EntityRef, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" || ref == "." || ref == "bench" {
		return &EntityRef{Kind: "bench", Dir: b.Root}, nil
	}
	if state := b.StateByRef(ref); state != nil {
		return &EntityRef{Kind: "state", Dir: filepath.Join(b.Root, StatesDir, state.ID), ID: state.ID}, nil
	}
	head, rest, _ := strings.Cut(ref, "/")
	found, err := b.ResolveCard(head)
	if err != nil {
		return nil, err
	}
	if rest == "" {
		entity := &EntityRef{Kind: "card", Dir: found.Card.Dir, ID: found.Card.ID, Card: found.Card}
		return entity, nil
	}
	path, err := walkBelowCard(found.Card, rest)
	if err != nil {
		return nil, err
	}
	dir := path
	kind := "card"
	switch filepath.Base(path) {
	case CommentAnchor:
		dir, kind = filepath.Dir(path), "comment"
	case AttachmentAnchor:
		dir, kind = filepath.Dir(path), "attachment"
	case ItemAnchor:
		dir, kind = filepath.Dir(path), "item"
	case CardAnchor:
		dir, kind = filepath.Dir(path), "card"
	default:
		return nil, contract.Refuse(contract.UnknownPath, rest)
	}
	return &EntityRef{Kind: kind, Dir: dir, ID: filepath.Base(dir), Card: found.Card}, nil
}
