package msg

import (
	"path/filepath"
	"testing"
)

// TestTheInteractiveKeysReachEveryCatalogue is the catalogue half of
// dinah-603/criteria/21, in the shape of
// TestTheRestoreKeysAreCarriedInEveryCatalogue: every key section 12 of the
// terminal head's specification names, and the one key the head added for
// the move menu's highlight, is carried by every catalogue. The base entry
// carries a text and a context. German and Hindi carry a translation holding
// the fingerprint of the English of the day, which differs from the English
// unless the entry is marked verbatim, and every other catalogue carries the
// English as a skeleton. The catalogue directory is enumerated rather than
// listed, and both it and the key list carry a floor.
func TestTheInteractiveKeysReachEveryCatalogue(t *testing.T) {
	keys := []string{
		"cmd.tui.summary",
		"param.tui.view.summary",
		"param.tui.plain.summary",
		"check.tui.1",
		"check.tui.2",
		"check.tui.3",
		"refusal.dinah.tui-unavailable",
		"refusal.dinah.tui-unavailable.size",
		"refusal.dinah.tui-unavailable.capability",
		"refusal.dinah.tui-unavailable.next-small",
		"refusal.dinah.tui-unavailable.next",
		"interactive.heading",
		"interactive.heading.filtered",
		"interactive.heading.acting",
		"interactive.heading.acting.operator",
		"interactive.heading.no-actor",
		"interactive.lane.section",
		"interactive.too-small",
		"interactive.acted.claim",
		"interactive.acted.release",
		"interactive.acted.move",
		"interactive.acted.comment",
		"interactive.line.nothing",
		"interactive.jump.no-lane",
		"interactive.prompt.comment",
		"interactive.menu.title",
		"interactive.menu.row",
		"interactive.menu.row.route",
		"interactive.menu.row.reject",
		"interactive.crashed",
		"interactive.help.show",
		"interactive.help.back",
		"interactive.help.claim",
		"interactive.help.accept",
		"interactive.help.advance",
		"interactive.help.send-back",
		"interactive.help.move",
		"interactive.help.release",
		"interactive.help.comment",
		"interactive.help.filter",
		"interactive.help.jump",
		"interactive.help.keys",
		"interactive.help.fewer",
		"interactive.help.quit",
		"interactive.help.cancel",
		"interactive.help.choose",
		"interactive.help.post",
		"interactive.help.newline",
		"interactive.help.go",
		"interactive.help.up",
		"interactive.help.down",
		"interactive.help.left",
		"interactive.help.right",
		"interactive.help.page",
		"interactive.help.ends",
		"interactive.help.scroll",
		"interactive.help.highlight",
		"interactive.key.enter",
		"interactive.key.backspace",
		"interactive.key.ctrl-c",
		"interactive.key.ctrl-d",
		"interactive.key.ctrl-g",
		"interactive.key.up",
		"interactive.key.down",
		"interactive.key.left",
		"interactive.key.right",
		"interactive.key.pgup",
		"interactive.key.pgdown",
		"interactive.key.home",
		"interactive.key.end",
		// The keys dinah-623 mints for the command line, item mode, the
		// actions menu, the read keys and key bindings.
		"refusal.dinah.not-in-tui",
		"refusal.dinah.not-in-tui.mcp",
		"refusal.dinah.not-in-tui.lsp",
		"refusal.dinah.not-in-tui.serve",
		"refusal.dinah.not-in-tui.ui",
		"refusal.dinah.not-in-tui.completion",
		"refusal.dinah.not-in-tui.tui",
		"refusal.dinah.not-in-tui.stdin",
		"refusal.dinah.not-in-tui.watch",
		"refusal.dinah.not-in-tui.wait",
		"refusal.dinah.not-in-tui.next",
		"refusal.dinah.invalid-key-binding",
		"refusal.dinah.invalid-key-binding.invalid-key",
		"refusal.dinah.invalid-key-binding.reserved-key",
		"refusal.dinah.invalid-key-binding.empty-template",
		"refusal.dinah.invalid-key-binding.shell-template",
		"refusal.dinah.invalid-key-binding.non-ascii-whitespace",
		"refusal.dinah.invalid-key-binding.numbered-placeholder",
		"refusal.dinah.invalid-key-binding.unknown-placeholder",
		"refusal.dinah.invalid-key-binding.next",
		"interactive.help.items",
		"interactive.help.actions",
		"interactive.help.next",
		"interactive.help.status",
		"interactive.help.search",
		"interactive.help.changes",
		"interactive.help.whoami",
		"interactive.help.answer",
		"interactive.help.verify",
		"interactive.help.fail",
		"interactive.help.waive",
		"interactive.help.withdraw",
		"interactive.help.reopen",
		"interactive.help.cite",
		"interactive.help.close",
		"interactive.help.complete",
		"interactive.key.tab",
		"interactive.line.done",
		"interactive.line.exit",
		"interactive.line.pinned",
		"interactive.line.paste",
		"interactive.line.candidates",
		"interactive.output.title",
		"interactive.output.cut",
		"interactive.items.title",
		"interactive.items.row",
		"interactive.items.none",
		"interactive.prompt.resolve",
		"interactive.prompt.verify",
		"interactive.prompt.fail",
		"interactive.prompt.waive",
		"interactive.prompt.withdraw",
		"interactive.prompt.reopen",
		"interactive.prompt.cite-scheme",
		"interactive.prompt.cite-target",
		"interactive.prompt.cite-observed",
		"interactive.prompt.search",
		"interactive.prompt.add-title",
		"interactive.prompt.block",
		"interactive.prompt.block-kind",
		"interactive.prompt.unblock",
		"interactive.prompt.raise",
		"interactive.prompt.attach-file",
		"interactive.prompt.attach-description",
		"interactive.prompt.file",
		"interactive.prompt.link-kind",
		"interactive.prompt.link-to",
		"interactive.prompt.rename",
		"interactive.prompt.set",
		"interactive.menu.scheme",
		"interactive.menu.item",
		"interactive.menu.add-column",
		"interactive.menu.tier",
		"interactive.menu.item-kind",
		"interactive.menu.item-column",
		"interactive.menu.item-owner",
		"interactive.menu.item-owner.none",
		"interactive.menu.permission",
		"interactive.menu.link",
		"interactive.menu.join",
		"interactive.menu.leave",
		"interactive.menu.archive",
		"interactive.menu.delete",
		"interactive.menu.delete-force",
		"interactive.menu.divergence",
		"interactive.menu.rename",
		"interactive.menu.field",
		"interactive.menu.value",
		"interactive.actions.title",
		"interactive.actions.add",
		"interactive.actions.claim",
		"interactive.actions.move",
		"interactive.actions.pull",
		"interactive.actions.release",
		"interactive.actions.block",
		"interactive.actions.unblock",
		"interactive.actions.raise",
		"interactive.actions.comment",
		"interactive.actions.attach",
		"interactive.actions.file",
		"interactive.actions.cite",
		"interactive.actions.resolve",
		"interactive.actions.verify",
		"interactive.actions.fail",
		"interactive.actions.waive",
		"interactive.actions.withdraw",
		"interactive.actions.reopen",
		"interactive.actions.grant",
		"interactive.actions.revoke",
		"interactive.actions.link",
		"interactive.actions.unlink",
		"interactive.actions.join",
		"interactive.actions.leave",
		"interactive.actions.archive",
		"interactive.actions.restore",
		"interactive.actions.delete",
		"interactive.actions.accept-divergence",
		"interactive.actions.rename",
		"interactive.actions.set",
		"interactive.actions.edit",
		"interactive.acted.resolve",
		"interactive.acted.verify",
		"interactive.acted.fail",
		"interactive.acted.waive",
		"interactive.acted.withdraw",
		"interactive.acted.reopen",
		"interactive.acted.cite",
		"interactive.acted.add",
		"interactive.acted.pull",
		"interactive.acted.block",
		"interactive.acted.unblock",
		"interactive.acted.raise",
		"interactive.acted.attach",
		"interactive.acted.file",
		"interactive.acted.grant",
		"interactive.acted.revoke",
		"interactive.acted.link",
		"interactive.acted.unlink",
		"interactive.acted.join",
		"interactive.acted.leave",
		"interactive.acted.archive",
		"interactive.acted.restore",
		"interactive.acted.delete",
		"interactive.acted.accept-divergence",
		"interactive.acted.rename",
		"interactive.acted.set",
		"interactive.binding.unused",
		"interactive.binding.needs",
		"interactive.binding.what.card",
		"interactive.binding.what.column",
		"interactive.binding.what.view",
		"interactive.binding.defect.invalid-key",
		"interactive.binding.defect.reserved-key",
		"interactive.binding.defect.empty-template",
		"interactive.binding.defect.shell-template",
		"interactive.binding.defect.non-ascii-whitespace",
		"interactive.binding.defect.numbered-placeholder",
		"interactive.binding.defect.unknown-placeholder",
	}
	if len(keys) < 70+158 {
		t.Fatalf("the subject set holds %d keys, and dinah-603 minted 70 and dinah-623 158 more", len(keys))
	}
	files, err := filepath.Glob(filepath.Join("locales", "*.json"))
	if err != nil {
		t.Fatalf("glob the catalogues: %v", err)
	}
	if len(files) < 8 {
		t.Fatalf("the catalogue directory holds %d files and the tool carries eight: %v", len(files), files)
	}
	translated := map[string]bool{"de": true, "hi": true}
	entries := 0
	for _, file := range files {
		tag := filepath.Base(file)
		tag = tag[:len(tag)-len(".json")]
		for _, key := range keys {
			base, carried := BaseEntry(key)
			if !carried {
				t.Fatalf("English carries no entry for %s", key)
			}
			if base.Text == "" || base.Context == "" {
				t.Errorf("the base entry for %s carries an empty text or an empty context", key)
			}
			entry, held := CatalogEntry(tag, key)
			if !held {
				t.Errorf("%s carries no entry for %s", tag, key)
				continue
			}
			entries++
			switch {
			case tag == Base:
				if entry.Source != "" || entry.Skeleton {
					t.Errorf("the base entry for %s carries a source or a skeleton mark", key)
				}
			case translated[tag]:
				if entry.Verbatim != (entry.Text == base.Text) {
					t.Errorf("%s carries %q for %s, and the verbatim mark says %v", tag, entry.Text, key, entry.Verbatim)
				}
				if entry.Skeleton {
					t.Errorf("%s marks %s a skeleton and it is translated", tag, key)
				}
				if want := Fingerprint(base.Text); entry.Source != want {
					t.Errorf("%s records source %q for %s and the English of the day fingerprints to %q", tag, entry.Source, key, want)
				}
			default:
				if !entry.Skeleton || entry.Source != "" || entry.Text != base.Text {
					t.Errorf("%s's entry for %s is not the English carried as a skeleton", tag, key)
				}
			}
		}
	}
	t.Logf("%d catalogue files enumerated, %d entries read", len(files), entries)
	if want := len(files) * len(keys); entries != want {
		t.Fatalf("the sweep read %d entries and %d keys across %d catalogues is %d", entries, len(keys), len(files), want)
	}
}
