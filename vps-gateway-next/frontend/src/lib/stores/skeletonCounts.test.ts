import { describe, expect, it } from 'vitest';
import { get } from 'svelte/store';
import { clampSkeletonCount, tunnelsSkeletonCount, serversSkeletonCount } from './skeletonCounts';

describe('clampSkeletonCount', () => {
	it('clamps to 1..6 and rounds', () => {
		expect(clampSkeletonCount(0, 3)).toBe(1);
		expect(clampSkeletonCount(42, 3)).toBe(6);
		expect(clampSkeletonCount(4.6, 3)).toBe(5);
		expect(clampSkeletonCount(3, 3)).toBe(3);
	});
	it('falls back on junk', () => {
		expect(clampSkeletonCount(NaN, 3)).toBe(3);
		expect(clampSkeletonCount(Infinity, 2)).toBe(2);
	});
});

describe('persisted count stores', () => {
	it('defaults: tunnels 3, servers 2 (no localStorage in node env)', () => {
		expect(get(tunnelsSkeletonCount)).toBe(3);
		expect(get(serversSkeletonCount)).toBe(2);
	});
});
