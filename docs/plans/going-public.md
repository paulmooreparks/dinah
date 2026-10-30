# Making Dinah Public

Paul decided on 30 October 2026 that Dinah goes public soon. Before it does, every mention of the private distribution that was built from Dinah, and of that distribution's client, leaves Dinah's files, including neutral ones. The public repository then starts from a fresh history, and today's history stays in the private repository.

## What Is Being Removed

A survey on 30 October found no web addresses, credentials or business code from the private distribution anywhere in Dinah's history. What remained was names and descriptions, in about 210 places across 40 files:

- the plan for building the distribution from Dinah, which is about nothing else, and passages in the package, chat Markdown and workspace export designs that say how the distribution differs or ports a feature;
- passing mentions in DESIGN.md and several plans;
- test data that uses a client's name as a sample workspace, and that client's document services as sample sources;
- a guard in the agent command policy that refuses the distribution's own command-line programs by name, and a few code comments.

The distribution's plan and its compatibility notes move to the distribution's own repository, which keeps the record of what it takes from Dinah. Dinah keeps the general mechanisms that the distribution uses (the profile, the package kinds and sources, the registration points), described without reference to it.

## How It Is Done

1. **The guard comes from the profile.** The list of command-line programs whose credential and flow commands an agent may not run is part of the build's profile. Dinah's profile lists `dinah`, and another build adds its own. This follows the rule already applied to providers, that a name lives in one place that owns it (Engine).
2. **Test data uses invented names.** The sample workspaces, paths and sources in Go and browser tests take neutral names that belong to nobody (Engine for Go, Front End for the page).
3. **The documents are rewritten,** each passage either removed or made general (Architecture).
4. **A check keeps the names out.** A Go test reads a list of forbidden terms and fails when any appears in a tracked file. The list itself would be a mention, so it is kept outside the repository, in a file the gates point to through an environment variable, and the test is skipped when the variable is not set. Dinah Release's gates always set it (Engine builds the test).
5. **The new history.** When the tree is clean, merged and through the gates, the public repository starts from one commit that holds that tree. The private repository is renamed to keep the full history as an archive and stays private. Paul takes the step of making the repository public, after checking the list below.

Everything else on the list was finished on 30 October, and Paul asked the same evening for the new history to start at the next quiet moment. Architecture does it in these steps.

1. The team stops. Each builder commits what it has on its own branch and starts nothing, and Dinah Release finishes any gate that is running.
2. The forbidden-terms check passes on master with the list set.
3. A new root commit is made with master's tree and the message "Dinah", and master is moved to it. Each work branch's commits that are not yet on master are replayed onto the new root, so unfinished work is carried across rather than lost.
4. On GitHub, the private repository `paulmooreparks/dinah` is renamed `dinah-history` and stays private, and a new private `paulmooreparks/dinah` is created. Master and the work branches are pushed to it, and the local clone keeps the archive as a second remote called `history`.
5. Each builder resets its worktree to its branch on the new root and carries on.

Commit references in these plans that are older than the new root point into the archive, which keeps the full history. The old bundle of the history from before 22 October stays where it is and is never pushed.

## Before Publishing

Paul checks these himself before the repository is made public, since they are his to judge rather than a test's.

- No other client's or employer's material, and no personal paths, addresses or notes that should stay private. `docs/dinah-ideas.md` is the person's working notes and is reviewed with that in mind.
- The licence is Apache 2.0, as ruled on 27 October, and the README says what Dinah is.
- Nothing from the history archive, and never the old bundle of the history from before 22 October.

## Done

The new history began on 30 October 2026 with the root commit ee241e1, and Paul made github.com/paulmooreparks/dinah public the same evening. One mistake on the way was caught before then. A push sent three branches of the old history to the new repository, so it was renamed aside and replaced by a clean one, and Paul deleted the affected copy before publishing. The full history stays in the private repository dinah-history. Everything pushed to the public repository now goes through the forbidden-terms check, and only branches whose root is ee241e1 are pushed.
