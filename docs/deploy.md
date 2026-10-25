# Deploy: plants-vs-zombies

## Image build (from the Dockerfile)

The Dockerfile builds `pvzd` from `./cmd/pvzd` in a `golang:1.26-alpine` stage and copies it into
a `gcr.io/distroless/static` image that runs as `nonroot:nonroot`. Build flags: `CGO_ENABLED=0`,
`GOWORK=off`, `-trimpath -buildvcs=false -ldflags="-s -w"`.

| Item | Value |
| --- | --- |
| Entrypoint | `/pvzd` |
| Exposed ports | `42230`, `44325` (TLS, game-fixed), `23540` (`/health`), `23541` (plain HTTP for hosts Traefik terminates) |
| Volume | `/data` |
| Environment variables | none declared in the Dockerfile |
| Image name | not recorded in this repository |

Nothing else about production deployment is recorded yet.
