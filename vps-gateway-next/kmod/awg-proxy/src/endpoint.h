/* SPDX-License-Identifier: GPL-2.0 */
/*
 * AWG Proxy — family-tagged remote endpoint address (IPv4 or IPv6).
 *
 * The remote AWG server may live behind an IPv4 or an IPv6 address; the
 * config keeps one union tagged by `family` so every consumer (dup check,
 * socket creation, TX route lookup, RX source filter, list/log formatting)
 * branches on the same field instead of growing parallel structs.
 *
 * Deliberately free of kernel-only APIs (no ipv6_addr_equal, no ip_hdr):
 * the helpers below are pure over caller-supplied pointers so the exact
 * same code compiles and runs in the host-only unit-test harness
 * (kmod/awg-proxy/tests/), where stubs/ maps the <linux/...> includes to
 * libc headers.
 */
#ifndef _AWG_PROXY_ENDPOINT_H
#define _AWG_PROXY_ENDPOINT_H

#include <linux/types.h>
#include <linux/socket.h>	/* AF_INET / AF_INET6 */
#include <linux/in6.h>		/* struct in6_addr */
#include <linux/string.h>

struct awg_endpoint_addr {
	u8 family;		/* AF_INET or AF_INET6 */
	union {
		__be32 ip4;
		struct in6_addr ip6;
	};
};

static inline bool awg_endpoint_addr_equal(const struct awg_endpoint_addr *a,
					   const struct awg_endpoint_addr *b)
{
	if (a->family != b->family)
		return false;
	if (a->family == AF_INET6)
		return memcmp(&a->ip6, &b->ip6, sizeof(a->ip6)) == 0;
	return a->ip4 == b->ip4;
}

/*
 * RX source filter — does a received datagram's source (address family,
 * source address, UDP source port) match the configured remote endpoint?
 *
 * The remote socket is unconnected (see create_remote_socket in proxy.c),
 * so ANY host that discovers our ephemeral port can inject datagrams:
 * flood the rx_queue, skew the counters, or feed forgeries into the
 * decrypt-failed-forward-as-is path. Only the configured server passes.
 *
 * saddr points at a __be32 (family == AF_INET, e.g. &ip_hdr(skb)->saddr)
 * or a struct in6_addr (family == AF_INET6, e.g. &ipv6_hdr(skb)->saddr).
 * All values are big-endian wire format on both sides — no conversion.
 */
static inline bool awg_src_matches(const struct awg_endpoint_addr *remote,
				   __be16 remote_port, u8 family,
				   const void *saddr, __be16 sport)
{
	if (sport != remote_port || family != remote->family)
		return false;
	if (family == AF_INET6)
		return memcmp(saddr, &remote->ip6, sizeof(remote->ip6)) == 0;
	return *(const __be32 *)saddr == remote->ip4;
}

#endif /* _AWG_PROXY_ENDPOINT_H */
