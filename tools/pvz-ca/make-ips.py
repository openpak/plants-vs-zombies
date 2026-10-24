#!/usr/bin/env python3
"""Build the IPS32 exefs patch that makes Plants vs. Zombies: Battle for Neighborville (Switch)
trust the OpenPak CA in place of EA's "GOS 2015 Certificate Authority".

The title does all its EA TLS through DirtySDK's ProtoSSL, which trusts only a CA list compiled
into the game, never the console's certificate store. Each entry of that list holds the CA's
subject name (six fixed-size strings) and a pointer to its RSA modulus (the exponent is stored
inline). ProtoSSL finds a server certificate's CA by comparing the certificate's issuer name with
those subjects, then checks the signature with that entry's modulus.

The patch rewrites one entry, data only: the GOS 2015 subject becomes the OpenPak CA's subject and
its 256-byte modulus becomes the OpenPak CA's. The entry's flags, exponent size and exponent
(65537, which the OpenPak CA also uses) and every instruction stay as shipped. It carries only
our public key and our CA's name, no game bytes.

Usage: make-ips.py <ca.pem> [outdir]
IPS offsets are NSO-header inclusive (+0x100), as Atmosphere and the emulators expect.
"""
import os
import struct
import subprocess
import sys

# build id -> (header-inclusive offset of the CA entry's subject, of its modulus)
BUILDS = {
    # update v262144 "1.0.4", the build consoles and emulators run
    "5AD255D6667B6EB5BDCF74B74BCD2602": (0x9543A60, 0x86E2FD8),
}

# ProtoSSL subject layout: country, state, city, organization (32 bytes each), unit (256),
# common name (64); every field NUL-padded.
FIELDS = [("C", 32), ("ST", 32), ("L", 32), ("O", 32), ("OU", 256), ("CN", 64)]


def subject(pem_path):
    out = subprocess.run(["openssl", "x509", "-in", pem_path, "-noout", "-subject",
                          "-nameopt", "sep_multiline,utf8,sname"],
                         check=True, capture_output=True, text=True).stdout
    attrs = {}
    for line in out.splitlines()[1:]:
        key, _, value = line.strip().partition("=")
        attrs[key] = value
    blob = b""
    for key, size in FIELDS:
        value = attrs.pop(key, "").encode()
        assert len(value) < size, (key, value)
        blob += value + b"\0" * (size - len(value))
    assert not attrs, f"subject attributes ProtoSSL does not keep: {attrs}"
    return blob


def modulus(pem_path):
    out = subprocess.run(["openssl", "x509", "-in", pem_path, "-noout", "-modulus"],
                         check=True, capture_output=True, text=True).stdout
    n = bytes.fromhex(out.strip().split("=", 1)[1])
    assert len(n) == 256, "the slot holds a 2048-bit RSA modulus"
    text = subprocess.run(["openssl", "x509", "-in", pem_path, "-noout", "-text"],
                          check=True, capture_output=True, text=True).stdout
    assert "Exponent: 65537" in text, "the entry keeps exponent 65537"
    return n


def record(offset, data):
    return struct.pack(">IH", offset, len(data)) + data


def main():
    pem = sys.argv[1]
    outdir = sys.argv[2] if len(sys.argv) > 2 else "."
    subj, n = subject(pem), modulus(pem)
    for bid, (subj_off, mod_off) in BUILDS.items():
        out = os.path.join(outdir, bid + ".ips")
        with open(out, "wb") as f:
            f.write(b"IPS32" + record(mod_off, n) + record(subj_off, subj) + b"EEOF")
        print(out, f"modulus at {mod_off:#x}, subject at {subj_off:#x}")


if __name__ == "__main__":
    main()
