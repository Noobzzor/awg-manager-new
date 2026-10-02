/* SPDX-License-Identifier: GPL-2.0 */
/* Stub for <asm/unaligned.h> — used by blake2s.c in host tests. */
#ifndef _ASM_UNALIGNED_STUB_H
#define _ASM_UNALIGNED_STUB_H

#include <string.h>
#include <stdint.h>
#ifdef __APPLE__
#include <libkern/OSByteOrder.h>
#define le32toh(x) OSSwapLittleToHostInt32(x)
#define htole32(x) OSSwapHostToLittleInt32(x)
#define le64toh(x) OSSwapLittleToHostInt64(x)
#define htole64(x) OSSwapHostToLittleInt64(x)
#else
#include <endian.h>
#endif

static inline uint32_t get_unaligned_le32(const void *p)
{
	uint32_t v;
	memcpy(&v, p, 4);
	return le32toh(v);
}

static inline void put_unaligned_le32(uint32_t v, void *p)
{
	v = htole32(v);
	memcpy(p, &v, 4);
}

static inline void put_unaligned_le64(uint64_t v, void *p)
{
	v = htole64(v);
	memcpy(p, &v, 8);
}

#endif /* _ASM_UNALIGNED_STUB_H */
