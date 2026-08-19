# Getting started

Dinah keeps a workbench as a directory of plain-text files. You can read
everything Dinah tracks about your work there with an editor, search it with
grep, and put it under git alongside the project it belongs to.

You may create a workbench in the same directory as the rest of your work:

```
dinah init --slug proj --operator alka
```

Every card in a workbench has a human-readable prefix, called a slug. With
slug `proj`, Dinah names your workbench's first card `proj-1`. If you leave
`--slug` out, Dinah derives one from the directory name.

Dinah assigns an operator to every workbench, the person who owns it and
answers for it, and only that operator may lift a block, so a workbench with
nobody in that seat has actions nobody can perform. If you leave
`--operator` out, Dinah records whoever you are acting as, called the actor;
you carry that name on every action you take, and Dinah prints it back when
you run `dinah whoami`.

If Dinah does not already know who you are, it records the operator you named
as your actor when it creates the workbench, and it prints the file it wrote
that name to. Dinah does not touch a name you have already given it with
`--actor`, with the `DINAH_ACTOR` variable, or with `dinah config set actor`.
You do not have to set an actor before you file your first card. To change the
name later, run `dinah config set actor <name>`, and to remove it, run
`dinah config set actor` with no name.

Dinah writes the workbench inside a `.dinah` directory here rather than loose
in your working directory, so it always sits somewhere later commands can find
it. Dinah prints the path it wrote, `.dinah/<id>/`, where `<id>` is a
generated identifier. Open the `workbench.md` file inside it to read the
prose that is served to whoever claims a card. You edit that same file
directly, so anyone who opens the workbench sees your change right away, and
you get a copy of it only when you ask for one.

File the first piece of work:

```
dinah add "Write the release notes"
```

Dinah lands the card in the first state of the flow with substate `ready`,
which means anybody may pull it. Ask what is waiting:

```
dinah next
dinah ls
```

If you want the whole workbench at once rather than one state at a time, ask
for the tree:

```
dinah tree
```

Dinah nests every card under the state it sits in and then under whether it is
ready, active, or blocked, and it counts each group for you. You can nest along
something else with `--group-by`, and `dinah tree --group-by holder` then shows
you who is sitting on what. You can also narrow the tree with the same query
`dinah query` takes, so `dinah tree "substate:blocked"` draws only the blocked
cards and tells you, group by group, how many it left out.

Take one up, do the work, and carry it on:

```
dinah claim proj-1
dinah move proj-1 doing
dinah release proj-1
```

Dinah prints the instructions of the position the card now sits at after each
of those commands, together with the moves the flow allows it. Pass `--quiet`
when you have read them already.

Dinah discovers its workbench the way git discovers a repository, by walking
up from where you are. Set `DINAH_WORKBENCH` or pass `--workbench` when you
want a different one. When the walk finds nothing, Dinah falls back to your
user base at `~/.dinah`, and you can move that base elsewhere by setting
`DINAH_HOME`, so a test run or a scratch tree can work without touching your
own settings and workbenches.
