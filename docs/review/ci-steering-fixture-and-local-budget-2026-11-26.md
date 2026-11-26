# Provider fixture ordering and local package budget

The Devin pause fixture announced receipt in wire.jsonl before writing the prompt file. The Go test waits for that receipt and immediately reads the file. It could therefore submit the completed first turn to UserPromptSubmit while the working turn was active. HostedTurnHook binds the marked turn from that text, so steering for the working turn never became available. Increasing the completion ceiling exposed the failure for longer; it did not repair this ordering race.

The peer now saves a Devin session prompt synchronously before publishing its receipt. The Go test also checks that the saved prompt contains the working-turn text before submitting the hook. The existing steering, pause delivery, durable restart and deduplication assertions remain intact. No product code or interface changes.

The local Go gate now supplies -timeout=30m when the caller supplies no timeout, matching the workflow. Explicit -timeout=45s and split -timeout 0 remain unchanged. The wrapper still invokes go test once and returns its exit code.

The failed runner package took 818.182 seconds, including the Devin case at 122.28 seconds. Removing that entire failing case would still leave about 696 seconds, above the default ten-minute package ceiling. Other passing tests included the chat-question steering test at 29.64 seconds and provider-switch tests at 17.17 and 15.94 seconds. The sweep changed positive completion ceilings rather than unconditional sleeps. Its earlier one-CPU three-repetition passing run took 906.023 seconds in total. These measurements support a finite larger package budget under load; they do not establish that every future slow run is healthy.

## Verification

Verification uses Go 1.26.11. The complete TestQualityPauseProviderContract passed five repetitions with -cpu=1 -p=1 in 44.948 seconds, including Devin, plain ACP, Codex, Claude and the model peer. The output is retained at %TEMP%/dinah-steering-fixture-focused.log. The timeout-selection block was executed directly for absent, equals-form and split-form timeout arguments. It added only the missing default. A full wrapper probe was cancelled while queued behind another gate, before it invoked go. No full package run, GitHub retry, push or installation was performed for this change.
