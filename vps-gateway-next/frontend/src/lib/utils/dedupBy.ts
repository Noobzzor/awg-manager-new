export interface DedupOptions {
	warnTag?: string;
}

export function dedupBy<T>(
	items: T[],
	keyFn: (item: T) => string,
	opts: DedupOptions = {},
): T[] {
	if (items.length === 0) return items;
	const seen = new Set<string>();
	const out: T[] = [];
	const droppedKeys: string[] = [];
	for (const it of items) {
		const k = keyFn(it);
		if (seen.has(k)) {
			droppedKeys.push(k);
			continue;
		}
		seen.add(k);
		out.push(it);
	}
	if (droppedKeys.length > 0 && opts.warnTag) {
		console.warn(
			`[dedupBy] dropped ${droppedKeys.length} duplicate ${opts.warnTag} entries; keys: ${Array.from(new Set(droppedKeys)).join(', ')}`,
		);
	}
	return droppedKeys.length === 0 ? items : out;
}
