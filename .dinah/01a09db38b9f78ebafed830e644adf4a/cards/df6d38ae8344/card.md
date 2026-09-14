---
title: Two index summaries claim more than their rules do
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
The contract's index gives each rule a one-line summary. Two of those summaries describe their rule as covering ground the rule itself now excludes, because the rule gained an exclusion and the summary did not follow. Neither summary is wrong in a way that contradicts anything, so nothing catches it and nothing breaks; they simply promise more than the rules deliver, which is the sort of drift a reader only discovers by reading both halves side by side. A separate card proposes checking a rule against its summary automatically, and this one is the instance that already exists.
