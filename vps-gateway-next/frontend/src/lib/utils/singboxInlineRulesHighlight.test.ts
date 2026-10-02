import { describe, expect, it } from 'vitest';
import { highlightInlineRuleListContent } from './singboxInlineRulesHighlight';

describe('highlightInlineRuleListContent', () => {
	it('highlights prefixes, comments and IPs', () => {
		const html = highlightInlineRuleListContent(
			'# note\ngeosite:GOOGLE\n1.1.1.1\nport:443,8443\n||domain.com^',
		);
		expect(html).toContain('hl-rule-comment');
		expect(html).toContain('hl-rule-prefix');
		expect(html).toContain('geosite:');
		expect(html).toContain('hl-rule-ip');
		expect(html).toContain('hl-rule-wild');
		expect(html).toContain('hl-rule-wild">||</span>');
		expect(html).toContain('port:');
	});
});
