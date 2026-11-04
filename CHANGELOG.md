# Changelog — plants-vs-zombies

Generated from git history on 2026-09-15. `git log` stays the source
of truth; this file is the readable summary.

## v0.2.0 — 2026-11-04

- Accounts (and their per-game blobs) move from `accounts.json` to Postgres (`pvz_accounts`, `PVZ_DATABASE_URL`, required; pool of 3). A leftover `accounts.json` is imported once and renamed `.imported-20261104`; the Blaze id sequence continues past it. Restart and import test against a real Postgres. The service is not deployed on the production box yet, so there was no prod file to import. (openpak/prds#85)

## v0.1.0 — 2026-11-04

- docs: scope Battle for Neighborville, verdict not feasible, no code [3544743]
- seed: empty plants-vs-zombies, AGPL-3.0-only, clean-room gate [426411a]

