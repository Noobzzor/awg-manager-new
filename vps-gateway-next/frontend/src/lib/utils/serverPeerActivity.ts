import type { ManagedPeer, ManagedPeerStats, WireguardServerPeer } from '$lib/types';

/** Active = online and not administratively disabled. */
export function isSystemPeerActive(peer: WireguardServerPeer): boolean {
	return peer.online && peer.enabled;
}

export function countActiveSystemPeers(peers: WireguardServerPeer[] | undefined): number {
	return (peers ?? []).filter(isSystemPeerActive).length;
}

/** Uses stats for online signal and peer config for enabled flag. */
export function countActiveManagedPeers(
	peers: ManagedPeer[] | undefined,
	statsPeers: ManagedPeerStats[] | undefined,
): number {
	const config = peers ?? [];
	if (!statsPeers?.length) return 0;
	const enabledByKey = new Map(config.map((p) => [p.publicKey, p.enabled]));
	return statsPeers.filter((p) => p.online && enabledByKey.get(p.publicKey) !== false).length;
}
