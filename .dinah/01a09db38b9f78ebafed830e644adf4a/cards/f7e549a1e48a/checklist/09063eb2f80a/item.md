---
kind: decision
state: resolved
ts: 2026-09-14T02:17:20Z
ordinal: 8
note: Grounded in docs/design/format.md:1753 (a claim is a lease measured in hours) against os.Getpid()/os.Hostname() being process- and machine-scoped values with no documented cross-session or cross-time uniqueness guarantee on the platforms Dinah ships for.
---
A process id and a hostname are documented and readable through documented APIs, so reading either is not the undocumented-behaviour problem. What is missing is a documented guarantee that either stays unique for as long as a claim's own lease lasts (hours). Any discriminator this design mints should be a self-generated random token from a documented cryptographic source (crypto/rand), which sidesteps the reuse question by making uniqueness a stated property of the generator rather than an assumption about OS behaviour.