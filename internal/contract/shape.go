package contract

// Shape declares what one refusal carries, so that the properties a reader
// needs belong to the refusal rather than to the place it was raised.
type Shape struct {
	// Name is the refusal name this shape governs.
	Name string
	// Subject names the value slot the sentence is about AND that the tool
	// can reach this refusal with empty, so it is declared only where an
	// empty value is reachable rather than wherever the sentence names
	// something. A shape declaring one carries a refusal.<name>.unnamed
	// sibling entry for the case where the value is empty, and its
	// fragments may condition on it.
	Subject string
	// Values are the named values this refusal's own entries fill beyond the
	// detail and the subject, including the three the head supplies. Every
	// one of them is non-empty wherever it renders, every one appears in at
	// least one of this shape's own entries, and the guard reads this list
	// to decide whether a placeholder anywhere in the shape is declared or
	// stray.
	Values []string
	// Fragments are catalog keys spliced after the base sentence, in this
	// order, each rendering only when its own condition holds. A next-step
	// clause split out of the base entry is declared first, because that is
	// the position it held inside the sentence.
	Fragments []Fragment
	// Listing names the enumerable set this refusal prints, empty on a
	// refusal that has none. The head resolves the name to rows.
	Listing string
	// NextStep names the fragments carrying what to do next, as an ordered
	// alternation: the composer renders the first one whose condition holds
	// and skips the rest. Exactly one of this and NoNext is set, and the
	// last key named here carries no condition, so every rendering of this
	// refusal gets exactly one next step. A condition may name the shape's
	// own Subject, which is how a refusal gives its named and unnamed
	// branches different advice.
	NextStep []string
	// NoNext is why this refusal offers no next step, empty on every
	// refusal that offers one.
	NoNext string
}

// Fragment is one catalog fragment spliced onto a refusal's sentence.
type Fragment struct {
	// Key is the catalog key, which is always the refusal's own key with a
	// suffix, so a translator reads the pieces of one sentence together.
	Key string
	// When is the named value whose non-empty presence switches the
	// fragment on, and it may name the shape's Subject. Empty means the
	// fragment is unconditional unless Unless is set.
	When string
	// Unless is the named value whose absence switches the fragment on,
	// which is how one fragment covers the cases another does not reach.
	// At most one of When and Unless is set, and a fragment named in
	// NextStep needs neither, since the alternation already orders them.
	Unless string
}

// The three values the head supplies, which a shape names in Values like any
// other and no raise site fills. A shape may name one only where every raise
// site of its refusal is reached after the value is known, which is why card
// is declared on four refusals rather than on every refusal whose sentence
// mentions one. No guard can check that rule.
const (
	// ValueCard is the card reference the verb resolved.
	ValueCard = "card"
	// ValueCommand is the command word this invocation named.
	ValueCommand = "command"
	// ValueUsage is that command's syntax line, which dinah help <command>
	// already prints.
	ValueUsage = "usage"
)

// Shapes declares the shape of every refusal name the tool raises. A name
// with no entry here fails the build, which is what keeps the name, the
// sentence and the shape from drifting apart.
//
// Three of the sentences below are written for the one command that raises
// their refusal, and the card that adds a second raise site to any of them
// owes a second entry rather than inheriting a sentence written elsewhere.
// refusal.no-reason.next is written for block, refusal.not-requester.next for
// claim, and refusal.not-holder.unnamed together with
// refusal.not-holder.next-unheld for release, which is the one verb that
// raises not-holder today. refusal.layer-collision.next has no raise site at
// all, since LayerCollisionErr is declared and never raised, so its wording
// is a reading of the catalog rather than of a rendering.
var Shapes = []Shape{
	{
		Name:      AtCapacity,
		Fragments: []Fragment{{Key: "refusal.at-capacity.next"}},
		NextStep:  []string{"refusal.at-capacity.next"},
	},
	{
		Name:      Blocked,
		Values:    []string{ValueCard},
		Fragments: []Fragment{{Key: "refusal.blocked.next"}},
		NextStep:  []string{"refusal.blocked.next"},
	},
	{
		Name:      Held,
		Fragments: []Fragment{{Key: "refusal.held.next"}},
		NextStep:  []string{"refusal.held.next"},
	},
	{
		Name:      LayerCollisionErr,
		Fragments: []Fragment{{Key: "refusal.layer-collision.next"}},
		NextStep:  []string{"refusal.layer-collision.next"},
	},
	{
		// The location clause and the repair are two alternations rather
		// than one: an anchor is confirmed with dinah check, a definition
		// file is not, because the command that read it created no
		// workbench to check, and a request argument names no file at all.
		Name:   Malformed,
		Values: []string{"path", "file", ValueUsage},
		Fragments: []Fragment{
			{Key: "refusal.malformed.at", When: "path"},
			{Key: "refusal.malformed.in-file", When: "file"},
			{Key: "refusal.malformed.fix", When: "path"},
			{Key: "refusal.malformed.next-file", When: "file"},
			{Key: "refusal.malformed.next"},
		},
		NextStep: []string{
			"refusal.malformed.fix",
			"refusal.malformed.next-file",
			"refusal.malformed.next",
		},
	},
	{
		Name:      NoOperator,
		Fragments: []Fragment{{Key: "refusal.no-operator.next"}},
		NextStep:  []string{"refusal.no-operator.next"},
	},
	{
		Name:      NoOwner,
		Fragments: []Fragment{{Key: "refusal.no-owner.next"}},
		NextStep:  []string{"refusal.no-owner.next"},
	},
	{
		Name:      NoReason,
		Values:    []string{ValueCard},
		Fragments: []Fragment{{Key: "refusal.no-reason.next"}},
		NextStep:  []string{"refusal.no-reason.next"},
	},
	{
		Name:      NotBlocked,
		Values:    []string{ValueCard},
		Fragments: []Fragment{{Key: "refusal.not-blocked.next"}},
		NextStep:  []string{"refusal.not-blocked.next"},
	},
	{
		// The two branches need different advice, so the alternation reads
		// the subject: a reader whose card somebody else holds is told who
		// to ask, and a reader whose card nobody holds is told to claim it.
		// Neither sentence reaches the other reader.
		Name:    NotHolder,
		Subject: "detail",
		Values:  []string{ValueCard},
		Fragments: []Fragment{
			{Key: "refusal.not-holder.next", When: "detail"},
			{Key: "refusal.not-holder.next-unheld"},
		},
		NextStep: []string{
			"refusal.not-holder.next",
			"refusal.not-holder.next-unheld",
		},
	},
	{
		Name:      NotOperator,
		Fragments: []Fragment{{Key: "refusal.not-operator.next"}},
		NextStep:  []string{"refusal.not-operator.next"},
	},
	{
		Name:      NotRequester,
		Fragments: []Fragment{{Key: "refusal.not-requester.next"}},
		NextStep:  []string{"refusal.not-requester.next"},
	},
	{
		Name:      Terminal,
		Fragments: []Fragment{{Key: "refusal.terminal.next"}},
		NextStep:  []string{"refusal.terminal.next"},
	},
	{
		Name:      UnknownCard,
		Fragments: []Fragment{{Key: "refusal.unknown-card.next"}},
		NextStep:  []string{"refusal.unknown-card.next"},
	},
	{
		// The next step names the command the reader typed, because this
		// refusal answers ls, move, add and next, and a sentence naming any
		// one of them reads as wrong advice from the other three.
		Name:      UnknownState,
		Values:    []string{ValueCommand},
		Listing:   "states",
		Fragments: []Fragment{{Key: "refusal.unknown-state.next"}},
		NextStep:  []string{"refusal.unknown-state.next"},
	},
	{
		// The window clause says what this build reads, so the next step
		// follows it rather than preceding it.
		Name:   UnsupportedVer,
		Values: []string{"floor", "ceiling"},
		Fragments: []Fragment{
			{Key: "refusal.unsupported-version.window", When: "floor"},
			{Key: "refusal.unsupported-version.next"},
		},
		NextStep: []string{"refusal.unsupported-version.next"},
	},
	{
		Name:      AddNeedsAState,
		Fragments: []Fragment{{Key: "refusal.dinah.add-needs-a-state.next"}},
		NextStep:  []string{"refusal.dinah.add-needs-a-state.next"},
	},
	{
		Name:      AmbiguousWorkbench,
		Values:    []string{"base"},
		Listing:   "workbenches",
		Fragments: []Fragment{{Key: "refusal.dinah.ambiguous-workbench.next"}},
		NextStep:  []string{"refusal.dinah.ambiguous-workbench.next"},
	},
	{
		Name:      Exists,
		Fragments: []Fragment{{Key: "refusal.dinah.exists.next"}},
		NextStep:  []string{"refusal.dinah.exists.next"},
	},
	{
		Name:      Interrupted,
		Fragments: []Fragment{{Key: "refusal.dinah.interrupted.next"}},
		NextStep:  []string{"refusal.dinah.interrupted.next"},
	},
	{
		Name:      LastState,
		Fragments: []Fragment{{Key: "refusal.dinah.last-state.next"}},
		NextStep:  []string{"refusal.dinah.last-state.next"},
	},
	{
		// One unconditional next step covers both branches, since the
		// advice for a lock naming nobody is the advice for a lock naming
		// somebody.
		Name:      Locked,
		Subject:   "detail",
		Fragments: []Fragment{{Key: "refusal.dinah.locked.next"}},
		NextStep:  []string{"refusal.dinah.locked.next"},
	},
	{
		// The pair the composer used to pick between by name, re-expressed
		// as an alternation. Its next step already lived outside the base
		// entry, so nothing was split.
		Name:   MultipleWords,
		Values: []string{"count", "label", "example"},
		Fragments: []Fragment{
			{Key: "refusal.dinah.multiple-words.quote-yourself", When: "quoteInText"},
			{Key: "refusal.dinah.multiple-words.example"},
		},
		NextStep: []string{
			"refusal.dinah.multiple-words.quote-yourself",
			"refusal.dinah.multiple-words.example",
		},
	},
	{
		Name:      NoConfiguredWorkbench,
		Fragments: []Fragment{{Key: "refusal.dinah.no-configured-workbench.next"}},
		NextStep:  []string{"refusal.dinah.no-configured-workbench.next"},
	},
	{
		Name:      NoEditor,
		Fragments: []Fragment{{Key: "refusal.dinah.no-editor.next"}},
		NextStep:  []string{"refusal.dinah.no-editor.next"},
	},
	{
		Name:      NoWorkbench,
		Fragments: []Fragment{{Key: "refusal.dinah.no-workbench.next"}},
		NextStep:  []string{"refusal.dinah.no-workbench.next"},
	},
	{
		Name:      NoWorkbenchFound,
		Values:    []string{"home"},
		Fragments: []Fragment{{Key: "refusal.dinah.no-workbench-found.next"}},
		NextStep:  []string{"refusal.dinah.no-workbench-found.next"},
	},
	{
		Name:      Occupied,
		Fragments: []Fragment{{Key: "refusal.dinah.occupied.next"}},
		NextStep:  []string{"refusal.dinah.occupied.next"},
	},
	{
		Name:      RepairWouldEmptyStates,
		Fragments: []Fragment{{Key: "refusal.dinah.repair-would-empty-states.next"}},
		NextStep:  []string{"refusal.dinah.repair-would-empty-states.next"},
	},
	{
		Name:      Unconfirmed,
		Fragments: []Fragment{{Key: "refusal.dinah.unconfirmed.next"}},
		NextStep:  []string{"refusal.dinah.unconfirmed.next"},
	},
	{
		// This refusal carries no listing and points at dinah help, which is
		// the operator's ruling: thirty bare command names beside a grouped,
		// annotated listing one keystroke away is not help, for the same
		// reason unknown-card declines to list a workbench's cards.
		Name:      UnknownVerb,
		Fragments: []Fragment{{Key: "refusal.dinah.unknown-command.next"}},
		NextStep:  []string{"refusal.dinah.unknown-command.next"},
	},
	{
		Name:      UnknownGuide,
		Listing:   "guides",
		Fragments: []Fragment{{Key: "refusal.dinah.unknown-guide.next"}},
		NextStep:  []string{"refusal.dinah.unknown-guide.next"},
	},
	{
		// The next step names no subcommand, because this refusal answers
		// config get and config set alike and the command word is config
		// either way.
		Name:      UnknownKey,
		Listing:   "settings",
		Fragments: []Fragment{{Key: "refusal.dinah.unknown-key.next"}},
		NextStep:  []string{"refusal.dinah.unknown-key.next"},
	},
	{
		// Two of the fifteen raise sites name a path on the filesystem and
		// thirteen name something inside the workbench, so the next step
		// splits on the value that separates the families.
		Name:   UnknownPath,
		Values: []string{"file"},
		Fragments: []Fragment{
			{Key: "refusal.dinah.unknown-path.next-file", When: "file"},
			{Key: "refusal.dinah.unknown-path.next"},
		},
		NextStep: []string{
			"refusal.dinah.unknown-path.next-file",
			"refusal.dinah.unknown-path.next",
		},
	},
	{
		Name:      UnreadableBench,
		Fragments: []Fragment{{Key: "refusal.dinah.unreadable-workbench.next"}},
		NextStep:  []string{"refusal.dinah.unreadable-workbench.next"},
	},
	{
		// The clause split out of the base entry sat ahead of the dash hint
		// inside the sentence, so its fragment is declared ahead of it here
		// and a reader sees the three pieces in the order they were written.
		Name: Usage,
		Fragments: []Fragment{
			{Key: "refusal.dinah.usage.next"},
			{Key: "refusal.dinah.usage.dash-hint", When: "dashHint"},
		},
		NextStep: []string{"refusal.dinah.usage.next"},
	},
	{
		Name:      WorkbenchNotApplicable,
		Values:    []string{"source"},
		Fragments: []Fragment{{Key: "refusal.dinah.workbench-not-applicable.next"}},
		NextStep:  []string{"refusal.dinah.workbench-not-applicable.next"},
	},
}

// ShapeOf returns the shape governing a refusal name, or nil for a name no
// shape declares, which the composer renders through refusal.unknown.
func ShapeOf(name string) *Shape {
	for i := range Shapes {
		if Shapes[i].Name == name {
			return &Shapes[i]
		}
	}
	return nil
}

// Fragment returns the fragment a shape declares under a key, or nil when it
// declares none, which the guard fails a shape for.
func (s *Shape) Fragment(key string) *Fragment {
	for i := range s.Fragments {
		if s.Fragments[i].Key == key {
			return &s.Fragments[i]
		}
	}
	return nil
}

// NamedInNextStep reports whether a fragment key is one of the alternation's
// members, which is what tells the composer to render it by alternation rather
// than by its own condition.
func (s *Shape) NamedInNextStep(key string) bool {
	for _, named := range s.NextStep {
		if named == key {
			return true
		}
	}
	return false
}
