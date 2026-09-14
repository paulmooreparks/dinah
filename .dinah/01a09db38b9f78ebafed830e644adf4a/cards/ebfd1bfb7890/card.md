---
title: Say plainly what DINAH_HOME moves
column: 5ea2db0272fc
state: ready
severity: trivial
priority: later
---
The quick start says twice that the home setting moves the fallback workbench directory. It moves your home, and the workbench directory is still appended underneath it. The sentences are loose rather than false, but a tester following them set up a scratch directory in the wrong place and lost time working out why, which is exactly the failure the guide exists to prevent. This wants one careful sentence in each place, checked against what the resolution actually does.
