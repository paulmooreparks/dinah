# Bus growth

The growing history scans need attention before retention does. This branch changes no product code or retention policy. Paul decides what may be removed.

The invented fixture follows the rehearsal's counts, with roughly 4.14 deliveries per message and 8 KiB bodies. Growth keeps 148 chats, 20 groups, five waiting messages, five unread mentions and seven open requests. No real database was used. Its 74.9 MB baseline is smaller than the 78.9 MB Paul reported.

The Decisions boundary stays fixed as history grows; the attention cursor advances. Roster growth, larger backlogs and enormous results remain unmeasured.

## Measurements

Current APIs ran through FIFO on Windows/amd64, Go 1.26.10. Times are median milliseconds over 21 warmed calls. Browser work, notification-file writes and contention are excluded.

| Measurement | Today | 10 times | 100 times |
| --- | ---: | ---: | ---: |
| Messages | 7,825 | 78,250 | 782,500 |
| Deliveries | 32,404 | 323,964 | 3,239,559 |
| Database MB, decimal | 74.9 | 746.8 | 7,467.0 |
| Global wake check | 3.5 | 73.0 | 826.0 |
| One chat's doorbell | <1 | <1 | <1 |
| Deliver five messages | 3.5 | 4.0 | 5.5 |
| Empty delivery check | 2.5 | 2.5 | 2.5 |
| Decisions entries, seven rows | 1.0 | 151.1 | 1,952.0 |
| Latest 100 group messages | 5.0 | 8.6 | 7.0 |
| Import five unread attention messages | 1.0 | 3.5 | 23.6 |
| Incremental attention, 16 or 17 messages | 9.0 | 66.5 | 744.7 |
| Check five attention answers | <1 | <1 | <1 |

The first tested slowdown is at 78,250 messages; the point between sizes is unknown. Wake scans deliveries, attention searches human-delivery history before applying its cursor, and Decisions considers every message since first use. A near-newest Decisions boundary returns the same seven fixture rows in 1.5 ms at 100 times. That control demonstrates the range cost; advancing a real boundary could hide unanswered mentions.

Database size grows nearly linearly. Bulk construction left up to 55.3 MB of WAL. A pinned-reader experiment left about 7.1 MB after passive checkpointing, then zero after reader release and truncation. This establishes no live WAL limit.

The [harness](../../internal/bus/growth_measurement_test.go) and [receipts](../review/bus-growth-2026-11-14.json) retain construction details, first calls, p95 values and diagnostic plans. All measurements passed in 532.287 seconds, as did the receipt checker. Go removed the temporary databases. No full suite or live operation ran.

## Storage

Result JSON and output enlarge message bodies. Deliveries and their indexes survive reading. Result keys, steering history and operation receipts also remain. Attachment bytes live outside SQLite; removing stored copies retains their metadata.

The WAL retains changed pages and reusable space; readers can delay checkpointing. [SQLite documents this behavior](https://sqlite.org/wal.html#avoiding_excessively_large_wal_files). Deletion usually leaves reusable database pages. [VACUUM](https://sqlite.org/lang_vacuum.html) reclaims space but needs temporary room and can change implicit rowids.

## Proposal for Paul

I recommend partial pending-delivery indexes, human-delivery indexes that support the unread and cursor filters, and Decisions candidates drawn from human deliveries and open requests. An incremental open-candidate set may follow if needed. [Partial indexes are documented](https://sqlite.org/partialindex.html), but these proposals have no measured improvement claim yet.

| Option | What it would lose or cost |
| --- | --- |
| Query indexes and incremental candidates | No history loss; index space, write costs and migration need measurement. |
| Idle checkpointing and controlled compaction | No history loss; readers can delay checkpoints. Compaction needs disk headroom and explicit group-member ordering before implicit rowids can change. |
| Archive settled conversations at an age Paul chooses | Online search and reply traversal need archive support. Attachments must accompany the archive. |
| Remove stored attachment copies | Attached content is lost; messages and hashes survive. SQLite hardly shrinks. This manual facility exists. |
| Truncate or delete old message and result bodies | Full transcripts, exact result replay and evidence are lost. Export first. |

Any removal must preserve pending work, unresolved decisions, uncertain steering and restart messages. Operation identities, result keys and consumed previews must still reject replay, or old retries could duplicate actions. No cutoff is selected here.
