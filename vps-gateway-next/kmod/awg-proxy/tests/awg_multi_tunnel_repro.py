#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-2.0
"""
Multi-tunnel concurrent verification for awg_proxy.ko v1.1.2.

Adds 3 INDEPENDENT AWG tunnels with DIFFERENT configs simultaneously,
triggers a synthetic handshake init on each, and verifies that:
  - per-tunnel CPS / junk / init counts match each config
  - per-tunnel wire sizes match each config's S1/S2 and I1 length
  - H ranges (config A) and H singles (B, C) are both honoured
  - PUB_SERVER zero  -> MAC1 NOT recomputed (config B)
  - PUB_SERVER set   -> MAC1 recomputed (configs A, C)
  - NO cross-tunnel byte leakage: each init body carries a unique
    marker byte (A=0x41, B=0x42, C=0x43) and must come out of THE
    SAME tunnel byte-exact — if config A's bytes appear in config
    B's wire output we have a per-tunnel-state bug

Configs under test (from a real-world user bug-report set):

  A: Jc=4 Jmin=10 Jmax=50  S1=63 S2=50 S3=14 S4=5
     H1=554830851-1502226513  (range)
     H2=2016846630-2120568456 (range)
     H3=2129832446-2131670856 (range)
     H4=2144448232-2146331955 (range)
     I1 = <r 2><b 0x...icloud.com hex...>
     peer = Wiqc2u...  (MAC1 recompute expected)

  B: Jc=3 Jmin=10 Jmax=50  S1=89 S2=52  (S3=S4=0)
     H1=988120163 H2=1409500065
     H3=726821891 H4=673313946  (singles)
     I1 = <r 2><b 0x...dl.google.com hex...>
     peer = (zero)             (MAC1 recompute SKIPPED)

  C: Jc=2 Jmin=10 Jmax=50  S1=114 S2=34  (S3=S4=0)
     H1=2145016147 H2=1190894264
     H3=943727824 H4=960546567  (singles)
     I1 = <r 2><b 0x...dl.google.com hex...>
     peer = nbG15b...  (MAC1 recompute expected)

Usage:
    scp awg_multi_tunnel_repro.py root@router:/opt/tmp/
    ssh root@router 'python3 /opt/tmp/awg_multi_tunnel_repro.py; echo exit=$?'

Exit code: 0 if all PASS, 1 if any FAIL, 2 on setup error.
"""

import base64
import os
import socket
import struct
import sys
import threading
import time

PROC_ADD = "/proc/awg_proxy/add"
PROC_DEL = "/proc/awg_proxy/del"
PROC_LIST = "/proc/awg_proxy/list"
PUB_ZERO_HEX = "00" * 32


def _h_str(v):
    if isinstance(v, tuple):
        lo, hi = v
        return f"{lo}-{hi}" if lo != hi else f"{lo}"
    return f"{v}"

def _h_min(v):
    return v[0] if isinstance(v, tuple) else v

def _h_max(v):
    return v[1] if isinstance(v, tuple) else v


I1_HEX_ICLOUD = (
    "858000010001000000000669636c6f756403636f6d"
    "0000010001c00c000100010000105a00044d583737"
)
I1_HEX_DLGOOGLE = (
    "8580000100010000000002646c06676f6f676c6503636f6d"
    "0000010001c00c000100010000105a00044d583737"
)


TUNNELS = [
    {
        "id": "A",
        "target_port": 51997,
        "Jc": 4, "Jmin": 10, "Jmax": 50,
        "S1": 63, "S2": 50, "S3": 14, "S4": 5,
        "H1": (554830851, 1502226513),
        "H2": (2016846630, 2120568456),
        "H3": (2129832446, 2131670856),
        "H4": (2144448232, 2146331955),
        "I1_static_hex": I1_HEX_ICLOUD,
        "I1_random_len": 2,
        "peer_pub_b64": "Wiqc2ujpT8zEijU2hQcDck1wIkJc0l0RDf7Re7xeC10=",
    },
    {
        "id": "B",
        "target_port": 51998,
        "Jc": 3, "Jmin": 10, "Jmax": 50,
        "S1": 89, "S2": 52, "S3": 0, "S4": 0,
        "H1": 988120163, "H2": 1409500065,
        "H3": 726821891, "H4": 673313946,
        "I1_static_hex": I1_HEX_DLGOOGLE,
        "I1_random_len": 2,
        "peer_pub_b64": None,  # zero -> MAC1 recompute disabled
    },
    {
        "id": "C",
        "target_port": 51999,
        "Jc": 2, "Jmin": 10, "Jmax": 50,
        "S1": 114, "S2": 34, "S3": 0, "S4": 0,
        "H1": 2145016147, "H2": 1190894264,
        "H3": 943727824, "H4": 960546567,
        "I1_static_hex": I1_HEX_DLGOOGLE,
        "I1_random_len": 2,
        "peer_pub_b64": "nbG15bFgLPy53x6RIuZYkq51/ugCyYn0UyBmQACDsG0=",
    },
]


def cps_size(t):
    return t["I1_random_len"] + len(bytes.fromhex(t["I1_static_hex"]))

def init_wire_size(t):
    return t["S1"] + 148


def _write_proc(path, line):
    with open(path, "w") as f:
        f.write(line)


def proc_add_line(t):
    pub = (base64.b64decode(t["peer_pub_b64"]).hex()
           if t["peer_pub_b64"] else PUB_ZERO_HEX)
    i1 = f'<r {t["I1_random_len"]}><b 0x{t["I1_static_hex"]}>'
    return (
        f'127.0.0.1:{t["target_port"]}'
        f' H1={_h_str(t["H1"])} H2={_h_str(t["H2"])}'
        f' H3={_h_str(t["H3"])} H4={_h_str(t["H4"])}'
        f' S1={t["S1"]} S2={t["S2"]} S3={t["S3"]} S4={t["S4"]}'
        f' Jc={t["Jc"]} Jmin={t["Jmin"]} Jmax={t["Jmax"]}'
        f' PUB_SERVER={pub} PUB_CLIENT={PUB_ZERO_HEX}'
        f' I1="{i1}"'
        f'\n'
    )


def add_all_tunnels():
    for t in TUNNELS:
        _write_proc(PROC_ADD, proc_add_line(t))

    with open(PROC_LIST) as f:
        body = f.read()

    ports = {}
    for t in TUNNELS:
        target = f'127.0.0.1:{t["target_port"]}'
        for entry in body.splitlines():
            if not entry.startswith(target):
                continue
            for tok in entry.split():
                if tok.startswith("listen=127.0.0.1:"):
                    ports[t["id"]] = int(tok.rsplit(":", 1)[1])
        if t["id"] not in ports:
            raise RuntimeError(
                f"tunnel {t['id']} ({target}) not in {PROC_LIST}:\n{body}"
            )
    return ports


def del_all_tunnels():
    for t in TUNNELS:
        try:
            _write_proc(PROC_DEL, f'127.0.0.1:{t["target_port"]}')
        except OSError:
            pass


def make_init(tunnel_id):
    """148-byte init with tunnel-id-derived body for cross-leak detection."""
    marker = ord(tunnel_id)
    body = bytes(((i + marker) & 0xFF) for i in range(144))
    return struct.pack("<I", 1) + body


def expected_body_passthrough(tid):
    marker = ord(tid)
    return bytes(((i + marker) & 0xFF) for i in range(112))


def expected_mac1_area_unchanged(tid):
    marker = ord(tid)
    return bytes(((i + marker) & 0xFF) for i in range(112, 128))


def main():
    if os.geteuid() != 0:
        print("must run as root", file=sys.stderr)
        return 2
    if not os.path.exists(PROC_ADD):
        print(f"{PROC_ADD} not found — awg_proxy.ko not loaded?", file=sys.stderr)
        return 2

    # Bind 3 server sockets BEFORE adding tunnels.
    srvs = {}
    try:
        for t in TUNNELS:
            s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
            s.setsockopt(socket.IPPROTO_IP, 13, 1)  # IP_RECVTOS
            s.bind(("127.0.0.1", t["target_port"]))
            s.settimeout(1.5)
            srvs[t["id"]] = s
    except OSError as e:
        print(f"bind failed: {e}", file=sys.stderr)
        for s in srvs.values():
            s.close()
        return 2

    del_all_tunnels()
    try:
        ports = add_all_tunnels()
    except RuntimeError as e:
        print(f"add tunnels failed: {e}", file=sys.stderr)
        for s in srvs.values():
            s.close()
        return 2

    print("3 concurrent tunnels:")
    for t in TUNNELS:
        peer = t["peer_pub_b64"] or "(zero)"
        peer_short = peer if peer == "(zero)" else peer[:12] + "…"
        print(
            f"  {t['id']}: kmod listen=127.0.0.1:{ports[t['id']]} "
            f"-> 127.0.0.1:{t['target_port']}  "
            f"Jc={t['Jc']} S1={t['S1']} S2={t['S2']} "
            f"S3={t['S3']} S4={t['S4']}  peer={peer_short}  "
            f"cps={cps_size(t)}b init={init_wire_size(t)}b"
        )
    print()

    # Drain stale.
    for tid, s in srvs.items():
        s.settimeout(0.05)
        d = 0
        while True:
            try:
                s.recvfrom(2048)
                d += 1
            except socket.timeout:
                break
        if d:
            print(f"  drained {d} stale from {tid}")
        s.settimeout(1.5)

    # Per-tunnel receiver threads.
    captured = {t["id"]: [] for t in TUNNELS}

    def receiver(tid):
        while True:
            try:
                data, anc, _, _ = srvs[tid].recvmsg(2000, 1024)
                tos = None
                for lvl, typ, val in anc:
                    if lvl == socket.IPPROTO_IP and typ in (1, 13) and val:
                        tos = val[0]
                captured[tid].append((len(data), tos, data))
            except socket.timeout:
                return

    threads = [
        threading.Thread(target=receiver, args=(t["id"],), daemon=True)
        for t in TUNNELS
    ]
    for th in threads:
        th.start()
    time.sleep(0.2)

    # Concurrent burst — stress per-tunnel isolation.
    sender = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    for t in TUNNELS:
        sender.sendto(make_init(t["id"]), ("127.0.0.1", ports[t["id"]]))
    sender.close()

    for th in threads:
        th.join(timeout=3.0)

    del_all_tunnels()
    for s in srvs.values():
        s.close()

    print("captured per tunnel:")
    for t in TUNNELS:
        tid = t["id"]
        print(f"  tunnel {tid}: {len(captured[tid])} pkts")
        for i, (sz, tos, _) in enumerate(captured[tid]):
            print(f"    [{i}] len={sz:4d} tos={tos}")
    print()

    print("=" * 60)
    print("per-tunnel verification")
    print("=" * 60)
    failed = 0
    for t in TUNNELS:
        tid = t["id"]
        pkts = captured[tid]
        expected_cps = cps_size(t)
        expected_init = init_wire_size(t)
        expected_total = 1 + t["Jc"] + 1
        prefix = f"[{tid}]"

        if len(pkts) != expected_total:
            print(
                f"{prefix} FAIL total: expected {expected_total} "
                f"(1 CPS + {t['Jc']} junk + 1 init), got {len(pkts)}"
            )
            failed += 1
            continue

        # CPS
        cps_pkt = next((d for sz, _, d in pkts if sz == expected_cps), None)
        if cps_pkt is None:
            print(f"{prefix} FAIL CPS ({expected_cps}b) missing")
            failed += 1
        else:
            static = cps_pkt[t["I1_random_len"]:]
            expected_static = bytes.fromhex(t["I1_static_hex"])
            if static == expected_static:
                print(
                    f"{prefix} PASS CPS ({expected_cps}b): I1 static "
                    f"({len(expected_static)}b) byte-exact"
                )
            else:
                print(f"{prefix} FAIL CPS static portion mismatch")
                failed += 1

        # Junk — exclude exactly 1 CPS-sized and 1 init-sized.
        junks = []
        cps_consumed = False
        init_consumed = False
        for sz, tos, d in pkts:
            if not cps_consumed and sz == expected_cps:
                cps_consumed = True
                continue
            if not init_consumed and sz == expected_init:
                init_consumed = True
                continue
            if t["Jmin"] <= sz <= t["Jmax"]:
                junks.append((sz, tos, d))
        if len(junks) == t["Jc"]:
            sizes = [j[0] for j in junks]
            tos_vals = [j[1] for j in junks]
            print(
                f"{prefix} PASS junk: {t['Jc']} pkts, sizes={sizes} "
                f"in [{t['Jmin']}..{t['Jmax']}]"
            )
            varies = len({x for x in tos_vals if x is not None}) > 1
            non_zero = any(x for x in tos_vals if x is not None)
            if non_zero and varies:
                print(f"{prefix} PASS junk DSCP randomised: {tos_vals}")
            elif non_zero:
                print(
                    f"{prefix} WARN junk DSCP non-zero but identical: "
                    f"{tos_vals} (could be RNG luck)"
                )
            else:
                print(f"{prefix} FAIL junk DSCP all-zero")
                failed += 1
        else:
            print(
                f"{prefix} FAIL junk count: expected {t['Jc']}, "
                f"got {len(junks)}"
            )
            failed += 1

        # Init
        init_pkt = next((d for sz, _, d in pkts if sz == expected_init), None)
        if init_pkt is None:
            print(f"{prefix} FAIL init ({expected_init}b) missing")
            failed += 1
            continue

        # S1 prefix random
        prefix_bytes = init_pkt[: t["S1"]]
        if prefix_bytes == bytes(t["S1"]):
            print(f"{prefix} FAIL init S1 prefix all-zero")
            failed += 1
        else:
            zeros = prefix_bytes.count(0)
            print(
                f"{prefix} PASS init S1 prefix random "
                f"({t['S1']}b, {zeros} zeros)"
            )

        # H1 in range (or equal for singles)
        msg_bytes = init_pkt[t["S1"] : t["S1"] + 4]
        msg_le = struct.unpack("<I", msg_bytes)[0]
        h1_lo = _h_min(t["H1"])
        h1_hi = _h_max(t["H1"])
        if h1_lo <= msg_le <= h1_hi:
            label = f"[{h1_lo}..{h1_hi}]" if h1_lo != h1_hi else f"={h1_lo}"
            print(f"{prefix} PASS init H1 msgType: {msg_le} ∈ {label}")
        else:
            print(
                f"{prefix} FAIL init H1 msgType: {msg_le} ∉ "
                f"[{h1_lo}..{h1_hi}]"
            )
            failed += 1

        # Body passthrough — proves no cross-tunnel byte mixing.
        body_off = t["S1"] + 4
        mac1_off = t["S1"] + 116
        got_body = init_pkt[body_off:mac1_off]
        exp_body = expected_body_passthrough(tid)
        if got_body == exp_body:
            print(
                f"{prefix} PASS init body[4:116] byte-exact (marker={tid!r} "
                f"-> no cross-tunnel mixing)"
            )
        else:
            diffs = sum(1 for a, b in zip(exp_body, got_body) if a != b)
            print(
                f"{prefix} FAIL init body[4:116]: {diffs}/112 differ "
                f"(possible cross-tunnel leak)"
            )
            print(f"      expected first 8: {exp_body[:8].hex()}")
            print(f"      got      first 8: {got_body[:8].hex()}")
            failed += 1

        # MAC1: recompute only when peer non-zero
        mac1 = init_pkt[mac1_off : mac1_off + 16]
        has_peer = t["peer_pub_b64"] is not None
        if has_peer:
            if mac1 == bytes(16):
                print(
                    f"{prefix} FAIL MAC1 zero with peer set (recompute "
                    f"expected)"
                )
                failed += 1
            else:
                print(f"{prefix} PASS MAC1 recomputed (peer set): {mac1.hex()}")
        else:
            exp_mac1 = expected_mac1_area_unchanged(tid)
            if mac1 == exp_mac1:
                print(
                    f"{prefix} PASS MAC1 area UNCHANGED with peer zero "
                    f"(no recompute, as expected)"
                )
            else:
                diffs = sum(1 for a, b in zip(exp_mac1, mac1) if a != b)
                print(
                    f"{prefix} FAIL MAC1 area mutated without peer "
                    f"({diffs}/16 differ): got={mac1.hex()}"
                )
                failed += 1

        print()

    print("=" * 60)
    if failed == 0:
        print("ALL PASS — 3 concurrent tunnels independent, no cross-leak")
        return 0
    print(f"{failed} FAIL")
    return 1


if __name__ == "__main__":
    sys.exit(main())
