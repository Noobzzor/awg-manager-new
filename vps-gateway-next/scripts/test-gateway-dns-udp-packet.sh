#!/usr/bin/env bash
# Prove DIRECT, VPN, and BLOCK DNS routing over UDP with isolated fixtures.
# All Docker networks are internal; fixtures are local; no ports or volumes persist.
set -Eeuo pipefail

IMAGE="${AWG_GATEWAY_TEST_IMAGE:-awg-manager:gateway-packet-test-20260929-dns-r2}"
DNS_UPSTREAM="198.51.100.53"
DNS_PORT=53
DNS_ANSWER_DIRECT="192.0.2.111"
DNS_ANSWER_VPN="192.0.2.222"
DNS_ANSWER_BLOCK="192.0.2.233"
DNS_ANSWER_DIRECT_A="192.0.2.211"
DNS_ANSWER_DIRECT_B="192.0.2.212"
DNS_ANSWER_VPN_A="192.0.2.221"
DNS_ANSWER_VPN_B="192.0.2.223"
ID="$(python -c 'import uuid; print(uuid.uuid4().hex[:10])')"
OUTER_NET="awgm-dns-outer-${ID}"
CLIENT_NET="awgm-dns-client-${ID}"
EXIT_NET="awgm-dns-exit-${ID}"
GATEWAY="awgm-dns-gateway-${ID}"
CLIENT="awgm-dns-client-${ID}"
VPN_EXIT="awgm-dns-vpn-exit-${ID}"
DIRECT_SINK="awgm-dns-direct-sink-${ID}"
VPN_SINK="awgm-dns-vpn-sink-${ID}"
DNS_CLIENT="awgm-dns-client-sidecar-${ID}"
GW_ALIAS="awg-dns-gateway-${ID}"
REQUESTED_ENDPOINT_TAG="vpn-${ID}"
PROFILE_ID="dns-packet-${ID}"
DIRECT_DOMAIN="direct-${ID}.example"
VPN_DOMAIN="vpn-${ID}.example"
VPN_QUERY_A="probe-a.${VPN_DOMAIN}"
VPN_QUERY_B="probe-b.${VPN_DOMAIN}"
BLOCK_DOMAIN="block-${ID}.example"
CREATED_NETWORK_IDS=()
CREATED_CONTAINER_IDS=()

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

cleanup() {
  result=$?
  trap - EXIT
  local resource
  for resource in "${CREATED_CONTAINER_IDS[@]}"; do docker rm -f "$resource" >/dev/null 2>&1 || true; done
  for resource in "${CREATED_NETWORK_IDS[@]}"; do docker network rm "$resource" >/dev/null 2>&1 || true; done
  exit "$result"
}
trap cleanup EXIT

if ! IMAGE_ID="$(docker image inspect -f '{{.Id}}' "$IMAGE" 2>/dev/null)" || [[ -z "$IMAGE_ID" ]]; then
  fail "local test image not found: $IMAGE"
fi
if ! BUSYBOX_IMAGE_ID="$(docker image inspect -f '{{.Id}}' busybox:latest 2>/dev/null)" || [[ -z "$BUSYBOX_IMAGE_ID" ]]; then
  fail "local BusyBox fixture image not found; refusing to pull an image"
fi
for name in "$CLIENT" "$GATEWAY" "$VPN_EXIT" "$DIRECT_SINK" "$VPN_SINK" "$DNS_CLIENT"; do
  if docker container inspect "$name" >/dev/null 2>&1; then fail "refusing to touch pre-existing container $name"; fi
done
for name in "$CLIENT_NET" "$OUTER_NET" "$EXIT_NET"; do
  if docker network inspect "$name" >/dev/null 2>&1; then fail "refusing to touch pre-existing network $name"; fi
done

CREATED_NETWORK_IDS+=("$(docker network create --internal "$OUTER_NET")")
CREATED_NETWORK_IDS+=("$(docker network create --internal "$CLIENT_NET")")
CREATED_NETWORK_IDS+=("$(docker network create --internal "$EXIT_NET")")
for name in "$OUTER_NET" "$CLIENT_NET" "$EXIT_NET"; do
  [[ "$(docker network inspect -f '{{.Internal}}' "$name")" == true ]] || fail "$name is not internal"
done

# The direct resolver is reachable only on OUTER_NET. The VPN resolver is
# reachable only from VPN_EXIT on EXIT_NET, behind its synthetic wg0 peer.
CREATED_CONTAINER_IDS+=("$(docker run -d --pull never --name "$DIRECT_SINK" --network "$OUTER_NET" \
  --cap-drop ALL --cap-add NET_BIND_SERVICE --read-only --tmpfs /tmp:rw,exec,size=8m \
  --entrypoint /bin/sh "$BUSYBOX_IMAGE_ID" -ec \
  'printf "%s %s\n%s %s\n%s %s\n" "$1" "$2" "$3" "$4" "$5" "$6" >/tmp/dnsd.conf; exec dnsd -c /tmp/dnsd.conf -v -i 0.0.0.0' \
  sh "$DIRECT_DOMAIN" "$DNS_ANSWER_DIRECT" "$VPN_QUERY_A" "$DNS_ANSWER_DIRECT_A" "$VPN_QUERY_B" "$DNS_ANSWER_DIRECT_B")")
CREATED_CONTAINER_IDS+=("$(docker run -d --pull never --name "$VPN_SINK" --network "$EXIT_NET" \
  --cap-drop ALL --cap-add NET_BIND_SERVICE --read-only --tmpfs /tmp:rw,exec,size=8m \
  --entrypoint /bin/sh "$BUSYBOX_IMAGE_ID" -ec \
  'printf "%s %s\n%s %s\n%s %s\n%s %s\n" "$1" "$2" "$3" "$4" "$5" "$6" "$7" "$8" >/tmp/dnsd.conf; exec dnsd -c /tmp/dnsd.conf -v -i 0.0.0.0' \
  sh "$VPN_DOMAIN" "$DNS_ANSWER_VPN" "$VPN_QUERY_A" "$DNS_ANSWER_VPN_A" "$VPN_QUERY_B" "$DNS_ANSWER_VPN_B" "$BLOCK_DOMAIN" "$DNS_ANSWER_BLOCK")")

CREATED_CONTAINER_IDS+=("$(docker run -d --pull never --name "$VPN_EXIT" --network "$OUTER_NET" \
  --cap-drop ALL --cap-add NET_ADMIN --sysctl net.ipv4.ip_forward=1 \
  --tmpfs /data:rw,size=16m,mode=0750 --tmpfs /run:rw,size=8m --tmpfs /tmp:rw,size=8m \
  --entrypoint /bin/sh "$IMAGE_ID" -ec 'sleep 1800')")
docker network connect "$EXIT_NET" "$VPN_EXIT"

wait_dns_fixture() {
  local container="$1" ready=0
  for _ in $(seq 1 40); do
    if docker logs "$container" 2>&1 | grep -Fq 'dnsd: accepting UDP packets'; then ready=1; break; fi
    sleep 0.25
  done
  [[ "$ready" == 1 ]] || fail "$container did not start its UDP DNS listener"
}
wait_dns_fixture "$DIRECT_SINK"
wait_dns_fixture "$VPN_SINK"

VPN_OUTER_IP="$(docker inspect -f "{{(index .NetworkSettings.Networks \"$OUTER_NET\").IPAddress}}" "$VPN_EXIT")"
DIRECT_SINK_IP="$(docker inspect -f "{{(index .NetworkSettings.Networks \"$OUTER_NET\").IPAddress}}" "$DIRECT_SINK")"
VPN_SINK_IP="$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$VPN_SINK")"
[[ -n "$VPN_OUTER_IP" && -n "$DIRECT_SINK_IP" && -n "$VPN_SINK_IP" ]] || fail "could not resolve isolated peer/fixture addresses"
EXIT_IFACE="$(docker exec "$VPN_EXIT" sh -ec 'route="$(ip route get "$1")"; set -- $route; while [ "$#" -gt 0 ]; do if [ "$1" = dev ]; then shift; printf "%s" "$1"; exit 0; fi; shift; done; exit 1' sh "$VPN_SINK_IP")"
[[ -n "$EXIT_IFACE" && "$EXIT_IFACE" != eth0 ]] || fail "VPN peer fixture interface is not isolated from its outer interface"
[[ -z "$(docker exec "$VPN_EXIT" ip -4 route show default)" ]] || fail "VPN exit unexpectedly has a default route"

GATEWAY_COMMAND='exec /usr/local/bin/entrypoint.sh /usr/local/bin/awg-manager'
CREATED_CONTAINER_IDS+=("$(docker run -d --pull never --name "$GATEWAY" --network "$OUTER_NET" \
  --cap-drop ALL --cap-add NET_ADMIN --cap-add DAC_OVERRIDE --cap-add SETUID --cap-add SETGID \
  --device /dev/net/tun --sysctl net.ipv4.ip_forward=1 \
  --tmpfs /data:rw,size=64m,mode=0750,uid=10001,gid=10001 \
  --tmpfs /run:rw,size=16m,mode=0750,uid=10001,gid=10001 --tmpfs /tmp:rw,size=64m,mode=1777 \
  -e AWG_GATEWAY_ENABLE=true -e AWG_GATEWAY_INTERFACE=awg0 \
  -e AWG_GATEWAY_TOOL=/usr/bin/wg -e AWG_GATEWAY_WAN_INTERFACE=eth0 \
  -e AWG_GATEWAY_PORT=51820 -e AWG_GATEWAY_CLIENT_POOL=10.66.0.0/24 \
  -e AWG_GATEWAY_SERVER_ADDRESS=10.66.0.1/24 -e AWG_GATEWAY_PUBLIC_ENDPOINT="${GW_ALIAS}:51820" \
  -e AWG_DNS_UPSTREAM="$DNS_UPSTREAM" -e AWG_HTTP_ADDR=0.0.0.0:2222 \
  --entrypoint /bin/sh "$IMAGE_ID" -ec "$GATEWAY_COMMAND")")
docker network connect --alias "$GW_ALIAS" "$CLIENT_NET" "$GATEWAY"
CREATED_CONTAINER_IDS+=("$(docker run -d --pull never --name "$CLIENT" --network "$CLIENT_NET" \
  --cap-drop ALL --cap-add NET_ADMIN --device /dev/net/tun \
  --tmpfs /data:rw,size=16m,mode=0750 --tmpfs /run:rw,size=16m --tmpfs /tmp:rw,size=8m \
  --entrypoint /bin/sh "$IMAGE_ID" -ec 'sleep 1800')")
# BusyBox shares the client's network namespace; DNS queries traverse awg-client.
CREATED_CONTAINER_IDS+=("$(docker run -d --pull never --name "$DNS_CLIENT" --network "container:$CLIENT" \
  --cap-drop ALL --read-only --tmpfs /tmp:rw,exec,size=8m \
  --entrypoint /bin/sh "$BUSYBOX_IMAGE_ID" -ec 'sleep 1800')")

container_networks() {
  docker inspect -f '{{range $name, $net := .NetworkSettings.Networks}}{{$name}} {{end}}' "$1"
}
GATEWAY_NETWORKS="$(container_networks "$GATEWAY")"
DIRECT_NETWORKS="$(container_networks "$DIRECT_SINK")"
VPN_SINK_NETWORKS="$(container_networks "$VPN_SINK")"
[[ " $GATEWAY_NETWORKS " == *" $OUTER_NET "* && " $GATEWAY_NETWORKS " == *" $CLIENT_NET "* ]] || fail "gateway is not attached to both expected networks"
[[ " $GATEWAY_NETWORKS " != *" $EXIT_NET "* ]] || fail "gateway unexpectedly joins the VPN-only exit network"
[[ " $DIRECT_NETWORKS " == *" $OUTER_NET "* && " $DIRECT_NETWORKS " != *" $EXIT_NET "* ]] || fail "direct DNS fixture network isolation failed"
[[ " $VPN_SINK_NETWORKS " == *" $EXIT_NET "* && " $VPN_SINK_NETWORKS " != *" $OUTER_NET "* ]] || fail "VPN DNS fixture network isolation failed"

# Route only the reserved synthetic DNS destination over OUTER_NET. No main
# default route exists; the exact OUTPUT DNAT below prevents public egress.
[[ -z "$(docker exec "$GATEWAY" ip -4 route show table main default)" ]] || fail "gateway unexpectedly has a main-table default route"
docker exec "$GATEWAY" ip -4 route replace "$DNS_UPSTREAM/32" dev eth0
DNS_ROUTE="$(docker exec "$GATEWAY" ip -4 route get "$DNS_UPSTREAM")"
[[ "$DNS_ROUTE" == *"dev eth0"* ]] || fail "direct DNS upstream is not routed on isolated OUTER_NET: $DNS_ROUTE"
MAIN_ROUTES="$(docker exec "$GATEWAY" ip -4 route show table main)"
[[ "$MAIN_ROUTES" == *"$DNS_UPSTREAM"* && "$MAIN_ROUTES" != *"default"* ]] || fail "main table is not restricted to connected networks and the synthetic DNS /32"
if docker exec "$GATEWAY" ip -4 route get 203.0.113.53 >/dev/null 2>&1; then
  fail "unrelated TEST-NET destination unexpectedly has a main route"
fi
docker exec "$GATEWAY" iptables -t nat -A OUTPUT -o eth0 -p udp -d "$DNS_UPSTREAM" --dport "$DNS_PORT" -j DNAT --to-destination "$DIRECT_SINK_IP:$DNS_PORT"

# Generate disposable keys inside containers; private keys remain in tmpfs.
VPN_SERVER_PUBLIC="$(docker exec "$VPN_EXIT" sh -ec 'umask 077; wg genkey >/run/server.key; wg pubkey </run/server.key')"
VPN_CLIENT_PUBLIC="$(docker exec "$GATEWAY" sh -ec 'umask 077; wg genkey >/run/vpn-client.key; wg pubkey </run/vpn-client.key')"
[[ -n "$VPN_SERVER_PUBLIC" && -n "$VPN_CLIENT_PUBLIC" ]] || fail "could not generate disposable VPN keys"
docker exec "$VPN_EXIT" sh -ec \
  'ip link add wg0 type wireguard; ip addr add 10.99.0.1/24 dev wg0; wg set wg0 private-key /run/server.key listen-port 51820 peer "$1" allowed-ips 10.99.0.2/32,10.66.0.0/24; ip link set wg0 up' \
  sh "$VPN_CLIENT_PUBLIC"
# Only UDP/53 arriving on wg0 is redirected to the VPN-only DNS fixture. At
# FORWARD, replies still have the fixture source; reverse DNAT happens later.
docker exec "$VPN_EXIT" sh -ec \
	'iptables -P FORWARD DROP; iptables -A FORWARD -i wg0 -o "$1" -p udp -d "$2" --dport 53 -m conntrack --ctstate NEW,ESTABLISHED -j ACCEPT; iptables -A FORWARD -i "$1" -o wg0 -p udp -s "$2" --sport 53 -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT; iptables -t nat -A PREROUTING -i wg0 -d "$3" -p udp --dport 53 -j DNAT --to-destination "$2:53"; iptables -t nat -A POSTROUTING -o "$1" -d "$2" -p udp --dport 53 -j MASQUERADE' \
	sh "$EXIT_IFACE" "$VPN_SINK_IP" "$DNS_UPSTREAM"

# No host ports, persistent Docker volumes, or non-internal test networks.
for container in "$GATEWAY" "$CLIENT" "$DNS_CLIENT" "$VPN_EXIT" "$DIRECT_SINK" "$VPN_SINK"; do
  [[ -z "$(docker port "$container")" ]] || fail "$container unexpectedly publishes host ports"
  mounts="$(docker inspect -f '{{range .Mounts}}{{.Type}}:{{.Destination}} {{end}}' "$container")"
  [[ "$mounts" != *volume* ]] || fail "$container unexpectedly uses a Docker volume"
done

ready=0
for _ in $(seq 1 60); do
  if docker exec "$GATEWAY" curl -fsS --max-time 2 http://127.0.0.1:2222/readyz >/dev/null 2>&1; then ready=1; break; fi
  sleep 1
done
[[ "$ready" == 1 ]] || fail "gateway did not become ready"

applied_hash() {
  docker exec "$GATEWAY" sha256sum /var/run/awg-manager/singbox-applied.json 2>/dev/null | cut -d ' ' -f 1 || true
}
wait_for_apply() {
  local before="$1" current
  for _ in $(seq 1 60); do
    current="$(applied_hash)"
    if [[ -n "$current" && "$current" != "$before" ]] && docker exec "$GATEWAY" ip link show awgm0 >/dev/null 2>&1; then return 0; fi
    sleep 0.5
  done
  return 1
}
diagnose_apply() {
  local before="$1" domain="$2"
  printf 'DEBUG DNS apply before=%s after=%s domain_in_slot=' "$before" "$(applied_hash)" >&2
  docker exec "$GATEWAY" sh -ec 'found=0; for file in $(find /data/sing-box/config.d -type f -name "*.json" 2>/dev/null); do if grep -Fq "$1" "$file"; then found=1; fi; done; if [ "$found" = 1 ]; then printf yes; else printf no; fi' sh "$domain" >&2 || true
  printf '\nDEBUG applied_state=' >&2
  docker exec "$GATEWAY" sh -ec 'cat /var/run/awg-manager/singbox-applied.json 2>/dev/null || true' >&2
  printf '\nDEBUG DNS route metadata=' >&2
  docker exec "$GATEWAY" curl -fsS --max-time 3 http://127.0.0.1:2222/api/dns-routes/list >&2 || true
  printf '\nDEBUG config slot files and hashes:\n' >&2
  docker exec "$GATEWAY" sh -ec 'find /data/sing-box/config.d -type f -name "*.json" -exec sha256sum {} \; 2>/dev/null || true' >&2
  if docker exec "$GATEWAY" pgrep -x sing-box >/dev/null 2>&1; then printf 'DEBUG sing-box_process=running\n' >&2; else printf 'DEBUG sing-box_process=not-running\n' >&2; fi
  printf 'DEBUG sing-box status (sanitized)=' >&2
  docker exec "$GATEWAY" curl -fsS --max-time 3 http://127.0.0.1:2222/api/singbox/status 2>/dev/null | python -c 'import json,re,sys; raw=sys.stdin.read(); data=json.loads(raw).get("data",{}); error=str(data.get("lastError", "")); error=re.sub(r"(?i)(private[_ -]?key|preshared[_ -]?key|header[_ -]?protection[_ -]?key)(\\s*[:=]\\s*)[^\\s,;]+", r"\\1\\2[REDACTED]", error); error=re.sub(r"(?<![A-Za-z0-9])[A-Za-z0-9+/]{40,}={0,2}(?![A-Za-z0-9])", "[REDACTED]", error); print(json.dumps({"running":data.get("running"),"pid":data.get("pid"),"lastError":error}, sort_keys=True))' >&2 || true
}
create_client() {
  docker exec "$GATEWAY" curl -fsS --max-time 5 -H 'Content-Type: application/json' -X POST \
    --data-binary "{\"id\":\"dns-packet-${ID}\",\"label\":\"Local DNS packet test\"}" \
    http://127.0.0.1:2222/api/gateway/clients/create >/dev/null
}
create_profile() {
  local body
  body="$(printf '{"id":"%s","name":"Local DNS packet test","defaultAction":"direct"}' "$PROFILE_ID")"
  docker exec "$GATEWAY" curl -fsS --max-time 5 -H 'Content-Type: application/json' -X POST \
    --data-binary "$body" http://127.0.0.1:2222/api/gateway/policies/profiles/create >/dev/null
}
apply_profile() {
  docker exec "$GATEWAY" curl -fsS --max-time 5 -X POST \
    "http://127.0.0.1:2222/api/gateway/policies/profiles/$PROFILE_ID/apply" >/dev/null
}
configure_client() {
  docker exec "$GATEWAY" curl -fsS --max-time 5 \
    "http://127.0.0.1:2222/api/gateway/clients/dns-packet-${ID}/config" |
    docker exec -i "$CLIENT" sh -ec '
      umask 077; cat >/run/awg-client.conf
      client_address="$(awk -F " = " "/^Address = / { print \$2; exit }" /run/awg-client.conf)"
      gateway_ip="$(getent ahostsv4 "$1" | awk "NR == 1 { print \$1 }")"
      test -n "$client_address" && test -n "$gateway_ip"
      ip route add "$gateway_ip/32" dev eth0
      ip link add awg-client type wireguard
      wg-quick strip /run/awg-client.conf | wg setconf awg-client /dev/stdin
      ip address add "$client_address" dev awg-client
      ip link set mtu 1420 up dev awg-client
      ip route replace default dev awg-client
      rm -f /run/awg-client.conf
    ' sh "$GW_ALIAS"
}
wait_for_client_handshake() {
  local handshake=0 timestamp
  for _ in $(seq 1 40); do
    timestamp="$(docker exec "$CLIENT" wg show awg-client latest-handshakes 2>/dev/null | awk 'NR == 1 {print $2}')"
    if [[ "$timestamp" =~ ^[1-9][0-9]*$ ]]; then handshake=1; break; fi
    sleep 0.5
  done
  [[ "$handshake" == 1 ]] || fail "WireGuard client-to-Gateway handshake did not complete"
}
server_counters() {
  local values peer received sent
  values="$(docker exec "$VPN_EXIT" wg show wg0 transfer)"
  read -r peer received sent <<<"$values"
  printf '%s %s\n' "$received" "$sent"
}
create_dns_route() {
  local name="$1" domain="$2" target="$3" body before
  body="$(printf '{\"name\":\"%s\",\"manualText\":\"%s\",\"backend\":\"singbox\",\"enabled\":true,\"routes\":[{\"interface\":\"\",\"tunnelId\":\"%s\"}]}' "$name" "$domain" "$target")"
  before="$(applied_hash)"
  docker exec "$GATEWAY" curl -fsS --max-time 8 -H 'Content-Type: application/json' -X POST \
    --data-binary "$body" http://127.0.0.1:2222/api/dns-routes/create >/dev/null
  if ! wait_for_apply "$before"; then
    diagnose_apply "$before" "$domain"
    fail "DNS route $name was not observed in the applied config"
  fi
}
query_dns() {
  local domain="$1" output status=0
  if output="$(docker exec "$DNS_CLIENT" timeout 5 nslookup -type=A "$domain" "$DNS_UPSTREAM" 2>&1)"; then status=0; else status=$?; fi
  printf '%s\n' "$output"
  return "$status"
}
debug_merged_dns_config() {
  docker exec "$GATEWAY" curl -fsS --max-time 3 http://127.0.0.1:2222/api/singbox/config-preview |
    python -c 'import json,sys; payload=json.load(sys.stdin); cfg=json.loads(payload["data"]["json"]); dns=cfg.get("dns",{}); route=cfg.get("route",{}); servers=[{k:s[k] for k in ("tag","type","detour") if k in s} for s in dns.get("servers",[])]; outbounds=[{k:o[k] for k in ("tag","type") if k in o} for o in cfg.get("outbounds",[])]; endpoints=[{k:e[k] for k in ("tag","type") if k in e} for e in cfg.get("endpoints",[])]; print(json.dumps({"log":cfg.get("log",{}),"dns_final":dns.get("final"),"dns_rule_order":dns.get("rules",[]),"route_final":route.get("final"),"route_rules":route.get("rules",[]),"servers":servers,"outbounds":outbounds,"endpoints":endpoints},sort_keys=True))'
}
dns_slot_text() {
  docker exec "$GATEWAY" dd if=/data/sing-box/config.d/19-dns-routes.json 2>/dev/null
}
dns_server_tag_from_slot() {
  dns_slot_text | python -c 'import json,sys; slot=json.load(sys.stdin); suffix=sys.argv[1]; rules=slot.get("dns",{}).get("rules",[]); matches=[r for r in rules if suffix in r.get("domain_suffix",[])]; assert len(matches)==1, f"expected one VPN suffix rule, got {len(matches)}"; assert matches[0].get("server"), "VPN rule has no DNS server tag"; print(matches[0]["server"])' "$VPN_DOMAIN"
}
slot_hashes_except_dns_routes() {
  docker exec "$GATEWAY" sh -ec 'for file in /data/sing-box/config.d/*.json; do [ "$file" = /data/sing-box/config.d/19-dns-routes.json ] || sha256sum "$file"; done' | sort
}
enable_explicit_dns_route_action() {
  docker exec "$GATEWAY" dd if=/data/sing-box/config.d/19-dns-routes.json 2>/dev/null |
    python -c 'import json,sys; slot=json.load(sys.stdin); suffix=sys.argv[1]; rules=slot.get("dns",{}).get("rules",[]); matches=[r for r in rules if suffix in r.get("domain_suffix",[])]; assert len(matches)==1, f"expected one VPN suffix rule, got {len(matches)}"; rule=matches[0]; assert "action" not in rule, "A variant unexpectedly already has an explicit action"; assert rule.get("server"), "VPN rule is missing its target DNS server tag"; rule["action"]="route"; json.dump(slot,sys.stdout,separators=(",",":"))' "$VPN_DOMAIN" |
    docker exec -i "$GATEWAY" sh -ec 'dd of=/data/sing-box/config.d/19-dns-routes.json.tmp 2>/dev/null; chmod 0644 /data/sing-box/config.d/19-dns-routes.json.tmp; mv /data/sing-box/config.d/19-dns-routes.json.tmp /data/sing-box/config.d/19-dns-routes.json'
}
assert_only_explicit_action_diff() {
  python -c 'import json,sys; before=json.loads(sys.argv[1]); after=json.loads(sys.argv[2]); suffix=sys.argv[3]; get=lambda x:[r for r in x.get("dns",{}).get("rules",[]) if suffix in r.get("domain_suffix",[])]; left=get(before); right=get(after); assert len(left)==len(right)==1, "VPN suffix rule count changed"; assert "action" not in left[0] and right[0].get("action")=="route", "expected only legacy->route action transition"; assert left[0].get("server")==right[0].get("server"), "target DNS server tag changed"; tag=left[0]["server"]; del right[0]["action"]; assert before==after, "A/B DNS slots differ beyond the explicit action"; print("AB_CONFIG_DIFF=only action:route; target_dns_server_tag="+tag)' "$1" "$2" "$VPN_DOMAIN"
}
vpn_exit_nat_packets() {
  docker exec "$VPN_EXIT" iptables -t nat -nvxL PREROUTING | awk 'NR > 2 && $3 == "DNAT" { packets += $1 } END { print packets + 0 }'
}
gateway_output_nat_packets() {
  docker exec "$GATEWAY" iptables -t nat -nvxL OUTPUT | awk 'NR > 2 && $3 == "DNAT" { packets += $1 } END { print packets + 0 }'
}
gateway_endpoint_counters() {
  docker exec "$GATEWAY" wg show awg0 transfer | awk '{rx += $2; tx += $3} END {printf "%d %d\n", rx, tx}'
}
start_singbox_dns_log_capture() {
  local clash_port
  clash_port="$(docker exec "$GATEWAY" sh -ec 'cat /data/sing-box/config.d/00-base.json' | python -c 'import json,sys; addr=json.load(sys.stdin)["experimental"]["clash_api"]["external_controller"]; host,sep,port=addr.rpartition(":"); assert sep and host=="127.0.0.1", f"Clash API is not loopback-bound: {addr!r}"; print(port)')"
  docker exec -d -e CLASH_PORT="$clash_port" "$GATEWAY" sh -ec 'rm -f /tmp/dns-ab-singbox.log /tmp/dns-ab-singbox.status /tmp/dns-ab-singbox.err /tmp/dns-ab-singbox.pid; curl -sSN --max-time 20 -o /tmp/dns-ab-singbox.log -w "%{http_code}" "http://127.0.0.1:${CLASH_PORT}/logs?level=trace" >/tmp/dns-ab-singbox.status 2>/tmp/dns-ab-singbox.err & echo $! >/tmp/dns-ab-singbox.pid; wait'
  sleep 0.2
}
stop_singbox_dns_log_capture() {
  docker exec "$GATEWAY" sh -ec 'if read -r pid </tmp/dns-ab-singbox.pid; then kill "$pid" 2>/dev/null || true; fi; sleep 0.2' 2>/dev/null || true
}
print_singbox_dns_match_logs() {
  local label="$1"
  printf 'DEBUG %s Sing-box runtime log lines:\n' "$label"
  printf 'DEBUG %s Sing-box log stream status/bytes=' "$label"
  docker exec "$GATEWAY" sh -ec 'status=missing; if read -r value </tmp/dns-ab-singbox.status; then status="$value"; fi; bytes="$(wc -c </tmp/dns-ab-singbox.log 2>/dev/null || printf 0)"; printf "%s/%s\n" "$status" "$bytes"'
  docker exec "$GATEWAY" dd if=/tmp/dns-ab-singbox.log 2>/dev/null |
    python -c 'import json,re,sys; needles=sys.argv[1:]; found=False
for raw in sys.stdin:
 try:
  item=json.loads(raw); text=str(item.get("payload",raw))
 except Exception:
  text=raw.rstrip()
 if any(n in text for n in needles) or "dns:" in text.lower() or "router:" in text.lower(): found=True
 text=re.sub(r"(?i)(private[_ -]?key|preshared[_ -]?key|header[_ -]?protection[_ -]?key)(\s*[:=]\s*)[^\s,;]+",r"\1\2[REDACTED]",text)
 text=re.sub(r"(?<![A-Za-z0-9])[A-Za-z0-9+/]{40,}={0,2}(?![A-Za-z0-9])","[REDACTED]",text)
 if text.strip(): print(text.rstrip())
if not found: print("NO_QUERY_OR_RULE_MATCH_LOG_CAPTURED")' "$VPN_DOMAIN" "$VPN_QUERY_A" "$VPN_QUERY_B"
}
dns_packet_count() {
  local count
  count="$(docker logs "$1" 2>&1 | grep -Fc 'dnsd: got UDP packet' || true)"
  printf '%s' "${count:-0}"
}
start_dns_trace() {
  docker exec -d "$GATEWAY" sh -ec 'rm -f /tmp/dns-trace.log; nft monitor trace >/tmp/dns-trace.log 2>&1 & echo $! >/tmp/dns-trace.pid; wait'
  sleep 0.2
}
stop_dns_trace() {
  docker exec "$GATEWAY" sh -ec 'kill "$(cat /tmp/dns-trace.pid)" 2>/dev/null || true; sleep 0.1; cat /tmp/dns-trace.log' 2>/dev/null || true
}
assert_policy_tun_trace() {
  local label="$1" trace="$2"
  if [[ "$trace" != *"inet sing-box prerouting rule meta mark set"* ]]; then
    printf 'DEBUG %s DNS trace did not show Sing-box prerouting mark:\n%s\n' "$label" "$trace" >&2
    fail "$label DNS query did not enter Sing-box redirect processing"
  fi
  if [[ "$trace" != *'oif "awgm0"'* ]]; then
    printf 'DEBUG %s DNS trace did not show policy TUN forwarding:\n%s\n' "$label" "$trace" >&2
    fail "$label DNS query did not traverse the policy TUN"
  fi
}

create_client
configure_client
wait_for_client_handshake

# Import a synthetic local AWG endpoint; route lists use its returned record ID.
IMPORT_RESPONSE="$(docker exec "$GATEWAY" sh -ec \
  'umask 077; printf '\''{"tag":"%s","config":{"type":"awg","tag":"%s","useIntegratedTun":false,"private_key":"%s","address":["10.99.0.2/32"],"peers":[{"address":"%s","port":51820,"public_key":"%s","allowed_ips":["0.0.0.0/0"]}]}}\n'\'' "$1" "$1" "$(cat /run/vpn-client.key)" "$2" "$3" >/run/awg3-import.json; curl -fsS --max-time 8 -H '\''Content-Type: application/json'\'' -X POST --data-binary @/run/awg3-import.json http://127.0.0.1:2222/api/awg3-endpoints; rm -f /run/awg3-import.json /run/vpn-client.key' \
  sh "$REQUESTED_ENDPOINT_TAG" "$VPN_OUTER_IP" "$VPN_SERVER_PUBLIC")"
ENDPOINT_REF="$(printf '%s' "$IMPORT_RESPONSE" | python -c 'import json,sys; expected=sys.argv[1]; data=json.load(sys.stdin).get("data",[]); hits=[x for x in data if x.get("tag")==expected]; assert len(hits)==1; print(hits[0]["id"],hits[0]["tag"])' "$REQUESTED_ENDPOINT_TAG")" || fail "import response did not contain the expected AWG endpoint ID/tag"
read -r ENDPOINT_ID ENDPOINT_TAG <<<"$ENDPOINT_REF"
[[ -n "$ENDPOINT_ID" && "$ENDPOINT_TAG" == "$REQUESTED_ENDPOINT_TAG" ]] || fail "unexpected imported AWG endpoint identity"

# DNS interception lives in the policy TUN. This default-direct profile
# activates capture; the DNS-route slots below select their own detours.
create_profile
HASH_BEFORE="$(applied_hash)"
apply_profile
wait_for_apply "$HASH_BEFORE" || fail "DNS policy TUN did not become active"

create_dns_route "DNS DIRECT ${ID}" "$DIRECT_DOMAIN" direct
create_dns_route "DNS VPN ${ID}" "$VPN_DOMAIN" "$ENDPOINT_ID"
create_dns_route "DNS BLOCK ${ID}" "$BLOCK_DOMAIN" reject
VPN_DNS_SERVER_TAG="$(dns_server_tag_from_slot)"
[[ -n "$VPN_DNS_SERVER_TAG" ]] || fail "VPN DNS route did not resolve to a server tag"

TRACE_CHAIN="dns_trace_${ID}"
docker exec "$GATEWAY" nft add chain inet awgm_gateway "$TRACE_CHAIN" '{ type filter hook prerouting priority raw; policy accept; }'
docker exec "$GATEWAY" nft add rule inet awgm_gateway "$TRACE_CHAIN" iifname awg0 ip saddr 10.66.0.0/24 udp dport 53 meta nftrace set 1
docker exec "$GATEWAY" nft add rule inet awgm_gateway "$TRACE_CHAIN" iifname awgm0 ip daddr 10.66.0.0/24 udp sport 53 meta nftrace set 1
start_dns_trace
DIRECT_BASE="$(dns_packet_count "$DIRECT_SINK")"
VPN_BASE="$(dns_packet_count "$VPN_SINK")"
if DIRECT_OUTPUT="$(query_dns "$DIRECT_DOMAIN")"; then DIRECT_STATUS=0; else DIRECT_STATUS=$?; fi
DIRECT_TRACE="$(stop_dns_trace)"
if [[ "$DIRECT_STATUS" != 0 || "$DIRECT_OUTPUT" != *"$DNS_ANSWER_DIRECT"* ]]; then
  printf 'DEBUG DIRECT query status=%s output=%s\n' "$DIRECT_STATUS" "$DIRECT_OUTPUT" >&2
  printf 'DEBUG nft trace:\n%s\n' "$DIRECT_TRACE" >&2
  printf 'DEBUG client WG transfer (RX TX): ' >&2
  docker exec "$CLIENT" wg show awg-client transfer | awk '{rx += $2; tx += $3} END {printf "%d %d\\n", rx, tx}' >&2 || true
  printf 'DEBUG gateway WG transfer (RX TX): ' >&2
  docker exec "$GATEWAY" wg show awg0 transfer | awk '{rx += $2; tx += $3} END {printf "%d %d\\n", rx, tx}' >&2 || true
  printf 'DEBUG gateway client WG transfer before=%s after=' "$GATEWAY_WG_BASE" >&2
  docker exec "$GATEWAY" wg show awg0 transfer | awk '{rx += $2; tx += $3} END {printf "%d %d\n", rx, tx}' >&2 || true
  printf 'DEBUG client WG transfer before=%s after=' "$CLIENT_WG_BASE" >&2
  docker exec "$CLIENT" wg show awg-client transfer | awk '{rx += $2; tx += $3} END {printf "%d %d\n", rx, tx}' >&2 || true
  printf 'DEBUG Gateway TUN counters:\n' >&2
  docker exec "$GATEWAY" ip -s link show dev awgm0 >&2 || true
  printf 'DEBUG fixture UDP packets direct=%s vpn=%s\n' "$(dns_packet_count "$DIRECT_SINK")" "$(dns_packet_count "$VPN_SINK")" >&2
  printf 'DEBUG direct fixture logs:\n' >&2
  docker logs "$DIRECT_SINK" 2>&1 >&2 || true
  printf 'DEBUG VPN fixture logs:\n' >&2
  docker logs "$VPN_SINK" 2>&1 >&2 || true
  printf 'DEBUG gateway routes and DNS NAT rule:\n' >&2
  docker exec "$GATEWAY" sh -ec 'ip route; iptables -t nat -S OUTPUT' >&2 || true
  printf 'DEBUG IPv4 policy rules and all route tables:\n' >&2
  docker exec "$GATEWAY" sh -ec 'ip -4 rule show; ip -4 route show table all' >&2 || true
  printf 'DEBUG marked lookup for captured client packet:\n' >&2
  docker exec "$GATEWAY" ip -4 route get "$DNS_UPSTREAM" from 10.66.0.2 iif awg0 mark 0x2024 >&2 || true
  printf 'DEBUG sing-box netfilter table:\n' >&2
  docker exec "$GATEWAY" nft -a list table inet sing-box >&2 || true
  printf 'DEBUG sing-box status (sanitized)=' >&2
  docker exec "$GATEWAY" curl -fsS --max-time 3 http://127.0.0.1:2222/api/singbox/status 2>/dev/null | python -c 'import json,re,sys; data=json.load(sys.stdin).get("data",{}); error=str(data.get("lastError", "")); error=re.sub(r"(?<![A-Za-z0-9])[A-Za-z0-9+/]{40,}={0,2}(?![A-Za-z0-9])", "[REDACTED]", error); print(json.dumps({"running":data.get("running"),"lastError":error}, sort_keys=True))' >&2 || true
  fail "DIRECT DNS query did not return the direct fixture A record"
fi
assert_policy_tun_trace DIRECT "$DIRECT_TRACE"
DIRECT_AFTER="$(dns_packet_count "$DIRECT_SINK")"
VPN_AFTER_DIRECT="$(dns_packet_count "$VPN_SINK")"
(( DIRECT_AFTER > DIRECT_BASE )) || fail "DIRECT DNS query did not hit the direct fixture"
[[ "$VPN_AFTER_DIRECT" == "$VPN_BASE" ]] || fail "DIRECT DNS query unexpectedly reached the VPN-only fixture"
printf 'PASS: DIRECT DNS query reached only its isolated direct fixture.\n'

if [[ "${AWG_DNS_AB_EXPERIMENT:-0}" == "1" ]]; then
  docker exec "$GATEWAY" curl -fsS --max-time 8 -H 'Content-Type: application/json' -X POST \
    --data-binary '{"logging":{"singboxLogLevel":"debug"}}' \
    http://127.0.0.1:2222/api/settings/update |
    python -c 'import json,sys; data=json.load(sys.stdin).get("data",{}); level=data.get("logging",{}).get("singboxLogLevel"); assert level=="debug", f"debug logging was not applied: {level!r}"; print("SINGBOX_DIAGNOSTIC_LOG_LEVEL=debug")'

  SLOT_A="$(dns_slot_text)"
  DNS_SERVER_TAG="$(dns_server_tag_from_slot)"
  VPN_DETOUR_TAG="$(docker exec "$GATEWAY" curl -fsS --max-time 3 http://127.0.0.1:2222/api/singbox/config-preview | python -c 'import json,sys; payload=json.load(sys.stdin); cfg=json.loads(payload["data"]["json"]); tag=sys.argv[1]; servers=cfg.get("dns",{}).get("servers",[]); matches=[s for s in servers if s.get("tag")==tag]; assert len(matches)==1, f"expected one DNS server {tag}, got {len(matches)}"; print(matches[0].get("detour", ""))' "$DNS_SERVER_TAG")"
  [[ "$VPN_DETOUR_TAG" == "$ENDPOINT_TAG" ]] || fail "VPN DNS server detour $VPN_DETOUR_TAG does not match imported AWG3 endpoint tag $ENDPOINT_TAG"
  printf 'AB_TARGETS dns_server_tag=%s awg3_endpoint_tag=%s dns_detour=%s\n' "$DNS_SERVER_TAG" "$ENDPOINT_TAG" "$VPN_DETOUR_TAG"
  printf 'AB_EFFECTIVE_CONFIG_A='; debug_merged_dns_config
  OTHER_SLOT_HASHES_A="$(slot_hashes_except_dns_routes)"

  A_DIRECT_BASE="$(dns_packet_count "$DIRECT_SINK")"
  A_VPN_BASE="$(dns_packet_count "$VPN_SINK")"
  A_DIRECT_DNAT_BASE="$(gateway_output_nat_packets)"
  A_EXIT_DNAT_BASE="$(vpn_exit_nat_packets)"
  read -r A_EXIT_BASE_RX A_EXIT_BASE_TX <<<"$(server_counters)"
  start_singbox_dns_log_capture
  start_dns_trace
  if A_OUTPUT="$(query_dns "$VPN_QUERY_A")"; then A_STATUS=0; else A_STATUS=$?; fi
  A_TRACE="$(stop_dns_trace)"
  stop_singbox_dns_log_capture
  A_DIRECT_AFTER="$(dns_packet_count "$DIRECT_SINK")"
  A_VPN_AFTER="$(dns_packet_count "$VPN_SINK")"
  A_DIRECT_DNAT_AFTER="$(gateway_output_nat_packets)"
  A_EXIT_DNAT_AFTER="$(vpn_exit_nat_packets)"
  read -r A_EXIT_AFTER_RX A_EXIT_AFTER_TX <<<"$(server_counters)"
  if (( A_VPN_AFTER > A_VPN_BASE && A_DIRECT_AFTER == A_DIRECT_BASE )); then A_PATH=VPN
  elif (( A_DIRECT_AFTER > A_DIRECT_BASE && A_VPN_AFTER == A_VPN_BASE )); then A_PATH=DIRECT
  elif (( A_DIRECT_AFTER == A_DIRECT_BASE && A_VPN_AFTER == A_VPN_BASE )); then A_PATH=NO_FIXTURE
  else A_PATH=AMBIGUOUS
  fi
  printf 'AB_A action=legacy qname=%s status=%s path=%s answer=%s direct_fixture=%s->%s vpn_fixture=%s->%s gateway_output_dnat=%s->%s vpn_exit_prerouting_dnat=%s->%s vpn_exit_wg_rx=%s->%s vpn_exit_wg_tx=%s->%s\n' \
    "$VPN_QUERY_A" "$A_STATUS" "$A_PATH" "$A_OUTPUT" "$A_DIRECT_BASE" "$A_DIRECT_AFTER" "$A_VPN_BASE" "$A_VPN_AFTER" \
    "$A_DIRECT_DNAT_BASE" "$A_DIRECT_DNAT_AFTER" "$A_EXIT_DNAT_BASE" "$A_EXIT_DNAT_AFTER" \
    "$A_EXIT_BASE_RX" "$A_EXIT_AFTER_RX" "$A_EXIT_BASE_TX" "$A_EXIT_AFTER_TX"
  print_singbox_dns_match_logs A

  enable_explicit_dns_route_action
  SLOT_B="$(dns_slot_text)"
  assert_only_explicit_action_diff "$SLOT_A" "$SLOT_B"
  OTHER_SLOT_HASHES_B="$(slot_hashes_except_dns_routes)"
  [[ "$OTHER_SLOT_HASHES_A" == "$OTHER_SLOT_HASHES_B" ]] || fail "A/B changed a config slot other than 19-dns-routes.json"

  RESTART_RESPONSE="$(docker exec "$GATEWAY" curl -fsS --max-time 25 -H 'Content-Type: application/json' -X POST \
    --data-binary '{"action":"restart"}' http://127.0.0.1:2222/api/singbox/control)" || fail "Sing-box rejected/restarted the explicit DNS Rule Action variant"
  printf '%s' "$RESTART_RESPONSE" | python -c 'import json,sys; data=json.load(sys.stdin).get("data",{}); print("AB_B_RESTART running="+str(data.get("running"))+" lastError="+str(data.get("lastError", "")))'
  ready=0
  for _ in $(seq 1 60); do
    RUNNING="$(docker exec "$GATEWAY" curl -fsS --max-time 2 http://127.0.0.1:2222/api/singbox/status 2>/dev/null | python -c 'import json,sys; print(int(bool(json.load(sys.stdin).get("data",{}).get("running"))))' 2>/dev/null || true)"
    if [[ "$RUNNING" == 1 ]] && docker exec "$GATEWAY" ip link show awgm0 >/dev/null 2>&1; then ready=1; break; fi
    sleep 0.5
  done
  [[ "$ready" == 1 ]] || fail "Sing-box did not return to running state with the B config"
  SLOT_B_RUNNING="$(dns_slot_text)"
  assert_only_explicit_action_diff "$SLOT_A" "$SLOT_B_RUNNING"
  printf 'AB_EFFECTIVE_CONFIG_B='; debug_merged_dns_config

  B_DIRECT_BASE="$(dns_packet_count "$DIRECT_SINK")"
  B_VPN_BASE="$(dns_packet_count "$VPN_SINK")"
  B_DIRECT_DNAT_BASE="$(gateway_output_nat_packets)"
  B_EXIT_DNAT_BASE="$(vpn_exit_nat_packets)"
  read -r B_EXIT_BASE_RX B_EXIT_BASE_TX <<<"$(server_counters)"
  start_singbox_dns_log_capture
  start_dns_trace
  if B_OUTPUT="$(query_dns "$VPN_QUERY_B")"; then B_STATUS=0; else B_STATUS=$?; fi
  B_TRACE="$(stop_dns_trace)"
  stop_singbox_dns_log_capture
  B_DIRECT_AFTER="$(dns_packet_count "$DIRECT_SINK")"
  B_VPN_AFTER="$(dns_packet_count "$VPN_SINK")"
  B_DIRECT_DNAT_AFTER="$(gateway_output_nat_packets)"
  B_EXIT_DNAT_AFTER="$(vpn_exit_nat_packets)"
  read -r B_EXIT_AFTER_RX B_EXIT_AFTER_TX <<<"$(server_counters)"
  if (( B_VPN_AFTER > B_VPN_BASE && B_DIRECT_AFTER == B_DIRECT_BASE )); then B_PATH=VPN
  elif (( B_DIRECT_AFTER > B_DIRECT_BASE && B_VPN_AFTER == B_VPN_BASE )); then B_PATH=DIRECT
  elif (( B_DIRECT_AFTER == B_DIRECT_BASE && B_VPN_AFTER == B_VPN_BASE )); then B_PATH=NO_FIXTURE
  else B_PATH=AMBIGUOUS
  fi
  printf 'AB_B action=route qname=%s status=%s path=%s answer=%s direct_fixture=%s->%s vpn_fixture=%s->%s gateway_output_dnat=%s->%s vpn_exit_prerouting_dnat=%s->%s vpn_exit_wg_rx=%s->%s vpn_exit_wg_tx=%s->%s\n' \
    "$VPN_QUERY_B" "$B_STATUS" "$B_PATH" "$B_OUTPUT" "$B_DIRECT_BASE" "$B_DIRECT_AFTER" "$B_VPN_BASE" "$B_VPN_AFTER" \
    "$B_DIRECT_DNAT_BASE" "$B_DIRECT_DNAT_AFTER" "$B_EXIT_DNAT_BASE" "$B_EXIT_DNAT_AFTER" \
    "$B_EXIT_BASE_RX" "$B_EXIT_AFTER_RX" "$B_EXIT_BASE_TX" "$B_EXIT_AFTER_TX"
  print_singbox_dns_match_logs B
  assert_policy_tun_trace AB-A "$A_TRACE"
  assert_policy_tun_trace AB-B "$B_TRACE"
  if [[ "$A_PATH" == "$B_PATH" ]]; then
    printf 'AB_BEHAVIOR=unchanged (legacy=%s explicit_action=%s)\n' "$A_PATH" "$B_PATH"
  else
    printf 'AB_BEHAVIOR=changed (legacy=%s explicit_action=%s)\n' "$A_PATH" "$B_PATH"
  fi
  if [[ "$A_PATH" == NO_FIXTURE || "$A_PATH" == AMBIGUOUS || "$B_PATH" == NO_FIXTURE || "$B_PATH" == AMBIGUOUS ]]; then
    fail "A/B did not produce one unambiguous DNS fixture path for both unique queries"
  fi
  printf 'PASS: isolated DNS A/B discriminator completed; this is not by itself a full VPN DNS gate pass.\n'
  exit 0
fi

LOGGING_HASH_BEFORE="$(applied_hash)"
docker exec "$GATEWAY" curl -fsS --max-time 8 -H 'Content-Type: application/json' -X POST \
  --data-binary '{"logging":{"singboxLogLevel":"debug"}}' \
  http://127.0.0.1:2222/api/settings/update |
  python -c 'import json,sys; data=json.load(sys.stdin).get("data",{}); level=data.get("logging",{}).get("singboxLogLevel"); assert level=="debug", f"debug logging was not applied: {level!r}"; print("SINGBOX_DIAGNOSTIC_LOG_LEVEL=debug")'
wait_for_apply "$LOGGING_HASH_BEFORE" || fail "Sing-box did not apply diagnostic debug logging"
VPN_BASE="$(dns_packet_count "$VPN_SINK")"
DIRECT_BASE="$(dns_packet_count "$DIRECT_SINK")"
BASE_COUNTS="$(server_counters)"
read -r BASE_RX BASE_TX <<<"$BASE_COUNTS"
[[ "$BASE_RX" =~ ^[0-9]+$ && "$BASE_TX" =~ ^[0-9]+$ ]] || fail "invalid baseline VPN counters"
EXIT_DNAT_BASE="$(vpn_exit_nat_packets)"
GATEWAY_WG_BASE="$(docker exec "$GATEWAY" wg show awg0 transfer | awk '{rx += $2; tx += $3} END {printf "%d %d\n", rx, tx}')"
CLIENT_WG_BASE="$(docker exec "$CLIENT" wg show awg-client transfer | awk '{rx += $2; tx += $3} END {printf "%d %d\n", rx, tx}')"
start_singbox_dns_log_capture
start_dns_trace
if VPN_OUTPUT="$(query_dns "$VPN_DOMAIN")"; then VPN_STATUS=0; else VPN_STATUS=$?; fi
VPN_TRACE="$(stop_dns_trace)"
stop_singbox_dns_log_capture
if [[ "$VPN_STATUS" != 0 || "$VPN_OUTPUT" != *"$DNS_ANSWER_VPN"* ]]; then
  printf 'DEBUG VPN query status=%s output=%s\n' "$VPN_STATUS" "$VPN_OUTPUT" >&2
  print_singbox_dns_match_logs VPN >&2
  printf 'DEBUG nft trace:\n%s\n' "$VPN_TRACE" >&2
  printf 'DEBUG fixture UDP packets direct=%s vpn=%s\n' "$(dns_packet_count "$DIRECT_SINK")" "$(dns_packet_count "$VPN_SINK")" >&2
  printf 'DEBUG VPN fixture logs:\n' >&2
  docker logs "$VPN_SINK" 2>&1 >&2 || true
  printf 'DEBUG direct fixture logs:\n' >&2
  docker logs "$DIRECT_SINK" 2>&1 >&2 || true
  printf 'DEBUG compiled DNS routes slot:\n' >&2
  docker exec "$GATEWAY" sh -ec 'cat /data/sing-box/config.d/19-dns-routes.json' >&2 || true
  printf 'DEBUG effective merged DNS config (sanitized):\n' >&2
  debug_merged_dns_config >&2 || true
  printf 'DEBUG DNS route API readback:\n' >&2
  docker exec "$GATEWAY" curl -fsS --max-time 3 http://127.0.0.1:2222/api/dns-routes/list >&2 || true
  printf 'DEBUG gateway WG transfer (RX TX): ' >&2
  printf 'DEBUG gateway awg0 peer transfer before=%s after=' "$GATEWAY_WG_BASE" >&2
  gateway_endpoint_counters >&2 || true
  printf 'DEBUG client awg-client transfer before=%s after=' "$CLIENT_WG_BASE" >&2
  docker exec "$CLIENT" wg show awg-client transfer | awk '{rx += $2; tx += $3} END {printf "%d %d\n", rx, tx}' >&2 || true
  printf 'DEBUG gateway awgm0 counters:\n' >&2
  docker exec "$GATEWAY" ip -s link show dev awgm0 >&2 || true
  AFTER_COUNTS="$(server_counters)"
  read -r AFTER_RX AFTER_TX <<<"$AFTER_COUNTS"
  printf 'DEBUG VPN_EXIT wg0 transfer before=%s/%s after=%s/%s\n' "$BASE_RX" "$BASE_TX" "$AFTER_RX" "$AFTER_TX" >&2
  printf 'DEBUG VPN_EXIT routes and DNS NAT/forward rules:\n' >&2
  docker exec "$VPN_EXIT" sh -ec 'ip route; iptables -t nat -nvxL PREROUTING; iptables -t nat -nvxL POSTROUTING; iptables -nvxL FORWARD; iptables -S FORWARD' >&2 || true
  printf 'DEBUG Gateway DNS OUTPUT DNAT counters:\n' >&2
  docker exec "$GATEWAY" iptables -t nat -nvxL OUTPUT >&2 || true
  printf 'DEBUG sing-box status (sanitized)=' >&2
  docker exec "$GATEWAY" curl -fsS --max-time 3 http://127.0.0.1:2222/api/singbox/status 2>/dev/null | python -c 'import json,re,sys; data=json.load(sys.stdin).get("data",{}); error=str(data.get("lastError", "")); error=re.sub(r"(?<![A-Za-z0-9])[A-Za-z0-9+/]{40,}={0,2}(?![A-Za-z0-9])", "[REDACTED]", error); print(json.dumps({"running":data.get("running"),"lastError":error}, sort_keys=True))' >&2 || true
  fail "VPN DNS query did not return the VPN fixture A record"
fi
assert_policy_tun_trace VPN "$VPN_TRACE"
VPN_RUNTIME_LOGS="$(print_singbox_dns_match_logs VPN)"
printf '%s\n' "$VPN_RUNTIME_LOGS"
[[ "$VPN_RUNTIME_LOGS" == *"inbound=awg-gateway-in port=53 => hijack-dns"* ]] || fail "runtime log did not confirm Gateway DNS hijack"
[[ "$VPN_RUNTIME_LOGS" == *"domain_suffix=$VPN_DOMAIN => route($VPN_DNS_SERVER_TAG)"* ]] || fail "runtime log did not confirm the VPN DNS server selection"
[[ "$VPN_RUNTIME_LOGS" == *"endpoint/awg[$ENDPOINT_TAG]"*"received handshake response"* ]] || fail "runtime log did not confirm AWG3 endpoint handshake"
[[ "$VPN_RUNTIME_LOGS" == *"dns: exchanged A $VPN_DOMAIN. 120 IN A $DNS_ANSWER_VPN"* ]] || fail "runtime log did not confirm the exact VPN DNS answer"
VPN_AFTER="$(dns_packet_count "$VPN_SINK")"
DIRECT_AFTER_VPN="$(dns_packet_count "$DIRECT_SINK")"
(( VPN_AFTER > VPN_BASE )) || fail "VPN DNS query did not hit the VPN-only fixture"
[[ "$DIRECT_AFTER_VPN" == "$DIRECT_BASE" ]] || fail "VPN DNS query unexpectedly used the direct fixture"
COUNTS="$(server_counters)"
read -r RX TX <<<"$COUNTS"
[[ "$RX" =~ ^[0-9]+$ && "$TX" =~ ^[0-9]+$ ]] || fail "invalid final VPN counters"
(( RX > BASE_RX && TX > BASE_TX )) || fail "VPN server WireGuard RX/TX counters did not both increase"
EXIT_DNAT_AFTER="$(vpn_exit_nat_packets)"
(( EXIT_DNAT_AFTER > EXIT_DNAT_BASE )) || fail "VPN_EXIT DNS DNAT counter did not increase"
printf 'PASS: VPN DNS query crossed the synthetic AWG peer, reached its fixture, and returned the exact A record.\n'

DIRECT_BASE="$(dns_packet_count "$DIRECT_SINK")"
VPN_BASE="$(dns_packet_count "$VPN_SINK")"
start_dns_trace
if BLOCK_OUTPUT="$(query_dns "$BLOCK_DOMAIN")"; then BLOCK_STATUS=0; else BLOCK_STATUS=$?; fi
BLOCK_TRACE="$(stop_dns_trace)"
[[ "$BLOCK_OUTPUT" != *"$DNS_ANSWER_BLOCK"* ]] || fail "BLOCK DNS query unexpectedly returned the fixture A record"
assert_policy_tun_trace BLOCK "$BLOCK_TRACE"
[[ "$(dns_packet_count "$DIRECT_SINK")" == "$DIRECT_BASE" ]] || fail "BLOCK DNS query reached the direct fixture"
[[ "$(dns_packet_count "$VPN_SINK")" == "$VPN_BASE" ]] || fail "BLOCK DNS query reached the VPN fixture"
printf 'PASS: BLOCK DNS query returned no fixture answer and reached neither fixture.\n'

FAIL_CLOSED_DOMAIN="failclosed-${ID}.${VPN_DOMAIN}"
FAIL_CLOSED_DIRECT_BASE="$(dns_packet_count "$DIRECT_SINK")"
FAIL_CLOSED_VPN_BASE="$(dns_packet_count "$VPN_SINK")"
docker exec "$VPN_EXIT" ip link set wg0 down
start_singbox_dns_log_capture
if FAIL_CLOSED_OUTPUT="$(query_dns "$FAIL_CLOSED_DOMAIN")"; then FAIL_CLOSED_STATUS=0; else FAIL_CLOSED_STATUS=$?; fi
stop_singbox_dns_log_capture
FAIL_CLOSED_LOGS="$(print_singbox_dns_match_logs FAIL_CLOSED)"
printf '%s\n' "$FAIL_CLOSED_LOGS"
[[ "$FAIL_CLOSED_LOGS" == *"domain_suffix=$VPN_DOMAIN => route($VPN_DNS_SERVER_TAG)"* ]] || fail "fail-closed query did not remain on the configured VPN DNS route"
[[ "$FAIL_CLOSED_STATUS" != 0 && "$FAIL_CLOSED_OUTPUT" != *"192.0.2."* ]] || fail "VPN-unavailable DNS query unexpectedly returned an answer"
[[ "$(dns_packet_count "$DIRECT_SINK")" == "$FAIL_CLOSED_DIRECT_BASE" ]] || fail "VPN-unavailable DNS query leaked to the direct fixture"
[[ "$(dns_packet_count "$VPN_SINK")" == "$FAIL_CLOSED_VPN_BASE" ]] || fail "VPN-unavailable DNS query reached the VPN fixture"
printf 'PASS: VPN-unavailable DNS query failed closed without reaching either fixture.\n'
printf 'PASS: isolated UDP/DNS routing gate completed (DIRECT + VPN + BLOCK + fail-closed).\n'
