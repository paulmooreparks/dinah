# Chat fixture completion boundaries under load

The first PUDL 0.110.0 integration run failed four assertions under eight-way browser load. This branch corrects three fixtures and leaves skins-startup unchanged for a separate startup investigation. No product or vendored source changes are included.

## Chat inventory

The comparison waited for the chart's row count, then sampled the sidebar and cleanup table once. The retained failure shows the restored Claude row in the chart and cleanup table while it is still absent from the sidebar. Those surfaces render independently. The fixture now waits for their complete projections to converge and takes one atomic snapshot. The assertion also requires the expected row count on every surface, preserving detection of a consistently missing row. A timeout records the actual mismatched projections and fails the assertion.

## Unread streaming

The initial extent check could finish before all sixteen earlier messages had rendered. More importantly, a newly attached divider could describe the tool row before its following message had rendered. The test compared that intermediate count with a later count. It now waits for the final earlier message before navigation, the new message before recording its unread baseline, and all five appended chunks before comparison. Two rendering frames allow the transcript observer to process those DOM changes. The logical unread-count assertion is unchanged.

The original test reproduced under fourfold CPU throttling on both f067b7e6 and d6c9afcf. It therefore predates PUDL 0.110.0. A slower host is exposing a fixture boundary, rather than requiring a longer latency ceiling.

## Saved drawing

The original fixture restored a closed chat and scheduled an unrelated background reconnect after 250 milliseconds. Waiting for its first visible row did not wait for the restore operation to finish. The fixture now supplies a live journal-backed saved copy and awaits the initial restore, avoiding that unrelated reconnect while testing draw cancellation. The scheduler callback is intercepted only in the fixture: the first batch receives an expired budget after its mandatory first piece of work, and the replacement starts only when that batch reports more work. A new assertion proves the first draw was unfinished. Both original row-ownership and waiter-completion assertions remain.

This does not establish a product defect in drawSaved. The unmodified draw test passed the measured local runs. The fixture removes competing startup work and strengthens the intended mid-draw test rather than weakening the observed failure condition.

## Verification

The original eight-suite selection passed normally in 42 seconds. With fourfold CPU throttling, the original selection failed only chat-unread on both d6c9afcf (81 seconds) and pre-release f067b7e6 (78 seconds). The corrected three suites and five neighbours passed under the same throttling and eight-way concurrency in 52 seconds. The temporary throttle is not committed. The three corrected suites also passed at normal speed in five seconds. The controlled draw operation has the existing deadline helper as a finite failure ceiling. The staged guard is recorded in the handoff. Evidence folders are %TEMP%/dinah-load-suites-before, dinah-load-suites-slow-before, dinah-load-suites-slow-old-pudl and dinah-load-suites-slow-after.

The skin startup finding remains separately owned. Its assertion and all product code are unchanged. No GitHub retry, push or installation is part of this branch.
