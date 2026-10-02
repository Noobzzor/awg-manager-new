/** Stable string hash (djb2) for deterministic palette indexing. */
export function hashString(input: string): number {
	let hash = 5381;
	for (let i = 0; i < input.length; i++) {
		hash = (hash * 33) ^ input.charCodeAt(i);
	}
	return hash >>> 0;
}

/**
 * Muted tile background from a label (name / slug).
 * Hue varies by hash; saturation and lightness are fixed for a restrained dark-theme look.
 */
export function letterIconBackground(label: string): string {
	const hue = hashString(label.trim().toLowerCase()) % 360;
	return `hsl(${hue} 36% 38%)`;
}

/** First visible letter for monogram tiles (supports Cyrillic). */
export function letterIconGlyph(label: string): string {
	const trimmed = label.trim();
	if (!trimmed) return '?';
	const ch = [...trimmed][0];
	return ch ? ch.toLocaleUpperCase('ru-RU') : '?';
}
