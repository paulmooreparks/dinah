---
title: A workbench does not record where its definition came from, though the design says it does
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
---
docs/design/surfaces.md specifies that a workbench records the source it was built from and that source's hash. Nothing writes it. Instantiate does not, init does not, and no other command does either, so the specification describes a key that never appears in a real workbench.

This surfaced on dinah-316, whose spec told the implementer to record provenance on reshape "the way Instantiate already specifies for init". Instantiate does not specify it in any way the code carries out. Writing it on reshape alone was rejected there and recorded as decision D-8: reshape would become the only command in the tool that records where a definition came from, and the sample fixture would carry a key nothing else populates, which invites a reader to trust a half-populated field as a general fact about workbenches. Provenance belongs to init and reshape together or to neither.

What this card has to settle, before it settles how to write anything. The specification for provenance sits under the template-library-by-URL feature, which has not shipped. So the first question is whether provenance is a property of every workbench or only of one built from a remote template, and the answer decides whether this is a small addition to two commands or a piece of a feature that should wait for the rest of it.

If it is general, then a definition applied by hand and a definition fetched from a URL both want recording, the hash has to be of something stable enough to be worth storing, and a workbench whose definition has since been reshaped needs an answer about whether provenance is the original source, the latest one, or a sequence. None of that is decided.

There is a real argument for closing it the other way, and whoever specs this should weigh it rather than assume the addition: delete the unimplemented specification instead. A design document describing a field nothing writes is the same defect this board has been closing all week, a claim with nothing behind it, and the cheaper honest fix may be to say provenance arrives with the template library rather than to build half of it now.
