---
title: Trees draw with proper line characters
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
Dinah draws the branches of a tree out of hyphens, pipes and a backtick, which is what a terminal offered thirty years ago. The box-drawing characters give the same shapes properly joined, they measure one screen column each so nothing in the width machinery changes, and a reader can follow a deep tree by eye rather than by counting indentation. The reason this is a card rather than a change is the fallback: deciding when a terminal cannot render those characters is done everywhere by reading environment variables and guessing, which is undocumented behaviour of somebody else's software and cannot be load-bearing here. The recommendation is to default to the better characters and let the plain form be a declared setting alongside language and actor, so anybody piping output somewhere fussy says so once rather than hoping the tool guesses right. Worth noting that the tool already prints Devanagari unconditionally, so anything that cannot render a box-drawing character cannot render a shipped language either.
