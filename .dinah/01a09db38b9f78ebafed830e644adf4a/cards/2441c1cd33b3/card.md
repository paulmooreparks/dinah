---
title: Seven screens nobody has ever printed in a test
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
---
The check that reads the tool's own output can only judge what a test actually prints, and seven screens have never been printed by one. They are named rather than covered, which means the alignment guard the table work installed is blind to them by construction and will stay blind until somebody exercises them. Each needs the tool driven into the state that produces it, which is why they were skipped rather than forgotten.
