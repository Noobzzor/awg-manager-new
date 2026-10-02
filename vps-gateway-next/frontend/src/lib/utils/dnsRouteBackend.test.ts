import { describe, expect, it } from 'vitest';
import type { DnsRoute } from '$lib/types';
import {
	filterDnsRoutesForBackend,
	dnsRouteTabCapability,
	showsNdmsDisclaimer,
} from './dnsRouteBackend';

const route = (id: string, backend: DnsRoute['backend']): DnsRoute => ({
	id,
	name: id,
	domains: [],
	manualDomains: [],
	routes: [],
	enabled: true,
	createdAt: '',
	updatedAt: '',
	backend,
});

describe('local DNS route backend capability', () => {
	it('places only sing-box records in the local routing tab', () => {
		const routes = [route('legacy', 'ndms'), route('local', 'singbox'), route('hr', 'hydraroute')];
		expect(filterDnsRoutesForBackend(routes, 'singbox').map((r) => r.id)).toEqual(['local']);
		expect(dnsRouteTabCapability('singbox')).toEqual({ visible: true, label: 'Sing-box DNS' });
	});

	it('keeps NDMS wording exclusive to the NDMS backend', () => {
		expect(showsNdmsDisclaimer('singbox')).toBe(false);
		expect(showsNdmsDisclaimer('hydraroute')).toBe(false);
		expect(showsNdmsDisclaimer('ndms')).toBe(true);
	});
});
