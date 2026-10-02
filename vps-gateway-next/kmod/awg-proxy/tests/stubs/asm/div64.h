/* Host-build stub. Not used in tests; only present so headers parse. */
#ifndef _STUB_ASM_DIV64_H
#define _STUB_ASM_DIV64_H
#define do_div(n, base) ({ uint32_t __r = (uint32_t)((n) % (base)); (n) /= (base); __r; })
#endif
