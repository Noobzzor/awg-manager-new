import { describe, expect, it } from 'vitest';
import {
	datInfo,
	resolveRuleSetDisplayType,
	ruleSetDisplayVariant,
} from '$lib/utils/ruleSetType';
import type { SingboxRouterRuleSet } from '$lib/types';

describe('ruleSetType', () => {
	it('detects dat-srs remote rule sets', () => {
		const rs: SingboxRouterRuleSet = {
			tag: 'geosite-google',
			type: 'remote',
			url: 'http://127.0.0.1:8081/api/singbox/router/rulesets/dat-srs?kind=geosite&tag=GOOGLE',
		};
		expect(datInfo(rs)).toEqual({ kind: 'geosite', tags: ['GOOGLE'] });
		expect(resolveRuleSetDisplayType(rs)).toBe('dat');
		expect(ruleSetDisplayVariant('dat')).toBe('purple');
	});

	it('keeps plain remote URLs as remote', () => {
		const rs: SingboxRouterRuleSet = {
			tag: 'geosite-youtube',
			type: 'remote',
			url: 'https://cdn.example.com/geosite-youtube.srs',
		};
		expect(datInfo(rs)).toBeNull();
		expect(resolveRuleSetDisplayType(rs)).toBe('remote');
	});

	it('maps inline and local types', () => {
		expect(resolveRuleSetDisplayType({ tag: 'x', type: 'inline' })).toBe('inline');
		expect(resolveRuleSetDisplayType({ tag: 'x', type: 'local', path: '/tmp/x.srs' })).toBe('local');
	});
});
