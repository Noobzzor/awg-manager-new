/* SPDX-License-Identifier: GPL-2.0 */
/*
 * AWG Proxy — tunnel configuration parsing.
 * Parses procfs config lines into awg_config_t (defined in transform.h).
 */
#ifndef _AWG_PROXY_TUNNEL_H
#define _AWG_PROXY_TUNNEL_H

#include "transform.h"

/*
 * Parse config line into an awg_config_t struct.
 * Format: "IP:PORT H1=min-max ... S1=N ... PUB_SERVER=hex PUB_CLIENT=hex I1=\"...\""
 * Calls config_compute() and cps_parse() internally.
 * Returns 0 on success, negative errno on error.
 */
int awg_config_parse(const char *config_line, awg_config_t *cfg);

/*
 * Parse one endpoint token into addr + port. Shared by /proc add
 * (awg_config_parse) and /proc del (main.c) so both accept the same forms:
 *   "A.B.C.D:PORT"    legacy IPv4 — behavior identical to the historical
 *                     strrchr(':') + in_aton path (hostnames and dotted
 *                     quads are both fed to in_aton, unchanged);
 *   "[IPV6...]:PORT"  bracketed IPv6, parsed with in6_pton.
 * Bare IPv6 without brackets (more than one ':' outside brackets) is
 * rejected with a pr_warn — the colons are ambiguous with the port
 * separator. Returns 0 on success, -EINVAL on malformed input.
 */
int awg_endpoint_parse(const char *str, struct awg_endpoint_addr *addr,
		       __be16 *port);

/* Free CPS templates allocated by awg_config_parse */
void awg_config_free(awg_config_t *cfg);

#endif /* _AWG_PROXY_TUNNEL_H */
