---
title: A stale configured workbench says so
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
A person can point the tool at a workbench once with a config setting and then forget they did. When that setting later points somewhere that no longer holds a readable workbench, the refusal talks about a file being unparseable and tells them to hand-edit it. It never mentions that the path came from a setting they wrote months ago, so nothing in what they read suggests the actual fault. The reviewer who found this judged the underlying design sound and raised it as a refinement rather than a defect: the refusal should name the setting when the path it failed on came from one.
