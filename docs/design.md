# Design: Plants vs. Zombies on Nintendo Switch

Status: **research only. Verdict is "not feasible now."** No code, no port block.
Written 2026-09-09 against public sources only, under
[clean-room policy](https://docs.openpak.org/contributors/clean-room/). No source from a non-free implementation
was opened for this document.

## 1. Which title

There is exactly one Plants vs. Zombies shooter on Nintendo Switch:

**Plants vs. Zombies: Battle for Neighborville — Complete Edition**, EA / PopCap Vancouver,
Switch release **19 March 2021**, engine **Frostbite 3**.

The Garden Warfare pair is not on the platform. *Garden Warfare* (2014) shipped on
PS3/PS4/Xbox 360/Xbox One/Windows and *Garden Warfare 2* (2016) on PS4/Xbox One/Windows;
neither was ever ported to Switch. The original tower-defence *Plants vs. Zombies* has no
Switch release either. So "the Plants vs. Zombies title on Switch" is unambiguous, and this
repository is scoped to **Battle for Neighborville (BfN) on Switch**.

Sources: [Wikipedia — Battle for Neighborville](https://en.wikipedia.org/wiki/Plants_vs._Zombies:_Battle_for_Neighborville),
[Wikipedia — Garden Warfare 2](https://en.wikipedia.org/wiki/Plants_vs._Zombies:_Garden_Warfare_2),
[EA press release, 17 Feb 2021](https://www.businesswire.com/news/home/20210217006006/en/EA-Announces-Plants-vs.-Zombies-Battle-For-Neighborville-Complete-Edition-Blasts-Onto-The-Nintendo-Switch-March-19).

## 2. The network stack it actually uses

BfN is an EA online title, so nothing in it touches NEX, NPLN or any Nintendo game service.
The console talks to EA directly over TLS. From public sources, the stack is four layers:

| Layer | What it is | Public evidence |
| --- | --- | --- |
| Identity | EA Account (Nucleus). On Switch the Nintendo Account is *linked* to an EA Account, either on the web or on first launch of an EA title. | [EA account-linking help](https://help.ea.com/en/help/account/link-your-console-accounts/), [Nintendo UK link/unlink guide](https://www.nintendo.com/en-gb/Support/Troubleshooting/How-to-Link-Unlink-an-EA-Account-From-Your-Nintendo-Account-2346370.html) |
| Session / matchmaking | **EA Blaze** — EA's proprietary backend protocol. A *redirector* host is queried first and hands back the address of the Blaze server for that environment; the client then speaks TDF (Tag Data Format) records over TCP/TLS. Documented components include Redirector, Authentication, Util, Association, GameManager, GameReporting, Messaging, Stats, Playgroups. | [open-ds2-server Blaze analysis](https://github.com/lowlevelmetal/open-ds2-server/blob/main/docs/04-blaze-connection-analysis.md), [grid-leak/blaze](https://github.com/grid-leak/blaze), [openBlase](https://github.com/openBlase/openBlase) |
| Gameplay | **EA-hosted authoritative dedicated servers**, allocated by Blaze GameManager. Every online mode — Turf Takeover, Team Vanquish, and the co-op free-roam regions — is a hosted session. Console has no server browser, no LAN mode, and no crossplay between versions. | [Wikipedia](https://en.wikipedia.org/wiki/Plants_vs._Zombies:_Battle_for_Neighborville), EA forum threads on crossplay and connection failures |
| Ancillary | EADP HTTPS services (entitlements, config, telemetry) alongside Blaze. | Inferred from the EA platform generally — **unknown for this title, needs a capture** |

Two Switch-specific details are public but unconfirmed by us. Community packet analysis of a
Switch EA title describes a redirector environment ("spring18") that hands the console off to
one of several Blaze servers, and an auth step in which the console presents a
**Nintendo-issued token that the Blaze server validates against Nintendo's backend** before
the session proceeds. Both are second-hand; treat as leads, not facts. The second one, if
true, is the single most interesting fact in this document — see §5.

## 3. Prior art, and what it proves

Three open-source projects already run private BfN/Garden Warfare servers. All three are
**PC only**, and all three work the same way:

| Project | Scope | Licence | Method |
| --- | --- | --- | --- |
| [GardenGate](https://github.com/nocss42/GardenGate) | GW1, GW2, BfN | GPL-3.0 | DLL injected into the Windows game client; one player's client acts as host |
| [Cypress](https://github.com/PvZ-Cypress/Cypress) | GW1, GW2, BfN | GPL-3.0 | `dinput8.dll` DirectInput hijack + MinHook hooks into Frostbite functions; master-server registration and a server browser. Derived from KYBER (Star Wars: Battlefront II) |
| [Z7](https://github.com/Khysnik/Z7) | GW2 | — | Blaze emulator for offline play |

This is the decisive finding, and it cuts both ways.

**The good half:** the Frostbite *client* binary contains the game-server code. Nobody has EA's
server build; the PC projects enable the one already shipped inside the client and point it at
a re-implemented Blaze. So an authoritative server for BfN is not, in principle, an unobtainable
artefact.

**The bad half:** every one of them reaches it by **patching the game client's machine code at
runtime** — `dinput8.dll` hijacking, MinHook detours, Frostbite function offsets. That is not a
network server; it is a Windows binary-patching project that happens to ship a Blaze stub
alongside. None of it ports. The code is x86-64 Windows C++/C#, against PC builds, with
per-build offsets.

## 4. Feasibility verdict: not feasible now

The gate is not a protocol we could write in Go. It is that **we have no game server for the
Switch build and no supported way to make one.**

1. **No server binary, and no injection path on Switch.** OpenPak's console onboarding is
   Atmosphere DNS.mitm plus CA trust patches — redirect and decrypt traffic, do not modify
   the game. There is a standing decision that OpenPak ships no custom sysmodule on console.
   Matching what Cypress does would mean reproducing KYBER-class Frostbite hooks against a
   stripped ARM64 build via IPS patches, re-derived for every game version. That is a
   reverse-engineering project of a different kind and size than a server repository, and it
   is not what this repository is for.
2. **A Blaze server alone buys nothing playable.** Suppose we shipped a perfect Go Blaze
   implementation tomorrow. The console would authenticate, reach the lobby, request a game
   session from GameManager — and we would have no session to give it. Every online mode is
   server-hosted. There is no peer-hosted fallback to fall back to.
3. **The protocol half is also unspecified for Switch.** The public Blaze write-ups are
   PC-derived and openly incomplete: packet header format, TDF encoding and component/command
   ID mappings are all listed as unresolved in the best public analysis. On top of that we do
   not know the Switch redirector hostname, the Switch environment id, the certificate the
   console pins, or the shape of the Nintendo-token auth step. Every one of those is
   "unknown, needs a capture."
4. **The preservation argument is weaker here than elsewhere.** EA's 2026 sunsets hit the PS3
   Garden Warfare, not BfN on Switch; BfN's Switch servers are live as of writing. And the
   Complete Edition plays offline solo and in split-screen without a server at all. When EA
   eventually does sunset it, Switch owners keep the offline game and lose exactly the part
   we cannot restore.

The honest summary: **BfN on Switch is blocked on an authoritative game server we cannot
obtain, not on a backend we have not yet written.** Writing the backend first would produce a
service that can never host a match.

## 5. The one thing that would change the verdict

If a capture shows the Switch build negotiating a session the *console itself* hosts — a
private/split-screen-over-network path, a co-op free-roam region hosted by the party leader,
anything where GameManager hands out a console's address rather than an EA data-centre's —
then the shape becomes ordinary and familiar: a Go Blaze-compatible service on a 22600-block
port, Switch traffic redirected by DNS.mitm, and the console hosting its own session. That is
the same shape as `photon` and `outbound`.

The second lead worth chasing is the Nintendo-token auth step. If Blaze validates a token
issued by *Nintendo* rather than by EA, then on an OpenPak console — which already gets its
tokens from our own BAAS — the auth side is something we control, not something we have to
forge.

Both are cheap to answer with one capture session. [`evidence-needed.md`](evidence-needed.md)
orders them so the cheapest kill shot comes first.

## 6. Scope decision

Documentation only. No Go module, no port block claimed in
[`ports.md`](https://docs.openpak.org/contributors/ports/), no skeleton. The 22600-block allocation stays free for a
third-party-network title that can actually use it.

Revisit when either §5 question comes back positive, or when someone demonstrates a
console-side host path for a Frostbite title on Switch.
