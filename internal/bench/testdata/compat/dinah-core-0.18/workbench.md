---
format: 6
profile: dinah-core/0.18
title: Sample workbench
slug: sample
operator: sam
columns:
  - 004acda2c28a
  - 7b5cfe51cb3b
  - 68091798ab26
levels:
  severity: [trivial, minor, major, critical]
  priority: [later, soon, next, now]
  tier: [workhorse, frontier, apex]
tiers:
  workhorse:
    meaning: scoped implementation against a clear contract
    models:
      - {provider: sample, model: workhorse-1}
  frontier:
    meaning: novel design judgement, or work where a wrong answer costs a lot
    models:
      - {provider: sample, model: frontier-1}
  apex:
    meaning: the hardest work this workbench carries
    models:
      - {provider: sample, model: apex-1, server: apex.example}
---
This workbench is a compatibility sample. Every command populate.txt replays runs against it.