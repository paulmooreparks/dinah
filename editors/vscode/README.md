# Dinah for VS Code

Dinah is a single-binary CLI that carries cards through a workbench: claim,
move, release, block, unblock. This extension puts that workbench in the
sidebar, so you can see which workbench the folder you have open resolves to,
which cards stand where, and which `dinah` binary the editor is talking to.

## What it gives you

The Dinah view lists the workbench's columns and the cards standing in each of
them. Selecting a card opens it. The view's commands run the same verbs the CLI
runs, so claiming a card here and claiming it from a terminal are the same act
against the same files.

- Claim, Move, Release, Block and Unblock, on the selected card.
- View Served Instructions, which opens the instruction chain the workbench
  serves for where that card stands as a read-only Markdown tab, and keeps that
  tab current while it is open.
- Check and Copy Path, on the workbench itself.
- Refresh, for when you would rather not wait for the poll.

## Requirements

You need the `dinah` binary. The extension finds it on your PATH, or you can
name it yourself with the `dinah.path` setting. `dinah.workbench` chooses a
workbench when the folder resolves to more than one, `dinah.pollIntervalSeconds`
sets how often the view re-reads, and `dinah.watchFiles` turns file watching on
and off.

Install the CLI from the project's own instructions at
https://github.com/paulmooreparks/dinah#install.

## The version numbers here and the version numbers there

This extension's version number and the `dinah` CLI's version number are
unrelated by design. They count different things on separate cadences, and
neither one is computed from the other, so comparing them tells you nothing.
Version 1.0.0 of this extension does not pair with version 1.0.0 of the CLI,
and a CLI still numbered 0.x is not behind an extension numbered 1.x.

What does tell you whether an installed extension and an installed binary
belong together is the profile revision, which is the `profile` field
`dinah --json version` publishes. That field names the command and storage
contract the binary speaks, and it moves when the contract moves rather than
when either project cuts a release. The extension already checks it on every
binary it considers, refusing one whose profile is older than the fields the
extension reads, so a mismatch surfaces as a message about the profile rather
than as a wrong answer.

Every version of this extension sits on one line, and the number by itself does
not say whether an archive was published as a pre-release. The marketplace
listing says so, because a pre-release is marked as one there. No archive this
repository publishes carries that mark, so every published version installs as
a release and asks nobody to opt in.
