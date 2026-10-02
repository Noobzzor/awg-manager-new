// Ручной порядок карточек дашборда (issue #142): применение сохранённого
// массива ключей и перестановка при drag-and-drop.

export function applyManualOrder<T extends { key: string }>(items: T[], order: string[]): T[] {
	const byKey = new Map(items.map((item) => [item.key, item]));
	const seen = new Set<string>();
	const result: T[] = [];

	for (const key of order) {
		const item = byKey.get(key);
		if (item && !seen.has(key)) {
			result.push(item);
			seen.add(key);
		}
	}

	for (const item of items) {
		if (!seen.has(item.key)) result.push(item);
	}

	return result;
}

/**
 * Splice-перестановка: удаляет элемент по from и вставляет по индексу to
 * (индекс уже ПОСЛЕ удаления). Семантика локальных reorder-хелперов
 * DnsTab/RoutesTab/RulesPanel. Невалидные индексы → тот же массив.
 */
export function reorder<T>(list: T[], from: number, to: number): T[] {
	if (!Number.isInteger(from) || !Number.isInteger(to)) return list;
	if (from < 0 || from >= list.length) return list;
	if (to < 0 || to >= list.length) return list;
	if (from === to) return list;

	const next = list.slice();
	const [moved] = next.splice(from, 1);
	next.splice(to, 0, moved);
	return next;
}

/**
 * Обновляет сохранённый ручной порядок по новому порядку видимых карточек,
 * сохраняя позиции ключей, которые сейчас не видны (фильтр/остановленные):
 * каждый видимый ранее ключ заменяется очередным ключом из visibleAfter,
 * невидимые остаются на местах, лишние ключи из visibleAfter — в конец.
 */
export function mergeManualOrder(
	saved: string[],
	visibleBefore: string[],
	visibleAfter: string[],
): string[] {
	const beforeSet = new Set(visibleBefore);
	const result: string[] = [];
	const used = new Set<string>();
	let next = 0;

	const push = (key: string) => {
		if (used.has(key)) return;
		used.add(key);
		result.push(key);
	};

	for (const key of saved) {
		if (beforeSet.has(key)) {
			while (next < visibleAfter.length && used.has(visibleAfter[next])) next += 1;
			if (next < visibleAfter.length) {
				push(visibleAfter[next]);
				next += 1;
			}
		} else {
			push(key);
		}
	}

	for (; next < visibleAfter.length; next += 1) push(visibleAfter[next]);

	return result;
}
