export type RestartWaitResult =
	| 'instance-changed'
	| 'offline-online'
	| 'stable-online'
	| 'timeout';

export interface WaitForBackendRestartOptions {
	previousInstanceId: string | null;
	readInstanceId: () => Promise<string | null>;
	sleep: (ms: number) => Promise<void>;
	now: () => number;
	timeoutMs?: number;
	pollMs?: number;
	stableOnlineMs?: number;
}

export async function waitForBackendRestart(
	opts: WaitForBackendRestartOptions,
): Promise<RestartWaitResult> {
	const timeoutMs = opts.timeoutMs ?? 45_000;
	const pollMs = opts.pollMs ?? 750;
	const stableOnlineMs = opts.stableOnlineMs ?? 3_000;

	const startedAt = opts.now();
	const deadline = startedAt + timeoutMs;
	let sawOffline = false;

	while (opts.now() < deadline) {
		try {
			const currentInstanceId = await opts.readInstanceId();
			if (!currentInstanceId) {
				sawOffline = true;
			}

			if (
				opts.previousInstanceId &&
				currentInstanceId &&
				currentInstanceId !== opts.previousInstanceId
			) {
				return 'instance-changed';
			}

			if (!opts.previousInstanceId && currentInstanceId && sawOffline) {
				return 'offline-online';
			}

			if (
				!opts.previousInstanceId &&
				currentInstanceId &&
				opts.now() - startedAt > stableOnlineMs
			) {
				return 'stable-online';
			}
		} catch {
			sawOffline = true;
		}

		await opts.sleep(pollMs);
	}

	return 'timeout';
}

