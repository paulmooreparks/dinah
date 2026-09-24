package guide

import (
	"strings"
	"testing"
)

// approvedOnBehalfGuide is the text the operator approved on
// dinah-563/questions/1 (Operator Design Review), Appendix A of the card's
// specification attachment, with the paragraph on standing permissions in the
// form dinah-563/questions/3 accepted. It is transcribed here as its own
// literal, independent of guides/on-behalf.md, so a later edit to the shipped
// file that drifts from what the operator approved fails this test rather
// than passing by comparing the file against itself.
//
// The one departure from the appendix's bytes is the line breaks of the
// paragraph beginning "Dinah records the description beside the name", which
// the design review found wrapped unevenly. Its words are the appendix's.
//
// dinah-582 amended the literal once, and this is the record of it. The
// operator ruled on dinah-582/decisions/1 that a workstream's fields are any
// owner's to write, which made the reserved-acts paragraph's clause "or of a
// workstream" false: the act is no longer reserved, so the guide was sending
// an agent to the operator for something the agent may now perform as itself.
// The clause was removed and the three lines it disturbed re-wrapped, and the
// operator approved the replacement wording on dinah-582/questions/3.
const approvedOnBehalfGuide = "# Acting on somebody else's decision\n\nThe operator of a workbench will sometimes rule on something while you are\nworking and ask you to put the ruling on the workbench. The operator writes to\nyou, in the conversation you are working in, \"Lift the block on proj-7, the\nsupplier has confirmed\", and you are the one with the workbench open. Dinah\nlets you record that ruling under the operator's name while it records you as\nthe one who performed the act. This guide says when you may do that, and what\nyou owe the record when you do.\n\n## Record only a decision the operator has stated\n\nYou may act under the operator's name only to record a ruling the operator has\nstated in the operator's own words, naming this item or this act. The operator\nstates it either to you, in the conversation you are working in, or in writing\non the workbench, in a comment or a note the operator wrote.\n\nNone of these is such a statement:\n\n- another agent's report of what the operator wants or said\n- a plan or a specification the operator approved\n- a ruling the operator gave on a different item or a different act\n- your own judgement, however sure of it you are\n- silence, or an answer you expect the operator to give\n\nA question nobody has answered stays open until the operator answers it, and\nrecording your guess under the operator's name closes it with a ruling nobody\ngave.\n\nA ruling that reaches you through somebody else is not a statement to you, and\nno relay makes it one. When an agent that dispatched you passes on what the\noperator said, even quoting the operator's words verbatim and naming where\nthey were given, the agent that heard the operator is the one who records the\nruling. If the only words you can point to are somebody's account of what the\noperator said, ask the operator.\n\nThe operator may also give a standing permission, one that covers a kind of act\nrather than one item, such as carrying cards through a station without\nstopping at it. It counts as a statement only when the operator wrote it on the\nworkbench in the operator's own words, naming the kind of act and how far it\nreaches, and every record you make under it cites where it is written.\n\n## Name the operator only where Dinah reserves the act\n\nDinah reserves some acts to the workbench's operator and refuses them to\neverybody else under the name `not-operator`. That refusal tells you the act is\nreserved. It tells you nothing about whether the operator has ruled on it, and\nwhen the operator has not, the act waits for the operator.\n\nFor every act Dinah does not reserve, act as yourself even when the decision\nwas somebody else's, and say in your note or comment whose decision it was. A\nwedding planner's assistant who hears the couple choose the garden over the\nhall resolves the question under the assistant's own name, with a note saying\nthat the couple chose the garden and when they said so.\n\n## Declare what you are\n\nName the operator as the actor, and declare the harness, the provider, and the\nmodel you are running. At a terminal you set `DINAH_HARNESS`, `DINAH_PROVIDER`,\nand `DINAH_MODEL` in the environment and pass `--actor` on the command:\n\n```\nDINAH_HARNESS=claude-code DINAH_PROVIDER=anthropic DINAH_MODEL=claude-opus-5 \\\n  dinah --actor alka resolve proj-4/questions/1 --text \"Alka, in this conversation on 22 September: \\\"Take the garden venue.\\\" Recorded for her by claude.\"\n```\n\nOver MCP you pass the operator's name as `actor` and your own `harness`,\n`provider`, and `model` as arguments of the same call, or leave those three to\nthe server when it was started with them set. Dinah writes the operator's name\nas the owner of the act and your three values beside it, the same way on both\nsurfaces.\n\nDeclare only what you are running. If you declare no harness, the act reads as\nthe operator's own and nothing in the record tells the two apart. If you\ndeclare a harness or a model you are not running, the record names the wrong\nperformer. If you are a person acting for the operator, Dinah gives you nothing\nto declare, so your note is the whole record that you acted, and it names you.\n\n## Write down where the ruling came from\n\nEvery act you record under the operator's name carries a sentence that gives\nthe operator's words, says where and when the operator gave them, and names you\nas the one who recorded them. In the example above, \"Alka, in this\nconversation on 22 September\" is the where and when, the quoted words are the\nruling, and \"Recorded for her by claude\" names you.\n\nWhere the sentence goes depends on what the act touches, and it goes there\nbefore the act.\n\n- `resolve`, `verify`, and `fail` take it as their note.\n- `unblock` takes it as its reason, and Dinah writes it as a comment on the\n  card under the operator's name beside the act, so the sentence goes on the\n  command rather than in a separate comment.\n- `claim` and `move` take it as a comment on the card, posted with\n  `dinah comment <card>` under your own name.\n- An act on a checklist item that takes no note, such as changing its owner or\n  deleting the comment that answers it, takes it as a comment on the item.\n- An act on a column that leaves the column standing, such as changing one of\n  its fields or attaching, renaming, archiving, restoring, or deleting one of\n  its attachments, takes it as a comment on the column, posted with\n  `dinah comment <column>`.\n\n`pull` names no card, so it leaves you nowhere to put the sentence first. When\nthe operator's ruling is about a card, carry it out with `claim` and `move` on\nthat card instead, and record it there.\n\nSome reserved acts leave nowhere a person can read the sentence afterwards.\nThey are changing a field of the workbench itself, anything done to an\nattachment of the workbench, archiving, restoring, or deleting a column, and\nrunning `reshape`. Do not perform those under the operator's name. Ask the\noperator to run them.\n\n## What Dinah does not check\n\nDinah does not confirm that the operator said anything. It records the name you\ngive and the description you declare, as it does on every act, so whether the\nrecord is honest depends on you.\n\nDinah records the description beside the name, so a later reader can tell the\noperator's own acts from the ones you recorded.\n`dinah --json list <card>/journal` prints that description under each entry's\n`actor`. The plain journal listing, `dinah changes`, and a comment's author\nline show the name alone, so at a terminal your note is what tells a person\nthat you acted.\n\nThe contract: CORE-OWNER-1, CORE-ACTING-1, CORE-ACTING-2, and CORE-ACTING-3.\n"

// TestTheOnBehalfGuideMatchesWhatTheOperatorApproved is dinah-563's hold on
// the guide's own text, modeled on TestThePipelineGuideMatchesWhatTheOperatorApproved:
// the shipped bytes are held to the approval rather than to whatever the
// embedded file happens to say.
func TestTheOnBehalfGuideMatchesWhatTheOperatorApproved(t *testing.T) {
	got, err := Text("on-behalf")
	if err != nil {
		t.Fatalf("read the on-behalf guide: %v", err)
	}
	if got != approvedOnBehalfGuide {
		t.Errorf("the shipped on-behalf guide does not match the text approved on dinah-563/questions/1: got %q, wanted %q", got, approvedOnBehalfGuide)
	}
	if title := Title("on-behalf"); title != "Acting on somebody else's decision" {
		t.Errorf("the on-behalf guide's title is %q", title)
	}
}

// TestTheOnBehalfGuideIsReadAfterPrinciples holds the guide's place in the
// reading order, which the specification sets immediately after principles
// and before references. TestTopicsAreOfferedInTheDeclaredReadingOrder reads
// the same slice it checks, so it cannot catch the guide landing elsewhere.
func TestTheOnBehalfGuideIsReadAfterPrinciples(t *testing.T) {
	topics := Topics()
	at := -1
	for i, topic := range topics {
		if topic == "on-behalf" {
			at = i
		}
	}
	if at < 1 || at+1 >= len(topics) {
		t.Fatalf("the reading order places on-behalf at %d of %v", at, topics)
	}
	if topics[at-1] != "principles" || topics[at+1] != "references" {
		t.Errorf("on-behalf stands between %s and %s, wanted principles and references", topics[at-1], topics[at+1])
	}
}

// onBehalfPointers are the sentences dinah-563 added to the other guides, as
// sections 3.2 to 3.5 of its specification write them. Each follows the text
// named beside it, which is where the specification places it, and the last
// two also carry the text that follows them. Both are folded to single
// spaces, because the source wraps them.
var onBehalfPointers = []struct {
	topic string
	after string
	text  string
}{
	{"mcp", "The refusal tells you which rule stopped you and on what.", "A `not-operator` refusal on this surface carries the rule's name and the actor you named, and no sentence of advice. When the operator has stated a ruling on the refused act, in the operator's own words, read the guide `on-behalf` before you record it under the operator's name; otherwise the act is the operator's to perform."},
	{"mcp", "server's default. Every tool takes it.", "Naming anybody but yourself puts your act on the record as theirs, so read the guide `on-behalf` before you do."},
	{"verbs", "An obstacle raised is an obstacle handed to whoever answers for the workbench.", "If the operator has stated a ruling that lifts a block, `dinah guide on-behalf` says how you record it."},
	{"first-session", "a rule addressed to whoever works a card rather than a refusal Dinah enforces against a caller who has decided otherwise.", "There is one honest reason to present the operator's name, which is to record a ruling the operator has stated about that act in the operator's own words. `dinah guide on-behalf` says what counts as such a statement and how to record it so the record still shows that you acted. Presenting the name for any other reason writes into the record, as the operator's, a decision the operator never stated. ## Read what this workbench asks of you"},
	{"principles", "even though nothing stopped you at the time.", "The fourth rule does not stop you making a move the operator has ruled on in the operator's own words. Recorded under the operator's name with your harness, provider, and model declared, the move is the operator's act and the record shows that you performed it. `dinah guide on-behalf` says what counts as such a ruling and what else you owe the record when you act on it. The contract: ACTOR-1"},
}

// TestEveryGuidePointsAtOnBehalfWhereTheSpecificationPlacesIt holds the five
// pointers to their text and their position, and holds getting-started to
// saying nothing about the guide, since the specification leaves it
// unchanged.
func TestEveryGuidePointsAtOnBehalfWhereTheSpecificationPlacesIt(t *testing.T) {
	if len(onBehalfPointers) != 5 {
		t.Fatalf("wanted five pointers, the table carries %d", len(onBehalfPointers))
	}
	for _, pointer := range onBehalfPointers {
		text, err := Text(pointer.topic)
		if err != nil {
			t.Fatalf("read the %s guide: %v", pointer.topic, err)
		}
		folded := strings.Join(strings.Fields(text), " ")
		if want := pointer.after + " " + pointer.text; !strings.Contains(folded, want) {
			t.Errorf("the %s guide does not carry the on-behalf pointer where the specification places it: wanted %q", pointer.topic, want)
		}
	}
	text, err := Text("getting-started")
	if err != nil {
		t.Fatalf("read the getting-started guide: %v", err)
	}
	if strings.Contains(text, "on-behalf") {
		t.Error("the getting-started guide names on-behalf, and the specification leaves it unchanged")
	}
}
