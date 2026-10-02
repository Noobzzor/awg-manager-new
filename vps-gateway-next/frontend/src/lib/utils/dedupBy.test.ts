import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { dedupBy } from './dedupBy';

describe('dedupBy', () => {
	const originalWarn = console.warn;

	beforeEach(() => {
		console.warn = vi.fn();
	});

	afterEach(() => {
		console.warn = originalWarn;
	});

	it('returns the same array when no duplicates exist', () => {
		const items = [{ id: 'a' }, { id: 'b' }, { id: 'c' }];
		expect(dedupBy(items, (i) => i.id)).toEqual(items);
		expect(console.warn).not.toHaveBeenCalled();
	});

	it('keeps first occurrence, drops later duplicates', () => {
		const items = [
			{ id: 'a', n: 1 },
			{ id: 'b', n: 2 },
			{ id: 'a', n: 3 },
			{ id: 'a', n: 4 },
		];
		const out = dedupBy(items, (i) => i.id);
		expect(out).toEqual([
			{ id: 'a', n: 1 },
			{ id: 'b', n: 2 },
		]);
	});

	it('emits console.warn when warnTag is provided and duplicates were dropped', () => {
		const items = [{ id: 'x' }, { id: 'x' }];
		dedupBy(items, (i) => i.id, { warnTag: 'rail items' });
		expect(console.warn).toHaveBeenCalledTimes(1);
		const call = vi.mocked(console.warn).mock.calls[0].join(' ');
		expect(call).toMatch(/rail items/);
		expect(call).toMatch(/x/);
	});

	it('does not warn when warnTag is provided but no duplicates exist', () => {
		dedupBy([{ id: 'a' }, { id: 'b' }], (i) => i.id, { warnTag: 'rail items' });
		expect(console.warn).not.toHaveBeenCalled();
	});

	it('handles empty array', () => {
		expect(dedupBy([], (i: { id: string }) => i.id)).toEqual([]);
	});
});
