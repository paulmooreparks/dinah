# Responsiveness timing snapshot

This draft records the preparation authorized in messages 4982 and 4983, incorporating the agreement in 4986 and 4987. It is based on master 1b4db72486de04404f7b20876f22b27cd0234e69. Engine owns the backend recorder, bridge instrumentation and shell read contract. Front End owns browser recording and Copy Timing Snapshot. Quality reviews this contract before implementation. No push, installation, restart or release exception follows from this draft.

## Recording and export

Record Responsiveness remains the single control. While it is enabled, browser and backend timing collectors retain fixed-capacity, in-memory data. Turning it off disconnects browser observers, stops sampling timers, clears all timing storage and increments the recording generation. The panel must say next to the control that turning recording off clears the captured timings. Copy Timing Snapshot remains available while recording is on.

Copy runs only on the person's button action. It reads the backend once, builds the closed schema below, validates the result, and copies a fixed explanatory header followed by JSON to the clipboard. It sends nothing and writes no file. There is no polling, automatic export, log dump or persistence of recorded timings. The existing preference may persist that recording is enabled; the timings themselves do not survive a process or page session.

The header is: "Dinah timing snapshot. Contains timings, counts and clock anchors only. It records when retained input events, such as key presses, happened and how long they waited. Omits conversation content, identities, paths and commands. Process-wide runtime totals do not identify the cause of an individual stall."

## Shell methods and ordering

The authenticated shell bridge exposes these methods only to its first-party page:

| Method | Input | Result |
| --- | --- | --- |
| SetResponsivenessRecording | enabled: boolean | RecordingState |
| GetResponsivenessTimingSnapshot | no arguments | BackendSnapshot |

The methods have pageQuery classification because their lifetime must not obstruct exit. They must be absent from applet manifests, bus tool lists and contributed iframe APIs. Existing sandboxed extension frames cannot acquire the shell's authority through their contribution API. This contract makes no isolation claim about trusted code already executing in the shell's own JavaScript realm.

Front End serializes recording-control requests and retains the latest desired state while a request is outstanding. A late response cannot restore an earlier intent. Repeating the current state is idempotent. Every actual enable/disable transition increments a process-local unsigned generation counter. Enable begins fresh storage and a new clock anchor. Disable clears storage. A snapshot carries that generation, and the page refuses to combine it with a browser generation that has since changed. It tells the person to copy again rather than relabeling old data. Get does not enable recording or probe any provider.

The page's own generation is also monotonic for its lifetime. It associates the acknowledged backend generation with the browser recording generation; these counters need not have equal numeric values. A copy begun during an outstanding control transition waits for that transition to settle or reports that recording changed. It does not silently export a mixed recording.

## Bounds and collection

| Constant | Value |
| --- | --- |
| AggregatePeriodMs | 1000 |
| AggregateCapacity | 600 slots per source |
| RetentionMs | 600000 |
| BrowserRawCapacity | 200 records in each of input, work and frame rings |
| BrowserRawWritesPerSecond | 100 per ring |
| BackendCategoryCount | 6 |
| BackendOwnedStorageLimit | 524288 bytes |
| CombinedExportLimit | 4194304 UTF-8 bytes, including header |
| CopyDeadlineMs | 10000, covering preparation and initiation of clipboard writing |

Both aggregate rings hold ten minutes of one-second slots. Each category retains count, total duration, maximum duration and the maximum's completion offset within its slot. An event is assigned to its completion slot. Its start can precede that slot; the maximum duration and completion offset preserve that fact. Long work must not be averaged away. Count and total use saturating arithmetic with an explicit saturation flag; they never wrap silently.

Browser raw samples expire after ten minutes and overwrite their fixed rings at capacity. Rate-limited or overwritten raw samples increment omission counters. Aggregate counts, totals and maxima still include observations omitted from the raw rings. Arbitrary render labels, script locations and unknown categories map immediately to the fixed other category. They are never retained as strings. No view identity is needed, so this version exports no chat/window labels, ordinal or otherwise.

Backend event updates use fixed arrays and numeric categories, without an event queue, per-event allocation or dynamic label map. One sampler runs at most once a second while enabled. Its records include missed-period counts so delayed sampling is visible. Backend owned arrays and metric buffers must fit BackendOwnedStorageLimit, measured from their concrete types. Browser storage is bounded by fixed record capacities and fixed-shape numeric records; no claim about a JavaScript engine's object byte layout is made. The implementation must demonstrate stable retained storage after repeated wraps and under floods.

Disabled hot paths perform only the disabled-state check. They do not read clocks, sample runtime metrics, allocate event records or acquire the recorder mutex. Enabled collection must not invoke page callbacks, bridge calls, logging or disk I/O from a recorder lock. Export returns a bounded copy, and serialization happens after releasing that lock. A combined export exceeding CombinedExportLimit fails with a fixed message and does not copy partial data.

## Closed export schema

Every object below rejects additional fields. Every listed field is required unless explicitly nullable. Arrays have the bounds stated here. Enum values are fixed literals, never producer-provided strings. All numeric values must be finite and nonnegative. No general metadata, error, labels or attributes object is permitted.

MeasuredNumber is `{state, value}`. State is measured, unavailable or not_supported. Value is a finite number only for measured and is null otherwise. A measured count or total may be zero. A maximum with no observed event is unavailable. An unavailable metric must never become measured zero.

Clock is `{utc, monoMs}`. UTC is an RFC 3339 timestamp and monoMs is a numeric monotonic offset in milliseconds. Backend monoMs uses time.Since from its recording anchor. Browser monoMs uses performance.now with its own anchor. Each source records a simultaneous UTC/monotonic pair and retains that pair for reconstruction. UTC changes do not alter monotonic durations. The two sources' UTC alignment is approximate, with bridge transit included in the copy operation's measured request span.

Span is `{anchor, startMonoMs, durationMs, complete}`. Anchor is Clock, durationMs is monotonic, and complete is boolean. Every retained raw or aggregate record has its own Span, so it carries both its duration and UTC anchor. A partial current slot has complete=false. A source suspended while the document is hidden must report the uncovered interval through its omission counters and complete=false rather than manufacturing measured zeros.

DurationStats is `{count, totalMs, maxMs, maxCompletedOffsetMs, saturated}`. The four numeric fields are MeasuredNumber; saturated is boolean. maxCompletedOffsetMs is relative to the slot start and below AggregatePeriodMs. A zero-count slot has measured zero count/total and unavailable maximum/offset. The same event supplies maxMs and its offset.

| Object | Exact fields and types |
| --- | --- |
| Export | schemaVersion: integer 1; copiedAt: Clock; copyRequest: Span; browser: BrowserSnapshot; backend: BackendSnapshot |
| RecordingState | enabled: boolean; generation: integer; anchor: Clock; state: on or off |
| BackendSnapshot | schemaVersion: integer 1; recording: RecordingState; capturedAt: Clock; coverage: BackendCoverage; slots: BackendSlot array, maximum 600 |
| BackendCoverage | retainedSlots: integer; overwrittenSlots: integer; missedPeriods: integer; disabledObservations: MeasuredNumber |
| BackendSlot | time: Span; categories: BackendCategories; queues: QueueStats; runtime: RuntimeInterval |
| BackendCategories | agent_text, agent_tool, chat_state, team_state, bus_change, other: each CategoryStats |
| CategoryStats | appEmit: DurationStats; bridgeEnqueue: DurationStats |
| QueueStats | subscriberPeak: MeasuredNumber; depthPeak: MeasuredNumber; overflowDisconnects: MeasuredNumber |
| RuntimeInterval | scope: process_wide_interval; time: Span; gcPauseCount: MeasuredNumber; gcPauseMaxBucketLowerMs: MeasuredNumber; gcPauseMaxBucketUpperMs: MeasuredNumber; gcPauseUpperOpen: boolean; mutexWaitMs: MeasuredNumber |
| BrowserSnapshot | schemaVersion: integer 1; generation: integer; enabled: boolean; anchor: Clock; capturedAt: Clock; features: BrowserFeatures; coverage: BrowserCoverage; slots: BrowserSlot array, maximum 600; raw: BrowserRaw |
| BrowserFeatures | eventTiming, longTasks, longAnimationFrames: each measured, unavailable or not_supported |
| BrowserCoverage | retainedSlots, overwrittenSlots, suspendedPeriods: integers; inputRateOmissions, inputWrapOmissions, workRateOmissions, workWrapOmissions, frameRateOmissions, frameWrapOmissions: integers |
| BrowserSlot | time: Span; inputDelay: DurationStats; renderWork: DurationStats; renderWait: DurationStats; longTask: DurationStats; longFrame: DurationStats |
| BrowserRaw | input: InputRecord array, maximum 200; work: WorkRecord array, maximum 200; frames: FrameRecord array, maximum 200 |
| InputRecord | time: Span; category: input_delay; delayMs: MeasuredNumber; processingMs: MeasuredNumber; eventDurationMs: MeasuredNumber |
| WorkRecord | time: Span; category: chat, layout_followup, unread, diagnostics, long_task or other; durationMs: MeasuredNumber; waitMs: MeasuredNumber |
| FrameRecord | time: Span; category: long_frame; durationMs: MeasuredNumber; blockingMs: MeasuredNumber; forcedStyleLayoutMs: MeasuredNumber |

BackendCoverage.disabledObservations is unavailable while disabled, because the disabled hot path deliberately counts nothing. It is not a claim that no work occurred. When recording is off, both sources return empty timing arrays and current disabled state. No old timing remains accessible.

The backend maps the known text/tool/chat/team/bus event classes to the six fixed categories. All other names, including malicious or unexpected names, become other. App emit duration covers local app fan-out through bridge enqueue completion. Bridge enqueue duration covers serialization, bridge locking and subscriber enqueue work. These figures do not establish when a browser received, executed or displayed the event. QueueStats describes the bounded bridge queues at observation time.

RuntimeInterval is displayed separately as process-wide totals over its actual sampling interval. The GC fields come from deltas in the documented runtime histogram; the bounds describe a bucket, not an exact individual pause. An open upper bucket has an unavailable upper value and gcPauseUpperOpen=true. With no GC pause, count is measured zero and maximum bounds are unavailable. Mutex wait is the documented approximate cumulative wait delta across goroutines and runtime locks. Neither field is attribution to a particular input or frame stall. Unsupported metric names or incompatible histogram layouts produce not_supported. An initial sample without a baseline is unavailable.

Browser features are detected before observer setup. Failure to observe a supported type is unavailable. Unsupported entry types are not_supported. Script names, URLs, source positions, function names, targets, input values and entry detail objects are excluded even when the platform provides them. Feature presence does not guarantee that every qualifying browser event was reported. Values omitted by rate limits remain counted as omissions.

## Privacy and reachability guards

The implementation must validate every exported field against this schema. Canary tests seed messages, prompts, titles, paths, provider output, URLs, command lines and arbitrary event/render labels. None may reach the copied text. Unknown properties and enum values must fail validation. The export must contain no persistent identity or free-form diagnostic error string.

Guards must prove fixed capacity after repeated wraps, bounded behavior under event floods, raw rate limits with aggregate maxima preserved, clearing on disable, generation-safe late responses and absence of automatic reads/exports. Reachability guards check applet manifests, bus tool lists and contributed iframe APIs. They also retain the explicit limitation for code already trusted inside the shell realm.

## Overhead gate and deployment

Recording on/off runs use the retained benchmark at zero, one and four streams, with at least three paired runs per load. The admissible increase in median script/style time is the larger of five percent of the disabled baseline or five milliseconds per ten seconds. Median key-to-frame p95 may increase by at most two milliseconds. Report every pair, variance, capture overhead and method. If variance cannot resolve a ceiling, the gate is unproven and cannot be relaxed into a pass. Disabled event instrumentation must allocate no per-event storage, and its hot-path benchmark must show no resolved regression beyond the same noise limits.

The backend must also measure enabled event-update and one-second sampler cost on synthetic canary data, including the disabled path and ring wrap. Browser retained-storage measurements accompany its bounds tests. These checks prove recording overhead and behavior; they are not evidence of relief for Paul's live stall.

Quality reviews the contract, schema, privacy, bounds and overhead, then reviews each implementation branch. The current manual panel capture remains the immediate supported route. This addition requires a reviewed build and normal deployment, with a coordinated restart before its backend measurements exist in the running desktop. Preparation authorizes none of those release actions and bypasses no WPR policy.

V1 uses passive Event Timing, Long Tasks and Long Animation Frames observations. It adds no requestAnimationFrame sampler or frame-interval metric. Event Timing duration is the browser-reported duration rounded to eight milliseconds, and the lowest observer durationThreshold is sixteen milliseconds. Faster events are missing by design; no count of those unreported events can be inferred, and they are never filled in as zero. InputRecord.eventDurationMs preserves that reported total separately from input delay and processing time. FrameRecord.forcedStyleLayoutMs is the sum of the scripts' forcedStyleAndLayoutDuration fields when all required fields are exposed; otherwise it is unavailable. It is not total browser style cost. Script objects and their identifying fields are never exported.

The browser limits follow the [Event Timing working draft](https://www.w3.org/TR/event-timing/) and the feature-detected [Long Animation Frames API](https://www.w3.org/TR/long-animation-frames/). The latter remains a working draft; unavailable or changed fields must produce explicit unavailable values.

CopyDeadlineMs starts at the person's button press and bounds waiting for control acknowledgment, reading and validating the backend, and initiating the clipboard write. A deadline observed before initiation expires that Copy; late control or read completions cannot initiate a write or change its result. The underlying control queue remains serialized until the request settles. Immediately after serialization and before submission, Copy checks both its monotonic deadline and recording generation again. Once clipboard writing has been submitted, the documented asynchronous clipboard API offers no cancellation guarantee. The UI says "Timing copy submitted; waiting for clipboard" and does not claim a definite failure or cancellation while writing can still complete. It prevents competing writes until that operation settles. Recording changes cannot revoke an already submitted write. A blocked browser thread can delay timer delivery, so the deadline is a logical cutoff rather than a promise of a screen update at exactly ten seconds. The [Clipboard API contract](https://www.w3.org/TR/clipboard-apis/#dom-clipboard-writetext) supplies no abort signal for writeText. Guards cover late reads, a new Copy after preparation timeout and clipboard completion racing the deadline.
