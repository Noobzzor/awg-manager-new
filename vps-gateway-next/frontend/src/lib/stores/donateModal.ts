import { writable } from 'svelte/store';

/** Shared with +layout donate modal (heart in header). */
export const donateModalOpen = writable(false);

export function openDonateModal(): void {
	donateModalOpen.set(true);
}

export function closeDonateModal(): void {
	donateModalOpen.set(false);
}
