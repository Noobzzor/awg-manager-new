import { describe, expect, it } from 'vitest';
import {
	outboundDegradedText,
	outboundDisplayLabel,
	outboundName,
	outboundNowTag,
} from './deviceProxyOutboundLabel';
import type { DeviceProxyRuntime } from '$lib/types';

function rt(partial: Partial<DeviceProxyRuntime> = {}): DeviceProxyRuntime {
	return { alive: false, activeTag: '', defaultTag: '', ...partial };
}

describe('outboundName', () => {
	it('конфиг первичен — runtime.activeTag не влияет на имя', () => {
		const input = {
			selectedOutbound: 'vpn',
			runtime: rt({ alive: true, activeTag: 'awg-awg10', defaultTag: 'awg-awg10' }),
		};
		expect(outboundName(input)).toBe('vpn');
	});

	it('без selectedOutbound берёт runtime.defaultTag', () => {
		expect(outboundName({ selectedOutbound: '', runtime: rt({ defaultTag: 'proxy-a' }) })).toBe(
			'proxy-a',
		);
	});

	it('без данных — «—»', () => {
		expect(outboundName({ selectedOutbound: '', runtime: null })).toBe('—');
		expect(outboundName({ selectedOutbound: '' })).toBe('—');
	});
});

describe('outboundNowTag', () => {
	it('показывает activeTag когда движок жив и тег отличается от имени', () => {
		const input = {
			selectedOutbound: 'vpn',
			runtime: rt({ alive: true, activeTag: 'awg-awg10' }),
		};
		expect(outboundNowTag(input)).toBe('awg-awg10');
	});

	it('прячет пометку когда activeTag совпадает с именем', () => {
		const input = { selectedOutbound: 'vpn', runtime: rt({ alive: true, activeTag: 'vpn' }) };
		expect(outboundNowTag(input)).toBeNull();
	});

	it('прячет пометку когда движок не жив (activeTag устарел)', () => {
		const input = {
			selectedOutbound: 'vpn',
			runtime: rt({ alive: false, activeTag: 'awg-awg10' }),
		};
		expect(outboundNowTag(input)).toBeNull();
	});

	it('нет runtime — нет пометки', () => {
		expect(outboundNowTag({ selectedOutbound: 'vpn' })).toBeNull();
	});
});

describe('outboundDegradedText', () => {
	it('деградация: показывает fallback', () => {
		const input = {
			selectedOutbound: 'vpn',
			runtime: rt({ degradedOutbound: 'vpn', fallbackTag: 'awg-awg10' }),
		};
		expect(outboundDegradedText(input)).toBe('выход недоступен — через awg-awg10');
	});

	it('деградация без fallbackTag — direct', () => {
		const input = { selectedOutbound: 'vpn', runtime: rt({ degradedOutbound: 'vpn' }) };
		expect(outboundDegradedText(input)).toBe('выход недоступен — через direct');
	});

	it('нет деградации — null', () => {
		expect(outboundDegradedText({ selectedOutbound: 'vpn', runtime: rt() })).toBeNull();
		expect(outboundDegradedText({ selectedOutbound: 'vpn' })).toBeNull();
	});
});

describe('outboundOptionLabel / outboundDisplayLabel', () => {
	const list: import('$lib/types').DeviceProxyOutbound[] = [
		{ tag: 'awg-sys-Wireguard1', kind: 'awg', label: 'Veesp SE', detail: 'nwg1' },
		{ tag: 'vpn', kind: 'router', label: 'vpn', detail: '' },
		{ tag: 'vpn2', kind: 'router', label: 'vpn2', detail: 'селектор' },
		{ tag: 'sub-a1b2', kind: 'subscription', label: 'Моя подписка', detail: '' },
		{ tag: 'sub-nolabel', kind: 'subscription', label: '', detail: '' },
		{ tag: 'direct', kind: 'direct', label: 'Direct (WAN)', detail: '' },
	];

	it('awg: label · detail (как в dropdown)', () => {
		expect(outboundDisplayLabel('awg-sys-Wireguard1', list)).toBe('Veesp SE · nwg1');
	});

	it('router: detail опционален', () => {
		expect(outboundDisplayLabel('vpn', list)).toBe('vpn');
		expect(outboundDisplayLabel('vpn2', list)).toBe('vpn2 · селектор');
	});

	it('subscription/direct: label, пустой label — тег', () => {
		expect(outboundDisplayLabel('sub-a1b2', list)).toBe('Моя подписка');
		expect(outboundDisplayLabel('sub-nolabel', list)).toBe('sub-nolabel');
		expect(outboundDisplayLabel('direct', list)).toBe('Direct (WAN)');
	});

	it('тег не в каталоге или пуст — сырой тег (fallback)', () => {
		expect(outboundDisplayLabel('ghost-tag', list)).toBe('ghost-tag');
		expect(outboundDisplayLabel('awg-sys-Wireguard1', [])).toBe('awg-sys-Wireguard1');
		expect(outboundDisplayLabel('', list)).toBe('');
	});
});
