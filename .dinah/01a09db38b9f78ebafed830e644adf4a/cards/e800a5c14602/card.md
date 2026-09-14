---
title: A workbench whose own definition file cannot be resolved is marked confirmed clean, because the panel has nowhere to write
column: 5ea2db0272fc
state: ready
severity: major
priority: soon
workstreams:
  - 58f3e3eb621a
---
The Problems panel needs a document to attach a diagnostic to. Findings that name no file are attached to the workbench's own definition file instead. When that file cannot be resolved, those findings go to the output channel, the panel stays empty, and the workbench is then marked confirmed clean.

The reader is told their workbench is healthy at the moment it is least likely to be.

It is reachable rather than theoretical, and the reason is the uncomfortable part: the condition that breaks the definition-file lookup is the same condition the checker reports findings about. So the case arrives exactly when the findings matter.

Found by the code reviewer on dinah-421, which built the panel projection. It deliberately did not push that card back, and the judgement was right. The implementation matches what its spec asked for, and there is no honest answer inside a panel that requires a document to write on. Saying this properly needs a different surface, which is why it is a card rather than a finding.

That makes this the seventh appearance of one defect on this board: the extension reporting a confident negative when it cannot tell. Five were closed inside the status bar over five implementation rounds, one in the CLI's reply, and this one has no room to be closed where it stands.

What a spec here owns is the surface, not the wording. A notification, a row in the sidebar, and a refusal to report a verdict at all are different products with different costs, and the choice wants making rather than assuming. Whatever it lands on, the rule dinah-419 established has to hold: anything the extension cannot positively identify falls through to not knowing.

Related: dinah-421 built the panel, dinah-419 established the rule, dinah-432 closed the CLI-side door, and dinah-433 carries a swallowed error one layer further down.
