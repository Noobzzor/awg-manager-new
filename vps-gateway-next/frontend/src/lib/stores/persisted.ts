import { browser } from '$app/environment';
import { writable } from 'svelte/store';

export interface PersistedStoreOptions<T> {
	/** Value used before hydration (SSR) and when nothing valid is stored. */
	defaultValue: T;
	/** Turn a raw localStorage string into T. Return defaultValue for junk. */
	deserialize: (raw: string) => T;
	/** Turn T into the string persisted to localStorage. */
	serialize: (value: T) => string;
}

/**
 * createPersistedStore is the single localStorage-backed writable used by the
 * small UI-preference stores. It centralises the browser guard, the
 * try/catch around a potentially-throwing localStorage (quota / private mode)
 * and the read-on-construct + explicit init() re-read that individual stores
 * used to hand-roll, each with slightly different guards and encodings.
 */
export function createPersistedStore<T>(storageKey: string, opts: PersistedStoreOptions<T>) {
	function read(): T {
		if (!browser) return opts.defaultValue;
		try {
			const raw = localStorage.getItem(storageKey);
			return raw === null ? opts.defaultValue : opts.deserialize(raw);
		} catch {
			return opts.defaultValue;
		}
	}

	function persist(value: T): void {
		if (!browser) return;
		try {
			localStorage.setItem(storageKey, opts.serialize(value));
		} catch {
			/* ignore quota / private mode */
		}
	}

	const { subscribe, set, update } = writable<T>(read());

	return {
		subscribe,
		set(value: T) {
			persist(value);
			set(value);
		},
		update(fn: (current: T) => T) {
			update((current) => {
				const next = fn(current);
				persist(next);
				return next;
			});
		},
		/** Re-read from localStorage — used by stores initialised before hydration. */
		init() {
			set(read());
		},
	};
}

/**
 * createPersistedFlag is the boolean specialisation of createPersistedStore.
 * It writes the canonical '1'/'0' but reads liberally so preferences persisted
 * by the older stores (which variously used 'true'/'false' or '1'/'0') keep
 * working across an upgrade instead of silently resetting to defaultValue.
 */
export function createPersistedFlag(storageKey: string, defaultValue: boolean) {
	return createPersistedStore<boolean>(storageKey, {
		defaultValue,
		deserialize: (raw) => raw === '1' || raw === 'true',
		serialize: (value) => (value ? '1' : '0'),
	});
}
