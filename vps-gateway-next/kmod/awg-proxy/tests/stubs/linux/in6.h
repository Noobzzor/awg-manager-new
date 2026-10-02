/* Host-build stub for <linux/in6.h>.
 * struct in6_addr (with the s6_addr accessor) comes from libc; the layout
 * is identical to the kernel's (16 raw bytes), which is all endpoint.h
 * relies on (memcmp / in6_pton fill). */
#ifndef _STUB_LINUX_IN6_H
#define _STUB_LINUX_IN6_H

#include <netinet/in.h>

#endif
