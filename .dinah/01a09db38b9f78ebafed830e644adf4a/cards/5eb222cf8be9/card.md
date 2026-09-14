---
title: The quick start's two check transcripts answer from workbenches nothing builds
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
Two transcripts in the quick start show `dinah check` reporting defects, one from a workbench damaged on purpose and one from a workbench written before slugs and creation ordinals existed. The narrative builds neither, so the guard dinah-144 adds cannot replay them and exempts both, which leaves the defect sentences a reader is shown held only by a catalog scan rather than by the tool's own output. Building the two fixtures would let those blocks replay with the rest.
