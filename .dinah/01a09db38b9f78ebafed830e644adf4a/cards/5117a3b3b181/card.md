---
title: A test's own cleanup can fail on Windows and take the extension release down with it
column: 5ea2db0272fc
state: ready
severity: major
priority: soon
---
The extension release workflow failed on the merge of dinah-455, so no extension release was cut for that commit and the published version still stands at 1.0.12. Nothing was wrong with the card.

What failed was a teardown hook, not an assertion. `verbCatalog-live.test.js` threw `ENOTEMPTY: directory not empty` while removing its own Windows temporary directory. Its two assertions had already passed, 540 of 541 tests passed, and the identical job on the identical commit passed in the standalone CI run minutes earlier. So the same tree both passes and fails depending on which workflow reads it.

That is the defect: a flaky cleanup can stop a release. A test that has finished proving what it exists to prove should not be able to withhold a published artifact by failing to tidy up after itself.

Two things make this worth a card rather than a re-run.

It will recur, and the next person will lose time deciding whether the failure is real. This one took a careful read to tell a teardown from an assertion, and the failure text names a directory rather than a behaviour. Anybody who sees red on a release and re-runs it without reading has learned to re-run releases without reading.

Re-running is not free here. The release workflow mints a tag and publishes, so the ordinary remedy for a flake is an action nobody should take casually, which is why the merge stage stopped and asked rather than clicking it.

What the card should settle, none of it the operator's to rule on:

Why the removal fails. `ENOTEMPTY` on Windows usually means something still holds a handle inside the directory, which for this test means a spawned process that has exited from the runner's point of view but whose handles have not been released, or a file the test itself left open. Find the cause rather than adding a retry; a retry loop around a cleanup is how this becomes permanent.

Whether a teardown failure should be able to fail the run at all. There is a real argument each way. A cleanup that silently fails leaves temp directories accumulating on a runner, and a suite that ignores its own teardown errors is a suite that cannot tell you it is leaking. But a release gated on tidying is a release gated on the wrong thing. If the answer is that teardown failures are reported without failing the run, say where they are reported so they are not simply lost.

Whether the release workflow should run the unit suite at all, given the standalone CI run already runs it on the same commit on three platforms. Two runs of the same tests on the same tree is two chances to flake for one guarantee. This is the question with the largest payoff and it should be answered deliberately rather than by removing the gate because it was inconvenient once.

The failing run is at https://github.com/paulmooreparks/dinah/actions/runs/34367034784. Cleanup ran correctly on the failure, so no orphan tag was left behind, and the release for that commit is simply absent rather than half-made.
