---
kind: acceptance_criterion
state: verified
column: c9428b3bc921
ts: 2026-09-14T02:16:35Z
ordinal: 5
note: "Direct run: `dinah add -- --lang de` exits 2 with dinah.multiple-words refusal in English (not German); both words treated as free text past the -- marker."
---
dinah add -- --lang de exits 2 with a dinah.multiple-words refusal rendered in English, not German, demonstrating scanLangFlag stops at the bare "--" marker and does not read a --lang that follows it (both words become add's literal free text, and two words there is itself a refusal under dinah-100's one-word rule). Verified with t.Setenv("DINAH_LANG", "") and no configured lang.