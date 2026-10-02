import { describe, expect, it } from 'vitest';
import { hashString, letterIconBackground, letterIconGlyph } from './letter-icon-color';

describe('letter-icon-color', () => {
	it('hashString is stable', () => {
		expect(hashString('Netflix')).toBe(hashString('Netflix'));
		expect(hashString('netflix')).not.toBe(hashString('Netflix'));
	});

	it('letterIconBackground uses hsl with fixed s/l', () => {
		const bg = letterIconBackground('YouTube');
		expect(bg).toMatch(/^hsl\(\d+ 36% 38%\)$/);
		expect(letterIconBackground('YouTube')).toBe(bg);
	});

	it('letterIconGlyph uppercases first grapheme', () => {
		expect(letterIconGlyph('netflix')).toBe('N');
		expect(letterIconGlyph('  telegram')).toBe('T');
		expect(letterIconGlyph('')).toBe('?');
	});
});
