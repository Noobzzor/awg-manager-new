/**
 * Syntax highlight for WireGuard / AmneziaWG .conf (vpn:// decoded bodies, paste import).
 *
 * Underlay must stay pixel-identical to a plain <textarea>: only full-line wrappers
 * ([Section], # comments). Splitting key=value into <span>s shifts kerning vs one text node.
 */

function escapeHtml(text: string): string {
	return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

function highlightLine(line: string): string {
	if (line === '') return '';
	const trimmedAll = line.trim();
	if (trimmedAll === '') {
		return escapeHtml(line);
	}

	const contentStart = line.search(/\S/);
	const leading = contentStart >= 0 ? line.slice(0, contentStart) : line;
	const rest = contentStart >= 0 ? line.slice(contentStart) : '';

	if (rest.startsWith('#')) {
		return `${escapeHtml(leading)}<span class="ace-comment">${escapeHtml(rest)}</span>`;
	}

	if (rest.startsWith('[') && trimmedAll.endsWith(']')) {
		return `${escapeHtml(leading)}<span class="ace-section">${escapeHtml(rest)}</span>`;
	}

	/* Key = value and everything else: one text node in <pre>, same as textarea. */
	return escapeHtml(line);
}

/** HTML underlay matching the raw config (line-based). */
export function highlightAmneziaConfContent(raw: string): string {
	if (!raw) return '';
	const lines = raw.split('\n');
	const parts: string[] = [];
	for (let i = 0; i < lines.length; i++) {
		parts.push(highlightLine(lines[i]));
		if (i < lines.length - 1) parts.push('\n');
	}
	return parts.join('');
}
