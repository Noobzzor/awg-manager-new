import { describe, expect, it } from 'vitest';
import type { ConntrackConnection } from '$lib/types';
import { connKey, groupConnections, routeLabel, routeVariant, normProto } from './connectionsView';

function conn(over: Partial<ConntrackConnection>): ConntrackConnection {
	return {
		protocol: 'tcp', src: '192.168.0.5', dst: '1.2.3.4', srcPort: 1000, dstPort: 443,
		state: 'ESTABLISHED', packets: 1, bytes: 30, bytesIn: 20, bytesOut: 10, ttl: 100,
		routeClass: 'direct', interface: 'eth3', tunnelId: '', tunnelName: '',
		clientMac: '', clientName: '', rules: [],
		...over,
	};
}

describe('connKey', () => {
	it('строит стабильный ключ по 5-tuple', () => {
		expect(connKey(conn({}))).toBe('tcp|192.168.0.5:1000|1.2.3.4:443');
	});
});

describe('groupConnections', () => {
	it('client: группирует по имени клиента с fallback на src', () => {
		const groups = groupConnections(
			[
				conn({ clientName: 'Phone' }),
				conn({ clientName: 'Phone', dstPort: 80 }),
				conn({ src: '192.168.0.9' }),
			],
			'client',
		);
		expect(groups).toHaveLength(2);
		expect(groups[0].name).toBe('Phone');
		expect(groups[0].conns).toHaveLength(2);
		expect(groups[0].sub).toBe('192.168.0.5');
		expect(groups[0].bytesIn).toBe(40);
		expect(groups[1].name).toBe('192.168.0.9');
		expect(groups[1].sub).toBe('');
	});

	it('host: группирует по FQDN первого правила с fallback на dst, отдаёт rule', () => {
		const rule = { listId: 'l1', listName: 'YouTube', fqdn: 'm.youtube.com' };
		const groups = groupConnections(
			[conn({ rules: [rule] }), conn({ rules: [rule], srcPort: 2 }), conn({ dst: '9.9.9.9' })],
			'host',
		);
		expect(groups[0].name).toBe('m.youtube.com');
		expect(groups[0].rule?.listName).toBe('YouTube');
		expect(groups[0].sub).toBe('1.2.3.4');
		expect(groups[1].name).toBe('9.9.9.9');
	});
});

describe('route helpers', () => {
	it('лейбл и вариант по routeClass', () => {
		expect(routeLabel(conn({ routeClass: 'tunnel', tunnelName: 'VPS' }))).toBe('VPS');
		expect(routeLabel(conn({ routeClass: 'singbox' }))).toBe('sing-box');
		expect(routeLabel(conn({ routeClass: 'local' }))).toBe('Локально');
		expect(routeLabel(conn({ routeClass: 'direct', interface: '' }))).toBe('—');
		expect(routeVariant(conn({ routeClass: 'tunnel' }))).toBe('accent');
		expect(routeVariant(conn({ routeClass: 'singbox' }))).toBe('info');
		expect(routeVariant(conn({ routeClass: 'direct' }))).toBe('muted');
	});
	it('normProto сводит icmpv6 к icmp', () => {
		expect(normProto('ICMPv6')).toBe('icmp');
		expect(normProto('TCP')).toBe('tcp');
	});
});
