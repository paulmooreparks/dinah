package verb

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// rootCursorVersion is the shape number the merged token carries, so a token
// minted by a later shape is refused by an earlier binary rather than misread
// by it. It is numbered independently of the single-workbench cursorVersion,
// because the two shapes are not versions of each other.
const rootCursorVersion = 1

// rootCursor is what a root-scoped changes call hands back, rendered as
// base64url of this object exactly as the single-workbench cursor is.
//
// It does not reuse that cursor's shape. A cursor is keyed to one workbench's
// own slug and journal digests, and this is a set of those tokens rather than
// a bigger one of them. Nothing here decodes or re-derives a member token:
// each is stored and replayed exactly as that workbench's own Changes call
// produced it, which is what keeps the merged token from having to know
// anything about how a member token is built.
//
// Entries is keyed on each workbench's absolute path because a path is the
// only handle a walk has today. A workbench carries a title and a slug, and
// neither is unique across a tree, and the identifier the format keys the user
// base by reaches no reader. dinah-285 is the card that gives a workbench an
// identifier worth keying on, and when it lands this map keys on that instead,
// which also takes the operator's directory names off the process command line
// where the CLI head carries this token as a flag value.
type rootCursor struct {
	Version int    `json:"v"`
	Root    string `json:"root"`
	// Entries maps each workbench's absolute path, the same string
	// bench.Candidate.Path carries, to that workbench's own cursor token as
	// Library.Changes already minted and returned it.
	Entries map[string]string `json:"entries"`
}

// encode renders a merged cursor as the opaque token a caller carries.
func (c rootCursor) encode() (string, error) {
	raw, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// decodeRootCursor reads a merged token back, refusing rather than resyncing,
// which is the discipline decodeCursor already holds for one workbench: a
// silent resync would hide the caller's bug and lose the events it was owed.
func decodeRootCursor(token string) (rootCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(token))
	if err != nil {
		return rootCursor{}, contract.Refuse(contract.Malformed, token)
	}
	var read rootCursor
	if err := json.Unmarshal(raw, &read); err != nil {
		return rootCursor{}, contract.Refuse(contract.Malformed, token)
	}
	if read.Version != rootCursorVersion || read.Root == "" {
		return rootCursor{}, contract.Refuse(contract.Malformed, token)
	}
	if read.Entries == nil {
		read.Entries = map[string]string{}
	}
	return read, nil
}

// RootChangeSet is one dinah changes --root answer.
type RootChangeSet struct {
	// Root is the directory the walk started from, as the caller named it.
	Root string `json:"root"`
	// Cursor is the token to hand back at the next checkpoint, opaque exactly
	// as ChangeSet.Cursor is, present on every answer including one reporting
	// no change anywhere.
	Cursor string `json:"cursor"`
	// Changed is true when any workbench beneath Root changed, or a new one
	// appeared. A caller wanting to know whether to do any further work at
	// all reads only this field.
	Changed bool `json:"changed"`
	// Workbenches are the walk's rows in its own order, never nil.
	Workbenches []WorkbenchChangeSet `json:"workbenches"`
}

// WorkbenchChangeSet is one workbench's row of a RootChangeSet.
type WorkbenchChangeSet struct {
	bench.Candidate
	// New is true when this workbench carried no entry in the cursor handed
	// in, meaning it appeared beneath Root since that cursor was minted. Its
	// Changes field then carries a freshly minted per-workbench cursor and no
	// events, because a workbench a caller has never asked about before has
	// no since to report against, which is the contract a first
	// single-workbench changes call already carries.
	New bool `json:"new,omitempty"`
	// Unanswered is the refusal name this workbench's own Changes call
	// raised, empty on a row that answered and on a row that never opened. It
	// is separate from the embedded Candidate.Refused, which says the
	// workbench would not read: a row carrying Unanswered read perfectly
	// well, carries its title and its slug, and declined the question. A
	// client drawing a workbench it could not read differently from one that
	// declined a read asks which field is set, rather than inspecting how a
	// refusal name is spelled.
	Unanswered string `json:"unanswered,omitempty"`
	// Changes is this workbench's own answer, absent on every row of a
	// minting call, on a row that would not read, and on a row that read and
	// did not answer.
	Changes *ChangeSet `json:"changes,omitempty"`
}

// ChangesForest walks root and asks every opened workbench the same Changes
// question, replaying each workbench's own token out of the merged cursor as
// that workbench's own Since, and re-assembling the results into one merged
// cursor for the answer.
//
// Minting, which is req.Since empty: every opened workbench is asked to mint
// its own cursor, its Changes field is left nil and its New left false, and
// Workbenches carries every row so a refused workbench is visible on the very
// first call. Changed is false. This mirrors the single-workbench first-call
// contract one level up, where a first call answers what happens from now
// rather than what already happened.
//
// Replay, which is req.Since decoded: a workbench the merged cursor names
// replays its own token exactly as a single-workbench call would; a workbench
// the walk opened that the cursor does not name is minted fresh and marked
// New; and a workbench the cursor names that this walk no longer finds is
// dropped when the new cursor is built and is not reported as a row at all.
// The walk rather than this function is the record of what exists beneath
// root, so a workbench it no longer finds is not this call's fact to assert
// one way or the other.
//
// A workbench the walk found and could not open keeps whatever entry the
// cursor already held for it, so a workbench that is briefly unreadable does
// not lose its position and replay its whole history on the call after it
// becomes readable again.
func ChangesForest(root, home string, req *Request, maxDepth int) (*RootChangeSet, error) {
	minting := strings.TrimSpace(req.Since) == ""
	held := rootCursor{Entries: map[string]string{}}
	if !minting {
		read, err := decodeRootCursor(req.Since)
		if err != nil {
			return nil, err
		}
		if read.Root != root {
			return nil, contract.Refuse(contract.Malformed, req.Since)
		}
		held = read
	}
	rows, err := forestCandidates(root, home, maxDepth)
	if err != nil {
		return nil, err
	}
	answer := &RootChangeSet{Root: root, Workbenches: make([]WorkbenchChangeSet, 0, len(rows))}
	minted := rootCursor{Version: rootCursorVersion, Root: root, Entries: map[string]string{}}
	for _, row := range rows {
		member, token := changesFor(row, req, held, minting)
		if token == "" {
			// A row that could not answer keeps the position the handed-in
			// cursor already held for it, and contributes nothing when the
			// cursor held nothing, since there is no position to record for a
			// workbench that has never answered.
			token = held.Entries[row.Candidate.Path]
		}
		if token != "" {
			minted.Entries[row.Candidate.Path] = token
		}
		if member.Changed() {
			answer.Changed = true
		}
		answer.Workbenches = append(answer.Workbenches, member)
	}
	token, err := minted.encode()
	if err != nil {
		return nil, err
	}
	answer.Cursor = token
	return answer, nil
}

// Changed reports whether this row is one a caller has to look at: a workbench
// whose own answer moved, or one that appeared since the cursor was minted. It
// is a method rather than a field because it restates two facts the row
// already carries, and a third copy of them could disagree with both.
func (w WorkbenchChangeSet) Changed() bool {
	if w.New {
		return true
	}
	return w.Changes != nil && w.Changes.Changed
}

// changesFor asks one row its own Changes question, exactly once, and returns
// the row and the token that row's answer minted. A row that would not open
// carries the walk's own Refused, a row whose Changes call refused carries
// that name on Unanswered instead, and either way the row answers with no
// token and leaves every sibling still to be asked.
//
// The token is returned beside the row rather than read back off it, because a
// minting call reports no ChangeSet by contract while still having a token to
// record. Asking a second time to recover it would parse every journal twice
// and could record a position the caller was never told about.
func changesFor(row forestRow, req *Request, held rootCursor, minting bool) (WorkbenchChangeSet, string) {
	member := WorkbenchChangeSet{Candidate: row.Candidate}
	if row.Library == nil {
		return member, ""
	}
	// The request is copied rather than mutated, because one Request is handed
	// to every workbench in the walk and each needs its own Since.
	own := *req
	entry, known := held.Entries[row.Candidate.Path]
	own.Since = entry
	set, err := row.Library.Changes(&own)
	if err != nil {
		member.Unanswered = refusalNameOf(err)
		return member, ""
	}
	if minting {
		return member, set.Cursor
	}
	member.New = !known
	member.Changes = set
	return member, set.Cursor
}
