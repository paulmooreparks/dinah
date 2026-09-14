---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:16:30Z
ordinal: 9
note: "Re-measured independently on Test cycle 2026-08-27, on the re-merged tree at a50b68c, using a freshly-installed tiktoken 0.14.0 (cl100k_base) and a binary rebuilt from this exact tree. Ran `python scripts/measure_compact_tokens.py --dinah <binary> --root <root>` twice against two independent fresh probe roots. Both runs report identical figures and identical digests: claim response 40.6% fewer tokens (325->193, digests 6c7df75671d7/346a8067c511), six-card ls listing 33.9% fewer (704->465, digests 6900279d2c22/fae66fb3f192), and the short-paragraph-instructions variant of the claim response 36.1% fewer (385->246, digests cff8cdee109b/f11815cf0c7a). All four figures match the previously recorded readings exactly and all clear the 30% floor."
---
Using a named, real byte-pair-encoding tokenizer (for example tiktoken's cl100k_base, run offline as a verification script rather than shipped in the binary), the compact encoding of a claim response against a card carrying Instructions and two LegalMoves, and the compact encoding of an ls listing of six ordinary cards, are each measured against their canonical --json encoding of the same answer. The measured percentage is recorded in the implementation's PR description or commit message rather than asserted here in advance; a result of less than 30% fewer tokens on either measurement is a finding for Implement to raise back to this card rather than something to ship silently.