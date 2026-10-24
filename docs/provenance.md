# Provenance

> **Current Switch client: Harbor**, at `~/REPOS/Openpak/emulators/harbor`. Harbor is an
> independent OpenPak project based on Eden. References to retired forks (Citron, Ryujinx, old
> Eden checkouts) in this repository record historical evidence only; do not run their commands.

Where each non-obvious fact in [`design.md`](design.md) and [`evidence-needed.md`](evidence-needed.md)
came from, per the [clean-room policy](https://docs.openpak.org/contributors/clean-room/). Raw
material stays in the workspace scratch folder (`~/REPOS/Openpak/scratch/`), never in this
repository. The detailed findings and milestone plan live in the private research notes for this
title.

| Fact | Source |
| --- | --- |
| Title id `0100C56010FD8000`, update `0100C56010FD8800` v262144 (v1.0.4) | OpenPak's own dump; multiplayer-sweep evidence JSON (2026-10-12) |
| EA hosts the binary carries (`spring18.gosredirector.ea.com` and env variants, `accounts/signin/gateway.ea.com`, `gosca.ea.com`, ...) | String table of the owner's decompressed `main` NSO (§2 of design) |
| DirtySDK is the whole network/TLS stack (`ProtoSSL*`, `DirtyCert*`, `ProtoHttp*`); **no** `nn::ssl`; **no** `nn::ldn` | Exported symbol table of the owner's dump (0 matches for `nn::ssl`/`nn::ldn`) |
| DirtyCert validates against an embedded CA list (`GOS 2013`/`GOS 2015 Certificate Authority`, DigiCert, VeriSign) | String table + `ProtoSSLSetCACert`/`DirtyCertCAPreloadCerts` symbols |
| Redirector may run over HTTP (`getServerInstanceHttp`, `redirector/getServerInstance`, `http://%s:%d/%s`); client loads Blaze-leg CA list from the redirector (`Blaze::Redirector::CACertificateRequest/Response`, `getCACertificates`) | String table + symbol table |
| All three topologies present (`CLIENT_SERVER_DEDICATED`, `CLIENT_SERVER_PEER_HOSTED`, `PEER_TO_PEER_FULL_MESH`); client-side host compiled in (`PVZClientOnlinePeerHostedGameManager`, `PVZServerConnection`, `PVZServerAuth`, `GameModeUtil::getIsPeerHosted/...`) | Exported symbol table + string table |
| Nucleus OAuth auth-code sign-in; persona + entitlement surface; `Blaze::Authentication::TrustedLoginRequest` | String table (`AuthCode not found from redirect URL`, `client_id=...&access_token=...`, `ListPersonaEntitlements2Request`, Nucleus enums) |
| Server-admission token is minted by the game's own backend component, not by Nintendo | `GetSignatureTokenRequest/Response` in the game's custom Blaze component + `PVZServerAuth::onSignatureToken` symbols |
| The account/economy/progression/store backend is one custom Blaze component with ~130 commands/types | Exported symbol table (component request/response/notification type names) |
| Prior art: three PC private-server projects re-enable the client's host code via DLL injection | Public project pages only; **not read for code** (clean-room) |
