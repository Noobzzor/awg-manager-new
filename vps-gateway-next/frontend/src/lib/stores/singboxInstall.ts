import { writable } from 'svelte/store';
import type { SingboxInstallProgressEvent } from '$lib/api/events';

/**
 * Live progress of an in-flight sing-box install or update flow. The
 * managed binary supports only one operation at a time, so we keep a
 * single nullable cell instead of a URL-keyed map (geoDownload's shape
 * leads to a UX bug where the bar disappears once the user types a
 * different URL — we deliberately avoid that here).
 *
 * On the terminal phase ('done' or 'error') we hold the final frame for
 * a short window so the UI can flash a "Готово ✓" / "Ошибка" state, then
 * clear automatically.
 */
function createSingboxInstallStore() {
	const { subscribe, set } = writable<SingboxInstallProgressEvent | null>(null);
	let dropTimer: ReturnType<typeof setTimeout> | null = null;

	function clearDropTimer() {
		if (dropTimer) {
			clearTimeout(dropTimer);
			dropTimer = null;
		}
	}

	return {
		subscribe,
		ingest(ev: SingboxInstallProgressEvent) {
			clearDropTimer();
			set(ev);
			if (ev.phase === 'done' || ev.phase === 'error') {
				dropTimer = setTimeout(() => {
					set(null);
					dropTimer = null;
				}, 2000);
			}
		},
		clear() {
			clearDropTimer();
			set(null);
		},
	};
}

export const singboxInstallProgress = createSingboxInstallStore();
