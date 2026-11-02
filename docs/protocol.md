# Protocol: plants-vs-zombies

Plants vs. Zombies: Battle for Neighborville Complete Edition (Switch, update v1.0.4). Title id
`0100C56010FD8000`, update `0100C56010FD8800` (v262144). It is the only PvZ title on Switch.
Design reasoning: [design.md](design.md). What would overturn the verdict:
[evidence-needed.md](evidence-needed.md).

## Backend

EA's **Blaze** backend; the account, sign-in, profile, progression and store layers are one custom
Blaze component (~130 commands/types) we can serve. The Switch binary carries all three network
topologies (`CLIENT_SERVER_DEDICATED`, `CLIENT_SERVER_PEER_HOSTED`, `PEER_TO_PEER_FULL_MESH`) and a
complete client-side host (`PVZClientOnlinePeerHostedGameManager`, `PVZServerConnection`,
`PVZServerAuth`), so the console can host; the Blaze server decides which topology a mode uses.
The server-admission token is minted by the game's own backend component
(`GetSignatureTokenRequest/Response`), not by Nintendo. Sign-in is a Nucleus OAuth auth-code flow
with persona and entitlement lookups (`Blaze::Authentication::TrustedLoginRequest`).

## Hosts and redirector

- Hosts the binary carries: `spring18.gosredirector.ea.com` (and environment variants),
  `accounts/signin/gateway.ea.com`, `gosca.ea.com`.
- The redirector is TLS only: `spring18.gosredirector.ea.com:42230`, TLS 1.2, SNI. With a
  certificate outside the embedded CA list the client stops after the server certificate.
- Request: `POST /redirector/getServerInstance`, XML `serverinstancerequest` (`name
  plantsvszombies-gw3-nx`, `connectionprofile standardSecure_v4`, Blaze SDK 15.1.1.8.2, DirtySDK
  15.1.5.0.2).
- The client can load its Blaze-leg CA list from the redirector
  (`Blaze::Redirector::CACertificateRequest/Response`, `getCACertificates`).

## TLS trust

The game does all networking over EA's DirtySDK (`ProtoSSL*`, `DirtyCert*`, `ProtoHttp*`) and links
no `nn::ssl` and no `nn::ldn`. DirtyCert validates against its own embedded CA list (GOS 2013 /
GOS 2015 Certificate Authority, DigiCert, VeriSign), independent of the Nintendo system store, so
the OpenPak CA in the system store does not reach this title and no console-side change gets past
the redirector.

### CA swap (documented exception to "no game patches")

Owner decision 2026-10-24, alongside the Crash Team Racing key swap and the Diablo II: Resurrected
trust patch: a data-only IPS replaces one CA-list entry (EA's "GOS 2015 Certificate Authority")
with the OpenPak CA's name and public key. No instruction changes. It covers update v1.0.4 (build
id `5AD255D6667B6EB5BDCF74B74BCD2602`) only and is no precedent for other titles. Generator, file
and evidence: [`tools/pvz-ca`](../tools/pvz-ca/README.md). Consoles get it from the OpenPak Switch
Patcher; Harbor builds it in.

CA entry layout: subject strings 32/32/32/32/256/64, flags, modulus pointer, exponent. The GOS 2015
modulus is at NSO offset `0x86E2FD8`, its entry's subject at `0x9543A60`; the modulus was
confirmed by verifying the live redirector certificate's signature with it.

## Open question

If a capture ever shows the console hosting its own session for any mode, the shape becomes an
ordinary Go service scoped to that mode (evidence-needed.md step 3). There is no Go module until
then.

## Prior art

GardenGate, Cypress and Z7 re-enable the client's host code by injecting a DLL into the Windows
client: PC-only, per-build offsets, not portable to Switch. Cited as prior art only; none was read
for code.
