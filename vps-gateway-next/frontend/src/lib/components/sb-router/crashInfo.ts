/**
 * Pure helpers для блока «падения движка» в StatusDrawer (#456):
 * форматирование времени окончания паузы авто-перезапуска. Чистые функции —
 * тестируются vitest'ом без DOM.
 */
import type { PluralWords } from '$lib/utils/pluralize';

export const CRASH_WORDS = ['падение', 'падения', 'падений'] as const satisfies PluralWords;

/**
 * «HH:MM» (локальное время) из RFC3339-строки restartSuppressedUntil.
 * null для пустой/битой даты — вызывающий скрывает блок подавления.
 */
export function formatSuppressedUntil(iso: string | null | undefined): string | null {
	if (!iso) return null;
	const d = new Date(iso);
	if (Number.isNaN(d.getTime())) return null;
	const hh = String(d.getHours()).padStart(2, '0');
	const mm = String(d.getMinutes()).padStart(2, '0');
	return `${hh}:${mm}`;
}
