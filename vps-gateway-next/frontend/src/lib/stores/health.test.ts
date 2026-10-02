import { describe, expect, it } from 'vitest';
import { shouldReloadForInstanceSwitch } from './health';

describe('shouldReloadForInstanceSwitch', () => {
	it('returns false when previous is empty', () => {
		expect(shouldReloadForInstanceSwitch(undefined, 'b')).toBe(false);
	});

	it('returns false when next is empty', () => {
		expect(shouldReloadForInstanceSwitch('a', undefined)).toBe(false);
	});

	it('returns false when ids are equal', () => {
		expect(shouldReloadForInstanceSwitch('same', 'same')).toBe(false);
	});

	it('returns true when ids differ', () => {
		expect(shouldReloadForInstanceSwitch('a', 'b')).toBe(true);
	});
});

