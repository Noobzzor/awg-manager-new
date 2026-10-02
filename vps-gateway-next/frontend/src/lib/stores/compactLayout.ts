import type { UsageLevel } from '$lib/types/usageLevel';
import { createPersistedFlag } from './persisted';

const store = createPersistedFlag('awg-manager-compact-layout', false);

export const compactLayout = {
	subscribe: store.subscribe,
	init: store.init,
	setEnabled: store.set,
};

/** Базовый режим — всегда; расширенный/продвинутый — по выбору пользователя. */
export function isCompactLayoutActive(level: UsageLevel, userEnabled: boolean): boolean {
	return level === 'basic' || userEnabled;
}
