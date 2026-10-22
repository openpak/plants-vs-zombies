# plants-vs-zombies

Research into replacing the network service used by **Plants vs. Zombies: Battle for
Neighborville — Complete Edition** on Nintendo Switch (EA / PopCap, 19 March 2021, Frostbite 3).
That is the only Plants vs. Zombies title on the platform — neither Garden Warfare game was
ever released for Switch.

## Status: documentation only. Verdict is "not feasible now."

There is no code here and there will not be until the verdict changes. This is not a
placeholder for a server; it is the record of why a server would not help.

BfN's online modes all run on **EA-hosted authoritative dedicated servers**, allocated over
EA's Blaze backend. There is no server browser, no LAN mode and no peer-hosted fallback on
console. A complete Blaze re-implementation would authenticate the console, take it to the
lobby, and then have no game session to hand it.

The three open-source private-server projects for this game
([GardenGate](https://github.com/nocss42/GardenGate),
[Cypress](https://github.com/PvZ-Cypress/Cypress),
[Z7](https://github.com/Khysnik/Z7)) all get around that by **injecting a DLL into the Windows
game client** to enable the server code shipped inside it. They are Windows binary-patching
projects, PC-only, with per-build Frostbite offsets. Nothing in them ports to a Switch, and
OpenPak's console onboarding redirects and decrypts traffic rather than modifying games.

So the blocker is not a protocol we have not written yet. It is a game server we cannot obtain
for this build.

Full reasoning and sources: [`docs/design.md`](docs/design.md).
What would overturn it, and how to find out: [`docs/evidence-needed.md`](docs/evidence-needed.md).

## Consequences

- **No port block claimed.** The 22600–22699 third-party-network range in
  [`ports.md`](../../ports.md) stays free for a title that can use it.
- **No Go module.** `go.mod` was removed; an empty module only ever produced a failing
  `go test ./...`. It comes back with the first real package.
- **One question decides everything.** If a capture ever shows the console hosting its own
  session for any mode, the shape becomes an ordinary Go service like `photon` or `outbound`,
  scoped to that mode. `evidence-needed.md` step 3 is that test, and it is cheap.

## Ground rules

- Clean room. See [`docs/clean-room-policy.md`](https://github.com/openpak/docs/blob/main/clean-room-policy.md). No source from
  a non-free implementation was opened to produce anything in this repository. Public documentation, our own
  captures, and compatibly licensed code are the only inputs.
- AGPL-3.0-only, English only, Go 1.25 when there is Go, no secrets and no raw captures in the
  repository.

This project is not affiliated with or endorsed by Nintendo, Electronic Arts, PopCap, or any
original service provider. It contains no proprietary game code, assets, SDK material,
binaries, certificates, keys, firmware, extracted files, credentials, or raw packet captures.
