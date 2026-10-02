import { createPersistedFlag } from './persisted';

const store = createPersistedFlag('awg-manager-service-letter-icons', true);

/** User preference: colored monogram tiles when no custom / brand icon applies. */
export const serviceLetterIcons = {
	subscribe: store.subscribe,
	init: store.init,
	setEnabled: store.set,
};
