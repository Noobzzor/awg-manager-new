import { describe, expect, it } from 'vitest';
import {
	SESSION_TTL_DEFAULT_HOURS,
	SESSION_TTL_MAX_HOURS,
	SESSION_TTL_MIN_HOURS,
	clampSessionTtlHours,
} from './sessionTtl';

describe('clampSessionTtlHours', () => {
	it('passes valid values through', () => {
		expect(clampSessionTtlHours(1)).toBe(1);
		expect(clampSessionTtlHours(24)).toBe(24);
		expect(clampSessionTtlHours(720)).toBe(720);
	});

	it('clamps out-of-range values to bounds', () => {
		expect(clampSessionTtlHours(0)).toBe(SESSION_TTL_MIN_HOURS);
		expect(clampSessionTtlHours(-5)).toBe(SESSION_TTL_MIN_HOURS);
		expect(clampSessionTtlHours(721)).toBe(SESSION_TTL_MAX_HOURS);
		expect(clampSessionTtlHours(10_000)).toBe(SESSION_TTL_MAX_HOURS);
	});

	it('rounds fractional input', () => {
		expect(clampSessionTtlHours(11.6)).toBe(12);
		expect(clampSessionTtlHours(11.4)).toBe(11);
	});

	it('falls back to default for absent/invalid values (legacy backends)', () => {
		expect(clampSessionTtlHours(undefined)).toBe(SESSION_TTL_DEFAULT_HOURS);
		expect(clampSessionTtlHours(null)).toBe(SESSION_TTL_DEFAULT_HOURS);
		expect(clampSessionTtlHours('')).toBe(SESSION_TTL_DEFAULT_HOURS);
		expect(clampSessionTtlHours('abc')).toBe(SESSION_TTL_DEFAULT_HOURS);
		expect(clampSessionTtlHours(NaN)).toBe(SESSION_TTL_DEFAULT_HOURS);
	});

	it('parses numeric strings (number input can yield strings)', () => {
		expect(clampSessionTtlHours('48')).toBe(48);
	});
});
