---
kind: acceptance_criterion
state: pending
column: c9428b3bc921
ts: 2026-09-14T02:16:58Z
ordinal: 8
---
test-deny-main-checkout-cwd.py builds and tears down its own throwaway fixture repository under a temporary directory and never references C:\Users\paul\source\repos\dinah in any command it constructs. Verified by reading the test file's own fixture-setup code.