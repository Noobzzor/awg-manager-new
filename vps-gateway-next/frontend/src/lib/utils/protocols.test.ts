import { describe, it, expect } from 'vitest';
import { protocols, calcTotalChars, MAX_SIGNATURE_CHARS } from './protocols';

describe('protocols catalog', () => {
	it('lists the five backend profiles in order, QUIC first', () => {
		expect(Object.keys(protocols)).toEqual(['quic_initial', 'stun', 'dns', 'dtls', 'sip']);
	});
	it('calcTotalChars matches backend TotalChars', () => {
		expect(calcTotalChars({ i1: '<t>', i2: '', i3: '', i4: '', i5: '<b 0xff>' })).toBe(11);
		// <r 4000> — 4000 байт нагрузки в 8 символах, лимит не задевает
		expect(calcTotalChars({ i1: '<r 4000>', i2: '', i3: '', i4: '', i5: '' })).toBeLessThan(MAX_SIGNATURE_CHARS);
	});
});
