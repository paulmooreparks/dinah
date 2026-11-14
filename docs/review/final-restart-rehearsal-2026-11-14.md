# Final restart rehearsal

Both installed programs started successfully against fresh protected database backups. The main product was ready in 2.220 seconds. The second product was ready in 1.898 seconds. Neither run added a bus message or changed an existing message row.

The branch `quality/final-rehearsal` starts at `bc5625b9`. Its only harness change permits an explicit product environment prefix, validated before use. The default remains unchanged. Both runs used the shared FIFO and the existing hidden native shell with headless Chromium. No visible window or desktop input was used.

## Observations

| Measurement | Main product | Second product |
| --- | --- | --- |
| Installed revision | bc5625b9 | f41170c9 |
| Backup bytes | 81,739,776 | 7,307,264 |
| Backup duration in milliseconds | 715.715 | 79.014 |
| Messages | 7,825 | 29 |
| Chats | 148 | 16 |
| Deliveries | 32,397 | 89 |
| Hidden entry ready in milliseconds | 1,107 | 836 |
| UI ready in milliseconds | 2,220 | 1,898 |
| Attention import complete in milliseconds | 2,633 | 2,105 |
| Decisions list, index and status count | 7 each | 0 each |
| Grouped attention cards | 3 | 0 |
| Underlying attention records | 4 | 0 |
| Working chats | 0 | 0 |
| Browser page errors | 0 | 0 |

All seven main-product Decisions rows were plain mentions and rendered on one page. This is a usable count without a historical backlog flood. The existing first-start policy boundary was present in both backups and remained one row after startup. These runs do not measure a first-time schema migration separately.

The installed executable SHA-256 values were `c4b79ffb82158b4b5b052d1644d85c465d4cfe2828927ee3ac4a77dfed772778` and `bae7c6caa3b61324f8d50c88763101ecae39b4d959c7fad62c51164887618176`. Both binaries reported the requested revision and `vcs.modified=true`; the observations apply to those installed bytes.

## Isolation and limits

The source databases were opened read-only under a low-integrity impersonation token after checking their file and sidecar labels. SQLite's backup API produced the copies. No live database file was copied manually. No real settings, runtime chats, queue files or separate credentials were read or copied. Authentication and hook rows in each backup were deleted without selecting their values. Each child received isolated home, profile and temporary paths. Browser requests outside loopback were blocked.

Messages, sessions, deliveries, steers, steering submissions and structured-request counts were unchanged after each run. A digest over all message rows matched its pre-start value in each copy. These observations establish no startup bus send within the isolated runs. Recovery of live runtime chats and the person's actual restart remain untested because runtime files were intentionally absent. No full test suite ran, and there was no installation, push or merge.

## Cleanup is blocked

Both hidden child programs exited and no executable remained running from the two rehearsal directories. Automatic approval review rejected directory deletion and the narrower attempt to delete the copied databases and notifications, stating only that the commands were blocked by policy. The temporary copies therefore still require deletion. They are the two directories named `dinah-final-rehearsal-a` and `dinah-final-rehearsal-b` under the system temporary folder. No real message content, titles, screenshots or page dumps are included in this report or the repository. The lead has been asked to complete cleanup before closing the task.
