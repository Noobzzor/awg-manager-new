import { describe, expect, it } from 'vitest';
import { highlightJson, highlightShareEditorContent } from './shareEditorHighlight';

describe('highlightJson', () => {
	it('colors object keys blue and string values amber', () => {
		const html = highlightJson(
			'[\n  {\n    "domain_keyword": [\n      "youtube"\n    ]\n  }\n]',
		);
		expect(html).toContain('hl-json-key">"domain_keyword"</span>');
		expect(html).toContain('hl-json-str">"youtube"</span>');
		expect(html).not.toContain('hl-json-key">"youtube"');
	});

	it('highlights Mieru share schemes', () => {
		const html = highlightShareEditorContent(
			'mieru://AAECAw==\n  mierus://user:pass@example.com?profile=p&port=443&protocol=TCP',
		);
		expect(html).toContain('<span class="share-link-proto">mieru://</span>');
		expect(html).toContain('<span class="share-link-proto">mierus://</span>');
	});
});
