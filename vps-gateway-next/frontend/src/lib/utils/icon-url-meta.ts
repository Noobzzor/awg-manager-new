/** Fragment suffix on iconUrl — no API change; stripped for &lt;img src&gt;. */
export const ICON_TILE_BG_FRAGMENT = '#awg-tile=';

const TILE_HEX_RE = /^[0-9a-fA-F]{6}$/;

export interface ParsedIconUrl {
	src: string;
	/** User override from #awg-tile=RRGGBB; omit to use auto brand color. */
	userTileBg?: string;
}

export function parseIconUrl(stored: string): ParsedIconUrl {
	const idx = stored.lastIndexOf(ICON_TILE_BG_FRAGMENT);
	if (idx === -1) return { src: stored };

	const src = stored.slice(0, idx);
	const hex = stored.slice(idx + ICON_TILE_BG_FRAGMENT.length);
	if (!TILE_HEX_RE.test(hex)) return { src: stored };

	return { src, userTileBg: `#${hex.toLowerCase()}` };
}

export function iconImageSrc(stored: string): string {
	return parseIconUrl(stored).src;
}

/** Normalize #rgb / #rrggbb / rrggbb → #rrggbb or null. */
export function normalizeTileHex(input: string): string | null {
	const raw = input.trim().replace(/^#/, '');
	if (/^[0-9a-fA-F]{6}$/.test(raw)) return `#${raw.toLowerCase()}`;
	if (/^[0-9a-fA-F]{3}$/.test(raw)) {
		const [r, g, b] = raw.split('');
		return `#${r}${r}${g}${g}${b}${b}`.toLowerCase();
	}
	return null;
}

/** Attach or remove user tile background on iconUrl. */
export function withIconTileBg(src: string, tileBg: string | null | undefined): string {
	const { src: clean } = parseIconUrl(src);
	if (!tileBg) return clean;
	const hex = normalizeTileHex(tileBg);
	if (!hex) return clean;
	return `${clean}${ICON_TILE_BG_FRAGMENT}${hex.slice(1)}`;
}
