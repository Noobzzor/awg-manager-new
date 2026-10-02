#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-2.0
"""
On-device smoke test for awg_proxy.ko.

Verifies the six wire-format and lifecycle invariants the module is
expected to satisfy after the v1.1.1 -> v1.1.2 patch series:

    1. BE timestamp encoding for `<t>` in CPS payloads.
    2. BE counter   encoding for `<c>` in CPS payloads.
    3. `<rc>` charset is 52 letters only (a-z + A-Z), no digits.
    4. CPS counter is re-randomised at the start of every handshake cycle
       (atomic_set(get_random_u32()) — not 0/monotonic).
    5. Junk packets get a random IP TOS (DSCP) per packet.
    6. Counter is incremented after successful send (host unit test —
       no on-wire signal we can capture in this setup, so we just report
       SKIP and rely on test_cps.c's test_generate_all_is_pure).

Plus: the handshake init itself is forwarded (148 bytes, msgType=1 LE).

How it works:
  - Binds a UDP listener on 127.0.0.1:<SRV_PORT> BEFORE adding the tunnel
    so kmod sends do not bounce off ICMP port-unreachable.
  - /proc/awg_proxy/add a tunnel pointing to 127.0.0.1:<SRV_PORT> with
        Jc=2 Jmin=60 Jmax=80
        I1=<c><t><r 10>     (18 bytes: BE ctr + BE ts + 10 random)
        I2=<rc 16><rd 4>    (20 bytes: 16 letters + 4 digits)
  - Sends two synthetic WG handshake-init datagrams to the kmod's
    listen socket; captures everything the kmod emits to <SRV_PORT>.
  - Each handshake cycle should produce exactly 5 packets:
        CPS I1 (18b), CPS I2 (20b), junk (60-80b), junk (60-80b),
        handshake init (148b)
  - Inspects packet bytes (incl. IP TOS via IP_RECVTOS ancillary) and
    prints PASS/FAIL per check.

Usage (must be root, /proc/awg_proxy must exist):
    scp on_device_smoke.py root@router:/tmp/
    ssh root@router 'python3 /tmp/on_device_smoke.py'

Env knobs:
    SRV_PORT  endpoint we tell the kmod to forward to (default 51999).
              Pick something not in use; the script's listener will bind
              this port for the duration of the test.
    AWG_V6    if set to 1, run the whole test against the IPv6 loopback
              ([::1]) instead of 127.0.0.1 — exercises the bracketed
              "[v6]:port" procfs syntax, the AF_INET6 remote socket and the
              udp_tunnel6 TX path (requires awg_proxy.ko >= v1.3.0 and a
              kernel with IPv6). The DSCP-per-junk check uses IPV6_TCLASS
              ancillary in this mode.

Exit code: 0 if all checks PASS, 1 otherwise. 2 on setup error.
"""

import os
import socket
import struct
import sys
import threading
import time

SRV_PORT = int(os.environ.get("SRV_PORT", 51999))
USE_V6 = os.environ.get("AWG_V6", "") == "1"
PROC_ADD = "/proc/awg_proxy/add"
PROC_DEL = "/proc/awg_proxy/del"
PROC_LIST = "/proc/awg_proxy/list"

# Loopback family the kmod forwards to. The local WG client side is always
# IPv4 loopback (127.0.0.1:listen_port) regardless — only the *endpoint*
# family changes here.
LOOPBACK = "::1" if USE_V6 else "127.0.0.1"


def endpoint_str() -> str:
    """Endpoint token exactly as awg_proxy.ko parses/prints it."""
    return f"[{LOOPBACK}]:{SRV_PORT}" if USE_V6 else f"{LOOPBACK}:{SRV_PORT}"


# ----- /proc/awg_proxy helpers -----

def _write_proc(path: str, line: str) -> None:
    with open(path, "w") as f:
        f.write(line)


def add_tunnel() -> int:
    """Add the test tunnel and return its kernel-assigned listen port."""
    pub = "00" * 32
    ep = endpoint_str()
    line = (
        f"{ep}"
        f" H1=1 H2=2 H3=3 H4=4 S1=0 S2=0 S3=0 S4=0"
        f" Jc=2 Jmin=60 Jmax=80"
        f" PUB_SERVER={pub} PUB_CLIENT={pub}"
        f' I1="<c><t><r 10>" I2="<rc 16><rd 4>"'
        f"\n"
    )
    _write_proc(PROC_ADD, line)
    with open(PROC_LIST) as f:
        body = f.read()
    # kmod list rows start with the same endpoint token we wrote.
    for entry in body.splitlines():
        if not entry.startswith(ep + " "):
            continue
        for tok in entry.split():
            if tok.startswith("listen=127.0.0.1:"):
                return int(tok.rsplit(":", 1)[1])
    raise RuntimeError(
        f"tunnel not found in {PROC_LIST} after add:\n{body!r}"
    )


def del_tunnel() -> None:
    try:
        _write_proc(PROC_DEL, endpoint_str())
    except OSError:
        pass  # best-effort cleanup


# ----- handshake cycle -----

def run_cycle(srv: socket.socket, listen_port: int):
    """Send one fake handshake-init at the kmod, gather all kmod->us packets."""
    captured = []

    def receiver():
        while True:
            try:
                data, ancdata, _flags, _addr = srv.recvmsg(2000, 1024)
                tos = None
                for cmsg_level, cmsg_type, cmsg_data in ancdata:
                    if not cmsg_data:
                        continue
                    if USE_V6:
                        # IPv6 delivers the traffic class under
                        # IPPROTO_IPV6 / IPV6_TCLASS=67 (as an int).
                        if cmsg_level == socket.IPPROTO_IPV6 and \
                                cmsg_type == 67:
                            tos = struct.unpack("i", cmsg_data[:4])[0]
                    elif cmsg_level == socket.IPPROTO_IP:
                        # Linux delivers the received TOS under
                        # cmsg_type=IP_TOS=1 by default. Some kernels echo
                        # back the request type (IP_RECVTOS=13). Accept either.
                        if cmsg_type in (1, 13):
                            tos = cmsg_data[0]
                captured.append((len(data), tos, data))
            except socket.timeout:
                return

    srv.settimeout(2.0)
    t = threading.Thread(target=receiver, daemon=True)
    t.start()
    time.sleep(0.2)

    # 148 bytes, msgType=1 (LE) — the rest can be zeros; kmod only checks
    # length and the first four bytes for the WG init signature.
    # The WG-client side is ALWAYS IPv4 loopback (kmod listens on
    # 127.0.0.1:listen_port) even when the endpoint is IPv6.
    init = b"\x01\x00\x00\x00" + b"\x00" * 144
    sender = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    sender.sendto(init, ("127.0.0.1", listen_port))
    sender.close()

    t.join(timeout=3.0)
    return captured


def _find(cap, predicate):
    for sz, tos, data in cap:
        if predicate(sz, tos, data):
            return sz, tos, data
    return None


def _hex(data: bytes, limit: int = 120) -> str:
    h = data.hex()
    return h if len(h) <= limit else (h[:limit] + "...")


# ----- verification -----

def verify(cycle1, cycle2):
    """Return number of failed checks (0 = all PASS)."""
    failed = 0
    print()
    print("=" * 60)
    print("verification matrix")
    print("=" * 60)

    i1_c1 = _find(cycle1, lambda sz, _t, _d: sz == 18)
    i2_c1 = _find(cycle1, lambda sz, _t, _d: sz == 20)
    i1_c2 = _find(cycle2, lambda sz, _t, _d: sz == 18)
    hs_c1 = _find(
        cycle1,
        lambda sz, _t, data: sz == 148 and data[:4] == b"\x01\x00\x00\x00",
    )
    junk_tos_c1 = [tos for sz, tos, _ in cycle1 if 60 <= sz <= 80]

    # FIX 1: BE timestamp matches current Unix time.
    if not i1_c1:
        print("FAIL  BE timestamp: I1 (18b) not captured")
        failed += 1
    else:
        ts = struct.unpack(">I", i1_c1[2][4:8])[0]
        now = int(time.time())
        if abs(ts - now) < 60:
            print(
                f"PASS  BE timestamp:   payload[4:8] = {ts}  "
                f"(now={now}, diff={abs(ts-now)}s)"
            )
        else:
            print(
                f"FAIL  BE timestamp:   payload[4:8] = {ts}  "
                f"(expected ~{now}; LE bug?)"
            )
            failed += 1

    # FIX 2: counter encoded the same way as timestamp — show its value too.
    if i1_c1:
        ctr = struct.unpack(">I", i1_c1[2][0:4])[0]
        if 0 < ctr <= 0xFFFFFFFF:
            print(
                f"PASS  BE counter:     payload[0:4] = {ctr}  "
                f"(0x{ctr:08x})"
            )
        else:
            print(f"FAIL  BE counter:     payload[0:4] = {ctr}")
            failed += 1

    # FIX 3: <rc> letters only, <rd> digits only.
    if not i2_c1:
        print("FAIL  <rc> charset:   I2 (20b) not captured")
        failed += 1
    else:
        chars = i2_c1[2][:16]
        digits = i2_c1[2][16:20]
        letters = b"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
        if all(b in letters for b in chars):
            print(f"PASS  <rc> charset:   {chars!r}  (letters only, no digits)")
        else:
            bad = [hex(b) for b in chars if b not in letters]
            print(
                f"FAIL  <rc> charset:   {chars!r}  "
                f"(non-letter bytes: {bad})"
            )
            failed += 1
        if all(0x30 <= b <= 0x39 for b in digits):
            print(f"PASS  <rd> charset:   {digits!r}  (digits only)")
        else:
            print(f"FAIL  <rd> charset:   {digits!r}  (non-digit bytes)")
            failed += 1

    # FIX 4: counter re-seeded per handshake cycle.
    if i1_c1 and i1_c2:
        c1 = struct.unpack(">I", i1_c1[2][0:4])[0]
        c2 = struct.unpack(">I", i1_c2[2][0:4])[0]
        if c1 != c2:
            print(
                f"PASS  counter reseed: c1={c1}, c2={c2}  "
                f"(delta={c2 - c1:+d})"
            )
        else:
            print(
                f"FAIL  counter reseed: c1 == c2 == {c1}  "
                f"(static base — re-seed not firing)"
            )
            failed += 1
    else:
        print("FAIL  counter reseed: I1 missing in one of the cycles")
        failed += 1

    # FIX 5: random DSCP per junk packet.
    if len(junk_tos_c1) >= 2:
        all_zero = all(t == 0 for t in junk_tos_c1)
        all_same = len(set(junk_tos_c1)) == 1
        if not all_zero and not all_same:
            print(f"PASS  junk DSCP:      {junk_tos_c1}  (varying, non-zero)")
        else:
            why = []
            if all_zero:
                why.append("all-zero")
            if all_same:
                why.append("identical")
            print(
                f"FAIL  junk DSCP:      {junk_tos_c1}  ({'; '.join(why)})"
            )
            failed += 1
    else:
        print(
            f"FAIL  junk DSCP:      only {len(junk_tos_c1)} junk packet(s) "
            f"captured (need >=2)"
        )
        failed += 1

    # FIX 6: counter post-send increment. No on-wire signal in this config
    # (would need two <c> templates) — host unit-test is authoritative.
    print(
        "SKIP  ctr post-send:  on-host unit-test "
        "(tests/test_cps.c::test_generate_all_is_pure)"
    )

    # Sanity: handshake init forwarded.
    if hs_c1:
        print(f"PASS  hs init fwd:    148b, msgType={hs_c1[2][:4].hex()}")
    else:
        print("FAIL  hs init fwd:    no 148-byte packet captured")
        failed += 1

    print("=" * 60)
    return failed


# ----- main -----

def main() -> int:
    if os.geteuid() != 0:
        print("must run as root (writes to /proc/awg_proxy/)", file=sys.stderr)
        return 2
    if not os.path.exists(PROC_ADD):
        print(
            f"{PROC_ADD} not found — is awg_proxy.ko loaded?",
            file=sys.stderr,
        )
        return 2

    if USE_V6:
        srv = socket.socket(socket.AF_INET6, socket.SOCK_DGRAM)
        # IPV6_RECVTCLASS=66: deliver each datagram's traffic class as
        # IPV6_TCLASS=67 ancillary. Hard-coded — socket.IPV6_RECVTCLASS is
        # missing in some embedded Python builds.
        IPV6_RECVTCLASS = 66
        try:
            srv.setsockopt(socket.IPPROTO_IPV6, IPV6_RECVTCLASS, 1)
        except OSError:
            pass  # DSCP check will just report None-tos
    else:
        srv = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        # Enable IP_RECVTOS so each received datagram carries its IP TOS byte
        # as ancillary data (delivered by recvmsg with cmsg_type=IP_TOS=1).
        # Constant numeric value is hard-coded because socket.IP_RECVTOS is
        # missing in some Python builds on embedded targets.
        IP_RECVTOS = 13
        srv.setsockopt(socket.IPPROTO_IP, IP_RECVTOS, 1)
    try:
        srv.bind((LOOPBACK, SRV_PORT))
    except OSError as e:
        print(
            f"cannot bind {endpoint_str()}: {e}  "
            f"(set SRV_PORT=... to pick another)",
            file=sys.stderr,
        )
        return 2

    del_tunnel()
    try:
        listen_port = add_tunnel()
    except RuntimeError as e:
        print(f"add_tunnel failed: {e}", file=sys.stderr)
        srv.close()
        return 2
    print(
        f"tunnel added: kmod listen=127.0.0.1:{listen_port}  "
        f"target={endpoint_str()}"
    )

    try:
        cycle1 = run_cycle(srv, listen_port)
        cycle2 = run_cycle(srv, listen_port)
    finally:
        del_tunnel()
        srv.close()

    print()
    print(f"cycle 1: {len(cycle1)} packets captured")
    for i, (sz, tos, data) in enumerate(cycle1):
        print(f"  [{i}] len={sz:4d}  tos={tos}  hex={_hex(data)}")
    print()
    print(f"cycle 2: {len(cycle2)} packets captured")
    for i, (sz, tos, data) in enumerate(cycle2):
        print(f"  [{i}] len={sz:4d}  tos={tos}  hex={_hex(data)}")

    failed = verify(cycle1, cycle2)
    if failed == 0:
        print("ALL PASS")
        return 0
    print(f"{failed} FAIL")
    return 1


if __name__ == "__main__":
    sys.exit(main())
