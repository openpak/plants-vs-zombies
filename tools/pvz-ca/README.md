# PvZ: Battle for Neighborville CA patch

## The PvZ exception to "no game patches"

OpenPak does not patch games: it changes where the console connects and puts its CA in the
console's certificate store. This patch is one of the documented exceptions, next to the Crash
Team Racing key swap and the Diablo II: Resurrected trust patch. The owner decided it on
2026-10-24. It is data only: the IPS rewrites one entry of a CA list, and every instruction stays
as shipped. The exception covers exactly the build below. It is no precedent for any other title.

Why no console-side fix reaches it: the title does all its EA networking through EA's DirtySDK.
Its TLS (ProtoSSL) trusts only a CA list compiled into the game, never `nn::ssl` or the console
certificate store. The first EA call, the redirector (`spring18.gosredirector.ea.com:42230`), is
TLS only. A capture on 2026-10-24 showed that with any certificate outside that list the client
stops after the server's certificate, and the game reports "Connection to EA servers timed out".

## What it changes

Each entry of the list holds the CA's subject name (country, state, city, organization at 32
bytes each, unit at 256, common name at 64, NUL-padded), flags, a pointer to its RSA modulus and
the exponent. ProtoSSL picks the entry whose subject equals the server certificate's issuer and
checks the signature with that entry's modulus.

The patch rewrites the entry for EA's "GOS 2015 Certificate Authority", which issues the
redirector's certificate:

| Build | Record | Offset (NSO header included) | Size |
|---|---|---|---|
| `5AD255D6667B6EB5BDCF74B74BCD2602`, update v262144 "1.0.4" | modulus -> OpenPak CA modulus | `0x86E2FD8` | 256 |
| | subject -> `O=OpenPak, CN=OpenPak Network CA` | `0x9543A60` | 448 |

The entry's flags, exponent size and exponent (65537, which the OpenPak CA also uses) stay as
shipped. The IPS carries only our public key and our CA's name, no game bytes. After the patch,
any leaf the OpenPak CA issues for the EA name the client dials is accepted, and GOS 2015-issued
certificates (EA's real servers) no longer are.

`make-ips.py <ca.pem> [outdir]` writes the IPS32 file. The CA must be RSA-2048 with exponent 65537
and have only subject attributes ProtoSSL keeps (C, ST, L, O, OU, CN). Regenerate the file when the
CA changes and ship it to both places below.

## Install

| Where | Path |
|---|---|
| Switch (Atmosphere) | `sd:/atmosphere/exefs_patches/openpak_pvz_ca/5AD255D6667B6EB5BDCF74B74BCD2602.ips` |
| Harbor | built in (`src/core/file_sys/openpak_builtin_patches.cpp`), or `<user>/load/0100C56010FD8000/openpak-pvz-ca/exefs/<build id>.ips` |

How it reaches players: the OpenPak Switch Patcher (`platforms/switch/clients/switch-patcher`)
installs it to `/atmosphere/exefs_patches/openpak_pvz_ca/` when OpenPak is selected and removes it
when Nintendo is selected (branch `feat/pvz-ca`). Harbor builds it in, keyed by build id, with a
SHA-1 check of the GOS 2015 modulus (branch `feat/pvz-builtin`). Both are review branches as of
2026-10-24.

Proof it took (Harbor, 2026-10-24): with the patch in the run's load folder, the redirector
handshake completes against an OpenPak-issued leaf (TLS 1.2, ECDHE-RSA-AES256-GCM-SHA384) and the
client sends `POST /redirector/getServerInstance`. Without it, the client goes silent after the
server certificate.
