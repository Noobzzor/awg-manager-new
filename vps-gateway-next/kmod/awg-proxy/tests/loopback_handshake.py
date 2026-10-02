#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-2.0
"""
Bidirectional loopback handshake / data-plane check for awg_proxy.ko.

The "is anything broken in data-plane after v1.1.2?" smoke test. Proves
end-to-end that the kmod still forwards every WireGuard packet type
unchanged in both directions when configured for plain-WG (identity
transform, no obfuscation).

Topology (all on 127.0.0.1):

    client_sock         awg_proxy.ko         server_sock
    bound :CLIENT_PORT     listen   remote     bound :SRV_PORT
          |                  |        |              |
          | --- outbound --> |        |              |
          |                  | -- transformed ---->  |
          |                                          |
          |                                  <-- inbound
          |        <-- transformed --        |
          | <----                            |

For each of {init=148b msgType=1, response=92b msgType=2, cookie=64b
msgType=3, transport=64b msgType=4}:
  1. client_sock sends a canonical predictable-bytes packet to the kmod's
     listen socket;
  2. server_sock recvfrom waits for the kmod's transformed outbound;
  3. compares received bytes to expected (BYTE-EXACT for identity);
  4. server_sock sends a different canonical packet back to the kmod's
     remote-socket source (recvfrom srcaddr);
  5. client_sock recvfrom waits for the kmod's transformed inbound;
  6. compares received bytes to expected (BYTE-EXACT for identity).

Configuration used:
    H1= H2= H3= H4=               -- defaults: 1, 2, 3, 4 (identity)
    S1=0 S2=0 S3=0 S4=0           -- no prefix padding
    Jc=0 Jmin=0 Jmax=0             -- no junk packets at handshake init
    PUB_SERVER=0..0 PUB_CLIENT=0..0 -- forces has_server_pub/has_client_pub
                                      to false, so MAC1 recompute is
                                      skipped and bytes truly identical

Why this proves "nothing broken":
  - v1.1.2's changes are scoped to CPS (`<c>`/`<t>`/`<rc>` encoding,
    counter init/increment timing) and per-packet junk DSCP. ALL of
    these activate only when Jc > 0 (gates `send_cps_packets` and
    `send_junk_packets` in c2s_thread_fn). For Jc=0 the gated branches
    never execute — identity behaviour MUST be unchanged from v1.1.1.
  - This test exercises every message-type branch of transform_outbound
    and transform_inbound. Any regression — wrong msgType byte, wrong
    length, dropped packet — surfaces as a byte-diff or recv timeout.

Usage:
    scp loopback_handshake.py root@router:/tmp/
    ssh root@router 'python3 /tmp/loopback_handshake.py'

Exit code: 0 if all PASS, 1 if any FAIL, 2 on setup error.

Env knobs:
    SRV_PORT     UDP port for the fake server socket on 127.0.0.1
                 (default 51998). The kmod's remote endpoint is set
                 to this. Must be free.
    CLIENT_PORT  UDP port for the fake client socket on 127.0.0.1
                 (default 51997). The kmod's listen socket will memorise
                 this as the client address. Must be free.
"""

import os
import socket
import struct
import sys
import time

SRV_PORT = int(os.environ.get("SRV_PORT", 51998))
CLIENT_PORT = int(os.environ.get("CLIENT_PORT", 51997))

PROC_ADD = "/proc/awg_proxy/add"
PROC_DEL = "/proc/awg_proxy/del"
PROC_LIST = "/proc/awg_proxy/list"

PUB_ZERO = "00" * 32


# ---------- /proc helpers ----------

def _write_proc(path: str, line: str) -> None:
    with open(path, "w") as f:
        f.write(line)


def add_identity_tunnel() -> int:
    """Add a plain-WG (identity) tunnel and return the kmod's listen port."""
    line = (
        f"127.0.0.1:{SRV_PORT}"
        f" H1= H2= H3= H4= S1=0 S2=0 S3=0 S4=0"
        f" Jc=0 Jmin=0 Jmax=0"
        f" PUB_SERVER={PUB_ZERO} PUB_CLIENT={PUB_ZERO}"
        f"\n"
    )
    _write_proc(PROC_ADD, line)
    with open(PROC_LIST) as f:
        body = f.read()
    for entry in body.splitlines():
        if not entry.startswith(f"127.0.0.1:{SRV_PORT}"):
            continue
        for tok in entry.split():
            if tok.startswith("listen=127.0.0.1:"):
                return int(tok.rsplit(":", 1)[1])
    raise RuntimeError(f"tunnel not found in {PROC_LIST}:\n{body!r}")


def del_tunnel() -> None:
    try:
        _write_proc(PROC_DEL, f"127.0.0.1:{SRV_PORT}")
    except OSError:
        pass


# ---------- packet builders ----------

def make_pkt(msg_type: int, size: int) -> bytes:
    """msgType in first 4 bytes (LE, WG wire format), rest = predictable
    pattern derived from msg_type so byte-diffs are obvious in output."""
    if size < 4:
        raise ValueError("size must be >= 4")
    head = struct.pack("<I", msg_type)
    body = bytes(((i + msg_type) & 0xFF) for i in range(size - 4))
    return head + body


# ---------- per-cycle bidirectional check ----------

def check_bidirectional(
    name: str,
    client: socket.socket,
    server: socket.socket,
    listen_port: int,
    outbound: bytes,
    inbound: bytes,
) -> tuple[bool, str]:
    """Send outbound from client->kmod->server, then inbound back. Verify
    both arrived byte-exact. Returns (ok, message)."""

    # Step 1: client -> kmod listen
    client.sendto(outbound, ("127.0.0.1", listen_port))

    # Step 2: server waits for forwarded packet
    try:
        recv_out, srcaddr = server.recvfrom(2048)
    except socket.timeout:
        return False, "outbound recv timeout (kmod did not forward)"

    if recv_out != outbound:
        if len(recv_out) != len(outbound):
            return (
                False,
                f"outbound size {len(recv_out)} != expected {len(outbound)}",
            )
        diffs = [i for i, (a, b) in enumerate(zip(outbound, recv_out)) if a != b]
        return (
            False,
            f"outbound bytes diverge at offset {diffs[0]} "
            f"(expected {outbound[diffs[0]]:02x}, got {recv_out[diffs[0]]:02x}; "
            f"{len(diffs)} total diff)",
        )

    # Step 3: server -> kmod remote source -> kmod -> client
    server.sendto(inbound, srcaddr)

    # Step 4: client waits for the inbound transformed back
    try:
        recv_in, _ = client.recvfrom(2048)
    except socket.timeout:
        return False, "inbound recv timeout (kmod did not return reply)"

    if recv_in != inbound:
        if len(recv_in) != len(inbound):
            return (
                False,
                f"inbound size {len(recv_in)} != expected {len(inbound)}",
            )
        diffs = [i for i, (a, b) in enumerate(zip(inbound, recv_in)) if a != b]
        return (
            False,
            f"inbound bytes diverge at offset {diffs[0]} "
            f"(expected {inbound[diffs[0]]:02x}, got {recv_in[diffs[0]]:02x}; "
            f"{len(diffs)} total diff)",
        )

    return True, "byte-exact passthrough both ways"


# ---------- main ----------

def main() -> int:
    if os.geteuid() != 0:
        print("must run as root (writes to /proc/awg_proxy)", file=sys.stderr)
        return 2
    if not os.path.exists(PROC_ADD):
        print(f"{PROC_ADD} not found — is awg_proxy.ko loaded?", file=sys.stderr)
        return 2

    client = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    server = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    try:
        client.bind(("127.0.0.1", CLIENT_PORT))
    except OSError as e:
        print(
            f"cannot bind client 127.0.0.1:{CLIENT_PORT}: {e}  "
            f"(set CLIENT_PORT=...)",
            file=sys.stderr,
        )
        return 2
    try:
        server.bind(("127.0.0.1", SRV_PORT))
    except OSError as e:
        print(
            f"cannot bind server 127.0.0.1:{SRV_PORT}: {e}  "
            f"(set SRV_PORT=...)",
            file=sys.stderr,
        )
        client.close()
        return 2
    client.settimeout(2.0)
    server.settimeout(2.0)

    del_tunnel()
    try:
        listen_port = add_identity_tunnel()
    except RuntimeError as e:
        print(f"add tunnel failed: {e}", file=sys.stderr)
        client.close(); server.close()
        return 2
    print(
        f"identity tunnel: client 127.0.0.1:{CLIENT_PORT} -> "
        f"kmod listen 127.0.0.1:{listen_port} -> "
        f"server 127.0.0.1:{SRV_PORT}"
    )

    # Settle: give the kmod's two kthreads a moment to spin up, then drain
    # any stray datagrams that may have been queued on either socket before
    # we bound (other processes scanning, leftovers from prior test runs,
    # initialisation noise). Without this, a single stale 0-byte packet
    # on the server socket shifts every recvfrom by one, making every
    # subsequent test see the previous test's outbound.
    time.sleep(0.2)
    for sock, name in ((client, "client"), (server, "server")):
        sock.settimeout(0.05)
        drained = []
        while True:
            try:
                data, _ = sock.recvfrom(2048)
                drained.append(len(data))
            except socket.timeout:
                break
        sock.settimeout(2.0)
        if drained:
            print(f"  drained {len(drained)} stale packet(s) from {name} (sizes: {drained})")

    cases = [
        # (name,                      outbound,            inbound)
        ("handshake init  (148/92)",  make_pkt(1, 148),    make_pkt(2, 92)),
        ("handshake resp  (92/148)",  make_pkt(2, 92),     make_pkt(1, 148)),
        ("cookie reply    (64/64)",   make_pkt(3, 64),     make_pkt(3, 64)),
        ("transport data  (64/64)",   make_pkt(4, 64),     make_pkt(4, 64)),
        ("transport large (512/512)", make_pkt(4, 512),    make_pkt(4, 512)),
    ]

    print()
    print("=" * 60)
    print("loopback bidirectional check (identity config)")
    print("=" * 60)

    failed = 0
    try:
        for name, outbound, inbound in cases:
            ok, msg = check_bidirectional(
                name, client, server, listen_port, outbound, inbound
            )
            tag = "PASS" if ok else "FAIL"
            if not ok:
                failed += 1
            print(f"{tag}  {name}: {msg}")
    finally:
        del_tunnel()
        client.close()
        server.close()

    print("=" * 60)
    if failed == 0:
        print("ALL PASS — identity-config data-plane intact")
        return 0
    print(f"{failed} FAIL — see byte-diff above")
    return 1


if __name__ == "__main__":
    sys.exit(main())
