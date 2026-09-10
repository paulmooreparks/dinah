package verb

import "strings"

// ReferenceKind is one of the kinds of thing a reference may name. The six
// values are the kinds the references guide's opening sentence lists, in the
// order it lists them, and they are the vocabulary a command's declaration is
// written in. Five of them are the columns of that guide's "Which command
// takes what" table; the workstream is the sixth, which the table leaves out
// and a sentence below the table answers instead.
type ReferenceKind string

const (
	ReferenceKindWorkbench  ReferenceKind = "workbench"
	ReferenceKindWorkstream ReferenceKind = "workstream"
	ReferenceKindColumn     ReferenceKind = "column"
	ReferenceKindCard       ReferenceKind = "card"
	ReferenceKindBelowCard  ReferenceKind = "below-card"
	ReferenceKindCollection ReferenceKind = "collection"
)

// ReferenceKindSeparator joins the kinds where a rendered clause names several
// of them. It is punctuation rather than prose, so it is written here rather
// than in the catalogues, and no kind label in any language may carry it.
const ReferenceKindSeparator = "; "

// referenceKindOrder is the order a rendered clause draws the kinds in, which
// is the order the references guide's opening sentence lists them.
var referenceKindOrder = []ReferenceKind{
	ReferenceKindWorkbench,
	ReferenceKindWorkstream,
	ReferenceKindColumn,
	ReferenceKindCard,
	ReferenceKindBelowCard,
	ReferenceKindCollection,
}

// ReferenceKindOrder is the order a rendered clause draws the kinds in. The
// caller gets a copy, so a head that sorts or trims what it is handed cannot
// reorder the kinds for every later reader.
func ReferenceKindOrder() []ReferenceKind {
	order := make([]ReferenceKind, len(referenceKindOrder))
	copy(order, referenceKindOrder)
	return order
}

// MessageKey is where a kind's written label lives.
func (k ReferenceKind) MessageKey() string {
	return "reference.kind." + string(k)
}

// ReferenceKindsFor reports the kinds a command's reference may name, in
// ReferenceKindOrder, and whether the command declares any at all. A command
// absent from the declaration answers false and renders no clause, which the
// roster check in reference_kinds_test.go catches rather than a reader
// meeting an empty parenthesis.
func ReferenceKindsFor(command string) ([]ReferenceKind, bool) {
	granted, ok := referenceKinds[command]
	if !ok {
		return nil, false
	}
	held := map[ReferenceKind]bool{}
	for _, kind := range granted {
		held[kind] = true
	}
	kinds := make([]ReferenceKind, 0, len(granted))
	for _, kind := range referenceKindOrder {
		if held[kind] {
			kinds = append(kinds, kind)
		}
	}
	return kinds, true
}

// referenceKinds is where the three published answers to "what does this
// command take" all come from: the references guide's table and its workstream
// sentence are held to it, the terminal help page renders it, and the tool
// schema renders it. What a command actually accepts is decided by its
// resolver, and this map is a declaration of that rather than the thing that
// enforces it, so it is held against the running binary by the probes in
// cmd/dinah's references_guide_test.go and
// references_command_resolution_test.go, for the cells those probes reach.
var referenceKinds = map[string][]ReferenceKind{
	"path": {
		ReferenceKindWorkbench, ReferenceKindWorkstream, ReferenceKindColumn,
		ReferenceKindCard, ReferenceKindBelowCard, ReferenceKindCollection,
	},
	"edit": {
		ReferenceKindWorkbench, ReferenceKindWorkstream, ReferenceKindColumn,
		ReferenceKindCard, ReferenceKindBelowCard,
	},
	"get": {
		ReferenceKindWorkbench, ReferenceKindWorkstream, ReferenceKindColumn,
		ReferenceKindCard, ReferenceKindBelowCard,
	},
	"set": {
		ReferenceKindWorkbench, ReferenceKindWorkstream, ReferenceKindColumn,
		ReferenceKindCard, ReferenceKindBelowCard,
	},
	"show": {
		ReferenceKindColumn, ReferenceKindCard, ReferenceKindBelowCard,
		ReferenceKindCollection,
	},
	"instructions": {
		ReferenceKindColumn, ReferenceKindCard,
	},
	"attach": {
		ReferenceKindWorkbench, ReferenceKindColumn, ReferenceKindCard,
		ReferenceKindBelowCard,
	},
	"archive": {
		ReferenceKindWorkstream, ReferenceKindColumn, ReferenceKindCard,
		ReferenceKindBelowCard,
	},
	"restore": {
		ReferenceKindWorkstream, ReferenceKindColumn, ReferenceKindCard,
		ReferenceKindBelowCard,
	},
	"delete": {
		ReferenceKindWorkstream, ReferenceKindColumn, ReferenceKindCard,
		ReferenceKindBelowCard,
	},
	"contents": {
		ReferenceKindWorkbench, ReferenceKindWorkstream, ReferenceKindColumn,
		ReferenceKindCard, ReferenceKindBelowCard, ReferenceKindCollection,
	},
	"attachments": {
		ReferenceKindWorkbench, ReferenceKindWorkstream, ReferenceKindColumn,
		ReferenceKindCard, ReferenceKindBelowCard, ReferenceKindCollection,
	},
	"rename":  {ReferenceKindBelowCard},
	"cite":    {ReferenceKindBelowCard},
	"resolve": {ReferenceKindBelowCard},
	"verify":  {ReferenceKindBelowCard},
	"fail":    {ReferenceKindBelowCard},
	"reopen":  {ReferenceKindBelowCard},
}

// ArgumentMeaning composes one argument's written meaning: the sentence written
// for it, followed by the address kinds its reference may name where the
// parameter takes a reference at all. translate is the caller's renderer, so
// this package composes the sentence without knowing where the words come from.
//
// It lives here beside SummaryKey, whose own doc comment says that both heads
// resolve the key in one place so that the page a person reads and the schema
// an agent reads carry one sentence. The clause obeys the same rule, so it is
// composed here rather than in either head.
func ArgumentMeaning(command string, p Param, translate func(key string, args ...string) string) string {
	summary := translate(p.SummaryKey(command))
	if p.Guide != referencesGuide {
		return summary
	}
	kinds, ok := ReferenceKindsFor(command)
	if !ok || len(kinds) == 0 {
		return summary
	}
	labels := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		labels = append(labels, translate(kind.MessageKey()))
	}
	joined := strings.Join(labels, ReferenceKindSeparator)
	return translate("help.reference-kinds", "summary", summary, "kinds", joined)
}
