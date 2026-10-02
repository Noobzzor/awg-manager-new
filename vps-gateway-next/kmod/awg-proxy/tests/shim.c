// SPDX-License-Identifier: GPL-2.0
/*
 * Deterministic PRNG + fixed-time stubs for unit tests.
 *
 * Tests should call shim_set_random_seed(SEED) and
 * shim_set_fixed_time(TS) in setup so byte-exact assertions against
 * generated CPS packets are reproducible across machines.
 *
 * PRNG is a minimal SplitMix32 — deterministic, fast, good enough for
 * tests. NOT cryptographic; do not use outside tests.
 */
#include "shim.h"

static uint32_t prng_state;
static uint64_t fixed_time = 1700000000ULL;
static int random_seeded;

void shim_set_random_seed(uint32_t seed)
{
	prng_state = seed;
	random_seeded = 1;
}

void shim_set_fixed_time(uint32_t unix_seconds)
{
	fixed_time = unix_seconds;
}

static uint32_t splitmix32(void)
{
	uint32_t z = (prng_state += 0x9E3779B9u);

	z = (z ^ (z >> 16)) * 0x85EBCA6Bu;
	z = (z ^ (z >> 13)) * 0xC2B2AE35u;
	return z ^ (z >> 16);
}

void get_random_bytes(void *buf, int n)
{
	uint8_t *p = (uint8_t *)buf;
	int i;

	if (!random_seeded)
		shim_set_random_seed(0xDEADBEEF);
	for (i = 0; i < n; i += 4) {
		uint32_t v = splitmix32();
		int take = (n - i) < 4 ? (n - i) : 4;
		memcpy(p + i, &v, take);
	}
}

uint64_t ktime_get_real_seconds(void)
{
	return fixed_time;
}
