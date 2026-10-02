import { describe, expect, it } from 'vitest';
import { patchSystemServerEnabledInSnapshot, systemServerIsUp } from './systemServerState';

describe('systemServerIsUp', () => {
	it('prefers NDMS enabled flag when known', () => {
		expect(systemServerIsUp({ enabled: true, enabledKnown: true, status: 'down' })).toBe(true);
		expect(systemServerIsUp({ enabled: false, enabledKnown: true, status: 'up' })).toBe(false);
	});

	it('falls back to status when enabled is unknown', () => {
		expect(systemServerIsUp({ status: 'up' })).toBe(true);
		expect(systemServerIsUp({ status: 'down' })).toBe(false);
		expect(systemServerIsUp({ enabled: false, enabledKnown: false, status: 'up' })).toBe(true);
	});
});

describe('patchSystemServerEnabledInSnapshot', () => {
	it('overrides stale status/connected for the toggled server', () => {
		const snapshot = {
			servers: [
				{
					id: 'Wireguard0',
					interfaceName: 'Wireguard0',
					description: 'Wireguard VPN Server',
					status: 'up' as const,
					connected: true,
					mtu: 1420,
					address: '10.0.0.1',
					mask: '255.255.255.0',
					publicKey: 'k',
					listenPort: 51820,
					peers: [],
				},
			],
			managed: [],
			managedStats: {},
		};

		const patched = patchSystemServerEnabledInSnapshot(snapshot, 'Wireguard0', false);
		expect(systemServerIsUp(patched.servers[0])).toBe(false);
		expect(patched.servers[0].status).toBe('down');
	});
});
