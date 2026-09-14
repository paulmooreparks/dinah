---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:16:30Z
ordinal: 8
note: "Re-verified independently on Test cycle 2026-08-27, on the re-merged tree at a50b68c. `go test ./internal/msg/... -count=1` and the cmd/dinah help/catalog tests are green. Built the branch binary and confirmed `dinah help` lists \"--format <name>    Select json or compact for the machine form\" beside --json and the Environment line reads \"DINAH_FORMAT=json|compact\". Armed by deleting the flag.format.summary block from internal/msg/locales/de.json: TestEveryDeclaredLanguageShips reddened naming the missing key; reverted and confirmed green."
---
dinah --help lists a --format <name> row next to --json, and the Environment line reads DINAH_FORMAT=json|compact; every locale file under internal/msg/locales/ carries a flag.format.summary key and an updated help.environment key, and the existing catalog-coverage test passes with no locale missing either key.