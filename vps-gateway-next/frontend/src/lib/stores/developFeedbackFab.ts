import { createPersistedFlag } from './persisted';

const store = createPersistedFlag('awg-manager-develop-feedback-fab-visible', true);

/** Whether the develop-channel feedback FAB is shown (persisted in localStorage). */
export const developFeedbackFabVisible = {
	subscribe: store.subscribe,
	set: store.set,
};
