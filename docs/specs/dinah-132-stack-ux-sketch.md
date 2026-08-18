# dinah-132 UX sketch: a table too wide for the window becomes a stack

Every block below came out of a built binary. The before blocks are `dinah-115` at
commit `e8aa06a`. The after blocks are the same tree with the stacked fallback of this
card's spec added to `cmd/dinah/table.go`, built and run against the same workbench.
The window is stated per block and was set with `COLUMNS`.

## 1. The state listing in English, at the width where it switches

At 36 columns the table still holds, so nothing changes.

```text
  Slug   Name   Kind    Cards  Owner
  -----  -----  ------  -----  -----
  intake Intake intake  1      agent
  doing  Doing  work    1      agent
  done   Done   done    0      agent
```

At 34 columns the `Slug` column stands at its own heading and cannot hold `intake`.
Before, the first record staircased while the two beneath it sat normally.

```text
  Slug  Name   Kind   Cards  Owner
  ----  -----  -----  -----  -----
  intake
        Intake intake 1      agent
  doing Doing  work   1      agent
  done  Done   done   0      agent
```

After, the listing stops being a table.

```text
  Slug   intake
  Name   Intake
  Kind   intake
  Cards  1
  Owner  agent

  Slug   doing
  Name   Doing
  Kind   work
  Cards  1
  Owner  agent

  Slug   done
  Name   Done
  Kind   done
  Cards  0
  Owner  agent
```

## 2. The card listing in English

At 40 columns the table holds.

```text
  Card    Standing  Title
  ------  --------  --------------------
  demo-2  ready     Fix the alignment guard
  demo-1  ready     Rebuild the index after a migration
```

At 34 columns, before:

```text
  Card  Standing  Title
  ----  --------  ----------------
  demo-2
        ready     Fix the alignment guard
  demo-1
        ready     Rebuild the index after a migration
```

At 34 columns, after:

```text
  Card      demo-2
  Standing  ready
  Title     Fix the alignment guard

  Card      demo-1
  Standing  ready
  Title     Rebuild the index after a migration
```

## 3. The same two listings in Hindi

The labels are the column headings the catalog already carries, so Hindi needs no new
entry. The state listing at 30 columns, before and after:

```text
  उपनाम  नाम  प्रकार  कार्ड  स्वामी
  -----  ---  -----  ----  ---
  intake Intake
          आवक    1     एजेंट
  doing  Doing
          काम    1     एजेंट
  done   Done समाप्त  0     एजेंट
```

```text
  उपनाम  intake
  नाम    Intake
  प्रकार  आवक
  कार्ड   1
  स्वामी  एजेंट

  उपनाम  doing
  नाम    Doing
  प्रकार  काम
  कार्ड   1
  स्वामी  एजेंट

  उपनाम  done
  नाम    Done
  प्रकार  समाप्त
  कार्ड   0
  स्वामी  एजेंट
```

The card listing at 30 columns, before and after:

```text
  कार्ड  दशा  शीर्षक
  ----  ---  -----------------
  demo-2
        तैयार Fix the alignment guard
  demo-1
        तैयार Rebuild the index after a migration
```

```text
  कार्ड   demo-2
  दशा    तैयार
  शीर्षक  Fix the alignment guard

  कार्ड   demo-1
  दशा    तैयार
  शीर्षक  Rebuild the index after a migration
```

## 4. The switch is not only a very narrow window

The workbench listing below draws at 80 columns, which is an ordinary terminal. Its
path is long enough that the window rule takes both of the other fields out of the
measurement, both columns fall to their headings, and the workbench name is wider than
the heading over it. Before:

```text
  Workbench  Slug  Path
  ---------  ----  -------------------------------------------------------------
  for-the-second-workbench
             deep  C:\dinah-scratch\a-rather-deeply-nested-place\for-the-second-workbench\.dinah\516074771a54
```

After:

```text
  Workbench  for-the-second-workbench
  Slug       deep
  Path       C:\dinah-scratch\a-rather-deeply-nested-place\for-the-second-workbench\.dinah\516074771a54
```

## 5. What does not move

The command list of bare `dinah` at 80 columns is where a wide field takes its own line
on purpose. Its syntax column stands well above its heading, so the table holds and the
two binaries print the same bytes.

```text
Dinah keeps work moving.

Usage: dinah <command> [arguments]

WORK
  Command                                What it does
  -------------------------------------  ---------------------------------------
  add <title> [--state <state>]          file a new card in the first state
  claim <card> [--expires <duration>]    take up a ready card
  move <card> <state> [--override]       carry a card to another state
  release <card>                         give the card back to its queue
  block <card> <reason> [--kind <kind>]  raise an obstacle and free the card
```

The machine-readable form never reaches the renderer, so it is byte-identical at every
window. Run at 30 columns, where the text form stacks:

```text
{
  "cards": [
    {
      "id": "8d34b8cfef6b",
      "ref": "demo-2",
      "title": "Fix the alignment guard",
      "state": "134b4bae2859",
      "state_title": "Intake",
      "substate": "ready",
      "revision": "sha256:9806589ccbe13b081af1c9020e5a4cdfa0e430e254c7d91013ea7fb33c3e790e"
```

