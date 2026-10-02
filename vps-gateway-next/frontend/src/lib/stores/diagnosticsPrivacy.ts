import { createPersistedFlag } from './persisted';

const store = createPersistedFlag('awgm.diagnostics.sanitizeLogs', true);

export const diagnosticsSanitized = {
	subscribe: store.subscribe,
	set: store.set,
	toggle() {
		store.update((value) => !value);
	},
};

export function toggleDiagnosticsSanitized() {
	diagnosticsSanitized.toggle();
}
