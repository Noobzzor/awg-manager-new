import { describe, expect, it } from 'vitest';
import { outboundFullLabel, splitParenMeta } from './outboundLabelFormat';

describe('outboundFullLabel', () => {
	it('joins label and metaSuffix with single space before parens', () => {
		expect(outboundFullLabel('Demo VPN', 'sub')).toBe('Demo VPN (sub)');
		expect(outboundFullLabel('Kto-VLESS', 't2s0')).toBe('Kto-VLESS (t2s0)');
	});
});

describe('splitParenMeta', () => {
	it('splits AWG option labels', () => {
		expect(splitParenMeta('DE Frankfurt (t2s0)')).toEqual({
			label: 'DE Frankfurt',
			metaSuffix: 't2s0',
		});
	});

	it('returns plain label when no meta', () => {
		expect(splitParenMeta('direct')).toEqual({ label: 'direct' });
	});
});
