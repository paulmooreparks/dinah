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

Dinah also offers every workbench this window has found to the editor's agent
mode, as an MCP server the editor can start on your behalf. Your own agent
session then reaches that workbench without you editing a configuration file.
Nothing starts by itself: VS Code asks you to trust a server before it starts
one for the first time, and nothing here writes to your settings or to an
`mcp.json` file. If you would rather nothing were offered, set
`dinah.registerMcpServer` to false.

The extension also runs Dinah's language server over the markdown files in
your folder, so a workbench's twelve-hex identifiers read back as names. A
card reference carries the card's title and the column it stands in, beside
the identifier and not instead of it, and hovering one tells you the same
thing at more length. Clicking through opens what the reference names. The
labels are live. The server rereads the workbench rather than remembering what
it last saw, so a card somebody moved a moment ago reads as moved. Turn the
whole of it off with `dinah.lsp.enabled`.

## Requirements

You need the `dinah` binary. The extension finds it on your PATH, or you can
name it yourself with the `dinah.path` setting. `dinah.workbench` chooses a
workbench when the folder resolves to more than one, `dinah.pollIntervalSeconds`
sets how often the view re-reads, `dinah.watchFiles` turns file watching on and
off, and `dinah.registerMcpServer` turns the MCP server offer on and off.

Four more settings belong to the language server. `dinah.lsp.enabled` turns it
on and off, `dinah.lsp.annotateProse` draws the inline label in prose as well
as in front matter, `dinah.lsp.pollIntervalSeconds` sets how often the server
rereads the workbench, and `dinah.lsp.trace.server` writes the server's own
wire into an output channel.

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
