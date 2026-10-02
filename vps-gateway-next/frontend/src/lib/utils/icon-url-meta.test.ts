import { describe, expect, it } from 'vitest';
import {
	ICON_TILE_BG_FRAGMENT,
	iconImageSrc,
	normalizeTileHex,
	parseIconUrl,
	withIconTileBg,
} from './icon-url-meta';

describe('icon-url-meta', () => {
	it('round-trips tile background in fragment', () => {
		const base = 'https://cdn.example.com/icon.png';
		const stored = withIconTileBg(base, '#29a9eb');
		expect(stored).toBe(`${base}${ICON_TILE_BG_FRAGMENT}29a9eb`);
		expect(parseIconUrl(stored).userTileBg).toBe('#29a9eb');
		expect(iconImageSrc(stored)).toBe(base);
	});

	it('strips fragment for image src on data URLs', () => {
		const base = 'data:image/png;base64,abc';
		const stored = withIconTileBg(base, '#ff0000');
		expect(iconImageSrc(stored)).toBe(base);
	});

	it('normalizeTileHex accepts shorthand', () => {
		expect(normalizeTileHex('#f00')).toBe('#ff0000');
	});
});
