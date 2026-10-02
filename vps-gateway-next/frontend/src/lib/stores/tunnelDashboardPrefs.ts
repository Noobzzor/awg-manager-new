// Дополнительные настройки дашборда туннелей (issue #142): ручной порядок
// карточек, режим группировки и теги. Все — persisted через createPersistedStore
// (SSR-safe: до гидратации отдаётся defaultValue, init() перечитывает).
import { normalizeTag, tagKey } from '$lib/utils/tunnelDashboardTags';
import { createPersistedStore } from './persisted';

export type TunnelDashboardOrderMode = 'auto' | 'manual';
export type TunnelDashboardGroupMode = 'type' | 'tags';

const orderModeStore = createPersistedStore<TunnelDashboardOrderMode>(
	'awg-manager-tunnel-dashboard-order-mode',
	{
		defaultValue: 'auto',
		deserialize: (raw) => (raw === 'auto' || raw === 'manual' ? raw : 'auto'),
		serialize: (mode) => mode,
	},
);

export const tunnelDashboardOrderMode = {
	subscribe: orderModeStore.subscribe,
	init: orderModeStore.init,
	setMode: orderModeStore.set,
};

const manualOrderStore = createPersistedStore<string[]>('awg-manager-tunnel-dashboard-order', {
	defaultValue: [],
	deserialize: (raw) => {
		try {
			const parsed: unknown = JSON.parse(raw);
			return Array.isArray(parsed) && parsed.every((key) => typeof key === 'string')
				? parsed
				: [];
		} catch {
			return [];
		}
	},
	serialize: (order) => JSON.stringify(order),
});

export const tunnelDashboardManualOrder = {
	subscribe: manualOrderStore.subscribe,
	init: manualOrderStore.init,
	set: manualOrderStore.set,
};

const groupModeStore = createPersistedStore<TunnelDashboardGroupMode>(
	'awg-manager-tunnel-dashboard-group',
	{
		defaultValue: 'type',
		deserialize: (raw) => (raw === 'type' || raw === 'tags' ? raw : 'type'),
		serialize: (mode) => mode,
	},
);

export const tunnelDashboardGroupMode = {
	subscribe: groupModeStore.subscribe,
	init: groupModeStore.init,
	setMode: groupModeStore.set,
};

const tagsStore = createPersistedStore<Record<string, string[]>>('awg-manager-tunnel-tags', {
	defaultValue: {},
	deserialize: (raw) => {
		try {
			const parsed: unknown = JSON.parse(raw);
			if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) return {};
			const valid: Record<string, string[]> = {};
			for (const [key, value] of Object.entries(parsed)) {
				if (!Array.isArray(value) || !value.every((tag) => typeof tag === 'string')) continue;
				// Санитизация правленных руками данных: нормализация, дедуп
				// без учёта регистра, пустые массивы отбрасываются.
				const seen = new Set<string>();
				const clean: string[] = [];
				for (const raw of value) {
					const tag = normalizeTag(raw);
					if (tag === null || seen.has(tagKey(tag))) continue;
					seen.add(tagKey(tag));
					clean.push(tag);
				}
				if (clean.length > 0) valid[key] = clean;
			}
			return valid;
		} catch {
			return {};
		}
	},
	serialize: (tags) => JSON.stringify(tags),
});

export const tunnelDashboardTags = {
	subscribe: tagsStore.subscribe,
	init: tagsStore.init,
	addTag(itemKey: string, raw: string) {
		const tag = normalizeTag(raw);
		if (!itemKey || tag === null) return;
		tagsStore.update((tags) => {
			const current = tags[itemKey] ?? [];
			if (current.some((existing) => tagKey(existing) === tagKey(tag))) return tags;
			return { ...tags, [itemKey]: [...current, tag] };
		});
	},
	removeTag(itemKey: string, tag: string) {
		tagsStore.update((tags) => {
			const current = tags[itemKey];
			if (!current || !current.includes(tag)) return tags;
			const remaining = current.filter((existing) => existing !== tag);
			const next = { ...tags };
			if (remaining.length === 0) {
				delete next[itemKey];
			} else {
				next[itemKey] = remaining;
			}
			return next;
		});
	},
};
