# Getting started

A workbench is a directory of plain-text files. Everything Dinah knows about
your work lives there, so you can read it with an editor, search it with grep,
and put it under git alongside the project it belongs to.

Create one where the work is:

```
dinah init --slug proj --operator alka
```

The slug is what a card reference is built from, so `proj-1` names the first
card you file; leave it out and Dinah derives one from the directory name. The
operator is the owner who answers for the workbench, and every workbench
designates one, because blocks are lifted by the operator alone and a
workbench with nobody in that seat has acts nobody can perform. Leave it out
and Dinah records whoever you are acting as.

That writes `workbench.md`, which carries the flow and the standing
instructions, and a `states/` directory holding one file per station. Open
`workbench.md` and edit the prose; it is served to whoever claims a card, and
nothing copies it anywhere, so an edit reaches every reader at once.

File the first piece of work:

```
dinah add "Write the release notes"
```

The card lands in the first state of the flow with substate `ready`, which
means anybody may pull it. Ask what is waiting:

```
dinah next
dinah ls
```

Take one up, do the work, and carry it on:

```
dinah claim proj-1
dinah move proj-1 doing
dinah release proj-1
```

Every one of those commands prints the instructions of the position the card
now sits at, together with the moves the flow allows it. Pass `--quiet` when
you have read them already.

Dinah discovers its workbench the way git discovers a repository, by walking
up from where you are. Set `DINAH_WORKBENCH` or pass `--workbench` when you
want a different one.
