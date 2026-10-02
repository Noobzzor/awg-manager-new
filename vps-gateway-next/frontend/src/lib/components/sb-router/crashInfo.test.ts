import { describe, it, expect } from 'vitest';
import { formatSuppressedUntil, CRASH_WORDS } from './crashInfo';
import { pluralize } from '$lib/utils/pluralize';

describe('formatSuppressedUntil', () => {
	it('пустое/absent значение — null (блок подавления скрыт)', () => {
		expect(formatSuppressedUntil(undefined)).toBeNull();
		expect(formatSuppressedUntil(null)).toBeNull();
		expect(formatSuppressedUntil('')).toBeNull();
	});

	it('битая дата — null', () => {
		expect(formatSuppressedUntil('not-a-date')).toBeNull();
	});

	it('RFC3339 → «HH:MM» в локальном времени', () => {
		const d = new Date(2026, 6, 6, 9, 5, 0); // локальные 09:05
		expect(formatSuppressedUntil(d.toISOString())).toBe('09:05');
	});

	it('часы/минуты дополняются нулями', () => {
		const d = new Date(2026, 0, 1, 0, 0, 0);
		expect(formatSuppressedUntil(d.toISOString())).toBe('00:00');
	});
});

describe('CRASH_WORDS', () => {
	it('русские формы для счётчика падений', () => {
		expect(pluralize(1, CRASH_WORDS)).toBe('1 падение');
		expect(pluralize(3, CRASH_WORDS)).toBe('3 падения');
		expect(pluralize(5, CRASH_WORDS)).toBe('5 падений');
		expect(pluralize(11, CRASH_WORDS)).toBe('11 падений');
	});
});
