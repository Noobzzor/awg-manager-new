// View-модели вкладок туннельной страницы, разделяемые секциями
// (выделено при декомпозиции routes/+page.svelte).
import type { Subscription, SubscriptionMember } from '$lib/types';

export interface SubscriptionActiveCardVM {
	subscription: Subscription;
	activeMember: SubscriptionMember;
}

export interface SubscriptionsTrafficStats {
	count: number;
	activeCount: number;
	inactiveCount: number;
	down: number;
	up: number;
	avgDelayMs: number | null;
	delaySamples: number;
	leaderBytes: number;
	leaderName: string;
	leaderSharePct: number;
}

export interface SingboxTunnelListStats {
	count: number;
	running: number;
	stopped: number;
	down: number;
	up: number;
	avgDelayMs: number | null;
	leaderBytes: number;
	leaderName: string;
}
