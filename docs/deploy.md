# Deploy: plants-vs-zombies

## Image build (from the Dockerfile)

The Dockerfile builds `pvzd` from `./cmd/pvzd` in a `golang:1.26-alpine` stage and copies it into
a `gcr.io/distroless/static` image that runs as `nonroot:nonroot`. Build flags: `CGO_ENABLED=0`,
`GOWORK=off`, `-trimpath -buildvcs=false -ldflags="-s -w"`.

| Item | Value |
| --- | --- |
| Entrypoint | `/pvzd` |
| Exposed ports | `42230`, `44325` (TLS, game-fixed), `23540` (`/health`), `23541` (plain HTTP for hosts Traefik terminates) |
| Volume | `/data` (only a leftover `accounts.json` is read from it, once, and renamed `accounts.json.imported-20261104`) |
| `PVZ_DATABASE_URL` | required: Postgres for accounts (table `pvz_accounts`, pool of 3) |
| Environment variables | none declared in the Dockerfile |
| Image name | not recorded in this repository |

## Account erasure

`DELETE /internal/accounts/{account_id}/data` on the health listener (`http://openpak-plants-vs-zombies:23540`, container network only, not routed by Traefik), header `Authorization: Bearer <ACCOUNT_ERASE_KEY>`; key unset answers 503, bad id 400, wrong key 401. Needs `ACCOUNT_ERASE_KEY`, `ACCOUNT_GRPC_ADDR` and `ACCOUNT_INTERNAL_KEY` (read-only core link lookup, namespace `switch`). Answers `{"service":"plants-vs-zombies","rows":N}` and logs one `account erased` line. Erased: the `pvz_accounts` rows (name, ids, per-game blobs) whose `baas_user_id` or `pid` matches one of the account's Switch links. Idempotent.

Nothing else about production deployment is recorded yet.
