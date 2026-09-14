---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:31Z
ordinal: 5
note: "Verified at 5a88bec, both halves run. First half: planted \"Dinah has no `restore`.\" in verbs.md with the command in backticks; the check went red naming internal/guide/guides/verbs.md and quoting the sentence with its backticks intact. Second half: with the call-site strip removed (`FindAllStringSubmatch(sentence, -1)` in place of `FindAllStringSubmatch(strings.ReplaceAll(sentence, \"`\", \"\"), -1)`), the backticked plant PASSED, while the unbackticked \"Dinah has no restore.\" plant still FAILED, so removing the strip does not simply switch the check off and the strip is doing work rather than being decoration. Both files restored byte-identically (cmp clean) and green. The strip is also load-bearing on real text: it raises the corpus firing count from 12 to 14, re-measured on this branch."
---
The same sentence with `restore` in backticks also fails the check, and removing the call-site backtick strip makes that backticked plant pass while the unbackticked plant still fails. Both halves are run, because the second is what shows the strip does work rather than being decoration.