/* SPDX-License-Identifier: GPL-2.0 */
/*
 * Host-build shim for kernel module sources.
 *
 * Used by kmod/awg-proxy/tests to compile cps.c (and parts of
 * transform.c) on a regular dev machine with vanilla gcc — no Linux
 * kernel headers required. The shim provides just enough kernel types,
 * helpers, and stubs to satisfy the source files under test.
 *
 * Tests that need determinism (e.g. random-counter init) seed our PRNG
 * via shim_set_random_seed() and fix the clock via shim_set_fixed_time().
 */
#ifndef AWG_PROXY_TEST_SHIM_H
#define AWG_PROXY_TEST_SHIM_H

#define _DEFAULT_SOURCE  /* expose htobe32 / htole32 from <endian.h> */
#include <stdint.h>
#include <stdbool.h>     /* kernel <linux/types.h> provides bool; host gets C99's */
#include <stddef.h>
#include <string.h>
#include <stdio.h>
#include <stdlib.h>
#include <errno.h>
#include <sys/types.h>
#ifdef __APPLE__
#include <libkern/OSByteOrder.h>
#define htole32(x) OSSwapHostToLittleInt32(x)
#define le32toh(x) OSSwapLittleToHostInt32(x)
#define htobe32(x) OSSwapHostToBigInt32(x)
#define be32toh(x) OSSwapBigToHostInt32(x)
#else
#include <endian.h>
#endif

/* ---- Linux integer aliases ---- */
typedef uint8_t  u8;
typedef uint16_t u16;
typedef uint32_t u32;
typedef uint64_t u64;
typedef int8_t   s8;
typedef int16_t  s16;
typedef int32_t  s32;
typedef int64_t  s64;
typedef uint16_t __le16;
typedef uint16_t __be16;
typedef uint32_t __le32;
typedef uint32_t __be32;

/* ---- Endianness helpers ---- */
#define cpu_to_le32(x) ((__le32)htole32(x))
#define cpu_to_be32(x) ((__be32)htobe32(x))
#define le32_to_cpu(x) le32toh((uint32_t)(x))
#define be32_to_cpu(x) be32toh((uint32_t)(x))

/* ---- Memory helpers ---- */
static inline void *kmalloc(size_t n, int gfp) { (void)gfp; return malloc(n); }
static inline void *kzalloc(size_t n, int gfp) { (void)gfp; return calloc(1, n); }
static inline void  kfree(void *p) { free(p); }
#define GFP_KERNEL 0

static inline int kstrtoint(const char *s, unsigned int base, int *res)
{
	char *end = NULL;
	long v;

	if (!s || !*s)
		return -EINVAL;
	errno = 0;
	v = strtol(s, &end, base);
	if (errno || !end || *end)
		return -EINVAL;
	*res = (int)v;
	return 0;
}

static inline ssize_t strscpy(char *dst, const char *src, size_t size)
{
	size_t len;

	if (!size)
		return -E2BIG;
	len = strlen(src);
	if (len >= size)
		len = size - 1;
	memcpy(dst, src, len);
	dst[len] = '\0';
	return (ssize_t)len;
}

static inline int hex_to_bin(char ch)
{
	if (ch >= '0' && ch <= '9')
		return ch - '0';
	if (ch >= 'a' && ch <= 'f')
		return ch - 'a' + 10;
	if (ch >= 'A' && ch <= 'F')
		return ch - 'A' + 10;
	return -1;
}

/* ---- Random + time stubs (deterministic for tests) ---- */
void shim_set_random_seed(uint32_t seed);
void shim_set_fixed_time(uint32_t unix_seconds);
void get_random_bytes(void *buf, int n);
uint64_t ktime_get_real_seconds(void);

/* ---- Kernel compat ---- */
#ifndef EBADMSG
#define EBADMSG 74
#endif
static inline void memzero_explicit(void *p, size_t n) { memset(p, 0, n); }

/* ---- Warning macros (no-op on host) ---- */
#define WARN_ON_ONCE(cond) ({ int _c = !!(cond); if (_c) fprintf(stderr, "WARN_ON_ONCE %s\n", #cond); _c; })
#define pr_warn(fmt, ...)  fprintf(stderr, "[pr_warn] " fmt, ##__VA_ARGS__)

#endif /* AWG_PROXY_TEST_SHIM_H */
