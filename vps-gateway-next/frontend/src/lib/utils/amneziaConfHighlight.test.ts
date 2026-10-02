import { describe, expect, it } from 'vitest';
import { highlightAmneziaConfContent } from './amneziaConfHighlight';

describe('highlightAmneziaConfContent', () => {
	it('wraps sections', () => {
		const html = highlightAmneziaConfContent('[Interface]');
		expect(html).toContain('ace-section');
		expect(html).toContain('[Interface]');
	});

	it('does not wrap key=value lines in spans (layout parity with textarea)', () => {
		const html = highlightAmneziaConfContent('Jc = 4\nPrivateKey = abc');
		expect(html).not.toContain('ace-key');
		expect(html).not.toContain('ace-val');
		expect(html).toContain('Jc = 4');
		expect(html).toContain('PrivateKey = abc');
	});

	it('colors comments', () => {
		const html = highlightAmneziaConfContent('  # note');
		expect(html).toContain('ace-comment');
	});

	it('escapes HTML in plain lines', () => {
		const html = highlightAmneziaConfContent('Name = <x>');
		expect(html).toContain('&lt;x&gt;');
		expect(html).not.toContain('<x>');
	});
});
