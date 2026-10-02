// Теги туннелей на дашборде (issue #142): нормализация, группировка и фильтр
// по карте key → tags из tunnelDashboardPrefs.

const MAX_TAG_LENGTH = 24;

/** Ключ сравнения тегов: регистронезависимый ('Home' ≡ 'home'). */
export function tagKey(tag: string): string {
	return tag.toLocaleLowerCase('ru');
}

export function normalizeTag(raw: string): string | null {
	const collapsed = raw.trim().replace(/\s+/g, ' ');
	if (!collapsed) return null;
	return collapsed.length > MAX_TAG_LENGTH
		? collapsed.slice(0, MAX_TAG_LENGTH).trimEnd()
		: collapsed;
}

export function getItemTags(tags: Record<string, string[]>, key: string): string[] {
	return tags[key] ?? [];
}

export function collectAllTags(tags: Record<string, string[]>): string[] {
	// Дедуп по tagKey, отображается первое встреченное написание.
	const unique = new Map<string, string>();
	for (const itemTags of Object.values(tags)) {
		for (const tag of itemTags) {
			const key = tagKey(tag);
			if (!unique.has(key)) unique.set(key, tag);
		}
	}
	return [...unique.values()].sort((a, b) => a.localeCompare(b, 'ru'));
}

export function groupFlatItemsByTag<T extends { key: string }>(
	items: T[],
	tags: Record<string, string[]>,
): Array<{ tag: string | null; items: T[] }> {
	const groups: Array<{ tag: string | null; items: T[] }> = [];

	for (const tag of collectAllTags(tags)) {
		const tagged = filterItemsByTag(items, tags, tag);
		if (tagged.length > 0) groups.push({ tag, items: tagged });
	}

	const untagged = items.filter((item) => getItemTags(tags, item.key).length === 0);
	if (untagged.length > 0) groups.push({ tag: null, items: untagged });

	return groups;
}

export function filterItemsByTag<T extends { key: string }>(
	items: T[],
	tags: Record<string, string[]>,
	tag: string,
): T[] {
	const wanted = tagKey(tag);
	return items.filter((item) =>
		getItemTags(tags, item.key).some((itemTag) => tagKey(itemTag) === wanted),
	);
}
