import type { DnsRoute } from '$lib/types';

export type DnsRouteBackend = NonNullable<DnsRoute['backend']>;

export function filterDnsRoutesForBackend(
	routes: DnsRoute[],
	backend: DnsRouteBackend,
): DnsRoute[] {
	return routes.filter((route) => (route.backend || 'ndms') === backend);
}

export function dnsRouteTabCapability(backend: DnsRouteBackend | null): {
	visible: boolean;
	label: string;
} {
	if (backend === 'singbox') return { visible: true, label: 'Sing-box DNS' };
	if (backend === 'ndms') return { visible: true, label: 'NDMS' };
	return { visible: false, label: '' };
}

export function showsNdmsDisclaimer(backend: DnsRouteBackend): boolean {
	return backend === 'ndms';
}
