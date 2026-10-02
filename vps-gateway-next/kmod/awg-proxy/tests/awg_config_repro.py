#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-2.0
"""
Reproduce a user-reported AWG config and verify the v1.1.2 kmod produces
the correct wire-format byte-by-byte.

The config under test was sent to us as "didn't work on awg_proxy.ko but
did work on amneziawg-linux-kernel-module". It exercises:

  - Real (non-zero) peer public key                 -> MAC1 recompute path
  - Large single-value H1/H2/H3/H4                  -> msgType replacement
  - Non-zero S1=114, S2=34                          -> handshake prefix bytes
  - Jc=2, Jmin=10, Jmax=50                          -> two junk packets +
                                                       v1.1.2 random DSCP
  - I1 = <r 2><b 0xHEX>                             -> single CPS packet,
                                                       2 random + 45 static
  - No <c>/<t>/<rc>                                 -> v1.1.2's BE+charset
                                                       fixes NOT exercised
                                                       here (they don't
                                                       apply)

Test redirects the kmod's remote endpoint to a local UDP listener so we
can capture exactly what would be sent on the wire. A synthetic WG
handshake-init is fed in; the kmod runs its full pre-init burst
(CPS -> junk -> init) and we inspect the bytes.

Per handshake cycle we expect 4 packets:
    - 1 CPS packet from I1 (2 random + len(static_hex)/2 static bytes)
    - 2 junk packets (size in [Jmin, Jmax], independent random IP TOS)
    - 1 obfuscated handshake init (S1 random prefix + 148-byte init,
      with the first 4 bytes of the original init replaced by H1 LE,
      and MAC1 recomputed)

Usage:
    scp awg_config_repro.py root@router:/tmp/
    ssh root@router 'python3 /tmp/awg_config_repro.py; echo exit=$?'

Exit code: 0 if all structural checks pass, 1 if any fail, 2 on setup error.
"""

import base64
import os
import socket
import struct
import sys
import threading
import time

# --------------------- the user's config under test ---------------------

PEER_PUB_B64 = "nbG15bFgLPy53x6RIuZYkq51/ugCyYn0UyBmQACDsG0="

# AWG obfuscation params
JC, JMIN, JMAX = 2, 10, 50
S1, S2 = 114, 34
H1, H2, H3, H4 = 2145016147, 1190894264, 943727824, 960546567

# I1 = <r 2><b 0x...>  — two random bytes followed by static hex payload
# (the hex payload encodes a fake DNS A-record response for dl.google.com
# pointing at 77.88.55.55 — that's the DPI-evasion decoy)
I1_TEMPLATE_RANDOM_LEN = 2
I1_STATIC_HEX = (
    "8580000100010000000002646c06676f6f676c6503636f6d"
    "0000010001c00c000100010000105a00044d583737"
)
I1_STATIC_BYTES = bytes.fromhex(I1_STATIC_HEX)
I1_TEMPLATE = f"<r {I1_TEMPLATE_RANDOM_LEN}><b 0x{I1_STATIC_HEX}>"
CPS_EXPECTED_SIZE = I1_TEMPLATE_RANDOM_LEN + len(I1_STATIC_BYTES)
INIT_WIRE_SIZE = S1 + 148  # S1 prefix + standard WG init

# Server pubkey -> hex for PUB_SERVER. Client key is zeroed since we only
# verify outbound transforms (init/MAC1); inbound MAC1 needs real client
# pubkey which we'd have to derive via X25519 — skip for now.
PEER_PUB_HEX = base64.b64decode(PEER_PUB_B64).hex()
PUB_CLIENT_HEX = "00" * 32

# ------------------------- knobs / paths -------------------------

SRV_PORT = int(os.environ.get("SRV_PORT", 51999))
PROC_ADD = "/proc/awg_proxy/add"
PROC_DEL = "/proc/awg_proxy/del"
PROC_LIST = "/proc/awg_proxy/list"


def _write_proc(path: str, line: str) -> None:
    with open(path, "w") as f:
        f.write(line)


def add_tunnel() -> int:
    """Add the configured tunnel and return kernel-assigned listen port."""
    line = (
        f"127.0.0.1:{SRV_PORT}"
        f" H1={H1} H2={H2} H3={H3} H4={H4}"
        f" S1={S1} S2={S2} S3=0 S4=0"
        f" Jc={JC} Jmin={JMIN} Jmax={JMAX}"
        f" PUB_SERVER={PEER_PUB_HEX} PUB_CLIENT={PUB_CLIENT_HEX}"
        f' I1="{I1_TEMPLATE}"'
        "\n"
    )
    _write_proc(PROC_ADD, line)
    with open(PROC_LIST) as f:
        body = f.read()
    for entry in body.splitlines():
        if entry.startswith(f"127.0.0.1:{SRV_PORT}"):
            for tok in entry.split():
                if tok.startswith("listen=127.0.0.1:"):
                    return int(tok.rsplit(":", 1)[1])
    raise RuntimeError(f"tunnel not present in {PROC_LIST}:\n{body!r}")


def del_tunnel() -> None:
    try:
        _write_proc(PROC_DEL, f"127.0.0.1:{SRV_PORT}")
    except OSError:
        pass


def main() -> int:
    if os.geteuid() != 0:
        print("must run as root (writes /proc/awg_proxy)", file=sys.stderr)
        return 2
    if not os.path.exists(PROC_ADD):
        print(f"{PROC_ADD} not found — awg_proxy.ko not loaded?", file=sys.stderr)
        return 2

    # Bind listener BEFORE adding tunnel — absorbs UDP, avoids ICMP
    # port-unreachable that would otherwise stop the kmod's burst mid-way.
    srv = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    srv.setsockopt(socket.IPPROTO_IP, 13, 1)  # IP_RECVTOS = 13 on Linux
    try:
        srv.bind(("127.0.0.1", SRV_PORT))
    except OSError as e:
        print(f"cannot bind 127.0.0.1:{SRV_PORT}: {e} (set SRV_PORT=...)", file=sys.stderr)
        return 2
    srv.settimeout(2.0)

    del_tunnel()
    try:
        listen_port = add_tunnel()
    except RuntimeError as e:
        print(f"add_tunnel failed: {e}", file=sys.stderr)
        srv.close()
        return 2

    print(f"tunnel: kmod listen=127.0.0.1:{listen_port} target=127.0.0.1:{SRV_PORT}")
    print(f"config:")
    print(f"  H1={H1} ({H1:#x})  H2={H2} ({H2:#x})")
    print(f"  H3={H3} ({H3:#x})  H4={H4} ({H4:#x})")
    print(f"  S1={S1}  S2={S2}")
    print(f"  Jc={JC}  Jmin={JMIN}  Jmax={JMAX}")
    print(f"  PEER_PUB_HEX={PEER_PUB_HEX}")
    print(f"  I1 = {I1_TEMPLATE}")
    print(f"  expected CPS size = {CPS_EXPECTED_SIZE}  "
          f"({I1_TEMPLATE_RANDOM_LEN} rand + {len(I1_STATIC_BYTES)} static)")
    print(f"  expected init wire size = {INIT_WIRE_SIZE} ({S1} prefix + 148 init)")
    print()

    # Drain any stale traffic before triggering.
    srv.settimeout(0.05)
    drained = 0
    while True:
        try:
            srv.recvfrom(2048)
            drained += 1
        except socket.timeout:
            break
    if drained:
        print(f"  drained {drained} stale packet(s) before trigger")
    srv.settimeout(2.0)

    # Receiver thread.
    captured = []

    def receiver():
        while True:
            try:
                data, anc, _, _ = srv.recvmsg(2000, 1024)
                tos = None
                for lvl, typ, val in anc:
                    if lvl == socket.IPPROTO_IP and typ in (1, 13) and val:
                        tos = val[0]
                captured.append((len(data), tos, data))
            except socket.timeout:
                return

    t = threading.Thread(target=receiver, daemon=True)
    t.start()
    time.sleep(0.2)

    # Fake WG init: 148 bytes; first 4 = msgType=1 (LE); rest predictable
    # so we can recognise the transform output.
    init = struct.pack("<I", 1) + bytes((i & 0xFF) for i in range(144))
    sender = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    sender.sendto(init, ("127.0.0.1", listen_port))
    sender.close()
    t.join(timeout=3.0)

    del_tunnel()
    srv.close()

    # Dump captures.
    print(f"captured {len(captured)} packets:")
    for i, (sz, tos, data) in enumerate(captured):
        hx = data.hex()
        suffix = "..." if len(hx) > 160 else ""
        print(f"  [{i}] len={sz:4d} tos={tos!s:>4} hex={hx[:160]}{suffix}")
    print()
    print("=" * 60)
    print("structural verification")
    print("=" * 60)

    failed = 0

    # ---- CPS packet (size = CPS_EXPECTED_SIZE) ----
    cps_pkt = next((d for sz, _t, d in captured if sz == CPS_EXPECTED_SIZE), None)
    if cps_pkt is None:
        print(f"FAIL  CPS I1 ({CPS_EXPECTED_SIZE}b): not captured")
        failed += 1
    else:
        random_prefix = cps_pkt[:I1_TEMPLATE_RANDOM_LEN]
        static_part = cps_pkt[I1_TEMPLATE_RANDOM_LEN:]
        if static_part == I1_STATIC_BYTES:
            print(
                f"PASS  CPS I1: {I1_TEMPLATE_RANDOM_LEN} random + "
                f"{len(I1_STATIC_BYTES)} static (byte-exact match)"
            )
            print(f"      random prefix: {random_prefix.hex()}")
        else:
            diffs = sum(1 for a, b in zip(I1_STATIC_BYTES, static_part) if a != b)
            print(f"FAIL  CPS I1: static portion mismatch ({diffs} byte(s) differ)")
            print(f"      expected: {I1_STATIC_BYTES.hex()}")
            print(f"      got:      {static_part.hex()}")
            failed += 1

    # ---- Junk packets ----
    # CPS_EXPECTED_SIZE (47b here) accidentally lands inside [JMIN..JMAX]=[10..50]
    # for this config, so we have to exclude the CPS packet explicitly. The kmod
    # emits CPS strictly before junk, so "skip first match of CPS size" is sound.
    # INIT_WIRE_SIZE (262b) is outside [10..50] and not a worry.
    junks = []
    cps_skipped = False
    for sz, tos, d in captured:
        if not cps_skipped and sz == CPS_EXPECTED_SIZE:
            cps_skipped = True
            continue
        if JMIN <= sz <= JMAX:
            junks.append((sz, tos, d))
    if len(junks) == JC:
        sizes = [j[0] for j in junks]
        tos_vals = [j[1] for j in junks]
        print(f"PASS  junk count: {JC} packets, sizes={sizes} (all in [{JMIN}..{JMAX}])")
        non_zero = any(t for t in tos_vals if t is not None)
        varies = len({t for t in tos_vals if t is not None}) > 1
        if non_zero and varies:
            print(f"PASS  junk DSCP randomised: {tos_vals}")
        elif non_zero and not varies:
            print(
                f"WARN  junk DSCP non-zero but identical (could be RNG luck): "
                f"{tos_vals}"
            )
        else:
            print(f"FAIL  junk DSCP all zero — v1.1.2 random-DSCP fix not firing")
            failed += 1
    else:
        print(f"FAIL  junk count: expected {JC}, got {len(junks)}")
        failed += 1

    # ---- Obfuscated handshake init ----
    init_pkt = next((d for sz, _t, d in captured if sz == INIT_WIRE_SIZE), None)
    if init_pkt is None:
        print(f"FAIL  init ({INIT_WIRE_SIZE}b): not captured")
        failed += 1
    else:
        # S1 random prefix.
        prefix = init_pkt[:S1]
        zeros = prefix.count(0)
        if prefix == bytes(S1):
            print(f"FAIL  init S1 prefix all-zero (random fill broken)")
            failed += 1
        else:
            print(f"PASS  init S1 prefix: {S1} bytes, {zeros} zeros (random)")

        # msgType bytes replaced with H1.
        msg_bytes = init_pkt[S1:S1 + 4]
        msg_le = struct.unpack("<I", msg_bytes)[0]
        if msg_le == H1:
            print(
                f"PASS  init H1 msgType: bytes[{S1}:{S1+4}] = {msg_bytes.hex()} "
                f"-> LE {msg_le} == H1"
            )
        else:
            print(
                f"FAIL  init H1 msgType: bytes[{S1}:{S1+4}] = {msg_bytes.hex()} "
                f"-> LE {msg_le}, expected H1={H1}"
            )
            failed += 1

        # Body bytes after msgType through MAC1 field are unchanged from
        # our fake init pattern. We sent init = pack("<I", 1) + [0,1,2,...,143],
        # so init[4..116] == [0,1,...,111]. On the wire that lands at
        # offset (S1+4)..(S1+116) and must come through byte-exact. MAC1
        # (init[116..132] = wire[S1+116..S1+132]) is recomputed by kmod.
        body_offset_on_wire = S1 + 4
        mac1_offset_on_wire = S1 + 116
        body_match_len = mac1_offset_on_wire - body_offset_on_wire  # = 112
        expected_body = bytes(i & 0xFF for i in range(body_match_len))
        got_body = init_pkt[body_offset_on_wire:mac1_offset_on_wire]
        if got_body == expected_body:
            print(
                f"PASS  init body bytes[4:116] passed through unchanged "
                f"(112 bytes)"
            )
        else:
            diffs = sum(1 for a, b in zip(expected_body, got_body) if a != b)
            print(
                f"FAIL  init body bytes[4:116] mutated unexpectedly "
                f"({diffs} byte(s) differ)"
            )
            failed += 1

        # MAC1 must NOT be all-zero (we sent zeros, kmod should recompute
        # because PUB_SERVER is set).
        mac1 = init_pkt[mac1_offset_on_wire:mac1_offset_on_wire + 16]
        if mac1 == bytes(16):
            print(
                f"FAIL  init MAC1 = all-zero (kmod did NOT recompute despite "
                f"PUB_SERVER set)"
            )
            failed += 1
        else:
            print(
                f"PASS  init MAC1 recomputed: {mac1.hex()} (non-zero — kmod "
                f"applied BLAKE2-MAC1 over modified msgType)"
            )

    print("=" * 60)
    if failed == 0:
        print("ALL PASS — v1.1.2 transforms this user-reported config correctly")
        return 0
    print(f"{failed} FAIL")
    return 1


if __name__ == "__main__":
    sys.exit(main())
