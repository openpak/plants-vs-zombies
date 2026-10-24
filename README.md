# plants-vs-zombies

Research into replacing the network service used by **Plants vs. Zombies: Battle for
Neighborville — Complete Edition** on Nintendo Switch (EA / PopCap, 19 March 2021, Frostbite 3).
That is the only Plants vs. Zombies title on the platform — neither Garden Warfare game was
ever released for Switch.

## Status: CA patch done (owner-approved exception); redirector and sign-in next.

BfN runs on EA's **Blaze** backend. Its account, sign-in, profile, progression and store layers
are a single custom Blaze component we can serve. A **2026-10-24 read of the game client**
(docs/design.md §7) refines the earlier "no peer-hosted fallback on console" verdict: the Switch
binary actually carries **all three network topologies** (dedicated, peer-hosted, peer-to-peer)
and a complete client-side host — so the console *can* host, and which topology a mode uses is
decided by the Blaze server we would control.

The console-side blocker was **TLS trust**. The game does all networking over EA's DirtySDK and
links **no** `nn::ssl`, so it validates certificates against DirtySDK's **own embedded CA list**,
independent of the Nintendo system store, and its first EA call (the redirector) is TLS only. A
capture on 2026-10-24 confirmed that no console-side change gets past it.

### The PvZ exception to "no game patches"

OpenPak does not patch games. This title is a documented exception, decided by the owner on
2026-10-24, alongside the Crash Team Racing key swap and the Diablo II: Resurrected trust patch: a
**data-only** IPS replaces one entry of the game's CA list (EA's "GOS 2015 Certificate Authority")
with the OpenPak CA's name and public key. No instruction changes. It covers update v1.0.4 (build
id `5AD255D6667B6EB5BDCF74B74BCD2602`) only and is no precedent for other titles. The generator,
the file and the evidence are in [`tools/pvz-ca`](tools/pvz-ca/README.md); consoles get it from
the OpenPak Switch Patcher and Harbor builds it in.

The three open-source private-server projects
([GardenGate](https://github.com/nocss42/GardenGate),
[Cypress](https://github.com/PvZ-Cypress/Cypress),
[Z7](https://github.com/Khysnik/Z7)) re-enable the client's own host code by **injecting a DLL
into the Windows client** — PC-only, per-build offsets, nothing that ports to a Switch, and the
wrong shape for OpenPak (which changes the console, not the game). They are cited as prior art
only; none was read for code.

Full reasoning and sources: [`docs/design.md`](docs/design.md).
What would overturn it, and how to find out: [`docs/evidence-needed.md`](docs/evidence-needed.md).

## Consequences

- **Port block pending.** The redirector and Blaze service ports are being claimed in
  [`ports.md`](https://docs.openpak.org/contributors/ports/) with the first service code.
- **No Go module.** `go.mod` was removed; an empty module only ever produced a failing
  `go test ./...`. It comes back with the first real package.
- **One question decides everything.** If a capture ever shows the console hosting its own
  session for any mode, the shape becomes an ordinary Go service like `photon` or `outbound`,
  scoped to that mode. `evidence-needed.md` step 3 is that test, and it is cheap.

## Ground rules

- Clean room. See [clean-room policy](https://docs.openpak.org/contributors/clean-room/). No source from
  a non-free implementation was opened to produce anything in this repository. Public documentation, our own
  captures, and compatibly licensed code are the only inputs.
- AGPL-3.0-only, English only, Go 1.25 when there is Go, no secrets and no raw captures in the
  repository.

This project is not affiliated with or endorsed by Nintendo, Electronic Arts, PopCap, or any
original service provider. It contains no proprietary game code, assets, SDK material,
binaries, certificates, keys, firmware, extracted files, credentials, or raw packet captures.
