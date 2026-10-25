# Plants vs. Zombies: Battle for Neighborville

OpenPak's Plants vs. Zombies: Battle for Neighborville Server.

| | |
| --- | --- |
| Game | [Plants vs. Zombies: Battle for Neighborville](https://openpak.org/titles#t-23) |
| Network | Nintendo Switch |
| Title ID | `0100C56010FD8000` |
| Licence | AGPL-3.0-only |

## What it serves

`pvzd` is EA's gosredirector, the Blaze server and the Nucleus endpoints for the title. It listens on
`42230` and `44325` (TLS, game-fixed ports), `23541` (plain HTTP for the hosts Traefik terminates) and
`23540` (`/health`).

## Run

No build or run command is recorded in the old README. Container build and ports are in
[docs/deploy.md](docs/deploy.md).

## Docs

- [Protocol](docs/protocol.md)
- [Evidence](docs/evidence.md)
- [Deploy](docs/deploy.md)
- [Notes](docs/notes.md)
- [Design](docs/design.md)
- [Evidence needed](docs/evidence-needed.md)
- [Provenance](docs/provenance.md)

Live compatibility status: [openpak.org/titles#t-23](https://openpak.org/titles#t-23).

## Licence

AGPL-3.0-only. See [LICENSE](LICENSE).
