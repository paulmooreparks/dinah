# Task

Implement `readiness.py`. `ready(cards)` receives dictionaries with `id`, `state`, and `depends_on`. Return ready card identifiers in input order. A card is ready only when its own state is `ready` and every named dependency exists and has state `done`. Reject duplicate card identifiers and dependency cycles with `ValueError`. A missing dependency leaves the card unready. Use only the Python standard library.
