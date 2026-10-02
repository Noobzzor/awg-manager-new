import type { SingboxRouterRule } from '$lib/types';

/** Stable list keys for {#each} — like React keys, unrelated to sing-box config. */
export function newRuleUiKey(): string {
	if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
		return crypto.randomUUID();
	}
	return `rule-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
}

export function ruleSignature(rule: SingboxRouterRule): string {
	return JSON.stringify(rule);
}

export function reconcileRuleUiKeys(
	nextRules: SingboxRouterRule[],
	prevRules: SingboxRouterRule[],
	prevKeys: string[],
): string[] {
	const usedPrev = new Set<number>();
	const prevKeyAt = (i: number) => prevKeys[i] ?? newRuleUiKey();

	return nextRules.map((nextRule, nextIdx) => {
		const sig = ruleSignature(nextRule);

		if (prevRules[nextIdx] && ruleSignature(prevRules[nextIdx]) === sig) {
			usedPrev.add(nextIdx);
			return prevKeyAt(nextIdx);
		}

		for (let i = 0; i < prevRules.length; i++) {
			if (usedPrev.has(i)) continue;
			if (ruleSignature(prevRules[i]) === sig) {
				usedPrev.add(i);
				return prevKeyAt(i);
			}
		}

		return newRuleUiKey();
	});
}
