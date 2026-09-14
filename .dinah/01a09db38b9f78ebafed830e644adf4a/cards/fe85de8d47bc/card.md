---
title: The tool renders a right-to-left language
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
links:
  - kind: blocks
    to: 7cfe3b30b4ab
  - kind: blocks
    to: 49334eebcac4
---
Every language the tool speaks today reads left to right, and the display path has never been asked what to do otherwise. A right-to-left language raises questions that are decisions rather than defaults: whether a column of such text aligns to the right while the table still reads left to right, and what a row does when one field is Hebrew or Arabic and the next is an English identifier. It also runs into a limit the tool does not control, since terminals disagree about whether they reorder text at all and mostly do not document what they do, so a documented limitation is a legitimate outcome here rather than a failure. This card settles the display questions and establishes what the tool can honestly promise, ahead of the two language catalogs that depend on it.
