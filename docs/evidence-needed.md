# Evidence needed

[`design.md`](design.md) says Battle for Neighborville on Switch is **not feasible now**. This
is the list that confirms or kills that verdict, cheapest and most decisive first. Stop at the
first step that settles it — steps 4 onward only matter if step 3 comes back positive.

Everything here is an observation of what the console or the emulator does. No source from a non-free implementation,
no EA server code, no game binaries in the repository. Captures stay off-repo, on the pool
(`/mnt/media/research/`), per the clean-room policy; only decoded findings come back
as text.

## Tooling that already exists

The retired Citron fork carried a DNS redirect hook for this title:
`GetPvzEaRedirectIp` in `src/core/hle/service/sockets/sfdnsres.cpp`, gated by an env
variable that nothing reads any more. It was wired into both
resolution paths and was dead unless the variable was set. Note that it matched the **whole
`.ea.com` suffix**, not one exact hostname — which is itself evidence that nobody has yet
observed which EA hosts this title actually resolves. Step 1 fixes that.

Launch the fork directly — the shared host/joiner personas and their launch scripts were
retired 2026-09-26; see
<https://github.com/openpak/docs/blob/main/playbooks/online-compatibility-playbook.md> §0.

## Step 1 — Which EA hosts does the Switch build resolve, in what order?

**Answers:** whether BfN uses Blaze at all on Switch, and what the redirector host and
environment are. Without this every later step is guesswork.

Run BfN in Harbor with no EA redirect and read the resolver log — there is no hook log, so instead add a one-line log of every resolved host, or take the list
from a `tcpdump -n port 53` on the host while the game boots to its main menu and then attempts
"Play Online".

Record: every hostname in resolution order, with timestamps; which resolve before the EA
account prompt and which after; the ports the console then opens to each.

Expected shape if the PC analysis carries over: a `gosredirector`-style host first, then a
second connection to a different host on a redirector-assigned port. Public PC-derived numbers
to compare against are **42127** (redirector) and **10041** (Blaze), both TCP/TLS.

**Kill condition:** if no EA host is contacted and the title instead reaches a Nintendo service,
the whole premise of this repository is wrong and the design must be rewritten.

## Step 2 — Does the console pin a certificate?

**Answers:** whether a DNS redirect is enough, or whether a MITM also needs a CA trust patch
(hardware) / a trusted stub cert (emulator).

Redirect the Step 1 hostname to a local TLS stub that presents a self-signed certificate for the
Step 1 hostname and logs the ClientHello. Record: SNI, offered cipher suites, ALPN, and whether
the console proceeds or sends an alert. A `ProtoHttp %d.%d/DS %d.%d.%d.%d` User-Agent in any
cleartext HTTP would confirm EA's DirtySock stack and pin the SDK version.

**Kill condition:** none — this only sets the difficulty of every later capture.

## Step 3 — THE DECIDER: does any mode produce a console-hosted session?

**Answers:** the one question in `design.md` §5 that changes the verdict. Everything above is
preparation for this.

With the MITM from Step 2 in place, get far enough into GameManager to see a session
allocation, for each of: Turf Takeover, Team Vanquish, a co-op free-roam region, and a
party/private match with a second persona. For each, record **whose address the client is told
to connect to** — an EA data-centre address, or another console's.

- **Every mode returns an EA-hosted address** → verdict confirmed. Close the question; this
  repository stays documentation-only until something external changes.
- **Any mode returns a console address** → verdict overturned for that mode. Proceed to step 4
  and scope the repository to exactly that mode, nothing more.

If the MITM cannot get that far, the fallback observation is cheaper and nearly as good: on a
live retail session, `tcpdump` the console's gameplay traffic and check whether the peer address
is an EA-owned range (AS or WHOIS lookup) or a residential one. Peer-to-peer gameplay is visible
from the outside without decrypting anything.

## Step 4 — (only if step 3 is positive) The Nintendo-token auth step

**Answers:** whether our own BAAS can mint what Blaze wants, or whether we would be forging EA
credentials.

Capture the Authentication component exchange and identify what the console presents. Community
analysis of another Switch EA title reports the console presenting a **Nintendo-issued** token
that the Blaze server validates against Nintendo's backend. If that is what BfN does, an OpenPak
console is already the token issuer and the auth path is ours to serve. If instead the console
presents an EA Nucleus access token obtained from an EA login flow, we would have to stand up a
Nucleus-compatible identity service as well — a second project, and worth its own verdict.

## Step 5 — (only if step 3 is positive) Blaze wire format for this build

**Answers:** how much of the protocol we would have to derive ourselves.

The best public analysis explicitly lists the packet header format, TDF encoding and
component/command ID mappings as unresolved. Compare our capture against the open
implementations ([grid-leak/blaze](https://github.com/grid-leak/blaze),
[openBlase](https://github.com/openBlase/openBlase),
[GardenGate](https://github.com/nocss42/GardenGate)) — all GPL-3.0 or unlicensed, all
PC-derived, all reference material rather than something to vendor. Record which command IDs
the Switch build actually uses; a BfN-era title is unlikely to need most of the nine components.

## What would *not* count as evidence

- Anything read out of an earlier server's source tree. If a step feels blocked without one, it needs a
  capture instead — that is the whole point of this list.
- PC captures. BfN's PC build authenticates differently and has a store/EA-app path the console
  does not; a PC capture would tell us about a different client.
- Inference from another EA Switch title. Useful as a lead (that is exactly the status of the
  redirector and Nintendo-token claims above), never as a confirmed fact for this title.
