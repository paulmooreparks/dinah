---
title: The Hindi catalog has grown by agent since its native review
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
dinah-3 records that English and Hindi shipped complete and reviewed by a native speaker. That was true of the catalog as it stood then. Since then agents have written Hindi strings directly, and no native speaker has read what accumulated.

Raised on 2026-08-26 out of dinah-192, where a code review found the new Hindi rendering faithful to its English but flagged a verb form that treats the noun for "name" as a verb stem. That is not standard Hindi. The reviewer also observed that the neighbouring string already does the same thing, which is the finding worth acting on. It is a house habit spreading by imitation rather than a single slip, and each agent that matches its neighbours carries it one string further.

The staleness machinery does not help here and is not meant to. A fingerprint proves a translation was written against the current English, which is exactly what makes this invisible. Every one of these strings is current, and some of them are wrong. dinah-252 exists because a fingerprint match cannot see a translation that is fluent, current and using the wrong word.

What this card wants is a native Hindi reader over the whole catalog rather than over one string, with the verb-stem habit as the first thing to rule on since it recurs. The deliverable is a corrected catalog and a short note on the pattern, so that agents writing Hindi later have something to match other than the nearest existing string.

One thing is worth deciding at the same time, and it is the reason this is filed rather than quietly fixed. Should an agent be writing translated strings at all, or should a language nobody on the project reads take the English until a native speaker supplies the translation? Five of the eight catalogs already work the second way today.
