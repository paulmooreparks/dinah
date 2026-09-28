package verb

import (
	"errors"
	"path/filepath"
	"strconv"
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// RedactionReport is what dinah redact answers: the member, the lines that
// carry its text, split into its own and the legacy answer lines, the
// journal they stand in, and the attachments the redaction leaves readable.
// Written says whether the lines were rewritten, which a run without --yes
// never does.
type RedactionReport struct {
	// Member is the reference the member was named by.
	Member string `json:"member"`
	// Kind is comment or item.
	Kind string `json:"kind"`
	// Journal is the journal the rewritten lines stand in.
	Journal string `json:"journal"`
	// OwnLines are the member's own lines carrying its text, and
	// LegacyLines the legacy answer lines carrying a version of it. Lines
	// is their sum, which the redacted line records.
	OwnLines    int `json:"own_lines"`
	LegacyLines int `json:"legacy_lines"`
	Lines       int `json:"lines"`
	// Written reports that the lines were rewritten.
	Written bool `json:"written"`
	// AttachmentsLeft are the member's attachments, which a redaction does
	// not touch and which stay readable.
	AttachmentsLeft []AttachmentLeft `json:"attachments_left"`
	// LeftoverRemoved is a stale journal.ndjson.redact the run found beside
	// the journal and removed.
	LeftoverRemoved string `json:"leftover_removed,omitempty"`
}

// AttachmentLeft is one attachment a redaction left in place.
type AttachmentLeft struct {
	// Ref is the attachment's reference.
	Ref string `json:"ref"`
	// Filename is its payload's name.
	Filename string `json:"filename"`
}

// Redact runs dinah redact: it replaces every text one comment or one item's
// journal lines carry with the text's SHA-256 and records the act, or, without
// the confirmation, answers what it would rewrite and writes nothing.
//
// It is the workbench operator's alone, and is refused, in this order, a
// workbench declaring no operator and any other actor; a store below the
// card-unit format or part way through its migration; a reference to
// anything but a comment or an item; a member already redacted; and a
// journal beside which a torn-tail sidecar stands.
func (l *Library) Redact(req *Request) (*RedactionReport, error) {
	if refused := l.malformedHarness(req, nil); refused != nil {
		return nil, contract.Refuse(contract.MalformedHarness, req.Harness)
	}
	if l.Bench.Operator == "" {
		return nil, contract.Refuse(contract.NoOperator, "")
	}
	if req.Actor != l.Bench.Operator {
		return nil, contract.Refuse(contract.NotOperator, req.Actor)
	}
	// A reference that names nothing, or names a collection, is refused as
	// every other command refuses it, since that is about what the caller
	// typed rather than about the store. Whether what it names can be
	// redacted is decided after the store's format, which is the order the
	// refusals are documented in.
	target, dir, err := l.redactTarget(req.Ref)
	var refusal *contract.Refusal
	if err != nil && (!errors.As(err, &refusal) || refusal.Name != contract.NotRedactable) {
		return nil, err
	}
	if !l.Bench.CardUnit() || l.Bench.Migrating != "" {
		return nil, contract.RefuseWith(contract.StoreAwaitingMigration, l.Bench.Root, map[string]string{
			bench.ValueMigration: bench.MigratingStorage,
		})
	}
	if err != nil {
		return nil, err
	}
	report := &RedactionReport{Member: req.Ref, Kind: target.Kind, Journal: target.Journal, AttachmentsLeft: []AttachmentLeft{}}
	if target.Kind == bench.KindComment && dir != "" {
		attachments, err := bench.Attachments(dir)
		if err != nil {
			return nil, err
		}
		for i, attachment := range attachments {
			report.AttachmentsLeft = append(report.AttachmentsLeft, AttachmentLeft{
				Ref:      req.Ref + "/" + bench.AttachmentsDir + "/" + strconv.Itoa(i+1),
				Filename: attachment.Filename,
			})
		}
	}
	now := bench.Stamp(l.Now())
	var held *bench.Lock
	if req.Confirm {
		held, err = l.Bench.Acquire(target.LockDir, req.Actor, now)
		if err != nil {
			return nil, err
		}
		defer held.Release()
	}
	account, err := bench.Redact(held, *target, bench.Event{TS: now, Actor: req.Acting()}, req.Confirm)
	if err != nil {
		return nil, err
	}
	report.OwnLines, report.LegacyLines, report.Lines = account.Own, account.Legacy, account.Lines()
	report.Written = req.Confirm
	report.LeftoverRemoved = account.Leftover
	return report, nil
}

// redactedAnchor reports whether a comment's or an item's composed anchor
// says dinah redact replaced its text.
func redactedAnchor(entity *bench.EntityRef, fm *bench.Frontmatter) bool {
	member := entity.Kind == bench.KindComment || entity.Kind == bench.KindItem
	return member && fm.Value(bench.RedactedField) == "true"
}

// redactTarget resolves what dinah redact names. A comment or an item any
// reference reaches is taken as it resolves; anything else a reference
// reaches, a payload included, is refused dinah.not-redactable. A reference
// that reaches nothing is tried as a member by identifier under its holder,
// which is how an archived or a deleted member is named, and the refusal the
// reference first met stands where that finds nothing either. dir is the
// directory a comment's attachments hang in.
func (l *Library) redactTarget(ref string) (*bench.RedactTarget, string, error) {
	entity, resolveErr := l.Bench.ResolveEntity(ref)
	if resolveErr == nil {
		if entity.Kind != bench.KindComment && entity.Kind != bench.KindItem {
			return nil, "", contract.Refuse(contract.NotRedactable, ref)
		}
		column := ""
		if entity.Card == nil {
			column = entity.Holder
		}
		target, found, err := l.Bench.FindRedactTarget(entity.Card, column, entity.Kind, entity.ID)
		if err != nil {
			return nil, "", err
		}
		if !found {
			return nil, "", contract.Refuse(contract.UnknownPath, ref)
		}
		return target, entity.Dir, nil
	}
	if prefix, ok := strings.CutSuffix(ref, "/payload"); ok {
		if _, err := l.Bench.ResolveEntity(prefix); err == nil {
			return nil, "", contract.Refuse(contract.NotRedactable, ref)
		}
	}
	parts := strings.Split(ref, "/")
	n := len(parts)
	if n < 3 || !bench.IsID(parts[n-1]) {
		return nil, "", resolveErr
	}
	collection, id := parts[n-2], parts[n-1]
	holder, err := l.Bench.ResolveEntity(strings.Join(parts[:n-2], "/"))
	if err != nil {
		return nil, "", resolveErr
	}
	var target *bench.RedactTarget
	found := false
	switch {
	case holder.Kind == bench.KindCard && collection == bench.CommentsDir:
		target, found, err = l.Bench.FindRedactTarget(holder.Card, "", bench.KindComment, id)
	case holder.Kind == bench.KindCard && collection == bench.ChecklistSegment:
		target, found, err = l.Bench.FindRedactTarget(holder.Card, "", bench.KindItem, id)
	case holder.Kind == bench.KindItem && collection == bench.CommentsDir:
		target, found, err = l.Bench.FindRedactTarget(holder.Card, "", bench.KindComment, id)
		found = found && target.Item == holder.ID
	case holder.Kind == bench.KindColumn && collection == bench.CommentsDir:
		target, found, err = l.Bench.FindRedactTarget(nil, holder.ID, bench.KindComment, id)
	}
	if err != nil {
		return nil, "", err
	}
	if !found {
		return nil, "", resolveErr
	}
	dir := ""
	if target.Kind == bench.KindComment {
		base := holder.Dir
		if holder.Card != nil {
			base = holder.Card.Dir
		}
		dir = filepath.Join(base, bench.CommentsDir, id)
	}
	return target, dir, nil
}
