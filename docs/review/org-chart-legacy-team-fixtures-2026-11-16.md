# Historical team fixture correction

Front End's full desktop non-race/all-vet run on 560dc215 completed in 258.970 seconds and retained two failures in team_views_test.go. This branch starts at that exact source and changes only the test fixture and this note. The original failed receipt stays with Front End.

The strip rendering case previously created a nested team through current SetLead admission. Current admission correctly refuses nesting. The case now first proves that refusal, then seeds the historical relation and its complete channel membership directly into the isolated database in one transaction. Every original rendering, state, task, nesting and relay assertion remains unchanged. No production admission path is relaxed.

The page mutation case previously checked for the former 'already leads' substring. It now requires the complete approved refusal with the current team's title. The refused assignment and all surrounding membership/group assertions remain unchanged. This is an explicit expectation update for the already approved wording change, rather than a new behavior change.

Focused verification is pending. Front End retains the full-package rerun and both-product receiving gates. This test-only correction makes no new product, rendered or installed acceptance claim.
