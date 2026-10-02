// SPDX-License-Identifier: GPL-2.0
/*
 * Userspace unit tests for kmod/awg-proxy/src/cps.c.
 *
 * Covers six wire-format invariants that must match the reference
 * implementation (amnezia-vpn/amneziawg-linux-kernel-module/src/junk.c):
 *
 *   1. <c> counter encoded big-endian (htonl semantics)
 *   2. <t> timestamp encoded big-endian
 *   3. <rc> charset = 52 letters only (a-z + A-Z), no digits
 *   4. <rd> charset = 10 digits only
 *   5. <b 0xHEX> static bytes copied verbatim
 *   6. cps_generate_all is pure: does NOT mutate the counter values
 *      passed to it (counter-increment-after-send is the caller's job)
 *
 * Run via `make test`.
 */

#include "shim.h"
#include "../src/cps.h"

#include <string.h>
#include <stdlib.h>
#include <stdio.h>
#include <stdarg.h>

/* ---- Tiny test harness ---- */

static int tests_run, tests_failed;

static void test_fail(const char *test, const char *fmt, ...)
{
	va_list ap;

	fprintf(stderr, "FAIL %s: ", test);
	va_start(ap, fmt);
	vfprintf(stderr, fmt, ap);
	va_end(ap);
	fputc('\n', stderr);
	tests_failed++;
}

#define ASSERT_EQ_BYTES(test, got, want, n) do {                          \
	if (memcmp((got), (want), (n)) != 0) {                            \
		test_fail((test), "bytes mismatch (len %d)", (int)(n));   \
		dump_hex("  got ", (got), (n));                           \
		dump_hex("  want", (want), (n));                          \
	}                                                                  \
} while (0)

#define ASSERT_TRUE(test, cond, msg) do {                                  \
	if (!(cond)) test_fail((test), "%s", (msg));                       \
} while (0)

static void dump_hex(const char *label, const void *buf, int n)
{
	const uint8_t *p = (const uint8_t *)buf;
	int i;

	fprintf(stderr, "%s: ", label);
	for (i = 0; i < n; i++)
		fprintf(stderr, "%02x ", p[i]);
	fputc('\n', stderr);
}

static cps_template_t *parse_or_die(const char *s)
{
	cps_template_t *t = calloc(1, sizeof(*t));
	if (cps_parse(s, t) != 0) {
		fprintf(stderr, "cps_parse(%s) failed\n", s);
		exit(2);
	}
	return t;
}

/* ---------- Tests ---------- */

/* Test 1: <c> counter is encoded big-endian.
 * Counter = 0x12345678 must serialize as bytes 12 34 56 78. */
static void test_counter_big_endian(void)
{
	cps_template_t *t;
	uint8_t buf[64];
	int n;
	const uint8_t want[] = { 0x12, 0x34, 0x56, 0x78 };

	tests_run++;
	t = parse_or_die("<c>");
	n = cps_generate(t, 0x12345678u, buf, sizeof(buf));
	if (n != 4) {
		test_fail("counter_big_endian", "expected 4 bytes, got %d", n);
		goto done;
	}
	ASSERT_EQ_BYTES("counter_big_endian", buf, want, 4);
done:
	free(t);
}

/* Test 2: <t> timestamp is encoded big-endian.
 * Fixed timestamp 0xCAFEBABE must serialize as bytes ca fe ba be. */
static void test_timestamp_big_endian(void)
{
	cps_template_t *t;
	uint8_t buf[64];
	int n;
	const uint8_t want[] = { 0xCA, 0xFE, 0xBA, 0xBE };

	tests_run++;
	shim_set_fixed_time(0xCAFEBABEu);
	t = parse_or_die("<t>");
	n = cps_generate(t, 0, buf, sizeof(buf));
	if (n != 4) {
		test_fail("timestamp_big_endian", "expected 4 bytes, got %d", n);
		goto done;
	}
	ASSERT_EQ_BYTES("timestamp_big_endian", buf, want, 4);
done:
	free(t);
}

/* Test 3: <rc> random chars MUST be letters only (a-z + A-Z), NO digits. */
static void test_rc_charset_no_digits(void)
{
	cps_template_t *t;
	uint8_t buf[1024];
	int n, i;

	tests_run++;
	shim_set_random_seed(0xC0FFEE);
	t = parse_or_die("<rc 512>");
	n = cps_generate(t, 0, buf, sizeof(buf));
	if (n != 512) {
		test_fail("rc_charset_no_digits", "expected 512, got %d", n);
		goto done;
	}
	for (i = 0; i < 512; i++) {
		uint8_t c = buf[i];
		int is_lower = (c >= 'a' && c <= 'z');
		int is_upper = (c >= 'A' && c <= 'Z');

		if (!is_lower && !is_upper) {
			test_fail("rc_charset_no_digits",
				  "byte %d = 0x%02x ('%c') not in [a-zA-Z]",
				  i, c, c);
			goto done;
		}
	}
done:
	free(t);
}

/* Test 4: <rd> random digits MUST be only 0-9. */
static void test_rd_charset_only_digits(void)
{
	cps_template_t *t;
	uint8_t buf[1024];
	int n, i;

	tests_run++;
	shim_set_random_seed(0xBADF00D);
	t = parse_or_die("<rd 200>");
	n = cps_generate(t, 0, buf, sizeof(buf));
	if (n != 200) {
		test_fail("rd_charset_only_digits", "expected 200, got %d", n);
		goto done;
	}
	for (i = 0; i < 200; i++) {
		if (buf[i] < '0' || buf[i] > '9') {
			test_fail("rd_charset_only_digits",
				  "byte %d = 0x%02x not in [0-9]",
				  i, buf[i]);
			goto done;
		}
	}
done:
	free(t);
}

/* Test 5: <b 0xHEX> static bytes copied verbatim. */
static void test_static_bytes(void)
{
	cps_template_t *t;
	uint8_t buf[64];
	int n;
	const uint8_t want[] = { 0xDE, 0xAD, 0xBE, 0xEF, 0x01, 0x02 };

	tests_run++;
	t = parse_or_die("<b 0xDEADBEEF0102>");
	n = cps_generate(t, 0, buf, sizeof(buf));
	if (n != 6) {
		test_fail("static_bytes", "expected 6 bytes, got %d", n);
		goto done;
	}
	ASSERT_EQ_BYTES("static_bytes", buf, want, 6);
done:
	free(t);
}

/* Test 6: cps_generate_all does NOT mutate state.
 * Caller passes counters[5]; each non-null template uses counters[k]
 * (slot-indexed, not template-slot indexed). Counter value at
 * generation time MUST exactly match the array entry. */
static void test_generate_all_is_pure(void)
{
	cps_template_t *t1, *t2, *t3;
	cps_template_t *templates[5] = { 0 };
	uint8_t bufs[5][1500];
	int lens[5];
	uint32_t counters[5];
	int count;
	uint32_t got_c1, got_c2;

	tests_run++;
	t1 = parse_or_die("<c>");
	t2 = parse_or_die("<c>");
	t3 = parse_or_die("<c>");
	templates[0] = t1;
	templates[2] = t2;  /* slot 1 deliberately NULL */
	templates[4] = t3;  /* slot 3 deliberately NULL */

	/* Caller pre-computes; cps_generate_all must not touch counters[]. */
	counters[0] = 100;
	counters[1] = 101;
	counters[2] = 102;
	counters[3] = 103;
	counters[4] = 104;

	count = cps_generate_all(templates, counters, bufs, lens);
	if (count != 3) {
		test_fail("generate_all_is_pure",
			  "expected 3 packets (3 non-null), got %d", count);
		goto done;
	}

	/* counters[] must be untouched. */
	if (counters[0] != 100 || counters[1] != 101 || counters[2] != 102 ||
	    counters[3] != 103 || counters[4] != 104) {
		test_fail("generate_all_is_pure",
			  "counters[] was mutated: {%u, %u, %u, %u, %u}",
			  counters[0], counters[1], counters[2],
			  counters[3], counters[4]);
		goto done;
	}

	/* Output slot 0 = counter 100 (BE), output slot 1 = counter 101, etc.
	 * Verifies the indexing fix: counters indexed by OUTPUT slot, not
	 * template-slot — NULL templates do not consume a counter value. */
	got_c1 = ((uint32_t)bufs[0][0] << 24) | ((uint32_t)bufs[0][1] << 16) |
		 ((uint32_t)bufs[0][2] <<  8) |  (uint32_t)bufs[0][3];
	got_c2 = ((uint32_t)bufs[1][0] << 24) | ((uint32_t)bufs[1][1] << 16) |
		 ((uint32_t)bufs[1][2] <<  8) |  (uint32_t)bufs[1][3];

	ASSERT_TRUE("generate_all_is_pure",
		    got_c1 == 100,
		    "first output packet should encode counters[0]=100");
	ASSERT_TRUE("generate_all_is_pure",
		    got_c2 == 101,
		    "second output packet should encode counters[1]=101");

done:
	free(t1); free(t2); free(t3);
}

/* ---------- Main ---------- */

int main(void)
{
	test_counter_big_endian();
	test_timestamp_big_endian();
	test_rc_charset_no_digits();
	test_rd_charset_only_digits();
	test_static_bytes();
	test_generate_all_is_pure();

	printf("\n=== %d run, %d failed ===\n", tests_run, tests_failed);
	return tests_failed == 0 ? 0 : 1;
}
