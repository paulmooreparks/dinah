//go:build tui

package main

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"dinah/internal/msg"
	"dinah/internal/screen/keyboard"
)

// interactiveBinding is one key binding the terminal head defines, with the
// mode it belongs to and the catalog key its description was drawn from, and
// the values that key's placeholders were filled with.
type interactiveBinding struct {
	mode    string
	key     string
	values  []string
	binding key.Binding
}

// The modes a binding belongs to, as interactiveBindings names them.
const (
	bindingBrowse  = "browse"
	bindingCard    = "card"
	bindingMenu    = "menu"
	bindingJump    = "jump"
	bindingComment = "comment"
	bindingItems   = "items"
	bindingOutput  = "output"
)

// interactiveKeys are every binding the terminal head matches keys against
// and draws its footer from, built in one language with the destinations the
// a and b keys name filled in.
type interactiveKeys struct {
	// The bindings of browse mode, most of which card mode shares.
	up, down, left, right, page, ends, show     key.Binding
	claim, accept, advance, sendBack, move      key.Binding
	release, comment, filter, jump, more, fewer key.Binding
	quit, interrupt, repaint                    key.Binding
	// The keys dinah-623 added to browse and card mode: item mode, the
	// actions menu and the five reads.
	items, actions, next, status, search, changes, whoami key.Binding
	// The letters of item mode.
	itemResolve, itemVerify, itemFail, itemWaive  key.Binding
	itemWithdraw, itemReopen, itemCite, itemClose key.Binding
	itemQuit                                      key.Binding
	// The keys of output mode that the other modes do not share.
	outputClose key.Binding
	// Tab at the command line.
	promptComplete key.Binding
	// The bindings of card mode that browse mode does not have.
	back, scrollUp, scrollDown key.Binding
	// The bindings of the move menu.
	menuUp, menuDown, menuChoose, menuNumber, menuCancel, menuQuit key.Binding
	// The bindings of the prompts.
	promptGo, promptCancel, promptNewline, promptPost key.Binding
	// bindings is every binding above, each with its mode and catalog key.
	bindings []interactiveBinding
}

// interactiveArrow is the arrow the a and b labels draw, in each glyph set.
func interactiveArrow(glyphs boardGlyphs) string {
	if glyphs == plainGlyphs {
		return "->"
	}
	return "→"
}

// newInteractiveKeys builds every binding in r's language. forward names the
// destination of a and back that of b, each already cleaned, and arrow is the
// glyph set's arrow.
func newInteractiveKeys(r *msg.Renderer, arrow, forward, back string) *interactiveKeys {
	k := &interactiveKeys{}
	add := func(mode, catalogKey string, values []string, binding key.Binding) key.Binding {
		k.bindings = append(k.bindings, interactiveBinding{mode: mode, key: catalogKey, values: values, binding: binding})
		return binding
	}
	enter := r.T("interactive.key.enter")
	ctrlC := r.T("interactive.key.ctrl-c")
	ctrlG := r.T("interactive.key.ctrl-g")
	moveTo := []string{"arrow", arrow, "column", forward}
	backTo := []string{"arrow", arrow, "column", back}

	k.up = add(bindingBrowse, "interactive.help.up", nil, key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp(r.T("interactive.key.up")+"/k", r.T("interactive.help.up")),
	))
	k.down = add(bindingBrowse, "interactive.help.down", nil, key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp(r.T("interactive.key.down")+"/j", r.T("interactive.help.down")),
	))
	k.left = add(bindingBrowse, "interactive.help.left", nil, key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp(r.T("interactive.key.left")+"/h", r.T("interactive.help.left")),
	))
	k.right = add(bindingBrowse, "interactive.help.right", nil, key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp(r.T("interactive.key.right")+"/l", r.T("interactive.help.right")),
	))
	k.page = add(bindingBrowse, "interactive.help.page", nil, key.NewBinding(
		key.WithKeys("pgup", "pgdown"),
		key.WithHelp(r.T("interactive.key.pgup")+"/"+r.T("interactive.key.pgdown"), r.T("interactive.help.page")),
	))
	k.ends = add(bindingBrowse, "interactive.help.ends", nil, key.NewBinding(
		key.WithKeys("home", "end"),
		key.WithHelp(r.T("interactive.key.home")+"/"+r.T("interactive.key.end"), r.T("interactive.help.ends")),
	))
	k.show = add(bindingBrowse, "interactive.help.show", nil, key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp(enter, r.T("interactive.help.show")),
	))
	k.back = add(bindingCard, "interactive.help.back", nil, key.NewBinding(
		key.WithKeys("enter", "backspace"),
		key.WithHelp(enter+"/"+r.T("interactive.key.backspace"), r.T("interactive.help.back")),
	))
	k.scrollUp = add(bindingCard, "interactive.help.scroll", nil, key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp(r.T("interactive.key.up")+"/k", r.T("interactive.help.scroll")),
	))
	k.scrollDown = add(bindingCard, "interactive.help.scroll", nil, key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp(r.T("interactive.key.down")+"/j", r.T("interactive.help.scroll")),
	))
	k.claim = add(bindingBrowse, "interactive.help.claim", nil, key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", r.T("interactive.help.claim")),
	))
	k.accept = add(bindingBrowse, "interactive.help.accept", moveTo, key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", r.T("interactive.help.accept", moveTo...)),
	))
	k.advance = add(bindingBrowse, "interactive.help.advance", moveTo, key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", r.T("interactive.help.advance", moveTo...)),
	))
	k.sendBack = add(bindingBrowse, "interactive.help.send-back", backTo, key.NewBinding(
		key.WithKeys("b"),
		key.WithHelp("b", r.T("interactive.help.send-back", backTo...)),
	))
	k.move = add(bindingBrowse, "interactive.help.move", nil, key.NewBinding(
		key.WithKeys("m"),
		key.WithHelp("m", r.T("interactive.help.move")),
	))
	k.release = add(bindingBrowse, "interactive.help.release", nil, key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", r.T("interactive.help.release")),
	))
	k.comment = add(bindingBrowse, "interactive.help.comment", nil, key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", r.T("interactive.help.comment")),
	))
	k.filter = add(bindingBrowse, "interactive.help.filter", nil, key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", r.T("interactive.help.filter")),
	))
	k.jump = add(bindingBrowse, "interactive.help.jump", nil, key.NewBinding(
		key.WithKeys(":"),
		key.WithHelp(":", r.T("interactive.help.jump")),
	))
	k.items = add(bindingBrowse, "interactive.help.items", nil, key.NewBinding(
		key.WithKeys("i"),
		key.WithHelp("i", r.T("interactive.help.items")),
	))
	k.actions = add(bindingBrowse, "interactive.help.actions", nil, key.NewBinding(
		key.WithKeys("x"),
		key.WithHelp("x", r.T("interactive.help.actions")),
	))
	k.next = add(bindingBrowse, "interactive.help.next", nil, key.NewBinding(
		key.WithKeys(">"),
		key.WithHelp(">", r.T("interactive.help.next")),
	))
	k.status = add(bindingBrowse, "interactive.help.status", nil, key.NewBinding(
		key.WithKeys("S"),
		key.WithHelp("S", r.T("interactive.help.status")),
	))
	k.search = add(bindingBrowse, "interactive.help.search", nil, key.NewBinding(
		key.WithKeys("F"),
		key.WithHelp("F", r.T("interactive.help.search")),
	))
	k.changes = add(bindingBrowse, "interactive.help.changes", nil, key.NewBinding(
		key.WithKeys("C"),
		key.WithHelp("C", r.T("interactive.help.changes")),
	))
	k.whoami = add(bindingBrowse, "interactive.help.whoami", nil, key.NewBinding(
		key.WithKeys("W"),
		key.WithHelp("W", r.T("interactive.help.whoami")),
	))
	k.more = add(bindingBrowse, "interactive.help.keys", nil, key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", r.T("interactive.help.keys")),
	))
	k.fewer = add(bindingBrowse, "interactive.help.fewer", nil, key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", r.T("interactive.help.fewer")),
	))
	k.quit = add(bindingBrowse, "interactive.help.quit", nil, key.NewBinding(
		key.WithKeys("q"),
		key.WithHelp("q", r.T("interactive.help.quit")),
	))
	k.interrupt = add(bindingBrowse, "interactive.help.quit", nil, key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp(ctrlC, r.T("interactive.help.quit")),
	))
	k.repaint = add(bindingBrowse, "interactive.help.repaint", nil, key.NewBinding(
		key.WithKeys("ctrl+l"),
		key.WithHelp(r.T("interactive.key.ctrl-l"), r.T("interactive.help.repaint")),
	))

	k.menuUp = add(bindingMenu, "interactive.help.highlight", nil, key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp(r.T("interactive.key.up")+"/k", r.T("interactive.help.highlight")),
	))
	k.menuDown = add(bindingMenu, "interactive.help.highlight", nil, key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp(r.T("interactive.key.down")+"/j", r.T("interactive.help.highlight")),
	))
	k.menuChoose = add(bindingMenu, "interactive.help.choose", nil, key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp(enter, r.T("interactive.help.choose")),
	))
	k.menuNumber = add(bindingMenu, "interactive.help.choose", nil, key.NewBinding(
		key.WithKeys("1", "2", "3", "4", "5", "6", "7", "8", "9"),
		key.WithHelp("1-9", r.T("interactive.help.choose")),
	))
	k.menuCancel = add(bindingMenu, "interactive.help.cancel", nil, key.NewBinding(
		key.WithKeys("ctrl+g"),
		key.WithHelp(ctrlG, r.T("interactive.help.cancel")),
	))
	k.menuQuit = add(bindingMenu, "interactive.help.cancel", nil, key.NewBinding(
		key.WithKeys("q"),
		key.WithHelp("q", r.T("interactive.help.cancel")),
	))

	k.itemResolve = add(bindingItems, "interactive.help.answer", nil, key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", r.T("interactive.help.answer")),
	))
	k.itemVerify = add(bindingItems, "interactive.help.verify", nil, key.NewBinding(
		key.WithKeys("v"),
		key.WithHelp("v", r.T("interactive.help.verify")),
	))
	k.itemFail = add(bindingItems, "interactive.help.fail", nil, key.NewBinding(
		key.WithKeys("f"),
		key.WithHelp("f", r.T("interactive.help.fail")),
	))
	k.itemWaive = add(bindingItems, "interactive.help.waive", nil, key.NewBinding(
		key.WithKeys("w"),
		key.WithHelp("w", r.T("interactive.help.waive")),
	))
	k.itemWithdraw = add(bindingItems, "interactive.help.withdraw", nil, key.NewBinding(
		key.WithKeys("x"),
		key.WithHelp("x", r.T("interactive.help.withdraw")),
	))
	k.itemReopen = add(bindingItems, "interactive.help.reopen", nil, key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("o", r.T("interactive.help.reopen")),
	))
	k.itemCite = add(bindingItems, "interactive.help.cite", nil, key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", r.T("interactive.help.cite")),
	))
	k.itemClose = add(bindingItems, "interactive.help.close", nil, key.NewBinding(
		key.WithKeys("ctrl+g"),
		key.WithHelp(ctrlG, r.T("interactive.help.close")),
	))
	k.itemQuit = add(bindingItems, "interactive.help.close", nil, key.NewBinding(
		key.WithKeys("q"),
		key.WithHelp("q", r.T("interactive.help.close")),
	))
	k.outputClose = add(bindingOutput, "interactive.help.close", nil, key.NewBinding(
		key.WithKeys("enter", "backspace", "q"),
		key.WithHelp(enter+"/"+r.T("interactive.key.backspace")+"/q", r.T("interactive.help.close")),
	))

	k.promptGo = add(bindingJump, "interactive.help.go", nil, key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp(enter, r.T("interactive.help.go")),
	))
	k.promptCancel = add(bindingJump, "interactive.help.cancel", nil, key.NewBinding(
		key.WithKeys("ctrl+g"),
		key.WithHelp(ctrlG, r.T("interactive.help.cancel")),
	))
	k.promptComplete = add(bindingJump, "interactive.help.complete", nil, key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp(r.T("interactive.key.tab"), r.T("interactive.help.complete")),
	))
	k.promptNewline = add(bindingComment, "interactive.help.newline", nil, key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp(enter, r.T("interactive.help.newline")),
	))
	k.promptPost = add(bindingComment, "interactive.help.post", nil, key.NewBinding(
		key.WithKeys("ctrl+d"),
		key.WithHelp(r.T("interactive.key.ctrl-d"), r.T("interactive.help.post")),
	))
	return k
}

// interactiveBindings answers every binding the head defines, in every mode,
// with the a and b labels filled from a fixed column, which is the one list
// a test reads to hold each description to its catalog text.
func interactiveBindings(r *msg.Renderer) []interactiveBinding {
	return newInteractiveKeys(r, "→", "Done", "Implement").bindings
}

// shortHelp answers the bindings the footer lists in browse and card mode:
// the rows of interactiveActs read in those modes, in the table's order, whose
// offered answers true, then the user's own bindings the footer lists, then
// ? and q. In card mode the first row is card mode's own Enter, and the
// filter is left out, since it narrows lanes card mode does not draw.
func (k *interactiveKeys) shortHelp(m *interactiveModel, full bool) []key.Binding {
	card := m.mode == modeCard
	var bindings []key.Binding
	for _, row := range interactiveActs {
		if row.mode != actBrowse || !row.offered(m) {
			continue
		}
		binding := row.binding(k)
		if row.name == "show" && card {
			binding = k.back
		}
		bindings = append(bindings, binding)
	}
	bindings = append(bindings, m.bindingHelp()...)
	if full {
		bindings = append(bindings, k.fewer)
	} else {
		bindings = append(bindings, k.more)
	}
	return append(bindings, k.quit)
}

// fullHelp answers the bindings full help lists in browse and card mode, in
// columns: how to move, then what the offer allows, then the rest, which are
// ctrl+l, which draws the whole screen again, and ctrl+c.
func (k *interactiveKeys) fullHelp(m *interactiveModel) [][]key.Binding {
	moving := []key.Binding{k.up, k.down, k.left, k.right, k.page, k.ends}
	if m.mode == modeCard {
		moving = []key.Binding{k.scrollUp, k.scrollDown, k.page, k.ends}
	}
	acts := k.shortHelp(m, true)
	return [][]key.Binding{moving, acts, {k.repaint, k.interrupt}}
}

// itemHelp answers the bindings the footer lists in item mode: the highlight
// keys, then the letters of the item acts offered on the highlighted item, in
// the act table's order, then ctrl+g, q and ctrl+c.
func (k *interactiveKeys) itemHelp(m *interactiveModel) []key.Binding {
	bindings := []key.Binding{k.menuUp, k.menuDown}
	for _, row := range interactiveActs {
		if row.mode == actItems && row.offered(m) {
			bindings = append(bindings, row.binding(k))
		}
	}
	return append(bindings, k.itemClose, k.itemQuit, k.interrupt)
}

// outputHelp answers the bindings the footer lists in output mode.
func (k *interactiveKeys) outputHelp() []key.Binding {
	return []key.Binding{k.scrollUp, k.page, k.ends, k.outputClose, k.jump, k.interrupt}
}

// menuHelp answers the bindings the footer lists while a menu is open.
func (k *interactiveKeys) menuHelp() []key.Binding {
	return []key.Binding{k.menuChoose, k.menuNumber, k.menuCancel, k.menuQuit, k.interrupt}
}

// promptHelp answers the bindings the footer lists while a prompt is open:
// the multi-line prompt's, or Enter, Tab at the command line alone, Ctrl+G
// and Ctrl+C.
func (k *interactiveKeys) promptHelp(prompt promptKind) []key.Binding {
	switch prompt {
	case promptText:
		return []key.Binding{k.promptNewline, k.promptPost, k.promptCancel, k.interrupt}
	case promptJump:
		return []key.Binding{k.promptGo, k.promptComplete, k.promptCancel, k.interrupt}
	}
	return []key.Binding{k.promptGo, k.promptCancel, k.interrupt}
}

// teaCodes maps each named key code the readers deliver to Bubble Tea's own,
// in the one table section 6 prescribes.
var teaCodes = map[keyboard.Code]rune{
	keyboard.CodeEnter:     tea.KeyEnter,
	keyboard.CodeBackspace: tea.KeyBackspace,
	keyboard.CodeDelete:    tea.KeyDelete,
	keyboard.CodeTab:       tea.KeyTab,
	keyboard.CodeUp:        tea.KeyUp,
	keyboard.CodeDown:      tea.KeyDown,
	keyboard.CodeLeft:      tea.KeyLeft,
	keyboard.CodeRight:     tea.KeyRight,
	keyboard.CodePgUp:      tea.KeyPgUp,
	keyboard.CodePgDown:    tea.KeyPgDown,
	keyboard.CodeHome:      tea.KeyHome,
	keyboard.CodeEnd:       tea.KeyEnd,
}

// teaCode is Bubble Tea's code for a decoded key: the table's entry for a
// named key, and the key's own rune for a printable or ctrl+ key.
func teaCode(decoded keyboard.Key) rune {
	if decoded.Code == keyboard.CodeRune {
		return decoded.Rune
	}
	return teaCodes[decoded.Code]
}

// interactiveKeyMsg is the message the head builds from one decoded key, in
// the shape the Bubbles components expect.
func interactiveKeyMsg(decoded keyboard.Key) tea.KeyPressMsg {
	pressed := tea.KeyPressMsg{Code: teaCode(decoded), Text: decoded.Text}
	if decoded.Ctrl {
		pressed.Mod = tea.ModCtrl
	}
	return pressed
}

// disabledBinding is a binding no key reaches.
func disabledBinding() key.Binding {
	return key.NewBinding(key.WithDisabled())
}

// interactiveInputKeys is the key map of the jump and filter prompts: the
// textinput's own, with its clipboard paste switched off so no clipboard
// program is ever started, and every Alt key removed from the word bindings.
func interactiveInputKeys() textinput.KeyMap {
	keys := textinput.DefaultKeyMap()
	keys.Paste = disabledBinding()
	keys.WordForward = key.NewBinding(key.WithKeys("ctrl+right"))
	keys.WordBackward = key.NewBinding(key.WithKeys("ctrl+left"))
	keys.DeleteWordBackward = key.NewBinding(key.WithKeys("ctrl+w", "ctrl+backspace"))
	keys.DeleteWordForward = key.NewBinding(key.WithKeys("ctrl+delete"))
	return keys
}

// interactiveAreaKeys is the key map of the comment prompt: the textarea's
// own, with its clipboard bindings and select-all switched off, ctrl+d left
// to post and ctrl+m to nothing, and every Alt key removed.
func interactiveAreaKeys() textarea.KeyMap {
	keys := textarea.DefaultKeyMap()
	keys.Paste = disabledBinding()
	keys.CopySelection = disabledBinding()
	keys.SelectAll = disabledBinding()
	keys.DeleteCharacterForward = key.NewBinding(key.WithKeys("delete"))
	keys.InsertNewline = key.NewBinding(key.WithKeys("enter"))
	keys.WordForward = key.NewBinding(key.WithKeys("ctrl+right"))
	keys.WordBackward = key.NewBinding(key.WithKeys("ctrl+left"))
	keys.DeleteWordBackward = key.NewBinding(key.WithKeys("ctrl+w", "ctrl+backspace"))
	keys.DeleteWordForward = key.NewBinding(key.WithKeys("ctrl+delete"))
	keys.InputBegin = key.NewBinding(key.WithKeys("ctrl+home"))
	keys.InputEnd = key.NewBinding(key.WithKeys("ctrl+end"))
	keys.SelectWordForward = key.NewBinding(key.WithKeys("ctrl+shift+right"))
	keys.SelectWordBackward = key.NewBinding(key.WithKeys("ctrl+shift+left"))
	keys.CapitalizeWordForward = disabledBinding()
	keys.LowercaseWordForward = disabledBinding()
	keys.UppercaseWordForward = disabledBinding()
	return keys
}
