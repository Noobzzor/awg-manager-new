/* SPDX-License-Identifier: GPL-2.0 */
/*
 * Host unit tests for awg_classify_recv() — the worker recv-loop
 * classifier in src/proxy_recv.h. The classifier is pure and has no
 * kernel-side dependencies, so we test it directly.
 *
 * Critical assertion for issue #234:
 *   awg_classify_recv(0, false) == AWG_RECV_RETRY_YIELD
 * i.e. an empty recv (UDP zero-length datagram, or kernel_sock_shutdown
 * before kthread_should_stop arms) does NOT exit the loop. Pre-fix
 * code treated n == 0 as EOF and broke out of the loop; that turned a
 * single empty datagram into a silent worker kill / local DoS.
 *
 * Build via tests/Makefile (TARGET=test_proxy_recv).
 */

/* proxy_recv.h is dual-mode (kernel/host); host build needs nothing
 * beyond stdbool + stdlib errno, which the header pulls itself. shim.h
 * is included for the test-framework conveniences (stdio, etc.).
 * endpoint.h supplies awg_src_matches — the pure RX source filter used
 * by awg_encap_rcv (proxy.c) — tested below for both address families. */
#include "shim.h"
#include "../src/proxy_recv.h"
#include "../src/endpoint.h"

#include <arpa/inet.h>

/* Tiny assertion framework — same pattern as test_cps.c / test_transform.c. */
static int g_run, g_failed;
#define EXPECT(cond) do {                                                     \
		if (!(cond)) {                                                \
			fprintf(stderr, "  FAIL %s:%d  %s\n",                 \
				__FILE__, __LINE__, #cond);                   \
			g_failed++;                                           \
		}                                                             \
	} while (0)
#define TEST(name) static void name(void); \
	static void name##_run(void) { g_run++; name(); } \
	static void name(void)

/* ---- shutdown wins over recv result ---- */

TEST(test_should_stop_with_zero_recv_breaks)
{
	EXPECT(awg_classify_recv(0, true) == AWG_RECV_BREAK);
}

TEST(test_should_stop_with_full_packet_breaks)
{
	EXPECT(awg_classify_recv(148, true) == AWG_RECV_BREAK);
}

TEST(test_should_stop_with_transient_error_breaks)
{
	EXPECT(awg_classify_recv(-EAGAIN, true) == AWG_RECV_BREAK);
}

TEST(test_should_stop_with_shutdown_errno_breaks)
{
	EXPECT(awg_classify_recv(-EBADF, true) == AWG_RECV_BREAK);
}

/* ---- shutdown-class errnos exit the loop ---- */

TEST(test_erestartsys_breaks)
{
	EXPECT(awg_classify_recv(-ERESTARTSYS, false) == AWG_RECV_BREAK);
}

TEST(test_eintr_breaks)
{
	EXPECT(awg_classify_recv(-EINTR, false) == AWG_RECV_BREAK);
}

TEST(test_eshutdown_breaks)
{
	EXPECT(awg_classify_recv(-ESHUTDOWN, false) == AWG_RECV_BREAK);
}

TEST(test_ebadf_breaks)
{
	EXPECT(awg_classify_recv(-EBADF, false) == AWG_RECV_BREAK);
}

TEST(test_epipe_breaks)
{
	EXPECT(awg_classify_recv(-EPIPE, false) == AWG_RECV_BREAK);
}

/* ---- transient errnos retry with sleep ---- */

TEST(test_eagain_retries_with_sleep)
{
	EXPECT(awg_classify_recv(-EAGAIN, false) == AWG_RECV_RETRY_SLEEP);
}

TEST(test_enomem_retries_with_sleep)
{
	EXPECT(awg_classify_recv(-ENOMEM, false) == AWG_RECV_RETRY_SLEEP);
}

TEST(test_unknown_negative_retries_with_sleep)
{
	/* Any not-listed negative errno is treated as transient. */
	EXPECT(awg_classify_recv(-999, false) == AWG_RECV_RETRY_SLEEP);
}

/* ---- ISSUE #234: zero-length recv MUST NOT break ---- */

TEST(test_zero_length_recv_yields_not_breaks)
{
	/* This is THE assertion that documents the #234 fix.
	 * Pre-fix this returned AWG_RECV_BREAK, which let any local
	 * sender kill the worker with a single empty UDP datagram. */
	EXPECT(awg_classify_recv(0, false) == AWG_RECV_RETRY_YIELD);
}

/* ---- runt packets (n < 4) yield ---- */

TEST(test_one_byte_packet_yields)
{
	EXPECT(awg_classify_recv(1, false) == AWG_RECV_RETRY_YIELD);
}

TEST(test_three_byte_packet_yields)
{
	EXPECT(awg_classify_recv(3, false) == AWG_RECV_RETRY_YIELD);
}

/* ---- full packets are processed ---- */

TEST(test_four_byte_packet_processes)
{
	/* 4 bytes is the boundary — minimum size we'd consider real. */
	EXPECT(awg_classify_recv(4, false) == AWG_RECV_PROCESS);
}

TEST(test_handshake_init_size_processes)
{
	/* 148 = WireGuard handshake init. */
	EXPECT(awg_classify_recv(148, false) == AWG_RECV_PROCESS);
}

TEST(test_full_mtu_processes)
{
	EXPECT(awg_classify_recv(1500, false) == AWG_RECV_PROCESS);
}

TEST(test_large_packet_processes)
{
	EXPECT(awg_classify_recv(2048, false) == AWG_RECV_PROCESS);
}

/* ---- awg_src_matches: RX encap source filter (endpoint.h) ---- */

static struct awg_endpoint_addr mk_v4(const char *ip)
{
	struct awg_endpoint_addr a;

	memset(&a, 0, sizeof(a));
	a.family = AF_INET;
	a.ip4 = (__be32)inet_addr(ip);
	return a;
}

static struct awg_endpoint_addr mk_v6(const char *ip)
{
	struct awg_endpoint_addr a;

	memset(&a, 0, sizeof(a));
	a.family = AF_INET6;
	inet_pton(AF_INET6, ip, &a.ip6);
	return a;
}

TEST(test_src_matches_v4_accepts_configured_server)
{
	struct awg_endpoint_addr remote = mk_v4("203.0.113.1");
	__be32 src = (__be32)inet_addr("203.0.113.1");

	EXPECT(awg_src_matches(&remote, htons(51820), AF_INET, &src,
			       htons(51820)));
}

TEST(test_src_matches_v4_rejects_wrong_addr)
{
	struct awg_endpoint_addr remote = mk_v4("203.0.113.1");
	__be32 src = (__be32)inet_addr("203.0.113.2");

	EXPECT(!awg_src_matches(&remote, htons(51820), AF_INET, &src,
				htons(51820)));
}

TEST(test_src_matches_v4_rejects_wrong_port)
{
	struct awg_endpoint_addr remote = mk_v4("203.0.113.1");
	__be32 src = (__be32)inet_addr("203.0.113.1");

	EXPECT(!awg_src_matches(&remote, htons(51820), AF_INET, &src,
				htons(51821)));
}

TEST(test_src_matches_v6_accepts_configured_server)
{
	struct awg_endpoint_addr remote = mk_v6("2001:db8::1");
	struct in6_addr src;

	inet_pton(AF_INET6, "2001:db8::1", &src);
	EXPECT(awg_src_matches(&remote, htons(443), AF_INET6, &src,
			       htons(443)));
}

TEST(test_src_matches_v6_rejects_wrong_addr)
{
	struct awg_endpoint_addr remote = mk_v6("2001:db8::1");
	struct in6_addr src;

	inet_pton(AF_INET6, "2001:db8::2", &src);
	EXPECT(!awg_src_matches(&remote, htons(443), AF_INET6, &src,
				htons(443)));
}

TEST(test_src_matches_v6_rejects_wrong_port)
{
	struct awg_endpoint_addr remote = mk_v6("2001:db8::1");
	struct in6_addr src;

	inet_pton(AF_INET6, "2001:db8::1", &src);
	EXPECT(!awg_src_matches(&remote, htons(443), AF_INET6, &src,
				htons(444)));
}

TEST(test_src_matches_rejects_family_mismatch)
{
	/* A v4-configured slot must not match a v6 source and vice versa,
	 * even if the leading 4 bytes of the v6 address happen to equal the
	 * configured v4 address. */
	struct awg_endpoint_addr remote4 = mk_v4("203.0.113.1");
	struct awg_endpoint_addr remote6 = mk_v6("2001:db8::1");
	struct in6_addr src6;
	__be32 src4 = (__be32)inet_addr("203.0.113.1");

	memset(&src6, 0, sizeof(src6));
	memcpy(&src6, &src4, 4);
	EXPECT(!awg_src_matches(&remote4, htons(51820), AF_INET6, &src6,
				htons(51820)));
	EXPECT(!awg_src_matches(&remote6, htons(51820), AF_INET, &src4,
				htons(51820)));
}

TEST(test_endpoint_addr_equal_families)
{
	struct awg_endpoint_addr a4 = mk_v4("203.0.113.1");
	struct awg_endpoint_addr b4 = mk_v4("203.0.113.1");
	struct awg_endpoint_addr c4 = mk_v4("203.0.113.2");
	struct awg_endpoint_addr a6 = mk_v6("2001:db8::1");
	struct awg_endpoint_addr b6 = mk_v6("2001:db8::1");
	struct awg_endpoint_addr c6 = mk_v6("2001:db8::2");

	EXPECT(awg_endpoint_addr_equal(&a4, &b4));
	EXPECT(!awg_endpoint_addr_equal(&a4, &c4));
	EXPECT(awg_endpoint_addr_equal(&a6, &b6));
	EXPECT(!awg_endpoint_addr_equal(&a6, &c6));
	EXPECT(!awg_endpoint_addr_equal(&a4, &a6));
}

int main(void)
{
	test_should_stop_with_zero_recv_breaks_run();
	test_should_stop_with_full_packet_breaks_run();
	test_should_stop_with_transient_error_breaks_run();
	test_should_stop_with_shutdown_errno_breaks_run();

	test_erestartsys_breaks_run();
	test_eintr_breaks_run();
	test_eshutdown_breaks_run();
	test_ebadf_breaks_run();
	test_epipe_breaks_run();

	test_eagain_retries_with_sleep_run();
	test_enomem_retries_with_sleep_run();
	test_unknown_negative_retries_with_sleep_run();

	test_zero_length_recv_yields_not_breaks_run();

	test_one_byte_packet_yields_run();
	test_three_byte_packet_yields_run();

	test_four_byte_packet_processes_run();
	test_handshake_init_size_processes_run();
	test_full_mtu_processes_run();
	test_large_packet_processes_run();

	test_src_matches_v4_accepts_configured_server_run();
	test_src_matches_v4_rejects_wrong_addr_run();
	test_src_matches_v4_rejects_wrong_port_run();
	test_src_matches_v6_accepts_configured_server_run();
	test_src_matches_v6_rejects_wrong_addr_run();
	test_src_matches_v6_rejects_wrong_port_run();
	test_src_matches_rejects_family_mismatch_run();
	test_endpoint_addr_equal_families_run();

	fprintf(stderr, "\n=== %d run, %d failed ===\n", g_run, g_failed);
	return g_failed ? 1 : 0;
}
