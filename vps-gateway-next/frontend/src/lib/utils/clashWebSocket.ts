export type WSStatus = 'connecting' | 'open' | 'closed' | 'error';

const BACKOFF_SCHEDULE_MS = [1000, 2000, 5000, 10000];

function buildWSURL(path: string): string {
	const loc = (globalThis as { location?: { protocol: string; host: string } }).location ?? {
		protocol: 'http:',
		host: 'localhost',
	};
	const proto = loc.protocol === 'https:' ? 'wss:' : 'ws:';
	return `${proto}//${loc.host}${path}`;
}

/**
 * Opens a WebSocket to the given backend path (e.g. "/api/singbox/clash/connections").
 * onMessage receives JSON.parse'd payloads. onStatus reports lifecycle transitions
 * for UI banner. Reconnects unexpected closes with capped exponential backoff
 * (1s, 2s, 5s, 10s, then 10s indefinitely). The returned close() ends the session
 * cleanly — no further reconnect attempts.
 */
export function createClashWS<T>(
	path: string,
	onMessage: (data: T) => void,
	onStatus: (s: WSStatus, err?: Error) => void,
): () => void {
	let stopped = false;
	let attempt = 0;
	let timer: ReturnType<typeof setTimeout> | null = null;
	let socket: WebSocket | null = null;

	function connect(): void {
		if (stopped) return;
		onStatus('connecting');
		const ws = new WebSocket(buildWSURL(path));
		socket = ws;
		ws.onopen = () => {
			attempt = 0;
			onStatus('open');
		};
		ws.onmessage = (ev) => {
			try {
				onMessage(JSON.parse(ev.data) as T);
			} catch (e) {
				onStatus('error', e instanceof Error ? e : new Error(String(e)));
			}
		};
		ws.onerror = () => {
			onStatus('error');
		};
		ws.onclose = () => {
			if (stopped) return;
			onStatus('closed');
			const delay = BACKOFF_SCHEDULE_MS[Math.min(attempt, BACKOFF_SCHEDULE_MS.length - 1)];
			attempt += 1;
			timer = setTimeout(connect, delay);
		};
	}

	connect();

	return () => {
		stopped = true;
		if (timer !== null) clearTimeout(timer);
		try {
			socket?.close();
		} catch {
			/* ignore */
		}
	};
}
