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
	// Variants are the commands that raise this refusal for a different act
	// and so carry a base entry of their own, refusal.<name>.<command>. One
	// refusal name can answer two acts, and the sentence then depends on
	// which command raised it, so the command word selects the entry the way
	// an absent subject selects the unnamed sibling. A command named here
	// carries its own next-step fragment too, switched on by WhenCommand,
	// because a variant that borrowed the shared clause would end on advice
	// written for another act. A shape declares Variants or a Subject and
	// never both, since no refusal today needs the two selectors at once and
	// the order between them would otherwise go unstated.
	Variants []string
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
	// Carried names the named value whose content this refusal prints as
	// rows, for a set the raise site computed and no enumerable Listing can
	// name. The value holds one member per line, and the composer splits on
	// newlines when it prints it. A shape declares Carried or a Listing and
	// never both, since the two ways to print rows serve the one purpose
	// from two sides and using both would leave the order between them
	// unstated.
	Carried string
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
	// WhenCommand is the command word that switches the fragment on, which
	// is how a shape gives one of its Variants a next step of its own. It
	// reads the command the reader typed rather than a named value, so it
	// answers a question no value carries, and at most one of When, Unless
	// and it is set.
	WhenCommand string
}

// The four values the head supplies, which a shape names in Values like any
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
	// ValueWorkbench is the workbench directory discovery resolved for this
	// invocation. It is the one value here a raise site can also fill, and
	// the head leaves such a value alone: a sweep over a tree knows which of
	// the workbenches it walked raised the refusal, where the head knows
	// only which one the invocation opened. A shape naming it declares an
	// alternative sentence for the runs that resolve no single workbench.
	ValueWorkbench = "workbench"
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
		//
		// A slug the reference grammar would read as a card carries cardRef
		// and splices the clause saying so. It names no location and no
		// file, so the alternation's tail gives it the command-spelling next
		// step, which is the advice a reader who typed the slug needs.
		//
		// The repair alternation leads with the sibling that names the
		// workbench, because a reader told to confirm his hand edit with a
		// check needs the check to reach the file he just edited, and a
		// bare check reaches only what the climb from where he stands
		// reaches. That sibling's condition is the workbench value, and
		// this is the one refusal in the family whose raise sites fill that
		// value themselves: openWithVocabulary attaches it beside the path,
		// so the named repair renders exactly where the unnamed one would
		// have and nowhere else. Naming Malformed in the head's
		// benchScopedAdvice table instead would attach a workbench to every
		// malformed refusal an opened workbench raises, including the empty
		// title dinah add refuses, and hand that reader a repair written
		// about a file he has not touched.
		Name:   Malformed,
		Values: []string{"path", "file", "cardRef", ValueUsage, ValueWorkbench},
		Fragments: []Fragment{
			{Key: "refusal.malformed.at", When: "path"},
			{Key: "refusal.malformed.in-file", When: "file"},
			{Key: "refusal.malformed.reads-as-a-card-reference", When: "cardRef"},
			{Key: "refusal.malformed.fix-named", When: ValueWorkbench},
			{Key: "refusal.malformed.fix", When: "path"},
			{Key: "refusal.malformed.next-file", When: "file"},
			{Key: "refusal.malformed.next"},
		},
		NextStep: []string{
			"refusal.malformed.fix-named",
			"refusal.malformed.fix",
			"refusal.malformed.next-file",
			"refusal.malformed.next",
		},
	},
	{
		// The four raise sites all sit behind an open, so the head has a
		// workbench to name and the named sibling is what renders. The
		// unqualified sibling behind it is the alternation's unconditional
		// last member.
		Name:   NoOperator,
		Values: []string{ValueWorkbench},
		Fragments: []Fragment{
			{Key: "refusal.no-operator.next-named", When: ValueWorkbench},
			{Key: "refusal.no-operator.next"},
		},
		NextStep: []string{
			"refusal.no-operator.next-named",
			"refusal.no-operator.next",
		},
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
		Name:      UnknownColumn,
		Values:    []string{ValueCommand},
		Listing:   "columns",
		Fragments: []Fragment{{Key: "refusal.unknown-column.next"}},
		NextStep:  []string{"refusal.unknown-column.next"},
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
		// The advice tells the reader to confirm the hand edit with a
		// check, and a check reaches the workbench it is about only where
		// the reader stands in it. Every raise site of this refusal sits
		// behind an open, so the head knows which workbench the invocation
		// was about and the named sibling is what renders.
		Name:   AddNeedsAColumn,
		Values: []string{ValueWorkbench},
		Fragments: []Fragment{
			{Key: "refusal.dinah.add-needs-a-column.next-named", When: ValueWorkbench},
			{Key: "refusal.dinah.add-needs-a-column.next"},
		},
		NextStep: []string{
			"refusal.dinah.add-needs-a-column.next-named",
			"refusal.dinah.add-needs-a-column.next",
		},
	},
	{
		// The shape mirrors AtCapacity's, because the two are one mechanism
		// read from opposite ends of the move: one sentence naming the
		// column and one next step naming the way past it. The detail is the
		// departure rather than the destination, since the limit is declared
		// and counted at the column the card is leaving.
		Name:      AtLoopLimit,
		Fragments: []Fragment{{Key: "refusal.dinah.at-loop-limit.next"}},
		NextStep:  []string{"refusal.dinah.at-loop-limit.next"},
	},
	{
		Name:      AmbiguousWorkbench,
		Values:    []string{"base"},
		Listing:   "workbenches",
		Fragments: []Fragment{{Key: "refusal.dinah.ambiguous-workbench.next"}},
		NextStep:  []string{"refusal.dinah.ambiguous-workbench.next"},
	},
	{
		// One next step covers every raise site, because the two ways
		// forward at a waiting column, releasing the card and moving it on
		// once the answer comes, are the same whichever act was refused.
		Name:      AwaitingOutside,
		Fragments: []Fragment{{Key: "refusal.dinah.awaiting-outside.next"}},
		NextStep:  []string{"refusal.dinah.awaiting-outside.next"},
	},
	{
		Name:      Exists,
		Fragments: []Fragment{{Key: "refusal.dinah.exists.next"}},
		NextStep:  []string{"refusal.dinah.exists.next"},
	},
	{
		// The reader of this refusal is the person whose own act was cut
		// short, so he is standing wherever he typed it, which is not
		// necessarily inside the workbench he named. The named sibling
		// carries the finish to that workbench from where he stands.
		Name:   Interrupted,
		Values: []string{ValueWorkbench},
		Fragments: []Fragment{
			{Key: "refusal.dinah.interrupted.next-named", When: ValueWorkbench},
			{Key: "refusal.dinah.interrupted.next"},
		},
		NextStep: []string{
			"refusal.dinah.interrupted.next-named",
			"refusal.dinah.interrupted.next",
		},
	},
	{
		Name:      LastColumn,
		Fragments: []Fragment{{Key: "refusal.dinah.last-column.next"}},
		NextStep:  []string{"refusal.dinah.last-column.next"},
	},
	{
		// One unconditional next step covers both branches, since the
		// advice for a lock naming nobody is the advice for a lock naming
		// somebody.
		Name:    Locked,
		Subject: "detail",
		Values:  []string{ValueWorkbench},
		Fragments: []Fragment{
			{Key: "refusal.dinah.locked.next-named", When: ValueWorkbench},
			{Key: "refusal.dinah.locked.next"},
		},
		NextStep: []string{
			"refusal.dinah.locked.next-named",
			"refusal.dinah.locked.next",
		},
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
		// The directory a caller named holds no workbench.md of its own, and
		// sometimes it holds exactly one workbench store a rung below, which
		// is the address the caller meant. The alternation gives that reader
		// the spelling that would have worked and everybody else the general
		// advice.
		Name:   NoWorkbench,
		Values: []string{"found"},
		Fragments: []Fragment{
			{Key: "refusal.dinah.no-workbench.found", When: "found"},
			{Key: "refusal.dinah.no-workbench.next"},
		},
		NextStep: []string{
			"refusal.dinah.no-workbench.found",
			"refusal.dinah.no-workbench.next",
		},
	},
	{
		// The walk found nothing, and sometimes it walked past a workbench
		// that would have answered before the containment rule: a
		// workbench.md sitting outside any .dinah container. Naming it is the
		// difference between an operator learning that his workbench stopped
		// being found and an operator being told, in the same sentence he
		// would read on an empty machine, that there is nothing there. The
		// alternation gives that reader the repair and everybody else the
		// general advice.
		Name:   NoWorkbenchFound,
		Values: []string{"home", "bare"},
		Fragments: []Fragment{
			{Key: "refusal.dinah.no-workbench-found.bare", When: "bare"},
			{Key: "refusal.dinah.no-workbench-found.next"},
		},
		NextStep: []string{
			"refusal.dinah.no-workbench-found.bare",
			"refusal.dinah.no-workbench-found.next",
		},
	},
	{
		Name:      Occupied,
		Fragments: []Fragment{{Key: "refusal.dinah.occupied.next"}},
		NextStep:  []string{"refusal.dinah.occupied.next"},
	},
	{
		// One next step covers every raise site, because the two ways
		// forward at a column that takes no work up, releasing the card and
		// pulling it into the column beyond, are the same whichever act was
		// refused. It reads as awaiting-outside's does, since the two names
		// carry one rule and differ only in whether a person can be named.
		Name:      TakesNoWork,
		Fragments: []Fragment{{Key: "refusal.dinah.takes-no-work.next"}},
		NextStep:  []string{"refusal.dinah.takes-no-work.next"},
	},
	{
		Name:      RepairWouldEmptyColumns,
		Fragments: []Fragment{{Key: "refusal.dinah.repair-would-empty-columns.next"}},
		NextStep:  []string{"refusal.dinah.repair-would-empty-columns.next"},
	},
	{
		// The repair this recommends carries one workbench forward, and
		// where it looks for that workbench is decided by the same rule
		// every other form of check obeys: it climbs from the current
		// directory unless the caller names a scope. A reader who reached
		// this refusal by naming a workbench with --workbench is not
		// standing anywhere that climb reaches, so the sentence names the
		// workbench for him wherever the head resolved one. The
		// unqualified alternative is the alternation's last member, which
		// carries no condition because the last member of an alternation
		// cannot, and no invocation renders it today. cmd/dinah's
		// advice_test.go records what would have to change for one to.
		Name:   NeedsVocabularyMigration,
		Values: []string{ValueWorkbench},
		Fragments: []Fragment{
			{Key: "refusal.dinah.needs-vocabulary-migration.next-named", When: ValueWorkbench},
			{Key: "refusal.dinah.needs-vocabulary-migration.next"},
		},
		NextStep: []string{
			"refusal.dinah.needs-vocabulary-migration.next-named",
			"refusal.dinah.needs-vocabulary-migration.next",
		},
	},
	{
		Name:      NeedsContainerMigration,
		Fragments: []Fragment{{Key: "refusal.dinah.needs-container-migration.next"}},
		NextStep:  []string{"refusal.dinah.needs-container-migration.next"},
	},
	{
		// The detail names the file inside the workbench, and path names it
		// on disk, so a reader of a tree-wide run learns which of several
		// hundred cards is the one refused. Malformed's location clause is
		// the model, and this refusal keeps only the half of it that
		// applies: every raise site holds a path and none of them reads a
		// definition file or a request argument.
		// The next step alternates for the reason the vocabulary-migration
		// refusal's does. It tells the reader to hand-edit the file and then
		// run the migration, and the migration acts on a workbench rather
		// than on a directory, so the sentence names the workbench wherever
		// the head resolved one.
		Name:   VocabularyMixed,
		Values: []string{"path", ValueWorkbench},
		Fragments: []Fragment{
			{Key: "refusal.dinah.vocabulary-mixed.at", When: "path"},
			{Key: "refusal.dinah.vocabulary-mixed.next-named", When: ValueWorkbench},
			{Key: "refusal.dinah.vocabulary-mixed.next"},
		},
		NextStep: []string{
			"refusal.dinah.vocabulary-mixed.next-named",
			"refusal.dinah.vocabulary-mixed.next",
		},
	},
	{
		// The retired-vocabulary refusal reads as the mixed one does, and
		// carries its own sentence because its file is not mixed. The detail
		// names the card inside the workbench and path names it on disk, so
		// a reader of a tree-wide run learns which card to edit.
		Name:   VocabularyRetired,
		Values: []string{"path"},
		Fragments: []Fragment{
			{Key: "refusal.dinah.vocabulary-retired.at", When: "path"},
			{Key: "refusal.dinah.vocabulary-retired.next"},
		},
		NextStep: []string{"refusal.dinah.vocabulary-retired.next"},
	},
	{
		// One refusal name answers two acts here. delete destroys history,
		// and a slug rename renames every card in the workbench, so the
		// workbench command carries its own sentence and its own next step
		// rather than ending on advice written for delete.
		Name:     Unconfirmed,
		Variants: []string{"workbench", "workstream"},
		Fragments: []Fragment{
			{Key: "refusal.dinah.unconfirmed.workbench.next", WhenCommand: "workbench"},
			{Key: "refusal.dinah.unconfirmed.workstream.next", WhenCommand: "workstream"},
			{Key: "refusal.dinah.unconfirmed.next"},
		},
		NextStep: []string{
			"refusal.dinah.unconfirmed.workbench.next",
			"refusal.dinah.unconfirmed.workstream.next",
			"refusal.dinah.unconfirmed.next",
		},
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
		// One name answers two of query's checks: a field the tool does not
		// have, and a field given an operator it does not take. The sentence
		// names the token the reader typed and lists the fields that are
		// legal in its place, so the detail alone cannot carry it and the
		// field list rides as a value read off the vocabulary itself.
		//
		// The ordered-operator clause is unconditional within the query,
		// because the two raise sites there are indistinguishable to a reader
		// who mistyped: one of them wrote an operator and the other did not,
		// and the clause says which field takes the four ordered ones either
		// way.
		//
		// Two commands raise this name for two acts. A query names a field of
		// the query language, and card names a field a card records, so the
		// command word selects the sentence and the ordered-operator clause
		// stops being unconditional: it is written about the query language's
		// one ranking field and says nothing a card reader can use.
		Name:     UnknownField,
		Values:   []string{"fields", "instantField"},
		Variants: []string{"card"},
		Fragments: []Fragment{
			{Key: "refusal.dinah.unknown-field.ordered", WhenCommand: "query"},
			{Key: "refusal.dinah.unknown-field.card.next", WhenCommand: "card"},
			{Key: "refusal.dinah.unknown-field.next"},
		},
		NextStep: []string{
			"refusal.dinah.unknown-field.card.next",
			"refusal.dinah.unknown-field.next",
		},
	},
	{
		// The declared set rides as a value rather than as a Listing, for the
		// reason unknown-depth gives: which axis's set this refusal
		// enumerates depends on the write that raised it, and a Listing name
		// resolves from the session alone.
		Name:      UnknownLevel,
		Values:    []string{"axis", "levels"},
		Fragments: []Fragment{{Key: "refusal.dinah.unknown-level.next"}},
		NextStep:  []string{"refusal.dinah.unknown-level.next"},
	},
	{
		// The axis rides as a value even though the detail could carry it,
		// because the sentence names the axis twice and the anchor path once,
		// and a detail doing both jobs reads as one of them.
		Name:      NoLevels,
		Values:    []string{"axis", "anchor"},
		Fragments: []Fragment{{Key: "refusal.dinah.no-levels.next"}},
		NextStep:  []string{"refusal.dinah.no-levels.next"},
	},
	{
		// The axis list rides as a value read off the disposition table
		// itself rather than written into the catalog, so an axis added to
		// the vocabulary reaches this sentence without a translator being
		// asked for anything.
		Name:      UnknownAxis,
		Values:    []string{"axes"},
		Fragments: []Fragment{{Key: "refusal.dinah.unknown-axis.next"}},
		NextStep:  []string{"refusal.dinah.unknown-axis.next"},
	},
	{
		// This sentence names the repeated axis and lists nothing, since the
		// axis it names is already one of the legal ones and listing them
		// would say so twice.
		Name:      RepeatedAxis,
		Fragments: []Fragment{{Key: "refusal.dinah.repeated-axis.next"}},
		NextStep:  []string{"refusal.dinah.repeated-axis.next"},
	},
	{
		// Two numbers and no axis name at all, because the chain that was
		// refused may name nothing illegal.
		Name:      ChainTooLong,
		Values:    []string{"asked", "allowed"},
		Fragments: []Fragment{{Key: "refusal.dinah.chain-too-long.next"}},
		NextStep:  []string{"refusal.dinah.chain-too-long.next"},
	},
	{
		// The levels ride as a value rather than as a Listing, because which
		// ladder this refusal enumerates depends on the command that raised
		// it and the head cannot resolve that without learning both.
		Name:      UnknownDepth,
		Values:    []string{"levels"},
		Fragments: []Fragment{{Key: "refusal.dinah.unknown-depth.next"}},
		NextStep:  []string{"refusal.dinah.unknown-depth.next"},
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
		// Two of the raise sites name a path on the filesystem and the rest
		// name something inside the workbench, so the next step splits on the
		// value that separates the families. A third family is a reader
		// reaching a card or a column through whatever holds it, where the
		// segment is a collection that plainly exists and the advice is to
		// name the thing by its own reference instead.
		Name:   UnknownPath,
		Values: []string{"file", "addressed"},
		Fragments: []Fragment{
			{Key: "refusal.dinah.unknown-path.next-file", When: "file"},
			{Key: "refusal.dinah.unknown-path.next-addressed", When: "addressed"},
			{Key: "refusal.dinah.unknown-path.next"},
		},
		NextStep: []string{
			"refusal.dinah.unknown-path.next-file",
			"refusal.dinah.unknown-path.next-addressed",
			"refusal.dinah.unknown-path.next",
		},
	},
	{
		// A closed vocabulary either has values to offer or it does not, and
		// the two branches need different advice: a reader who mistyped a
		// state is told to name one of the values the clause just listed,
		// and a reader who named a workstream no live card lists has nothing
		// to be pointed at, so the term itself is what has to go.
		//
		// The vocabulary is a value rather than a Listing, because which set
		// this refusal enumerates depends on the field the term named, and
		// the head cannot resolve that without learning the query language.
		Name:   UnknownValue,
		Values: []string{"term", "field", "legal"},
		Fragments: []Fragment{
			{Key: "refusal.dinah.unknown-value.legal", When: "legal"},
			{Key: "refusal.dinah.unknown-value.none", Unless: "legal"},
			{Key: "refusal.dinah.unknown-value.next", When: "legal"},
			{Key: "refusal.dinah.unknown-value.next-none"},
		},
		NextStep: []string{
			"refusal.dinah.unknown-value.next",
			"refusal.dinah.unknown-value.next-none",
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
		// A workstream a live card still belongs to is not deleted, and the
		// next step names the two ways past it: take the cards out, or
		// archive the workstream, which is allowed while cards belong to it.
		Name:      Referenced,
		Fragments: []Fragment{{Key: "refusal.dinah.referenced.next"}},
		NextStep:  []string{"refusal.dinah.referenced.next"},
	},
	{
		// The next step names the listing rather than printing it, the way
		// unknown-command names dinah help: a workbench can carry many
		// workstreams, and a reader who mistyped one needs the command that
		// shows them rather than the whole set under a refusal.
		Name:      UnknownWorkstream,
		Fragments: []Fragment{{Key: "refusal.dinah.unknown-workstream.next"}},
		NextStep:  []string{"refusal.dinah.unknown-workstream.next"},
	},
	{
		// The next step names the two ways past an explicit collision: choose
		// a different slug, or drop --slug and let creation derive one the
		// way it already does for a title alone, counting suffix and all.
		Name:      WorkstreamSlugTaken,
		Fragments: []Fragment{{Key: "refusal.dinah.workstream-slug-taken.next"}},
		NextStep:  []string{"refusal.dinah.workstream-slug-taken.next"},
	},
	{
		Name:      WorkbenchNotApplicable,
		Values:    []string{"source"},
		Fragments: []Fragment{{Key: "refusal.dinah.workbench-not-applicable.next"}},
		NextStep:  []string{"refusal.dinah.workbench-not-applicable.next"},
	},
	{
		// OutsideRoot carries the resolved path and the named root value the
		// sentence and its context member both reference, so a reader
		// looking at either side sees the same pair.
		Name:      OutsideRoot,
		Values:    []string{"root"},
		Fragments: []Fragment{{Key: "refusal.dinah.outside-root.next"}},
		NextStep:  []string{"refusal.dinah.outside-root.next"},
	},
	{
		Name:      UnknownRoot,
		Fragments: []Fragment{{Key: "refusal.dinah.unknown-root.next"}},
		NextStep:  []string{"refusal.dinah.unknown-root.next"},
	},
	{
		// The workbench the second scope named rides as a value beside the
		// root in the detail, because the reader has to see both to know
		// which of the two to drop, and a sentence naming only the one they
		// typed last would tell them nothing about the other.
		Name:      ConflictingScope,
		Values:    []string{"workbench"},
		Fragments: []Fragment{{Key: "refusal.dinah.conflicting-scope.next"}},
		NextStep:  []string{"refusal.dinah.conflicting-scope.next"},
	},
	{
		// The detail is the depth the caller wrote, so the sentence quotes
		// the flag back rather than describing it, and the next step names
		// the option that would give it something to bound.
		Name:      DepthWithoutRoot,
		Fragments: []Fragment{{Key: "refusal.dinah.depth-without-root.next"}},
		NextStep:  []string{"refusal.dinah.depth-without-root.next"},
	},
	{
		Name:      MalformedDepth,
		Fragments: []Fragment{{Key: "refusal.dinah.malformed-depth.next"}},
		NextStep:  []string{"refusal.dinah.malformed-depth.next"},
	},
	{
		// A name selector against a collection that declares a name field
		// matched more than one entity. The sentence names the selector and
		// the ordinal of every match, so the caller can retry with one of
		// them as attachments/<n>.
		Name:      AmbiguousName,
		Values:    []string{"selector", "ordinals"},
		Fragments: []Fragment{{Key: "refusal.dinah.ambiguous-name.next"}},
		NextStep:  []string{"refusal.dinah.ambiguous-name.next"},
	},
	{
		// A rename aimed at something that is not an attachment. The
		// detail names what the reference resolved to, so the sentence
		// names the entity the caller meant rather than the word they
		// typed.
		Name:      NotRenamable,
		Values:    []string{"kind"},
		Fragments: []Fragment{{Key: "refusal.dinah.not-renamable.next"}},
		NextStep:  []string{"refusal.dinah.not-renamable.next"},
	},
	{
		// A bare pull found more than one column it could pull into, and
		// the qualifying columns ride as a Carried set rather than a Listing
		// since the value depends on the invocation. The next step names
		// the syntax the reader should type to pick one.
		Name:      AmbiguousColumn,
		Carried:   "columns",
		Fragments: []Fragment{{Key: "refusal.dinah.ambiguous-column.next"}},
		NextStep:  []string{"refusal.dinah.ambiguous-column.next"},
	},
	{
		// A column standing first in the flow refuses at the named form,
		// because no upstream column precedes it. The detail names the
		// column, which a reader needs to see alongside the rule.
		Name:      NoUpstream,
		Values:    []string{"column"},
		Fragments: []Fragment{{Key: "refusal.dinah.no-upstream.next"}},
		NextStep:  []string{"refusal.dinah.no-upstream.next"},
	},
	{
		// A --format or DINAH_FORMAT value naming no output form. The
		// detail carries what the caller wrote, so the sentence shows
		// them their own spelling, and the next step names the forms
		// they may write instead. The set is short enough to say in the
		// sentence, so no listing is declared for it.
		Name:      UnknownFormat,
		Fragments: []Fragment{{Key: "refusal.dinah.unknown-format.next"}},
		NextStep:  []string{"refusal.dinah.unknown-format.next"},
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

// Variant reports whether a command carries a base entry of its own on this
// shape, which is what tells the composer to render refusal.<name>.<command>
// in place of the shape's own base entry.
func (s *Shape) Variant(command string) bool {
	for _, named := range s.Variants {
		if named == command {
			return true
		}
	}
	return false
}

// VariantKeyOf is the base catalog key one of a shape's variants renders.
func (s *Shape) VariantKeyOf(command string) string {
	return "refusal." + s.Name + "." + command
}
