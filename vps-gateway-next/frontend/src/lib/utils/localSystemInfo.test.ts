import { describe, expect, it } from 'vitest';
import { createLocalSystemInfo } from './localSystemInfo';

describe('createLocalSystemInfo', () => {
	it('returns safe portable defaults for settings when router system info is unavailable', () => {
		const info = createLocalSystemInfo('2.19.0');

		expect(info.version).toBe('2.19.0');
		expect(info.isOS5).toBe(false);
		expect(info.activeBackend).toBe('singbox');
		expect(info.backendAvailability).toEqual({ nativewg: false, kernel: false });
		expect(info.bootInProgress).toBe(false);
	});
});
