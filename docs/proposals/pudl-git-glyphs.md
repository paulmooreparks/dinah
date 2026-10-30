# Proposal: Version-Control Glyphs for PUDL

Dinah asks PUDL for the glyphs a version-control interface needs and PUDL does not have. The Git extension draws stand-ins from PUDL's set today, which PUDL Development accepted on 25 October 2026 for as long as each has its label or tip. This is a proposal for Paul to hand off; nothing in PUDL is changed by it.

## The Need

On 28 October PUDL Development asked for "the list of what's missing, such as commit, merge, tag and stash", noting that PUDL already has a branch glyph. I checked the list against PUDL 0.89.5, which is the release Dinah runs and whose glyph set is the 80 SVG files in specification 0.46.3 (0.90.0 changed toasts only). The check found one thing to settle before the list itself.

PUDL's `branch` glyph is an elbow, a vertical stroke turning into a horizontal one, and chapter 5 defines it as "a child window's row beneath its parent in a list". It is the connector of a tree, and chapter 6 turns it round in a right-to-left page for that reason. The Git extension uses it for Git branches in six places: the branch rows in the pane, the branch button in the pane's head, the list of branches that button opens, the Branches view's grid, the chosen branch's heading and the Switch Branch command. Nobody reading it there sees a line of work splitting off, and an application that also lists child windows beneath their parents would be using one glyph for two meanings. So I count a version-control branch as missing too.

The other uses are the ones named in the request. A commit, a merge, a tag and a stash appear in the Git extension's pane, its window and its menus, and the same ideas exist in every version-control tool and in any application that keeps a document's history, such as a wiki's page history or a design file's versions.

## What Is Asked

Eight new glyphs, drawn on the 16-unit grid with the stroke weight and round caps of `inbox`, `reload` and `download`, each with its meaning in chapter 5. I have written each meaning so that it does not depend on Git, since chapter 5's other glyphs mean kinds of thing and kinds of action rather than one product's features.

| Glyph | Meaning | Suggested drawing | Where Dinah would use it | Dinah's stand-in today |
|---|---|---|---|---|
| `commit` | A saved revision of a body of work, such as a commit or a version | A ring on a horizontal line through its centre | The commit box's Commit button, the History tab of the Git window, Show Commit in the menus of tags and refs, a commit's heading in its detail | None. Commit rows and the Commit button have no glyph, and the history graph draws its own dots |
| `fork` | A line of work that splits from another, such as a branch in version control | Two nodes on a vertical line with a third joined to it by a curve | Branch rows, the branch button, the Branches view, Create Branch and Switch Branch | `branch`, whose meaning is a child row |
| `merge` | Two lines of work joined into one | Two lines from the top meeting in one that runs on downward | Merge Into in a branch's menu, a merge commit's row, the notice of a merge in progress with conflicts | None. The commands are words alone |
| `tag` | A label fixed to one thing to find it again, such as a release tag or a note's tag | A luggage tag with its hole | The Tags group and its rows, Create Tag, Push Tag | `ticket`, whose meaning is an issue or task in a tracker |
| `stash` | Work set aside, to be taken up again later | A box with an arrow going down into it | The Stash button on the pane's toolbar, the Stashes group and its rows, Stash in the menus | `inbox`, whose meaning is items waiting to be dealt with |
| `history` | The earlier states of something, in time order | A clock face with an arrow running anticlockwise round it | The History tab, File History in a file's menu | None |
| `compare` | Two versions shown together to see what changed | Two pages side by side with a two-headed arrow between them | Open Diff on files and commits, the Editor's Diff window in the taskbar and its title | `window`, whose meaning is a window's menu |
| `upload` | Send something to a place elsewhere, the counterpart of `download` | `download` turned round: the arrow leaving the tray upward | Push on the pane's toolbar, the Git menu's Push, Publish Branch | `arrow-up`, whose meaning is moving something up, such as a step in a list |

Every one of these will also appear at 11px beside a status letter or a count, so each needs to stay readable at that size as well as at 14px in rows and toolbars.

## Stand-ins Dinah Would Keep

These stand-ins already match their documented meanings well enough, and Dinah does not ask for replacements.

| What | Glyph | Why it fits |
|---|---|---|
| Fetch | `reload` | Fetching loads the remote's state again without changing the reader's work |
| Pull | `download` | It brings work from elsewhere into the reader's copy, if PUDL agrees to widen `download` from "download a file" to "bring something here from elsewhere" |
| Ahead and behind counts | `arrow-up`, `arrow-down` | They show a direction beside a number, which is close to what the arrows mean |
| A remote | `link` | A remote is a link to somewhere else |
| A worktree | `folder` | A worktree is a folder |
| The Command Log | `script` | It is a record of commands, which `terminal` or `script` both suit |
| A conflicted file | `warning` | A conflict needs attention, which is the `warn` status |
| An untracked file | `file` | It is a file of no particular state yet |
| Added and deleted files | `plus`, `minus` | The same pair the diff proposal uses for its signs |
| Take Ours and Take Theirs | `back`, `forward` | They choose one side or the other in a conflict |

One file status still has a poor stand-in. A renamed file is marked with `forward`, whose meaning is going on to the next page or item. A `move` glyph meaning "moved from one place to another", such as a renamed file or a file moved between folders, would serve file managers as well as Git, and I list it here as a lower priority than the eight above.

Cherry-pick, rebase, revert, reset and blame appear only as words in menus. PUDL does not require a glyph on a menu row, so they are not part of this request.

## What Stays With the Application

- The application decides which command or row gets which glyph, and whether a toolbar shows the glyph alone or with its words.
- The application draws the commit graph. Its dots, lines and curves are data drawn in an SVG with the categorical palette proposed alongside this one, outside the set of chrome glyphs.
- The application writes the words of every label and tip, which the stand-ins rely on now and the new glyphs will still need.

## Questions for PUDL

1. Should the version-control branch be a new glyph beside `branch`, as proposed, or should the existing elbow be renamed `child` and `branch` given the new meaning? Renaming is clearer in the long run, but `--glyph-branch` is in the contract and other applications may use it, so it would need a major release or a period with both names.
2. Is `fork` the right name? It reads well for the shape, though GitHub uses "fork" for a copy of a whole repository.
3. May `download` widen to cover Pull, or should Pull have a glyph of its own?
4. Would PUDL like Dinah to draft the SVGs on the 16-unit grid for review, or does PUDL prefer to draw them itself?

## How Dinah Would Adopt It

The Git extension sets each glyph through PUDL's `.glyph` element and a `--glyph` token, in the pane, the window and the command list it gives the host, so adopting the new glyphs means changing one name per use: `branch` to `fork` in six places, `ticket` to `tag`, `inbox` to `stash`, `window` to `compare` on Open Diff, `arrow-up` to `upload` on Push, and adding `commit`, `merge` and `history` where the table above shows them. Dinah's Editor would give its Diff windows the `compare` glyph in the taskbar through `data-win-glyph`. Both would move to the PUDL release that includes the glyphs as soon as it is out, and every glyph keeps its label or tip.
