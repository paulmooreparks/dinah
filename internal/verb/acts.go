package verb

import (
	"slices"

	"dinah/internal/bench"
)

// OfferedActs is what may be offered for one card to the owner a request
// names: each act a real act would pass every check it runs before writing,
// save the ones that read an argument the person has yet to give.
type OfferedActs struct {
	Claim   bool
	Release bool
	Comment bool
	// Moves are the destinations MoveDestinations answers, in flow order.
	Moves []LegalMove
	// Items are the card's checklist items, each with the item acts offered
	// on it.
	Items []OfferedItem
	// Add says a card may be filed on the workbench, and Pull that the
	// column the request names would give the owner a card to pull.
	Add, Pull bool
	// Block, Unblock, Raise, Attach and File say the card verb of that name
	// would pass the rows it runs before reading what the person types.
	Block, Unblock, Raise, Attach, File bool
	// RaiseTiers are the declared tiers a raise of the card at its column
	// would accept, in the order the workbench declares them.
	RaiseTiers []string
	// Archive, Restore and Delete say the card may be archived, restored
	// from the archive or deleted; DeleteForce says the forced form of the
	// deletion would pass where the plain form would not, which it never
	// does for a card, since only a designated comment needs the force.
	Archive, Restore, Delete, DeleteForce bool
	// Edit says the card resolves to a file an editor can be handed.
	Edit bool
	// Grants are the permissions the card may be granted, and Revokes the
	// ones it holds that may be revoked.
	Grants  []string
	Revokes []string
	// Links are the card's links that may be removed, and LinkNew says a
	// new one may be made.
	Links   []OfferedLink
	LinkNew bool
	// Joins are the live workstreams the card may join, and Leaves the
	// workstreams it may leave, each as the reference join and leave take.
	Joins  []string
	Leaves []string
	// Divergences are the card's comments whose edited body may be made the
	// record, and Renames the card's attachments that may be renamed, each
	// as the reference the verb takes.
	Divergences []string
	Renames     []string
	// Fields are the card's fields set may write, the built-in fields
	// first in the order the format declares them, then the declared
	// fields the workbench lets a card carry, in byte order.
	Fields []string
}

// OfferedItem is one checklist item of the card and the item acts the
// request's owner may take on it: each is true where the act would pass every
// check it runs before writing, save the ones that read the answer or the
// reason the person has yet to type.
type OfferedItem struct {
	// Ref is the item's reference, per kind, as dinah list <card>/checklist
	// prints it, such as dinah-12/criteria/3.
	Ref   string
	Kind  string
	State string
	Owner string
	// Scheme is the evidence scheme the item demands, empty where it
	// demands none.
	Scheme string
	// Text is the item's first line, capped as the listing caps it.
	Text                                                 string
	Resolve, Verify, Fail, Waive, Withdraw, Reopen, Cite bool
}

// Offered reports whether any item act is offered on the item.
func (item OfferedItem) Offered() bool {
	return item.Resolve || item.Verify || item.Fail || item.Waive || item.Withdraw || item.Reopen || item.Cite
}

// OfferedLink is one link of the card that may be removed: its kind, the
// target as the card stores it, which unlink takes, and the target's
// reference as a person reads it.
type OfferedLink struct {
	Kind string
	To   string
	Ref  string
}

// OfferActs answers the acts the request's owner may take on the card the
// request names, on the workbench, and in the column the request names for a
// pull. It takes no lock and writes nothing.
//
// Every answer comes from the function the act itself calls, so no row is
// written twice: canComment for the comment, admit for the rows Do runs before
// the card's lock, canClaim and canRelease for those two acts, and
// destinationsFor, through canRoute and canLand, for each move; and one
// shared check function per card verb beside them, named where each member is
// answered. The card's affordance list is read only as the upper bound on
// claim, release and move, because it is a list by state: an active card lists
// release whoever holds it, and a ready card lists claim whatever the caller's
// tier.
//
// A lapsed claim is cleared on the card in memory, as MoveDestinations clears
// it, so the offer answers what the act would find after the act's own lapse.
// Three refusals of a real act are left unchecked here, as they are by
// MoveDestinations: a read or write that fails, a lock another process holds,
// and a stale basis, each of which the head answers when the act meets it.
//
// A request carrying Archived asks about a card in the archive, which is
// offered restore and delete alone.
func (l *Library) OfferActs(req *Request) (*OfferedActs, error) {
	if req.Archived {
		return l.offerArchived(req), nil
	}
	offered := &OfferedActs{}
	offered.Add = l.admitAdd(asking(req, "add")) == nil && l.addHasColumns(asking(req, "add")) == nil
	if req.Column != "" {
		pulled, err := l.offerPull(req)
		if err != nil {
			return nil, err
		}
		offered.Pull = pulled
	}
	commentReq := asking(req, "comment")
	if _, refused := l.canComment(commentReq); refused == nil {
		offered.Comment = true
	}
	// dinah edit asks nothing of the owner, the harness or the operator: it
	// resolves the file and hands it to the editor, so the offer asks the
	// same resolver before the rows the library's own acts run.
	_, err := l.Bench.ResolveEditTarget(req.Card)
	offered.Edit = err == nil
	found, refused := l.admit(req)
	if refused != nil {
		return offered, nil
	}
	card := found.Card
	if card.Lapsed(l.Now()) {
		clearLapsedClaim(card)
	}
	allowed := l.affordances(card)
	if slices.Contains(allowed, Claim) {
		offered.Claim = l.canClaim(asking(req, Claim), card) == nil
	}
	if slices.Contains(allowed, Release) {
		offered.Release = l.canRelease(asking(req, Release), card) == nil
	}
	if slices.Contains(allowed, Move) {
		moves, err := l.destinationsFor(req, card)
		if err != nil {
			return nil, err
		}
		offered.Moves = moves
	}
	offered.Block = l.canBlock(asking(req, Block), card) == nil
	offered.Unblock = l.canUnblock(asking(req, Unblock), card) == nil
	l.offerRaise(req, offered)
	l.offerCardEntity(req, card, offered)
	items, err := l.offerItems(req, card)
	if err != nil {
		return nil, err
	}
	offered.Items = items
	l.offerGrants(req, card, offered)
	l.offerLinks(req, card, offered)
	if err := l.offerWorkstreams(req, card, offered); err != nil {
		return nil, err
	}
	if err := l.offerMembers(req, card, offered); err != nil {
		return nil, err
	}
	offered.Fields = l.offerFields(req, card)
	return offered, nil
}

// asking is a copy of the request naming another verb, which is the request
// each shared check is asked with.
func asking(req *Request, verb string) *Request {
	copied := *req
	copied.Verb = verb
	return &copied
}

// cardAsking is a copy of the request naming a verb and reaching the card
// through the reference parameter, which the verbs that take any entity read.
func cardAsking(req *Request, verb, ref string) *Request {
	copied := asking(req, verb)
	copied.Ref = ref
	return copied
}

// offerArchived answers the offer on a card in the archive: restore, where
// canRestore passes, and nothing else, since every other card verb resolves
// the live half alone. Delete resolves the live half too, so an archived card
// is deleted by restoring it first.
func (l *Library) offerArchived(req *Request) *OfferedActs {
	offered := &OfferedActs{}
	_, refused := l.canRestore(cardAsking(req, "restore", req.Card))
	offered.Restore = refused == nil
	return offered
}

// offerPull answers whether a pull into the column the request names would
// take a card and pass every row the pull runs before it writes. The head
// selection runs as Pull runs it, through pullHead, and the rows of the pull
// itself run through canPull on the head it chose, with the head's lapsed
// claim cleared in memory as the act would clear it.
func (l *Library) offerPull(req *Request) (bool, error) {
	pulling := asking(req, Pull)
	pulling.Card = ""
	head, answer := l.pullHead(pulling)
	if answer != nil || head == nil {
		return false, nil
	}
	card, err := l.Bench.LoadCardIn(l.Bench.CardsRoot(), head.ID)
	if err != nil {
		return false, err
	}
	if card.Lapsed(l.Now()) {
		clearLapsedClaim(card)
	}
	_, _, _, refusal, err := l.canPull(pulling, card)
	if err != nil {
		return false, err
	}
	return refusal == nil, nil
}

// offerRaise answers Raise and the tiers it would accept, from canRaise and
// raiseColumn, which Raise itself runs around the reason.
func (l *Library) offerRaise(req *Request, offered *OfferedActs) {
	raising := asking(req, Raise)
	card, refused := l.canRaise(raising)
	if refused != nil {
		return
	}
	column, refused := l.raiseColumn(raising, card)
	if refused != nil {
		return
	}
	offered.RaiseTiers = l.raiseTiers(raising, card, column)
	offered.Raise = len(offered.RaiseTiers) > 0
}

// offerCardEntity answers the acts that reach the card as an entity by its
// reference: attach through canAttach, file through canFile, and archive and
// delete through admitRemoval and canRemove.
func (l *Library) offerCardEntity(req *Request, card *bench.Card, offered *OfferedActs) {
	ref := card.Ref(l.Bench.Slug)
	_, refused := l.canAttach(cardAsking(req, "attach", ref))
	offered.Attach = refused == nil
	_, refused = l.canFile(asking(req, "file"))
	offered.File = refused == nil
	for _, verb := range []string{"archive", "delete"} {
		removing := cardAsking(req, verb, ref)
		entity, refused := l.admitRemoval(removing)
		if refused != nil {
			continue
		}
		if l.canRemove(removing, entity) != nil {
			continue
		}
		if verb == "archive" {
			offered.Archive = true
			continue
		}
		offered.Delete = true
	}
}

// offerItems answers every item of the card with the item acts whose checks
// pass. Each item's target is built from the item's own files without taking
// the card's lock, and each act's shared check function is asked of it:
// canCloseItem and canCloseItemEvidence for resolve, verify and fail,
// canWaive, canWithdraw, canReopen, and admitItem for cite, which every
// other act's check follows too.
func (l *Library) offerItems(req *Request, card *bench.Card) ([]OfferedItem, error) {
	items, err := bench.Items(card.Dir)
	if err != nil {
		return nil, err
	}
	cardRef := card.Ref(l.Bench.Slug)
	kindPosition := map[string]int{}
	var offered []OfferedItem
	for _, item := range items {
		kindPosition[item.Kind]++
		position, err := memberPosition(item.Dir, bench.ItemAnchor)
		if err != nil {
			return nil, err
		}
		ref := itemRef(cardRef, item.Kind, kindPosition[item.Kind], position)
		fm, body, err := bench.ReadItemAnchor(item.Dir)
		if err != nil {
			return nil, err
		}
		row := OfferedItem{
			Ref:    ref,
			Kind:   item.Kind,
			State:  item.State,
			Owner:  fm.Value(bench.ItemOwnerField),
			Scheme: fm.Value(bench.ItemEvidenceField),
			Text:   capRunes(firstLine(body), subjectCap),
		}
		citing := cardAsking(req, "cite", ref)
		if _, refused := l.admitItem(citing); refused != nil {
			offered = append(offered, row)
			continue
		}
		loaded, err := bench.LoadItem(item.Dir)
		if err != nil {
			return nil, err
		}
		target := &itemTarget{ref: ref, dir: item.Dir, card: card, item: loaded, fm: fm, body: body}
		row.Cite = true
		row.Resolve = l.closeOffered(cardAsking(req, "resolve", ref), target, bench.ItemResolved)
		row.Verify = l.closeOffered(cardAsking(req, "verify", ref), target, bench.ItemVerified)
		row.Fail = l.closeOffered(cardAsking(req, "fail", ref), target, bench.ItemFailed)
		row.Waive = l.canWaive(cardAsking(req, "waive", ref), target) == nil
		_, refused := l.canWithdraw(cardAsking(req, "withdraw", ref), target)
		row.Withdraw = refused == nil
		row.Reopen = l.canReopen(cardAsking(req, "reopen", ref), target) == nil
		offered = append(offered, row)
	}
	return offered, nil
}

// closeOffered answers whether a terminal verb landing at state passes the
// rows closeItem runs on either side of the designation.
func (l *Library) closeOffered(req *Request, target *itemTarget, state string) bool {
	if l.canCloseItem(req, target, state) != nil {
		return false
	}
	return l.canCloseItemEvidence(req, target) == nil
}

// offerGrants answers the permissions the card may be granted and the ones
// it may have revoked, from admitGrantVerb and canRevoke asked of every
// permission the grant verbs know.
func (l *Library) offerGrants(req *Request, card *bench.Card, offered *OfferedActs) {
	for _, permission := range []string{CriterionRetirement} {
		granting := asking(req, GrantPermission)
		granting.Permission = permission
		if l.admitGrantVerb(granting, card) == nil {
			offered.Grants = append(offered.Grants, permission)
		}
		revoking := asking(req, RevokePermission)
		revoking.Permission = permission
		if l.canRevoke(revoking, card) == nil {
			offered.Revokes = append(offered.Revokes, permission)
		}
	}
}

// offerLinks answers whether a new link may be made and which of the card's
// links may be removed, from malformedHarness and admitLink, which link and
// unlink both run first, and from the target resolver unlink runs on each
// stored target.
func (l *Library) offerLinks(req *Request, card *bench.Card, offered *OfferedActs) {
	linking := asking(req, "link")
	if l.malformedHarness(linking, nil) != nil {
		return
	}
	if _, refused := l.admitLink(linking); refused != nil {
		return
	}
	offered.LinkNew = true
	for _, link := range card.Links {
		to, refusal := l.Bench.ResolveLinkTarget(link.To)
		if refusal != nil || to != link.To {
			continue
		}
		offered.Links = append(offered.Links, OfferedLink{Kind: link.Kind, To: link.To, Ref: l.linkRef(link.To)})
	}
}

// offerWorkstreams answers the live workstreams the card may join and the
// workstreams it may leave, from canJoin, which join and leave run before
// they read the workstream named.
func (l *Library) offerWorkstreams(req *Request, card *bench.Card, offered *OfferedActs) error {
	if l.canJoin(asking(req, Join), card) != nil {
		return nil
	}
	live, err := l.Bench.Workstreams()
	if err != nil {
		return err
	}
	for _, workstream := range live {
		if slices.Contains(card.Workstreams, workstream.ID) {
			continue
		}
		offered.Joins = append(offered.Joins, workstreamHandle(workstream))
	}
	for _, id := range card.Workstreams {
		workstream := l.Bench.Workstream(id)
		if workstream == nil {
			offered.Leaves = append(offered.Leaves, id)
			continue
		}
		offered.Leaves = append(offered.Leaves, workstreamHandle(workstream))
	}
	return nil
}

// workstreamHandle is what join and leave take for a workstream: its slug
// where it has one and its identifier otherwise.
func workstreamHandle(workstream *bench.Workstream) string {
	if workstream.Slug != "" {
		return workstream.Slug
	}
	return workstream.ID
}

// offerMembers answers the card's comments whose edited body may be made the
// record, through canAcceptDivergence, and its attachments that may be
// renamed, through canRename, each by the reference the verb takes.
func (l *Library) offerMembers(req *Request, card *bench.Card, offered *OfferedActs) error {
	cardRef := card.Ref(l.Bench.Slug)
	comments, err := bench.Comments(card.Dir)
	if err != nil {
		return err
	}
	for _, comment := range comments {
		fm, body, err := bench.ReadCommentAnchor(comment.Dir)
		if err != nil || !bench.CommentDiverged(fm, body) {
			continue
		}
		position, err := memberPosition(comment.Dir, bench.CommentAnchor)
		if err != nil || position == 0 {
			continue
		}
		ref := commentRef(cardRef, position)
		if _, refused := l.canAcceptDivergence(cardAsking(req, "accept-divergence", ref)); refused == nil {
			offered.Divergences = append(offered.Divergences, ref)
		}
	}
	attachments, err := bench.Attachments(card.Dir)
	if err != nil {
		return err
	}
	for _, attachment := range attachments {
		position, err := memberPosition(attachment.Dir, bench.AttachmentAnchor)
		if err != nil || position == 0 {
			continue
		}
		ref := attachmentRef(cardRef, position)
		if _, refused := l.canRename(cardAsking(req, "rename", ref)); refused == nil {
			offered.Renames = append(offered.Renames, ref)
		}
	}
	return nil
}

// offerFields answers the card's fields set may write: every built-in field
// of a card, then every declared field the workbench lets a card carry, where
// canWriteEntity, the rows every field write runs that read neither the field
// nor the value, passes.
func (l *Library) offerFields(req *Request, card *bench.Card) []string {
	ref := card.Ref(l.Bench.Slug)
	setting := cardAsking(req, "set", ref)
	entity, err := l.Bench.ResolveEntity(ref)
	if err != nil {
		return nil
	}
	if l.Bench.Operator == "" || l.canWriteEntity(setting, entity) != nil {
		return nil
	}
	fields := bench.FieldsOf(bench.KindCard)
	declared := l.Bench.DeclaredFieldKeysOn(bench.KindCard)
	slices.Sort(declared)
	return append(fields, declared...)
}
