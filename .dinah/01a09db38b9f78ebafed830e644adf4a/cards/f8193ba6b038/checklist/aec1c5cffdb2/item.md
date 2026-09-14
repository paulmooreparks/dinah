---
kind: decision
state: resolved
column: c9428b3bc921
owner: holder
ts: 2026-09-14T02:16:31Z
ordinal: 21
note: "Asked mechanically rather than by reading the encoder. A binary built at 9b60386 ran the acts the suite runs, in the compact form, against a fixture matching the test fixture, and a script split every record and recorded, per record kind and per position, whether any payload carried a value there. Before this lap four positions were empty in every payload and formed two pairs, which is what makes a transposition invisible to a comparison that strips empty values: the rsp record's warning and warning_detail, and the card record's expires and block_kind. Two more were empty and alone in their record, so no transposition could hide in them, but each was asserted only by its own absence: the instr record's global layer and the move record's reject flag. The fixture now fills all six, by claiming with an expiry, blocking with a kind, addressing one act by a stale prefix, writing the user base's instructions file, and declaring reject_to on Doing. The sweep re-run after those changes reports no always-empty position in any of aff, card, ctx, fmt, instr, lst, move, msgval, off, rsp, wb or wstream. One field of the Go structs is unreachable from the head at all rather than merely unexercised: Response.Basis is populated on every OK outcome, so it is covered, but a stale outcome cannot be produced because cmd/dinah/args.go records that the tool exposes no --basis flag by design, and the compact form's stale-outcome envelope is therefore never written by any CLI invocation."
---
Every position of every compact record kind now carries a value in at least one payload the fixture produces, so no field of the encoding is asserted only as an absence.