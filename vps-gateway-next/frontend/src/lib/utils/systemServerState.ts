import type { WireguardServer } from '$lib/types';
import type { ServersSnapshot } from '$lib/stores/servers';

/** Whether a built-in/marked system server is administratively enabled in NDMS. */
export function systemServerIsUp(server: Pick<WireguardServer, 'enabled' | 'enabledKnown' | 'status'>): boolean {
	if (server.enabledKnown === true) {
		return server.enabled === true;
	}
	return server.status === 'up';
}

/** Apply the toggled admin state locally when the mutation snapshot is still stale. */
export function patchSystemServerEnabledInSnapshot(
	snapshot: ServersSnapshot,
	serverId: string,
	enabled: boolean,
): ServersSnapshot {
	return {
		...snapshot,
		servers: snapshot.servers.map((s) =>
			s.id === serverId
				? {
						...s,
						enabled,
						enabledKnown: true,
						status: enabled ? 'up' : 'down',
					}
				: s,
		),
	};
}
