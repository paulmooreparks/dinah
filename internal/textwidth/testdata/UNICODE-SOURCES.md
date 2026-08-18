# Where the vendored Unicode data came from

`TestDisplayWidthMeasuresEveryPublishedEmojiSequence` sweeps the three files
Unicode publishes that enumerate emoji sequences. All three are pinned to the
Unicode release the shipped tables were generated from, because a table built
at one release measures a sequence a later release added from its parts rather
than as one glyph, and the sweep would then fail for a reason that is not a
defect in the rule.

The pinned release is **Unicode 15.0**, which is what `unicode.Version` reports
for the Go toolchain this module builds with and what
`golang.org/x/text/width.UnicodeVersion` reports for the East Asian Width data.
`internal/textwidth/emoji_properties.go` names the same release, and
`TestTheUnicodeTablesAgree` fails when any of them drift apart.

The three files do not come from one directory, and a fetch that assumes they
do gets a 404.

| File | Source |
|---|---|
| `emoji-variation-sequences.txt` | <https://www.unicode.org/Public/15.0.0/ucd/emoji/emoji-variation-sequences.txt> |
| `emoji-sequences.txt` | <https://www.unicode.org/Public/emoji/15.0/emoji-sequences.txt> |
| `emoji-zwj-sequences.txt` | <https://www.unicode.org/Public/emoji/15.0/emoji-zwj-sequences.txt> |

`internal/textwidth/emoji-data.txt`, which the generator reads, comes from
<https://www.unicode.org/Public/15.0.0/ucd/emoji/emoji-data.txt>.

Regenerate the tables after replacing any of these:

	go run internal/textwidth/gen_emoji_properties.go
