package main

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"dinah/internal/msg"
	"dinah/internal/screen"
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
)

// interactiveKeys are every binding the terminal head matches keys against
// and draws its footer from, built in one language with the destinations the
// a and b keys name filled in.
type interactiveKeys struct {
	// The bindings of browse mode, most of which card mode shares.
	up, down, left, right, page, ends, show     key.Binding
	claim, accept, advance, sendBack, move      key.Binding
	release, comment, filter, jump, more, fewer key.Binding
	quit, interrupt                             key.Binding
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

	k.promptGo = add(bindingJump, "interactive.help.go", nil, key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp(enter, r.T("interactive.help.go")),
	))
	k.promptCancel = add(bindingJump, "interactive.help.cancel", nil, key.NewBinding(
		key.WithKeys("ctrl+g"),
		key.WithHelp(ctrlG, r.T("interactive.help.cancel")),
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
// only the acts the offer allows, in the order enter, t, a, b, m, r, c, /, :,
// ?, q.
func (k *interactiveKeys) shortHelp(card bool, offer interactiveOffer, full bool) []key.Binding {
	first := k.show
	if card {
		first = k.back
	}
	bindings := []key.Binding{first}
	if offer.claim {
		bindings = append(bindings, k.claim)
	}
	if offer.forward != nil {
		if offer.forwardTerminal {
			bindings = append(bindings, k.accept)
		} else {
			bindings = append(bindings, k.advance)
		}
	}
	if offer.back != nil {
		bindings = append(bindings, k.sendBack)
	}
	if len(offer.moves) > 0 {
		bindings = append(bindings, k.move)
	}
	if offer.release {
		bindings = append(bindings, k.release)
	}
	if offer.comment {
		bindings = append(bindings, k.comment)
	}
	if !card {
		bindings = append(bindings, k.filter, k.jump)
	}
	if full {
		bindings = append(bindings, k.fewer)
	} else {
		bindings = append(bindings, k.more)
	}
	return append(bindings, k.quit)
}

// fullHelp answers the bindings full help lists in browse and card mode, in
// columns: how to move, then what the offer allows, then the rest.
func (k *interactiveKeys) fullHelp(card bool, offer interactiveOffer) [][]key.Binding {
	moving := []key.Binding{k.up, k.down, k.left, k.right, k.page, k.ends}
	if card {
		moving = []key.Binding{k.scrollUp, k.scrollDown, k.page, k.ends}
	}
	acts := k.shortHelp(card, offer, true)
	return [][]key.Binding{moving, acts, {k.interrupt}}
}

// menuHelp answers the bindings the footer lists while the move menu is open.
func (k *interactiveKeys) menuHelp() []key.Binding {
	return []key.Binding{k.menuChoose, k.menuNumber, k.menuCancel, k.menuQuit, k.interrupt}
}

// promptHelp answers the bindings the footer lists while a prompt is open.
func (k *interactiveKeys) promptHelp(comment bool) []key.Binding {
	if comment {
		return []key.Binding{k.promptNewline, k.promptPost, k.promptCancel, k.interrupt}
	}
	return []key.Binding{k.promptGo, k.promptCancel, k.interrupt}
}

// interactiveKeyMsg is the message the head builds from one decoded key, in
// the shape the Bubbles components expect.
func interactiveKeyMsg(decoded screen.Key) tea.KeyPressMsg {
	pressed := tea.KeyPressMsg{Code: decoded.Code, Text: decoded.Text}
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
