import { describe, expect, it } from 'vitest';
import {
	countActiveManagedPeers,
	countActiveSystemPeers,
	isSystemPeerActive,
} from './serverPeerActivity';

describe('isSystemPeerActive', () => {
	it('requires online and enabled', () => {
		expect(isSystemPeerActive({ online: true, enabled: true } as never)).toBe(true);
		expect(isSystemPeerActive({ online: true, enabled: false } as never)).toBe(false);
		expect(isSystemPeerActive({ online: false, enabled: true } as never)).toBe(false);
	});
});

describe('countActiveSystemPeers', () => {
	it('counts only active peers', () => {
		const peers = [
			{ online: true, enabled: true },
			{ online: true, enabled: false },
			{ online: false, enabled: true },
		] as never[];
		expect(countActiveSystemPeers(peers)).toBe(1);
	});
});

describe('countActiveManagedPeers', () => {
	it('joins stats online with config enabled', () => {
		const peers = [
			{ publicKey: 'a', enabled: true },
			{ publicKey: 'b', enabled: false },
		] as never[];
		const statsPeers = [
			{ publicKey: 'a', online: true },
			{ publicKey: 'b', online: true },
		] as never[];
		expect(countActiveManagedPeers(peers, statsPeers)).toBe(1);
	});

	it('returns 0 without stats', () => {
		expect(countActiveManagedPeers([{ publicKey: 'a', enabled: true }] as never[], undefined)).toBe(
			0,
		);
	});
});
