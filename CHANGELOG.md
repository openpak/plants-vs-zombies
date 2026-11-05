# Changelog — plants-vs-zombies

Generated from git history on 2026-09-15. `git log` stays the source
of truth; this file is the readable summary.

## v0.3.0 — 2026-11-05

- Data duplication audit (openpak/prds#131): `pvz_accounts.name` was a copy of the player's Switch nickname refreshed at every login. The column is dropped (`ALTER TABLE ... DROP COLUMN IF EXISTS name` at start-up) and the name is held per session only, from the identity answer at login ("Player<n>" when the identity has none), so the game still sees the same name and nothing about it is stored. A rollback to v0.2.x needs the column back (`name text NOT NULL DEFAULT ''`).

## v0.2.2 — 2026-11-05

- Account erasure (openpak/prds#120): `DELETE /internal/accounts/{account_id}/data` on the health listener removes the account's `pvz_accounts` rows, resolved through the account core's Switch links. Needs `ACCOUNT_ERASE_KEY`, `ACCOUNT_GRPC_ADDR`, `ACCOUNT_INTERNAL_KEY`. The health listener now logs every request.

## v0.2.1 — 2026-11-04

- Release workflow: tags publish ghcr.io/openpak/<repo> for amd64 and arm64 (same as the sibling game servers). No code change.

## v0.2.0 — 2026-11-04

- Accounts (and their per-game blobs) move from `accounts.json` to Postgres (`pvz_accounts`, `PVZ_DATABASE_URL`, required; pool of 3). A leftover `accounts.json` is imported once and renamed `.imported-20261104`; the Blaze id sequence continues past it. Restart and import test against a real Postgres. The service is not deployed on the production box yet, so there was no prod file to import. (openpak/prds#85)

## v0.1.0 — 2026-11-04

- docs: scope Battle for Neighborville, verdict not feasible, no code [3544743]
- seed: empty plants-vs-zombies, AGPL-3.0-only, clean-room gate [426411a]

