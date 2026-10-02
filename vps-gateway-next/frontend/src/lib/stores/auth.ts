import { writable, derived } from 'svelte/store';
import { browser } from '$app/environment';
import { api } from '$lib/api/client';

interface AuthState {
	authenticated: boolean;
	authDisabled: boolean;
	/** Вход по учётным данным Entware (/opt/etc/shadow) включён на бэкенде. */
	entwareAuthEnabled: boolean;
	login: string | null;
	loading: boolean;
	error: string | null;
}

function createAuthStore() {
	const { subscribe, set, update } = writable<AuthState>({
		authenticated: false,
		authDisabled: false,
		entwareAuthEnabled: false,
		login: null,
		loading: true,
		error: null
	});

	// Setup 401 handler to auto-logout on session expiry
	if (browser) {
		api.setUnauthorizedHandler(() => {
			update((s) => {
				// Only show "session expired" if user was previously authenticated.
				// If not authenticated yet (e.g. on login page), just ignore the 401.
				if (!s.authenticated) return s;
				return {
					authenticated: false,
					authDisabled: false,
					entwareAuthEnabled: s.entwareAuthEnabled,
					login: null,
					loading: false,
					error: 'Сессия истекла'
				};
			});
		});
	}

	return {
		subscribe,

		async checkStatus() {
			if (!browser) return;

			// Skip auth in dev mode
			if (import.meta.env.DEV) {
				set({
					authenticated: true,
					authDisabled: false,
					entwareAuthEnabled: false,
					login: 'dev',
					loading: false,
					error: null
				});
				return;
			}

			update((s) => ({ ...s, loading: true, error: null }));

			try {
				const status = await api.getAuthStatus();
				set({
					authenticated: status.authenticated,
					authDisabled: status.authDisabled ?? false,
					// Legacy backends omit the field → treat as disabled.
					entwareAuthEnabled: status.entwareAuthEnabled ?? false,
					login: status.login || null,
					loading: false,
					error: null
				});
			} catch (e) {
				update((s) => ({
					...s,
					authenticated: false,
					authDisabled: false,
					login: null,
					loading: false,
					error: null
				}));
			}
		},

		async login(login: string, password: string) {
			update((s) => ({ ...s, loading: true, error: null }));

			try {
				const result = await api.login(login, password);
				update((s) => ({
					...s,
					authenticated: true,
					authDisabled: false,
					login: result.login,
					loading: false,
					error: null
				}));
				return true;
			} catch (e) {
				update((s) => ({
					...s,
					loading: false,
					error: e instanceof Error ? e.message : 'Ошибка авторизации'
				}));
				return false;
			}
		},

		async logout() {
			api.abortAll();
			try {
				await api.logout();
			} catch {
				// Ignore logout errors
			}
			// entwareAuthEnabled сохраняем — форма логина показывает правильную
			// подсказку сразу после выхода, без повторного /auth/status.
			update((s) => ({
				...s,
				authenticated: false,
				authDisabled: false,
				login: null,
				loading: false,
				error: null
			}));
		},

		clearError() {
			update((s) => ({ ...s, error: null }));
		}
	};
}

export const auth = createAuthStore();
export const isAuthenticated = derived(auth, ($auth) => $auth.authenticated);
export const isLoading = derived(auth, ($auth) => $auth.loading);
