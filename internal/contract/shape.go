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
	// Absent is the suffix of the sibling entry rendered where the Subject
	// is empty, without its leading full stop. It is unnamed on every shape
	// that leaves it blank, which is what an absent subject has always meant:
	// a card nobody holds, an owner nobody named. A shape whose empty
	// subject means something else names a suffix that says so, on the terms
	// a gate storing no value is unset rather than unnamed. AbsentKeyOf is
	// how the composer and the guards read it.
	Absent string
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
// Two of the sentences below are written for the one command that raises
// their refusal, and the card that adds a second raise site to either of them
// owes a second entry rather than inheriting a sentence written elsewhere.
// refusal.not-requester.next is written for claim, and
// refusal.not-holder.unnamed together with refusal.not-holder.next-unheld are
// written for release. raise raises not-holder as well, and those two entries
// were read against it and kept, because a card nobody holds still has to be
// claimed before anybody can raise it.
//
// refusal.no-reason was a third until dinah-409. It was written for block, so
// a raise typed with no reason ended on advice to run `dinah block`, which is
// a different act against the same card. raise now carries a variant of its
// own, which is what this paragraph asks a second raise site to do.
//
// refusal.layer-collision.next has no raise site at all, since
// LayerCollisionErr is declared and never raised, so its wording is a reading
// of the catalog rather than of a rendering.
var Shapes = []Shape{
	{
		// Overridable, on the same terms the exit hold's own shape below
		// documents: canRoute refuses req.Override to anybody but the
		// operator before canLand is reached, so a caller who meets this
		// refusal and is not the operator can never pass it regardless of
		// what the next step says, and only the operator's own reading
		// names the flag.
		Name: AtCapacity,
		Fragments: []Fragment{
			{Key: "refusal.at-capacity.next-operator", When: "operator"},
			{Key: "refusal.at-capacity.next"},
		},
		NextStep: []string{
			"refusal.at-capacity.next-operator",
			"refusal.at-capacity.next",
		},
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
		// oneLine carries the one case where the value is neither missing nor
		// unparseable: a field stored as a frontmatter key was given a value
		// carrying a line break. The base sentence's "is missing, empty, or
		// will not parse" does not tell that reader which of the three they
		// met, so the clause says the newline was the problem.
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
		Values: []string{"path", "file", "cardRef", "claimants", "retired", "legalValues", ValueUsage, ValueWorkbench},
		Fragments: []Fragment{
			{Key: "refusal.malformed.one-line", When: "oneLine"},
			{Key: "refusal.malformed.legal-values", When: "legalValues"},
			{Key: "refusal.malformed.at", When: "path"},
			{Key: "refusal.malformed.in-file", When: "file"},
			{Key: "refusal.malformed.reads-as-a-card-reference", When: "cardRef"},
			{Key: "refusal.malformed.claimed-twice", When: "claimants"},
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
		// The harness-variant fragment renders when the request that raised
		// this refusal declared a harness, which is why the configuration
		// rung's own value never resolved: a harness-declaring call is
		// refused rather than promoted to whatever the shared config file
		// carries. It does not replace the base sentence, only the
		// next-step clause, and it never names the configured value,
		// because nothing that fills it ever read that value in the first
		// place.
		Name:   NoOwner,
		Values: []string{ValueHarness},
		Fragments: []Fragment{
			{Key: "refusal.no-owner.next-harness", When: ValueHarness},
			{Key: "refusal.no-owner.next"},
		},
		NextStep: []string{"refusal.no-owner.next-harness", "refusal.no-owner.next"},
	},
	{
		// One refusal name answers two acts. block asks for the question the
		// operator has to answer, and raise asks what the caller found that
		// puts the work beyond the tier the stop declares, so raise carries
		// its own sentence and its own next step. Without the variant a raise
		// typed with no reason ends on advice that blocks the card, which is
		// a different act against the same card, and a caller who follows it
		// does the wrong thing rather than being stopped.
		Name:     NoReason,
		Values:   []string{ValueCard},
		Variants: []string{"raise"},
		Fragments: []Fragment{
			{Key: "refusal.no-reason.raise.next", WhenCommand: "raise"},
			{Key: "refusal.no-reason.next"},
		},
		NextStep: []string{
			"refusal.no-reason.raise.next",
			"refusal.no-reason.next",
		},
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
		// A caller that declared a harness is an agent, and the next step it is
		// given points at the guide on recording the operator's stated ruling
		// rather than naming the flag, so the conditions arrive with the route.
		// A caller declaring none is given the step it always had.
		Name:   NotOperator,
		Values: []string{ValueHarness},
		Fragments: []Fragment{
			{Key: "refusal.not-operator.next-harness", When: ValueHarness},
			{Key: "refusal.not-operator.next"},
		},
		NextStep: []string{"refusal.not-operator.next-harness", "refusal.not-operator.next"},
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
		// The reference names no card, and a bare reference naming a
		// workstream is the one case where the tool knows what the reader
		// meant. That case gets a next step of its own, because the usual
		// one sends the reader to the card listing, which cannot answer a
		// question about a workstream. workstream is filled by the
		// resolver alone, so every other raise site falls to the listing.
		Name:   UnknownCard,
		Values: []string{"workstream"},
		Fragments: []Fragment{
			{Key: "refusal.unknown-card.workstream.next", When: "workstream"},
			{Key: "refusal.unknown-card.next"},
		},
		NextStep: []string{
			"refusal.unknown-card.workstream.next",
			"refusal.unknown-card.next",
		},
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
		// The detail names the item rather than the card, because a card
		// carrying several unresolved items sends its reader to a file to
		// edit and one card reference would not say which.
		//
		// Overridable, on the exit hold's own terms below: only the
		// operator's own reading of this refusal can ever pass it, so only
		// his fragment names --override.
		Name: UnresolvedItem,
		Fragments: []Fragment{
			{Key: "refusal.unresolved-item.next-operator", When: "operator"},
			{Key: "refusal.unresolved-item.next"},
		},
		NextStep: []string{
			"refusal.unresolved-item.next-operator",
			"refusal.unresolved-item.next",
		},
	},
	{
		// The detail names the key the caller typed, and the rows name the
		// keys the workbench does declare on the kind the reference resolved
		// to. The rows ride as a Carried set rather than a Listing because
		// the set depends on that kind, and no enumerable listing names it.
		//
		// One shape answers both mistakes this name covers. Where the
		// workbench declares the key on some other kind, the raise site fills
		// kinds with the kinds it does declare it on, and the fragment that
		// clause switches on is what tells the reader the key exists
		// somewhere rather than nowhere.
		Name:    UndeclaredField,
		Values:  []string{"kind", "kinds"},
		Carried: "declared",
		Fragments: []Fragment{
			{Key: "refusal.undeclared-field.elsewhere", When: "kinds"},
			{Key: "refusal.undeclared-field.next"},
		},
		NextStep: []string{"refusal.undeclared-field.next"},
	},
	{
		// The detail names the key the column requires, and the sentence
		// names the column, because the reader has to know which station
		// asked before they can decide whether to set the value or to move
		// somewhere else. The next step is the write that would let the move
		// through.
		//
		// Overridable, on the exit hold's own terms below: the write is one
		// way past this row and --override is the operator's own second way,
		// so his fragment names both while anybody else reads the write
		// alone.
		Name:   MissingField,
		Values: []string{ValueColumn},
		Fragments: []Fragment{
			{Key: "refusal.missing-field.next-operator", When: "operator"},
			{Key: "refusal.missing-field.next"},
		},
		NextStep: []string{
			"refusal.missing-field.next-operator",
			"refusal.missing-field.next",
		},
	},
	{
		// The departure's own hold, mirroring the entry shape above down
		// to which entity the detail names, because the two refusals send
		// a reader to the same item and differ only in which side of the
		// column stood in the way.
		//
		// This is the one exit hold left standing on a forward departure or
		// one into a done column, since a regressive departure passes
		// beneath canLand and never reaches this refusal at all. The caller
		// it stops is therefore always somebody trying to advance, and where
		// that caller is the operator, --override is a way past this
		// station's own item that no other reader has, so the operator
		// fragment names it. operator carries no placeholder of its own; it
		// only switches the fragment, on the same pattern
		// dinah.multiple-words's quoteInText already uses.
		Name: UnresolvedItemExit,
		Fragments: []Fragment{
			{Key: "refusal.dinah.unresolved-item-exit.next-operator", When: "operator"},
			{Key: "refusal.dinah.unresolved-item-exit.next"},
		},
		NextStep: []string{
			"refusal.dinah.unresolved-item-exit.next-operator",
			"refusal.dinah.unresolved-item-exit.next",
		},
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
		//
		// The un-conditioned .next fragment already told every reader to ask
		// the operator to carry the move through with --override, which was
		// only ever true for a reader who is not the operator: the operator
		// reading his own refusal needs no asking. The operator fragment
		// says so directly instead.
		Name: AtLoopLimit,
		Fragments: []Fragment{
			{Key: "refusal.dinah.at-loop-limit.next-operator", When: "operator"},
			{Key: "refusal.dinah.at-loop-limit.next"},
		},
		NextStep: []string{
			"refusal.dinah.at-loop-limit.next-operator",
			"refusal.dinah.at-loop-limit.next",
		},
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
		// init and extract raise this over a directory that already carries
		// a workbench, and restore raises it over a live entity standing in
		// the slot an archived one goes back to. The two acts share nothing
		// but the word already, so restore carries a base entry and a next
		// step of its own rather than telling a reader their card directory
		// holds a workbench.md.
		Name:     Exists,
		Variants: []string{"restore"},
		Fragments: []Fragment{
			{Key: "refusal.dinah.exists.restore.next", WhenCommand: "restore"},
			{Key: "refusal.dinah.exists.next"},
		},
		NextStep: []string{
			"refusal.dinah.exists.restore.next",
			"refusal.dinah.exists.next",
		},
	},
	{
		// init refuses to write a workbench into a directory that already
		// existed and already held something, unless the caller passed
		// --here, which is what this shape's next step offers.
		Name:      DirectoryNotEmpty,
		Fragments: []Fragment{{Key: "refusal.dinah.directory-not-empty.next"}},
		NextStep:  []string{"refusal.dinah.directory-not-empty.next"},
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
		// A search with nothing to search for. It carries no value of its own:
		// the phrase the caller wrote is empty, which is the whole of what the
		// sentence says, so there is nothing to name back to them.
		Name:      EmptySearch,
		Fragments: []Fragment{{Key: "refusal.dinah.empty-search.next"}},
		NextStep:  []string{"refusal.dinah.empty-search.next"},
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
		// The climb stopped at a repository root instead of climbing past
		// it, having found no workbench at or below that root, and it never
		// tried the user base: falling back there from inside a bounded
		// repository is the exact hazard this refusal exists to stop, so it
		// carries its own sentence rather than NoWorkbenchFound's, which
		// says the user base was tried. The alternation gives a reader whose
		// climb passed a bare workbench.md on the way the repair, and
		// everybody else the general advice past the boundary.
		Name:   WorkbenchBoundary,
		Values: []string{"boundary", "bare"},
		Fragments: []Fragment{
			{Key: "refusal.dinah.workbench-boundary.bare", When: "bare"},
			{Key: "refusal.dinah.workbench-boundary.next"},
		},
		NextStep: []string{
			"refusal.dinah.workbench-boundary.bare",
			"refusal.dinah.workbench-boundary.next",
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
		// Add is the only act that allocates a number, so Add is the only
		// raise site, and the detail names the workbench root because the
		// workbench rather than the card is what stands on the half of the
		// format the migration has not reached.
		Name:      NeedsNumberMigration,
		Fragments: []Fragment{{Key: "refusal.dinah.needs-number-migration.next"}},
		NextStep:  []string{"refusal.dinah.needs-number-migration.next"},
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
		// One refusal name answers three acts here. delete destroys history,
		// and a slug change stops every reference already written down from
		// matching, so set carries its own sentence and its own next step
		// rather than ending on advice written for delete.
		//
		// The workbench and the workstream carried a variant each until the
		// kind-prefixed field commands became one, and one command reaches
		// both kinds now. What separated the two sentences was a fact about
		// the kind rather than about the command, so it rides as a spliced
		// clause the raise site switches on: a workbench slug change renames
		// every card in the workbench at the same time, and a column's or a
		// workstream's does not.
		//
		// The number repairs are the third act, raised on check. Both
		// migrate-numbers and renumber change what a card is called, and a
		// reference somebody wrote down stops resolving, which is a different
		// stake from delete's and from set's. The sentence names the repair
		// the detail brings, and the next step is the one word both repairs
		// read.
		Name:     Unconfirmed,
		Variants: []string{"set", "check"},
		Fragments: []Fragment{
			{Key: "refusal.dinah.unconfirmed.set.cards", When: "renamesCards"},
			{Key: "refusal.dinah.unconfirmed.set.next", WhenCommand: "set"},
			{Key: "refusal.dinah.unconfirmed.check.next", WhenCommand: "check"},
			{Key: "refusal.dinah.unconfirmed.next"},
		},
		NextStep: []string{
			"refusal.dinah.unconfirmed.set.next",
			"refusal.dinah.unconfirmed.check.next",
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
		// Three commands raise this name for three acts. A query names a
		// field of the query language, and get and set each name a field the
		// resolved kind records, so the command word selects the sentence and
		// the ordered-operator clause stops being unconditional: it is written
		// about the query language's one ranking field and says nothing a
		// reader of an entity's fields can use.
		//
		// get and set carry an entry each rather than sharing one, because
		// their next steps differ: each points at its own help page.
		// show carries a variant of its own because the act it refuses is a
		// third one: a read naming a member of a card's detail that show
		// cannot select. Its base sentence says what was refused and what a
		// card carries, and stops there, because show refuses two different
		// reads and each carries a clause the other would make false. Where
		// the names are the fault the sentence names them, and where the
		// reference is not a card the names may all be legal and naming
		// them as unselectable would contradict the set the same sentence
		// lists. So each rides a fragment of its own, switched on the value
		// its raise site fills: unknown on the one, reference on the other.
		// Both are filled at a raise site and each is its own fragment's
		// condition, so they are declared there rather than in Values.
		Name:     UnknownField,
		Values:   []string{"fields", "instantField", "kind"},
		Variants: []string{"get", "set", "show"},
		Fragments: []Fragment{
			{Key: "refusal.dinah.unknown-field.ordered", WhenCommand: "query"},
			{Key: "refusal.dinah.unknown-field.show.unknown", When: "unknown"},
			{Key: "refusal.dinah.unknown-field.show.reference", When: "reference"},
			{Key: "refusal.dinah.unknown-field.get.next", WhenCommand: "get"},
			{Key: "refusal.dinah.unknown-field.set.next", WhenCommand: "set"},
			{Key: "refusal.dinah.unknown-field.show.next", WhenCommand: "show"},
			{Key: "refusal.dinah.unknown-field.next"},
		},
		NextStep: []string{
			"refusal.dinah.unknown-field.get.next",
			"refusal.dinah.unknown-field.set.next",
			"refusal.dinah.unknown-field.show.next",
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
		// The detail names the slot, the gate and the admitting values ride
		// as values because the sentence says why the slot does not apply
		// and what would make it, and the gate's stored value is the subject:
		// the sentence about a card whose gate carries no value is a different
		// sentence rather than the same one with a hole in it, so it is the
		// unset sibling rather than the base entry. One next step serves
		// both, since writing the gate is the repair either way.
		Name:      InapplicableField,
		Subject:   "value",
		Absent:    "unset",
		Values:    []string{"gate", "admits"},
		Fragments: []Fragment{{Key: "refusal.dinah.inapplicable-field.next"}},
		NextStep:  []string{"refusal.dinah.inapplicable-field.next"},
	},
	{
		// The column rides as a value even though the detail carries the same
		// reference, because the sentence names it and the next step names
		// what to do about it, and the expression is the other half of the
		// sentence and has nowhere else to travel.
		Name:      NoTierDefault,
		Values:    []string{"column", "expr"},
		Fragments: []Fragment{{Key: "refusal.dinah.no-tier-default.next"}},
		NextStep:  []string{"refusal.dinah.no-tier-default.next"},
	},
	{
		// The attempted rung rides as a value because the detail carries what
		// the caller typed and the sentence needs both: a reader fixing a
		// relative write wants to see the step they wrote and the rung it
		// came out at. The declared set rides as a value for the reason
		// UnknownLevel gives, since it is the same set read off the same
		// workbench.
		Name:      TierOutOfRange,
		Values:    []string{"attempted", "levels"},
		Fragments: []Fragment{{Key: "refusal.dinah.tier-out-of-range.next"}},
		NextStep:  []string{"refusal.dinah.tier-out-of-range.next"},
	},
	{
		// The required tier rides as a value as well as in the detail,
		// because the base sentence and the next step both name it and a
		// detail cannot be spliced into the fragment. The column rides
		// alongside it, since what the card asks for depends on which column
		// the claim is being made at. The caller's own model and the entries
		// that would satisfy the requirement ride with them, because the
		// repair is to run one of those and a reader cannot compose that list
		// from anything else the refusal carries.
		Name:      BelowTier,
		Values:    []string{"required", "column", "model", "satisfied_by"},
		Fragments: []Fragment{{Key: "refusal.dinah.below-tier.next"}},
		NextStep:  []string{"refusal.dinah.below-tier.next"},
	},
	{
		// The three values BelowTier carries plus the caller's own
		// declaration, for the same reasons. The sentence differs because the
		// repair does: a model the table lists nowhere is added to the table
		// or switched away from, where one listed below the requirement can
		// only be switched away from.
		Name:      UnlistedModel,
		Values:    []string{"required", "column", "model", "satisfied_by"},
		Fragments: []Fragment{{Key: "refusal.dinah.unlisted-model.next"}},
		NextStep:  []string{"refusal.dinah.unlisted-model.next"},
	},
	{
		// No model value here, because this refusal is raised by a caller
		// that declared none. Its reader is a harness nobody configured, so
		// the next step names the variables to set and the parameters to
		// send rather than a model to switch to.
		Name:      UndeclaredModel,
		Values:    []string{"required", "column", "satisfied_by"},
		Fragments: []Fragment{{Key: "refusal.dinah.undeclared-model.next"}},
		NextStep:  []string{"refusal.dinah.undeclared-model.next"},
	},
	{
		// The declared name travels in the detail alone, because the sentence
		// names it once and the next step names the variable to set rather
		// than the value again.
		//
		// dinah setup raises it over the recipe name it was handed, which is
		// a harness name in the same grammar and not a variable anybody set,
		// so setup carries a sentence and a next step of its own.
		Name:     MalformedHarness,
		Variants: []string{"setup"},
		Fragments: []Fragment{
			{Key: "refusal.dinah.malformed-harness.setup.next", WhenCommand: "setup"},
			{Key: "refusal.dinah.malformed-harness.next"},
		},
		NextStep: []string{
			"refusal.dinah.malformed-harness.setup.next",
			"refusal.dinah.malformed-harness.next",
		},
	},
	{
		// The member's name travels in the detail alone, written as a JSON
		// string literal so the line ending in it is an escape: printing the
		// name raw would split this refusal's own line, which is the very
		// thing the refusal exists to stop the document doing to a header.
		Name:      MalformedMemberName,
		Fragments: []Fragment{{Key: "refusal.dinah.malformed-member-name.next"}},
		NextStep:  []string{"refusal.dinah.malformed-member-name.next"},
	},
	{
		// Both tiers ride as values for the reason BelowTier gives: the base
		// sentence names what the card already asks for and what the raise
		// resolved to, and the next step names the first of them again, so
		// neither can travel in the detail alone.
		Name:      TierNotHigher,
		Values:    []string{"current", "attempted"},
		Fragments: []Fragment{{Key: "refusal.dinah.tier-not-higher.next"}},
		NextStep:  []string{"refusal.dinah.tier-not-higher.next"},
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
		// The sentence does not echo the word typed, because a completion
		// asked for no shell has no word to echo. The word travels in the
		// detail, where the machine form carries it.
		Name:      UnknownShell,
		Listing:   "shells",
		Fragments: []Fragment{{Key: "refusal.dinah.unknown-shell.next"}},
		NextStep:  []string{"refusal.dinah.unknown-shell.next"},
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
		Name:      InvalidAlias,
		Values:    []string{"defect"},
		Fragments: []Fragment{{Key: "refusal.dinah.invalid-alias.next"}},
		NextStep:  []string{"refusal.dinah.invalid-alias.next"},
	},
	{
		Name:      AliasShadow,
		Values:    []string{"command"},
		Fragments: []Fragment{{Key: "refusal.dinah.alias-shadows-command.next"}},
		NextStep:  []string{"refusal.dinah.alias-shadows-command.next"},
	},
	{
		Name:      AliasMissing,
		Values:    []string{"argument"},
		Fragments: []Fragment{{Key: "refusal.dinah.missing-alias-argument.next"}},
		NextStep:  []string{"refusal.dinah.missing-alias-argument.next"},
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
		// One sentence and one next step, matching UnreadableBench's own
		// shape, because the two refusals answer the same question about a
		// workbench.md the walk could not use and neither has an alternation
		// to resolve.
		Name:      DamagedBench,
		Fragments: []Fragment{{Key: "refusal.dinah.damaged-workbench.next"}},
		NextStep:  []string{"refusal.dinah.damaged-workbench.next"},
	},
	{
		// One sentence and one next step again, mirroring UnreadableBench,
		// because this refusal reports the same kind of failure one directory
		// further out and has no alternation to resolve either.
		Name:      UnreadableContainer,
		Fragments: []Fragment{{Key: "refusal.dinah.unreadable-container.next"}},
		NextStep:  []string{"refusal.dinah.unreadable-container.next"},
	},
	{
		// The clause split out of the base entry sat ahead of the dash hint
		// inside the sentence, so its fragment is declared ahead of it here
		// and a reader sees the three pieces in the order they were written.
		//
		// dinah setup raises it mostly over an argument that was understood
		// and does not fit beside the others, such as --list beside a harness,
		// so setup carries a sentence and a next step of its own rather than
		// saying the argument was not understood.
		Name:     Usage,
		Variants: []string{"setup"},
		Fragments: []Fragment{
			{Key: "refusal.dinah.usage.setup.next", WhenCommand: "setup"},
			{Key: "refusal.dinah.usage.next"},
			{Key: "refusal.dinah.usage.dash-hint", When: "dashHint"},
		},
		NextStep: []string{"refusal.dinah.usage.setup.next", "refusal.dinah.usage.next"},
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
		Name:      ColumnSlugTaken,
		Fragments: []Fragment{{Key: "refusal.dinah.column-slug-taken.next"}},
		NextStep:  []string{"refusal.dinah.column-slug-taken.next"},
	},
	{
		Name:      ColumnRoutingDisrupted,
		Fragments: []Fragment{{Key: "refusal.dinah.column-routing-disrupted.next"}},
		NextStep:  []string{"refusal.dinah.column-routing-disrupted.next"},
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
		// The three legal profiles are written into the base sentence rather
		// than carried as a value, since the set is fixed by this head
		// rather than read off the workbench.
		Name:      UnknownToolProfile,
		Fragments: []Fragment{{Key: "refusal.dinah.unknown-tool-profile.next"}},
		NextStep:  []string{"refusal.dinah.unknown-tool-profile.next"},
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
		// attach and comment are the only verbs that write a new entity
		// below the reference they are handed, so between them they are the
		// only ones that can be aimed at a kind the containment grammar
		// gives nothing to hang from. The next step splits on the kind,
		// because the honest advice differs: an item takes its evidence by
		// citation, and an attachment wraps bytes and holds nothing. Those
		// two are the whole of what attach can resolve and then refuse, so
		// no kind reaches the unconditional member below them. It stays
		// because checkEveryShapeSaysWhatToDoNext refuses a shape whose last
		// member carries a condition, and a reader whose values matched
		// none of the branches would otherwise be given no next step at all;
		// its English names no kinds, so it cannot go stale the way an
		// enumeration of the containment table did.
		Name:   NotAttachable,
		Values: []string{"kind", "item", "attachment"},
		Fragments: []Fragment{
			{Key: "refusal.dinah.not-attachable.next-item", When: "item"},
			{Key: "refusal.dinah.not-attachable.next-attachment", When: "attachment"},
			{Key: "refusal.dinah.not-attachable.next"},
		},
		NextStep: []string{
			"refusal.dinah.not-attachable.next-item",
			"refusal.dinah.not-attachable.next-attachment",
			"refusal.dinah.not-attachable.next",
		},
	},
	{
		// A comment aimed at a kind the containment table gives no comments
		// collection: a comment, an attachment, a workstream or the
		// workbench itself. No alternation, because the honest advice is the
		// same for every kind that fails it: a comment hangs on a column, on
		// a card, or on one of that card's checklist items.
		Name:      NotCommentable,
		Values:    []string{"kind"},
		Fragments: []Fragment{{Key: "refusal.dinah.not-commentable.next"}},
		NextStep:  []string{"refusal.dinah.not-commentable.next"},
	},
	{
		// A collection reference resolves, so the reader is told what it
		// names rather than that it names nothing. The next step is an
		// alternation of three. show is answered first and by name, because
		// show reads one entity and the command that lists a collection is
		// list, so the sentence hands the reader that invocation. Every
		// other command falls to the pair below it: a collection holding
		// members can offer one of them to type and an empty collection
		// cannot, and the empty branch carries no condition so every
		// rendering ends on a next step. count is carried for a machine caller and named in
		// no entry, so it is not declared here: the guard reads Values to
		// find a name that outlived the entry using it, and a declared
		// value no sentence carries fails it.
		Name:   IsACollection,
		Values: []string{"member"},
		Fragments: []Fragment{
			{Key: "refusal.dinah.is-a-collection.next-list", WhenCommand: "show"},
			{Key: "refusal.dinah.is-a-collection.next-member", When: "member"},
			{Key: "refusal.dinah.is-a-collection.empty"},
		},
		NextStep: []string{
			"refusal.dinah.is-a-collection.next-list",
			"refusal.dinah.is-a-collection.next-member",
			"refusal.dinah.is-a-collection.empty",
		},
	},
	{
		// A reader who types one of these four commands against something
		// they can see is told which half it is in rather than that it does
		// not exist, which is the mistake dinah.is-a-collection was minted
		// against one layer up. The name answers two acts and four commands,
		// so restore carries its own base sentence and the alternation
		// carries one branch per case the reader can be in: inside an
		// archived holder, at the workbench, naming a member of a
		// collection, or asking to archive something still live. Each branch
		// names an act the reader can carry out.
		//
		// The workbench branch's value is slug rather than workbench.
		// ValueWorkbench is the workbench directory discovery resolved for
		// the invocation, and a shape spelling its own value that way and
		// filling it with a slug would put a slug where every other reader
		// of that name expects an absolute path.
		Name:     NotArchived,
		Values:   []string{"holder", "slug", "collection"},
		Variants: []string{"restore"},
		Fragments: []Fragment{
			{Key: "refusal.dinah.not-archived.next-holder", When: "holder"},
			{Key: "refusal.dinah.not-archived.next-workbench", When: "slug"},
			{Key: "refusal.dinah.not-archived.next-collection", When: "collection"},
			{Key: "refusal.dinah.not-archived.restore.next", WhenCommand: "restore"},
			{Key: "refusal.dinah.not-archived.next"},
		},
		NextStep: []string{
			"refusal.dinah.not-archived.next-holder",
			"refusal.dinah.not-archived.next-workbench",
			"refusal.dinah.not-archived.next-collection",
			"refusal.dinah.not-archived.restore.next",
			"refusal.dinah.not-archived.next",
		},
	},
	{
		// A card reference's number is carried by more than one card, so the
		// resolution refuses rather than answering with the lowest
		// identifier. The candidates ride as a Carried set rather than a
		// Listing, because the set depends on the reference that was typed
		// and no enumerable listing names it. The next step tells the reader
		// to retype one of the identifiers the rows carry.
		Name:      AmbiguousCard,
		Carried:   "cards",
		Fragments: []Fragment{{Key: "refusal.dinah.ambiguous-card.next"}},
		NextStep:  []string{"refusal.dinah.ambiguous-card.next"},
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
	{
		// A reshape retiring a column live cards still stand in, with
		// nothing naming where they go. The detail is the column and the
		// count rides beside it, because the decision the refusal is asking
		// for is made on both: which station the work continues at, and how
		// much work is standing there.
		Name:      ReshapeNeedsDestination,
		Values:    []string{"cards"},
		Fragments: []Fragment{{Key: "refusal.dinah.reshape-needs-a-destination.next"}},
		NextStep:  []string{"refusal.dinah.reshape-needs-a-destination.next"},
	},
	{
		// A reshape that would leave a held card where no owner takes work
		// up. The detail names the column and the cards ride beside it, so
		// the reader knows which claims to release rather than being sent to
		// find them.
		Name:      ReshapeHeldCardInQueue,
		Values:    []string{"cards"},
		Fragments: []Fragment{{Key: "refusal.dinah.reshape-held-card-in-queue.next"}},
		NextStep:  []string{"refusal.dinah.reshape-held-card-in-queue.next"},
	},
	{
		// A --map entry whose left side carries no card anywhere. The detail
		// is what the caller typed, so the sentence shows them their own
		// spelling, and the next step names dinah check, which is the
		// command that reports a stranded identifier by name.
		Name:      ReshapeMapSourceEmpty,
		Fragments: []Fragment{{Key: "refusal.dinah.reshape-map-source-empty.next"}},
		NextStep:  []string{"refusal.dinah.reshape-map-source-empty.next"},
	},
	{
		// A destination resolving into the set the same run retires. The
		// detail is the reference as written and the column it resolved to
		// rides beside it, because a slug or a title gives the reader no way
		// to see which retirement they landed on.
		Name:      ReshapeDestinationRetiring,
		Values:    []string{"column"},
		Fragments: []Fragment{{Key: "refusal.dinah.reshape-destination-retiring.next"}},
		NextStep:  []string{"refusal.dinah.reshape-destination-retiring.next"},
	},
	{
		// A destination matching the title of two or more columns the new
		// definition adds. An added column has no live identifier yet, so
		// the candidates ride as their positions in the definition's own
		// columns array, which is the only handle both of them carry.
		Name:      ReshapeDestinationAmbiguous,
		Values:    []string{"positions"},
		Fragments: []Fragment{{Key: "refusal.dinah.reshape-destination-ambiguous.next"}},
		NextStep:  []string{"refusal.dinah.reshape-destination-ambiguous.next"},
	},
	{
		// The three legal kinds ride as a value rather than as a Listing,
		// because the set is fixed by the format rather than read off the
		// workbench a Listing name would resolve against.
		Name:      UnknownItemKind,
		Values:    []string{"kinds"},
		Fragments: []Fragment{{Key: "refusal.dinah.unknown-item-kind.next"}},
		NextStep:  []string{"refusal.dinah.unknown-item-kind.next"},
	},
	{
		// The four legal states are written into the base sentence rather
		// than carried as a value, since the set is fixed by the format
		// rather than read off the workbench, and settle's own dispatch
		// raises this refusal ahead of resolving any item.
		Name:      UnknownItemState,
		Fragments: []Fragment{{Key: "refusal.dinah.unknown-item-state.next"}},
		NextStep:  []string{"refusal.dinah.unknown-item-state.next"},
	},
	{
		// The detail is the item's own kind rather than the verb the caller
		// typed, since the caller knows what they typed and does not know
		// what is on disk.
		Name:      WrongItemKind,
		Fragments: []Fragment{{Key: "refusal.dinah.wrong-item-kind.next"}},
		NextStep:  []string{"refusal.dinah.wrong-item-kind.next"},
	},
	{
		// The detail is the state on disk, and the next step names reopen,
		// which is the one way back to pending.
		Name:      NotPending,
		Fragments: []Fragment{{Key: "refusal.dinah.not-pending.next"}},
		NextStep:  []string{"refusal.dinah.not-pending.next"},
	},
	{
		Name:      NotResolved,
		Fragments: []Fragment{{Key: "refusal.dinah.not-resolved.next"}},
		NextStep:  []string{"refusal.dinah.not-resolved.next"},
	},
	{
		// The detail is the item's state on disk. The next step says what a
		// waiver is for, because a caller who reached this name asked to
		// lift a hold on an item that is not holding anything.
		Name:      NotWaivable,
		Fragments: []Fragment{{Key: "refusal.dinah.not-waivable.next"}},
		NextStep:  []string{"refusal.dinah.not-waivable.next"},
	},
	{
		// The detail is the item's state, which is always withdrawn here,
		// and the next step names reopen, which is the one way back.
		Name:      AlreadyWithdrawn,
		Fragments: []Fragment{{Key: "refusal.dinah.already-withdrawn.next"}},
		NextStep:  []string{"refusal.dinah.already-withdrawn.next"},
	},
	{
		// The detail is the item's state, which is failed or waived. The
		// next step says that a finding is the operator's to retire, so the
		// reader learns what the grant does not cover rather than being told
		// to try again.
		Name:      GrantExcludesFinding,
		Fragments: []Fragment{{Key: "refusal.dinah.grant-excludes-finding.next"}},
		NextStep:  []string{"refusal.dinah.grant-excludes-finding.next"},
	},
	{
		// The detail is the card, and the next step names grant, because the
		// caller meant to end a permission and there was none to end.
		Name:      NoGrant,
		Fragments: []Fragment{{Key: "refusal.dinah.no-grant.next"}},
		NextStep:  []string{"refusal.dinah.no-grant.next"},
	},
	{
		// The detail is the item's state, and the next step names reopen,
		// which is the one erasure of an answer of record the tool performs
		// and which clears the state along with the key.
		Name:      DesignationRequired,
		Fragments: []Fragment{{Key: "refusal.dinah.designation-required.next"}},
		NextStep:  []string{"refusal.dinah.designation-required.next"},
	},
	{
		// The detail is the first claimed card the conversion found and the
		// owner holding it rides as a value, so the operator can go and ask
		// that owner rather than hunting for which card stopped the run.
		Name:      WorkbenchInUse,
		Values:    []string{"owner"},
		Fragments: []Fragment{{Key: "refusal.dinah.workbench-in-use.next"}},
		NextStep:  []string{"refusal.dinah.workbench-in-use.next"},
	},
	{
		// The detail is the item's reference, so the reader can hand the same
		// spelling to cite, which is what the next step tells them to do.
		Name:      Uncited,
		Fragments: []Fragment{{Key: "refusal.dinah.uncited.next"}},
		NextStep:  []string{"refusal.dinah.uncited.next"},
	},
	{
		// The detail is the scheme the item demands and the item's reference
		// rides as a value, so the next step can hand the reader the cite
		// command with both slots filled.
		Name:      EvidenceSchemeRequired,
		Values:    []string{"item"},
		Fragments: []Fragment{{Key: "refusal.dinah.evidence-scheme-required.next"}},
		NextStep:  []string{"refusal.dinah.evidence-scheme-required.next"},
	},
	{
		// The detail is the comment's reference. The sentence says an edit
		// happened and never who made it, because the tool has a digest and
		// no witness, and the next step names the two routes out, which
		// differ in meaning rather than in convenience.
		Name:      CommentBodyDiverged,
		Fragments: []Fragment{{Key: "refusal.dinah.comment-body-diverged.next"}},
		NextStep:  []string{"refusal.dinah.comment-body-diverged.next"},
	},
	{
		// The detail is the reference as the caller typed it, which is the
		// spelling they can compare against, and it is the whole of what
		// the sentence needs: every way of failing this check comes out the
		// same for the reader, which is that the reference they typed is
		// not a comment of the item they are settling. The next step names
		// both routes to one that is.
		Name:      NotADesignation,
		Fragments: []Fragment{{Key: "refusal.dinah.not-a-designation.next"}},
		NextStep:  []string{"refusal.dinah.not-a-designation.next"},
	},
	{
		// The detail is the comment and the designating item rides as a
		// value, because the reader has to be told which item would be left
		// settled with nothing behind it.
		// Two acts raise it and they share nothing but the record they are
		// refused over, so set carries a base entry and a next step of its
		// own. A caller who ran a write and was told that deleting the
		// comment would leave the item settled with nothing behind it reads
		// a sentence about an act they did not perform, and the shared next
		// step offers a --force that set does not accept, which section 5.14
		// of this card's contract ruled out by name.
		Name:     NotDesignatable,
		Values:   []string{"item"},
		Variants: []string{"set"},
		Fragments: []Fragment{
			{Key: "refusal.dinah.not-designatable.set.next", WhenCommand: "set"},
			{Key: "refusal.dinah.not-designatable.next"},
		},
		NextStep: []string{
			"refusal.dinah.not-designatable.set.next",
			"refusal.dinah.not-designatable.next",
		},
	},
	{
		// The detail is the workbench's own path, because a person meeting
		// this is running a read against a store and the path is what tells
		// them which one.
		Name:      StoreAwaitingMigration,
		Fragments: []Fragment{{Key: "refusal.dinah.store-awaiting-migration.next"}},
		NextStep:  []string{"refusal.dinah.store-awaiting-migration.next"},
	},
	{
		Name:      ObservationRequired,
		Fragments: []Fragment{{Key: "refusal.dinah.observation-required.next"}},
		NextStep:  []string{"refusal.dinah.observation-required.next"},
	},
	{
		// The detail is the target as the caller typed it rather than the
		// identifier it resolved to, because the reader is being told that
		// the spelling they just used names no link on the card, and the
		// spelling they used is the one they can compare against. The kind
		// rides as a value, since a card may carry a link to that same
		// target under a different word and the sentence has to say which
		// word was looked for.
		Name:      UnknownLink,
		Values:    []string{"kind"},
		Fragments: []Fragment{{Key: "refusal.dinah.unknown-link.next"}},
		NextStep:  []string{"refusal.dinah.unknown-link.next"},
	},
	{
		// The declared set rides as a value rather than as a Listing, for the
		// reason UnknownLevel gives for its own: the roster is read off the
		// workbench the write landed on, and a Listing name resolves from the
		// session alone.
		//
		// Both entries that print it are switched on by it, because a
		// workbench declaring no route at all reaches this refusal and has no
		// set to list. That case gets a next step of its own, since there is
		// no name to offer the reader instead and the repair is to declare a
		// route rather than to correct one.
		Name:   UnknownRoute,
		Values: []string{"routes"},
		Fragments: []Fragment{
			{Key: "refusal.dinah.unknown-route.declared", When: "routes"},
			{Key: "refusal.dinah.unknown-route.next", When: "routes"},
			{Key: "refusal.dinah.unknown-route.none"},
		},
		NextStep: []string{
			"refusal.dinah.unknown-route.next",
			"refusal.dinah.unknown-route.none",
		},
	},
	{
		// The detail names the route, and the item and the column it names
		// ride as values, because the sentence has to say which item holds
		// the card and where, and the next step names both repairs.
		Name:      RouteStrandsItem,
		Values:    []string{"item", "column"},
		Fragments: []Fragment{{Key: "refusal.dinah.route-strands-item.next"}},
		NextStep:  []string{"refusal.dinah.route-strands-item.next"},
	},
	{
		// The detail names the route and the column rides as a value, since
		// the sentence names the station the road would carry the card past
		// and the reader repairs one or the other.
		Name:      RouteSkipsOperatorColumn,
		Values:    []string{"column"},
		Fragments: []Fragment{{Key: "refusal.dinah.route-skips-operator-column.next"}},
		NextStep:  []string{"refusal.dinah.route-skips-operator-column.next"},
	},
	{
		// The detail names the column the item was filed against, and the
		// route rides as a value, because the sentence is about a road that
		// does not reach that station and the reader needs both halves.
		Name:      ItemOffRoute,
		Values:    []string{"route"},
		Fragments: []Fragment{{Key: "refusal.dinah.item-off-route.next"}},
		NextStep:  []string{"refusal.dinah.item-off-route.next"},
	},
	{
		// The detail names the column the filing would land in and the route
		// rides as a value, on ItemOffRoute's reasoning.
		Name:      RouteOffColumn,
		Values:    []string{"route"},
		Fragments: []Fragment{{Key: "refusal.dinah.route-off-column.next"}},
		NextStep:  []string{"refusal.dinah.route-off-column.next"},
	},
	// The twelve refusals dinah setup raises each carry their subject in the
	// detail and one unconditional next step. Two of them print a list, the
	// conflicting locations and the programs a recipe runs, which ride as a
	// Carried set so the rows stand beneath the sentence and the next step
	// stands on a line of its own beneath them. The detail carries the same
	// list joined by line breaks, for the machine form.
	{
		Name:      UnknownRecipe,
		Fragments: []Fragment{{Key: "refusal.dinah.unknown-recipe.next"}},
		NextStep:  []string{"refusal.dinah.unknown-recipe.next"},
	},
	{
		Name:      MalformedRecipe,
		Fragments: []Fragment{{Key: "refusal.dinah.malformed-recipe.next"}},
		NextStep:  []string{"refusal.dinah.malformed-recipe.next"},
	},
	{
		Name:      UnknownScope,
		Fragments: []Fragment{{Key: "refusal.dinah.unknown-scope.next"}},
		NextStep:  []string{"refusal.dinah.unknown-scope.next"},
	},
	{
		Name:      SetupNoTarget,
		Fragments: []Fragment{{Key: "refusal.dinah.setup-no-target.next"}},
		NextStep:  []string{"refusal.dinah.setup-no-target.next"},
	},
	{
		Name:      SetupAgentIsOperator,
		Fragments: []Fragment{{Key: "refusal.dinah.setup-agent-is-operator.next"}},
		NextStep:  []string{"refusal.dinah.setup-agent-is-operator.next"},
	},
	{
		Name:      SetupUnreadableTarget,
		Fragments: []Fragment{{Key: "refusal.dinah.setup-unreadable-target.next"}},
		NextStep:  []string{"refusal.dinah.setup-unreadable-target.next"},
	},
	{
		Name:      SetupConflict,
		Carried:   "locations",
		Fragments: []Fragment{{Key: "refusal.dinah.setup-conflict.next"}},
		NextStep:  []string{"refusal.dinah.setup-conflict.next"},
	},
	{
		Name:      UntrustedRecipe,
		Fragments: []Fragment{{Key: "refusal.dinah.untrusted-recipe.next"}},
		NextStep:  []string{"refusal.dinah.untrusted-recipe.next"},
	},
	{
		Name:      SetupRelocatedHome,
		Fragments: []Fragment{{Key: "refusal.dinah.setup-relocated-home.next"}},
		NextStep:  []string{"refusal.dinah.setup-relocated-home.next"},
	},
	{
		Name:      SetupOtherWorkbench,
		Fragments: []Fragment{{Key: "refusal.dinah.setup-other-workbench.next"}},
		NextStep:  []string{"refusal.dinah.setup-other-workbench.next"},
	},
	{
		Name:      SetupRunNotAllowed,
		Carried:   "steps",
		Fragments: []Fragment{{Key: "refusal.dinah.setup-run-not-allowed.next"}},
		NextStep:  []string{"refusal.dinah.setup-run-not-allowed.next"},
	},
	{
		Name:      SetupStepFailed,
		Fragments: []Fragment{{Key: "refusal.dinah.setup-step-failed.next"}},
		NextStep:  []string{"refusal.dinah.setup-step-failed.next"},
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

// AbsentKeyOf is the catalog key of the sibling entry rendered where a shape's
// Subject is empty: the shape's own key with the Absent suffix, which is
// unnamed on a shape declaring none. It is read here by the composer and by
// every guard that pairs a subject with its sibling, so the suffix is spelled
// in one place.
func (s *Shape) AbsentKeyOf(base string) string {
	suffix := s.Absent
	if suffix == "" {
		suffix = "unnamed"
	}
	return base + "." + suffix
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
