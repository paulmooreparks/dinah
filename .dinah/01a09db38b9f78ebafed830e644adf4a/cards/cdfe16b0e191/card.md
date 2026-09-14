---
title: The replay guard drives every guide, not only the quick start
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
The replay that holds the documentation to the tool reads one document. It finds every fenced block whose first line opens with a prompt, runs it, and compares the bytes, and its own header records that a transcript added to an embedded guide is guarded by nothing. So the guides may teach commands but may not show what those commands print, which is a real limit on how much a guide can teach. This card asks what it takes to drive a second document through the same replay, since the machinery reads two package variables for the document and its exemption file and nothing else about it is specific to the quick start.
