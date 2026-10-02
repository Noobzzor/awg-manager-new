import { describe, expect, it } from 'vitest';
import { newRuleUiKey, reconcileRuleUiKeys } from './ruleUiKeys';
import type { SingboxRouterRule } from '$lib/types';

describe('ruleUiKeys', () => {
	it('newRuleUiKey returns unique strings', () => {
		const a = newRuleUiKey();
		const b = newRuleUiKey();
		expect(a).not.toBe(b);
		expect(a.length).toBeGreaterThan(8);
	});

	it('reconcile keeps keys on reorder', () => {
		const rules: SingboxRouterRule[] = [
			{ domain_suffix: ['a.com'], outbound: 'direct' },
			{ domain_suffix: ['b.com'], outbound: 'direct' },
		];
		const keys = ['k-a', 'k-b'];
		const reordered = [rules[1], rules[0]];
		expect(reconcileRuleUiKeys(reordered, rules, keys)).toEqual(['k-b', 'k-a']);
	});

	it('reconcile assigns new keys for new rules', () => {
		const prev: SingboxRouterRule[] = [{ domain_suffix: ['a.com'], outbound: 'direct' }];
		const next: SingboxRouterRule[] = [
			{ domain_suffix: ['a.com'], outbound: 'direct' },
			{ domain_suffix: ['c.com'], outbound: 'direct' },
		];
		const out = reconcileRuleUiKeys(next, prev, ['k-a']);
		expect(out[0]).toBe('k-a');
		expect(out[1]).toBeTruthy();
		expect(out[1]).not.toBe('k-a');
	});
});
