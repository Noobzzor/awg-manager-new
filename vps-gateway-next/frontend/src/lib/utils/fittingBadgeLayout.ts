export interface BadgeRowFitInput {
	badgeWidths: number[];
	arrowWidth: number;
	overflowChipWidth: number;
	gap: number;
	availableWidth: number;
}

/** Сколько бейджей поместить в строку до overflow +N. */
export function countVisibleBadges({
	badgeWidths,
	arrowWidth,
	overflowChipWidth,
	gap,
	availableWidth,
}: BadgeRowFitInput): number {
	const total = badgeWidths.length;
	if (total === 0) return 0;
	if (availableWidth <= 0) return total;

	let allUsed = arrowWidth;
	for (let i = 0; i < total; i++) {
		allUsed += gap + badgeWidths[i];
	}
	if (allUsed <= availableWidth) return total;

	let used = arrowWidth;
	let fit = 0;

	for (let i = 0; i < total; i++) {
		const badgeW = badgeWidths[i];
		const isLastBadge = i === total - 1;
		const cost = gap + badgeW + (isLastBadge ? 0 : gap + overflowChipWidth);
		if (used + cost <= availableWidth) {
			used += gap + badgeW;
			fit++;
		} else {
			break;
		}
	}

	if (fit < total) {
		while (fit > 1) {
			let rowWidth = arrowWidth;
			for (let i = 0; i < fit; i++) {
				rowWidth += gap + badgeWidths[i];
			}
			rowWidth += gap + overflowChipWidth;
			if (rowWidth <= availableWidth) break;
			fit--;
		}
	}

	return Math.max(1, fit || 1);
}

/** Ширина бюджета: поднимаемся к layout-контейнеру, а не к свёрнутому контенту. */
export function readBadgeRowBudgetWidth(container: HTMLElement): number {
	let best = 0;
	for (let el: HTMLElement | null = container; el; el = el.parentElement) {
		best = Math.max(best, el.clientWidth);
		if (el.classList.contains('action')) break;
		if (el.classList.contains('card-route')) break;
	}
	return best;
}
