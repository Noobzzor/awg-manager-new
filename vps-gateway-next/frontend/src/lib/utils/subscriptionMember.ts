import type { Subscription } from '$lib/types';

export function resolveSubscriptionMemberTag(
	subscription: Subscription,
	liveActiveMember?: string | null,
): string {
	if (liveActiveMember && subscription.memberTags.includes(liveActiveMember)) {
		return liveActiveMember;
	}
	if (subscription.activeMember && subscription.memberTags.includes(subscription.activeMember)) {
		return subscription.activeMember;
	}
	return subscription.memberTags[0] ?? '';
}
