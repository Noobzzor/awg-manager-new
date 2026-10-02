import { writable } from 'svelte/store';
import { browser } from '$app/environment';

export interface SortState<T extends string> {
	sortBy: T | null;
	sortAsc: boolean;
}

/**
 * createSortStore is the shared localStorage-backed core for table-sort
 * preferences. Both peerSort and the tunnel/subscription table sorts used to
 * duplicate this getInitial/persist/defaultState block verbatim; they now
 * layer their own mutators on top of `mutate`.
 *
 * `defaults[key]` supplies the initial direction when a persisted state names
 * a column but omits (or corrupts) its sortAsc flag.
 */
export function createSortStore<T extends string>(
	storageKey: string,
	validKeys: readonly T[],
	defaults: Record<T, boolean>,
) {
	const valid = new Set<T>(validKeys);

	function defaultState(): SortState<T> {
		return { sortBy: null, sortAsc: true };
	}

	function getInitial(): SortState<T> {
		if (!browser) return defaultState();
		try {
			const raw = localStorage.getItem(storageKey);
			if (!raw) return defaultState();
			const parsed = JSON.parse(raw) as Partial<SortState<T>> | null;
			if (!parsed || typeof parsed !== 'object') return defaultState();
			const sortBy = parsed.sortBy ?? null;
			if (sortBy !== null && !valid.has(sortBy)) return defaultState();
			return {
				sortBy,
				sortAsc:
					typeof parsed.sortAsc === 'boolean'
						? parsed.sortAsc
						: sortBy !== null
							? defaults[sortBy]
							: true,
			};
		} catch {
			return defaultState();
		}
	}

	function persist(state: SortState<T>): void {
		if (!browser) return;
		try {
			localStorage.setItem(storageKey, JSON.stringify(state));
		} catch {
			// ignore quota / private mode
		}
	}

	const { subscribe, update } = writable<SortState<T>>(getInitial());

	return {
		subscribe,
		/** Apply a pure transition to the current state and persist the result. */
		mutate(fn: (state: SortState<T>) => SortState<T>) {
			update((state) => {
				const next = fn(state);
				persist(next);
				return next;
			});
		},
	};
}
