import { describe, expect, it } from 'vitest';
import { waitForBackendRestart } from './restartRecovery';

describe('waitForBackendRestart', () => {
	it('returns instance-changed when previous instance differs', async () => {
		let t = 0;
		const result = await waitForBackendRestart({
			previousInstanceId: 'old-id',
			readInstanceId: async () => 'new-id',
			sleep: async (ms) => {
				t += ms;
			},
			now: () => t,
			timeoutMs: 10_000,
			pollMs: 100,
		});
		expect(result).toBe('instance-changed');
	});

	it('returns offline-online when previous is unknown and instance appears after offline', async () => {
		let t = 0;
		let n = 0;
		const result = await waitForBackendRestart({
			previousInstanceId: null,
			readInstanceId: async () => {
				n += 1;
				if (n === 1) return null;
				return 'instance-1';
			},
			sleep: async (ms) => {
				t += ms;
			},
			now: () => t,
			timeoutMs: 10_000,
			pollMs: 100,
		});
		expect(result).toBe('offline-online');
	});

	it('returns stable-online when previous is unknown and online is stable', async () => {
		let t = 0;
		const result = await waitForBackendRestart({
			previousInstanceId: null,
			readInstanceId: async () => 'instance-1',
			sleep: async (ms) => {
				t += ms;
			},
			now: () => t,
			timeoutMs: 10_000,
			pollMs: 100,
			stableOnlineMs: 300,
		});
		expect(result).toBe('stable-online');
	});

	it('returns timeout when no confirmation arrives', async () => {
		let t = 0;
		const result = await waitForBackendRestart({
			previousInstanceId: 'same',
			readInstanceId: async () => 'same',
			sleep: async (ms) => {
				t += ms;
			},
			now: () => t,
			timeoutMs: 500,
			pollMs: 100,
		});
		expect(result).toBe('timeout');
	});

	it('treats read errors as offline and continues', async () => {
		let t = 0;
		let n = 0;
		const result = await waitForBackendRestart({
			previousInstanceId: null,
			readInstanceId: async () => {
				n += 1;
				if (n === 1) throw new Error('network');
				return 'instance-2';
			},
			sleep: async (ms) => {
				t += ms;
			},
			now: () => t,
			timeoutMs: 10_000,
			pollMs: 100,
		});
		expect(result).toBe('offline-online');
	});
});

