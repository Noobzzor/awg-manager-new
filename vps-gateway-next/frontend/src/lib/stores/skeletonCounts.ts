// Память формы для skeleton-загрузки: сколько плейсхолдеров рисовать на
// холодной загрузке = сколько элементов было в прошлый визит. Clamp 1..6 —
// защита и от мусора в localStorage, и от простыни на больших списках.
import { createPersistedStore } from './persisted';

const MIN_COUNT = 1;
const MAX_COUNT = 6;

export function clampSkeletonCount(n: number, fallback: number): number {
	if (!Number.isFinite(n)) return fallback;
	return Math.min(MAX_COUNT, Math.max(MIN_COUNT, Math.round(n)));
}

function countStore(storageKey: string, defaultValue: number) {
	return createPersistedStore<number>(storageKey, {
		defaultValue,
		deserialize: (raw) => clampSkeletonCount(Number(raw), defaultValue),
		serialize: String,
	});
}

export const tunnelsSkeletonCount = countStore('awg.skeleton.tunnels', 3);
export const serversSkeletonCount = countStore('awg.skeleton.servers', 2);
