---
kind: decision
state: resolved
column: 4b38abe7ebd5
owner: holder
ts: 2026-09-14T02:18:15Z
ordinal: 24
note: "Taken at round 2 of code review, on the reviewer's major. internal/msg/glossary.json declared संग्रह as the Hindi for \"collection\" and the catalogue had honoured that in six older entries; this card's new Hindi then spent the same word on the archive, and refusal.dinah.not-archived.next-collection carried both senses in one sentence, reading as the contents of that collection in the collection. The archive is a recurring concept of this contract, which is what a glossary term is for, so it is declared rather than patched string by string. Hindi form: अभिलेखागार, the standard Hindi noun for an archive and the word in the official name of the National Archives of India (भारतीय राष्ट्रीय अभिलेखागार), so the evidence is a published proper name rather than a preference. German form: Archiv, which all four German entries already carried unchanged, so declaring the term costs German nothing and pins what it already does. Only Archiv is declared for German and no lowercase variant: the guard matches by containment, and a lowercase \"archiv\" would also be satisfied by archiviert, the participle for \"archived\", which is the very looseness being removed on the Hindi side. Mirrored into editors/vscode/src/locales/flags.json, which carries the CLI's term list byte for byte; both extension jobs failed until it was, which is that mirror test doing its job."
---
"the archive" is declared as a glossary term of its own, with Archiv for German and अभिलेखागार for Hindi, rather than letting the archive keep borrowing the word declared for a collection.