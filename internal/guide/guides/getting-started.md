# Getting started

A workbench is a directory of plain-text files. Everything Dinah knows about
your work lives there, so you can read it with an editor, search it with grep,
and put it under git alongside the project it belongs to.

You may create a workbench in the same directory as the rest of your work:

```
dinah init --slug proj --operator alka
```

Every card in a workbench has a human-readable prefix, called a slug. A
workbench with slug `proj` names its first card `proj-1`. If you leave
`--slug` out, Dinah derives one from the directory name.

Every workbench designates an operator, the person who owns it and answers for
it. Only the operator lifts a block, so a workbench with nobody in that seat
has actions nobody can perform. If you leave `--operator` out, Dinah records
whoever you are acting as.

Dinah writes the workbench inside a `.dinah` directory here rather than loose
in your working directory, so it always sits somewhere later commands can find
it. It prints the path it wrote, `.dinah/<id>/`, where `<id>` is a generated
identifier; open the `workbench.md` inside it to read or edit the prose that is
served to whoever claims a card. Nothing copies that file anywhere, so anyone
who opens the workbench sees your edit right away.

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
want a different one. When the walk finds nothing it falls back to your user
base at `~/.dinah`, and `DINAH_HOME` moves that base somewhere else, so a test
run or a scratch tree can work without touching your own settings and
workbenches.
