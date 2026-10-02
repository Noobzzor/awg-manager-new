/* Host-build stub for <linux/inet.h>. */
#ifndef _STUB_LINUX_INET_H
#define _STUB_LINUX_INET_H

#include <arpa/inet.h>
#include <netinet/in.h>

static inline __be32 in_aton(const char *str)
{
	return (__be32)inet_addr(str);
}

/*
 * Minimal in6_pton: matches the kernel contract used by tunnel.c —
 * returns 1 on success / 0 on failure, whole-string parse (delim is
 * ignored; callers pass -1 = no delimiter, so trailing garbage fails in
 * both implementations), writes 16 raw bytes to dst, and sets *end past
 * the consumed input when requested.
 */
static inline int in6_pton(const char *src, int srclen, u8 *dst,
			   int delim, const char **end)
{
	char tmp[64];
	size_t len = srclen < 0 ? strlen(src) : (size_t)srclen;
	struct in6_addr a;

	(void)delim;
	if (len >= sizeof(tmp))
		return 0;
	memcpy(tmp, src, len);
	tmp[len] = '\0';
	if (inet_pton(AF_INET6, tmp, &a) != 1)
		return 0;
	memcpy(dst, &a, 16);
	if (end)
		*end = src + len;
	return 1;
}

#endif
