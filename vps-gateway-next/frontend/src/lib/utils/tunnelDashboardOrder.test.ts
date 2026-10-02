import { describe, it, expect } from 'vitest';
import { applyManualOrder, mergeManualOrder, reorder } from './tunnelDashboardOrder';

const item = (key: string) => ({ key, name: key });

describe('applyManualOrder', () => {
	it('sorts items by the order array', () => {
		const items = [item('a'), item('b'), item('c')];
		const result = applyManualOrder(items, ['c', 'a', 'b']);
		expect(result.map((i) => i.key)).toEqual(['c', 'a', 'b']);
	});

	it('appends items missing from the order preserving input order', () => {
		const items = [item('a'), item('b'), item('c'), item('d')];
		const result = applyManualOrder(items, ['c', 'a']);
		expect(result.map((i) => i.key)).toEqual(['c', 'a', 'b', 'd']);
	});

	it('ignores order keys with no matching item and duplicates', () => {
		const items = [item('a'), item('b')];
		const result = applyManualOrder(items, ['ghost', 'b', 'b', 'a']);
		expect(result.map((i) => i.key)).toEqual(['b', 'a']);
	});

	it('does not mutate the input arrays', () => {
		const items = [item('a'), item('b')];
		const order = ['b', 'a'];
		applyManualOrder(items, order);
		expect(items.map((i) => i.key)).toEqual(['a', 'b']);
		expect(order).toEqual(['b', 'a']);
	});

	it('handles empty inputs', () => {
		expect(applyManualOrder([], ['a'])).toEqual([]);
		const items = [item('a')];
		expect(applyManualOrder(items, []).map((i) => i.key)).toEqual(['a']);
	});
});

describe('reorder', () => {
	it('moves an element forward (to is the post-removal index)', () => {
		expect(reorder(['a', 'b', 'c', 'd'], 0, 2)).toEqual(['b', 'c', 'a', 'd']);
	});

	it('moves an element backward', () => {
		expect(reorder(['a', 'b', 'c', 'd'], 3, 1)).toEqual(['a', 'd', 'b', 'c']);
	});

	it('moves an element to the end', () => {
		expect(reorder(['a', 'b', 'c'], 0, 2)).toEqual(['b', 'c', 'a']);
	});

	it('returns the same reference when from equals to', () => {
		const list = ['a', 'b', 'c'];
		expect(reorder(list, 1, 1)).toBe(list);
	});

	it('returns the same reference for out-of-bounds or non-integer indices', () => {
		const list = ['a', 'b', 'c'];
		expect(reorder(list, -1, 1)).toBe(list);
		expect(reorder(list, 3, 1)).toBe(list);
		expect(reorder(list, 0, -1)).toBe(list);
		expect(reorder(list, 0, 3)).toBe(list);
		expect(reorder(list, 0.5, 1)).toBe(list);
	});

	it('does not mutate the input array', () => {
		const list = ['a', 'b', 'c'];
		reorder(list, 2, 0);
		expect(list).toEqual(['a', 'b', 'c']);
	});

	it('handles empty input', () => {
		const list: string[] = [];
		expect(reorder(list, 0, 0)).toBe(list);
	});
});

describe('mergeManualOrder', () => {
	it('keeps slots of saved keys that are not currently visible', () => {
		const result = mergeManualOrder(['a', 'x', 'b'], ['a', 'b'], ['b', 'a']);
		expect(result).toEqual(['b', 'x', 'a']);
	});

	it('appends visibleAfter keys missing from saved', () => {
		const result = mergeManualOrder(['a', 'b'], ['a', 'b'], ['b', 'a', 'c']);
		expect(result).toEqual(['b', 'a', 'c']);
	});

	it('returns visibleAfter order when saved is empty', () => {
		expect(mergeManualOrder([], ['a', 'b'], ['b', 'a'])).toEqual(['b', 'a']);
	});

	it('returns visibleAfter order when saved equals visibleBefore', () => {
		expect(mergeManualOrder(['a', 'b', 'c'], ['a', 'b', 'c'], ['c', 'a', 'b'])).toEqual([
			'c',
			'a',
			'b',
		]);
	});

	it('drops slots of visible keys when visibleAfter is shorter', () => {
		const result = mergeManualOrder(['a', 'x', 'b'], ['a', 'b'], ['a']);
		expect(result).toEqual(['a', 'x']);
	});

	it('never emits duplicate keys', () => {
		const result = mergeManualOrder(['x', 'a', 'x', 'b'], ['a', 'b'], ['b', 'a']);
		expect(result).toEqual(['x', 'b', 'a']);
	});
});
