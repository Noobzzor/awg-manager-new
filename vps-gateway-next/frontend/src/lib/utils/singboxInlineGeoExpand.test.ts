import { describe, expect, it, vi } from 'vitest';
import { expandGeoLinesInInput } from '$lib/utils/singboxInlineGeoExpand';

describe('expandGeoLinesInInput', () => {
	it('expands geosite and geoip lines', async () => {
		const expand = vi.fn(async (kind: 'geosite' | 'geoip', tag: string) => {
			if (kind === 'geosite' && tag === 'GOOGLE') return ['google.com', '.youtube.com'];
			if (kind === 'geoip' && tag === 'RU') return ['5.8.0.0/21'];
			throw new Error('not found');
		});

		const { text, warnings } = await expandGeoLinesInInput(
			'geosite:GOOGLE\ngeoip:RU\nplain.com',
			expand,
		);

		expect(text).toBe('google.com\nyoutube.com\n5.8.0.0/21\nplain.com');
		expect(warnings.some((w) => w.includes('geosite:GOOGLE'))).toBe(true);
		expect(warnings.some((w) => w.includes('geoip:RU'))).toBe(true);
	});

	it('keeps line on expand error', async () => {
		const expand = vi.fn(async () => {
			throw new Error('missing tag');
		});
		const { text, warnings } = await expandGeoLinesInInput('geosite:MISSING', expand);
		expect(text).toBe('geosite:MISSING');
		expect(warnings[0]).toContain('missing tag');
	});
});
