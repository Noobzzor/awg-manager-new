import { describe, expect, it } from 'vitest';
import { formatSubnetPlaceholder, maskToPrefix, resolveNatMode } from './network';

describe('resolveNatMode', () => {
	it('prefers explicit natMode', () => {
		expect(resolveNatMode('internet-only', true)).toBe('internet-only');
		expect(resolveNatMode('none', true)).toBe('none');
	});

	it('falls back to natEnabled when natMode absent', () => {
		expect(resolveNatMode(undefined, true)).toBe('full');
		expect(resolveNatMode(undefined, false)).toBe('none');
	});

	it('does not treat false natEnabled as missing', () => {
		expect(resolveNatMode('internet-only', false)).toBe('internet-only');
	});
});

describe('maskToPrefix', () => {
	it('passes numeric prefix through', () => {
		expect(maskToPrefix('24')).toBe('24');
	});

	it('converts dotted mask', () => {
		expect(maskToPrefix('255.255.255.0')).toBe('24');
	});
});

describe('formatSubnetPlaceholder', () => {
	it('masks host bits in /24', () => {
		expect(formatSubnetPlaceholder('10.0.0.1', '255.255.255.0')).toBe('10.0.0.X/24');
	});
});
