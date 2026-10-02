import { describe, expect, it } from 'vitest';
import { countVisibleBadges } from './fittingBadgeLayout';

describe('countVisibleBadges', () => {
	const base = {
		arrowWidth: 14,
		overflowChipWidth: 28,
		gap: 6,
	};

	it('shows all badges when row fits', () => {
		expect(
			countVisibleBadges({
				...base,
				badgeWidths: [120, 130, 125],
				availableWidth: 500,
			}),
		).toBe(3);
	});

	it('collapses with +N when space is tight', () => {
		expect(
			countVisibleBadges({
				...base,
				badgeWidths: [120, 130, 125],
				availableWidth: 180,
			}),
		).toBeLessThan(3);
	});

	it('expands again after widening (no stale collapsed state in math)', () => {
		const widths = [100, 100, 100];
		expect(countVisibleBadges({ ...base, badgeWidths: widths, availableWidth: 160 })).toBe(1);
		expect(countVisibleBadges({ ...base, badgeWidths: widths, availableWidth: 500 })).toBe(3);
	});
});
