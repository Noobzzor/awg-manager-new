import { writable, type Readable } from 'svelte/store';

export type HealthState = {
	online: boolean;
	lastCheckAt: number;
	consecutiveFailures: number;
	instanceId?: string;
};

export interface HealthMonitor extends Readable<HealthState> {
	start: () => void;
	stop: () => void;
}

const POLL_INTERVAL_MS = 5_000;
const OFFLINE_THRESHOLD = 2; // consecutive failures before flipping online=false

export function shouldReloadForInstanceSwitch(previous?: string, next?: string): boolean {
	return Boolean(previous && next && previous !== next);
}

function createHealthMonitor(): HealthMonitor {
	const state = writable<HealthState>({
		online: true,
		lastCheckAt: 0,
		consecutiveFailures: 0,
		instanceId: undefined,
	});

	let timer: ReturnType<typeof setInterval> | null = null;
	let running = false;
	let visibilityHandler: (() => void) | null = null;

	async function tick() {
		try {
			const res = await fetch('/api/health', { method: 'GET', cache: 'no-store', credentials: 'same-origin' });
			if (!res.ok) throw new Error(`health ${res.status}`);
			const body = await res.json().catch(() => ({}));
			const nextInstanceId = typeof body?.data?.instanceId === 'string' ? body.data.instanceId : undefined;
			let shouldReload = false;
			state.update((s) => {
				if (shouldReloadForInstanceSwitch(s.instanceId, nextInstanceId)) {
					shouldReload = true;
				}
				return {
					online: true,
					lastCheckAt: Date.now(),
					consecutiveFailures: 0,
					instanceId: nextInstanceId ?? s.instanceId,
				};
			});
			if (shouldReload && typeof location !== 'undefined') {
				location.reload();
			}
		} catch {
			state.update(s => {
				const fails = s.consecutiveFailures + 1;
				return { ...s, lastCheckAt: Date.now(), consecutiveFailures: fails, online: fails < OFFLINE_THRESHOLD };
			});
		}
	}

	function startTimer() {
		if (timer !== null) return;
		timer = setInterval(tick, POLL_INTERVAL_MS);
	}

	function stopTimer() {
		if (timer !== null) {
			clearInterval(timer);
			timer = null;
		}
	}

	return {
		subscribe: state.subscribe,
		start() {
			if (running) return;
			running = true;
			void tick();
			startTimer();
			if (typeof document !== 'undefined' && visibilityHandler === null) {
				visibilityHandler = () => {
					if (document.visibilityState === 'hidden') {
						stopTimer();
					} else {
						void tick();
						startTimer();
					}
				};
				document.addEventListener('visibilitychange', visibilityHandler);
			}
		},
		stop() {
			running = false;
			stopTimer();
			if (typeof document !== 'undefined' && visibilityHandler !== null) {
				document.removeEventListener('visibilitychange', visibilityHandler);
				visibilityHandler = null;
			}
		},
	};
}

export const healthMonitor = createHealthMonitor();
