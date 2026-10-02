/**
 * Syntax highlight HTML underlay for inline rule set smart-list (RuleSetAddModal).
 */

function escapeHtml(text: string): string {
	return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

const RULE_PREFIX =
	/^(domain_regex|domain_keyword|domain_suffix|domain|suffix|keyword|regex|geosite|geoip|source_ip|src_ip|process_path|process_name|process|package_name|package|network|port_range|port|ip|cidr):/i;

const URL_RE = /^https?:\/\//i;
const IP_CIDR_RE = /^\d+\.\d+\.\d+\.\d+(?:\/\d+)?$/;
const ADBLOCK_RE = /^\|\|.+(\^)?$/;

/** Commas in port lists, adblock || ^ — purple (hl-rule-wild). */
function highlightRuleSymbols(text: string): string {
	let out = '';
	for (let i = 0; i < text.length; i++) {
		const ch = text[i];
		if (ch === ',' || ch === '^') {
			out += `<span class="hl-rule-wild">${escapeHtml(ch)}</span>`;
		} else if (ch === '|' && text[i + 1] === '|') {
			out += '<span class="hl-rule-wild">||</span>';
			i++;
		} else {
			out += escapeHtml(ch);
		}
	}
	return out;
}

function highlightAdblockRule(v: string): string {
	let body = v;
	let out = '';
	if (body.startsWith('||')) {
		out += '<span class="hl-rule-wild">||</span>';
		body = body.slice(2);
	}
	if (body.endsWith('^')) {
		body = body.slice(0, -1);
		out += highlightRuleSymbols(body);
		out += '<span class="hl-rule-wild">^</span>';
		return out;
	}
	return out + highlightRuleSymbols(body);
}

function splitInlineComment(line: string): { main: string; tail: string } {
	const hash = line.match(/^(.*?)(\s+#\s.*)$/);
	if (hash) return { main: hash[1], tail: hash[2] };
	const semi = line.match(/^(.*?)(\s+;\s.*)$/);
	if (semi) return { main: semi[1], tail: semi[2] };
	return { main: line, tail: '' };
}

function highlightRuleValue(val: string): string {
	const v = val.trim();
	if (!v) return '';
	if (URL_RE.test(v)) {
		return `<span class="hl-rule-url">${escapeHtml(v)}</span>`;
	}
	if (IP_CIDR_RE.test(v) || /^\d+\.\d+\.\d+\.\d+$/.test(v)) {
		return `<span class="hl-rule-ip">${escapeHtml(v)}</span>`;
	}
	if (ADBLOCK_RE.test(v)) {
		return highlightAdblockRule(v);
	}
	if (/^\d+(?:,\d+)*$/.test(v)) {
		return highlightRuleSymbols(v);
	}
	if (v.startsWith('*.')) {
		return `<span class="hl-rule-wild">${escapeHtml(v.slice(0, 2))}</span>${escapeHtml(v.slice(2))}`;
	}
	if (v.startsWith('.')) {
		return `<span class="hl-rule-dot">${escapeHtml(v.slice(0, 1))}</span>${escapeHtml(v.slice(1))}`;
	}
	return escapeHtml(v);
}

function highlightInlineRuleMain(line: string): string {
	const trimmed = line.trimStart();
	if (trimmed === '') return escapeHtml(line);
	if (trimmed.startsWith('#') || trimmed.startsWith('//') || trimmed.startsWith(';')) {
		return `<span class="hl-rule-comment">${escapeHtml(line)}</span>`;
	}

	const colonMatch = RULE_PREFIX.exec(trimmed);
	if (colonMatch) {
		const prefix = colonMatch[1];
		const prefixLen = colonMatch[0].length;
		const lead = line.slice(0, line.length - trimmed.length);
		const rest = trimmed.slice(prefixLen);
		return (
			`${escapeHtml(lead)}<span class="hl-rule-prefix">${escapeHtml(prefix)}:</span>${highlightRuleValue(rest)}`
		);
	}

	if (URL_RE.test(trimmed)) {
		return `<span class="hl-rule-url">${escapeHtml(line)}</span>`;
	}
	if (IP_CIDR_RE.test(trimmed)) {
		return `<span class="hl-rule-ip">${escapeHtml(line)}</span>`;
	}
	if (ADBLOCK_RE.test(trimmed)) {
		const lead = line.slice(0, line.length - trimmed.length);
		return `${escapeHtml(lead)}${highlightAdblockRule(trimmed)}`;
	}
	if (trimmed.startsWith('*.')) {
		const lead = line.slice(0, line.length - trimmed.length);
		return `${escapeHtml(lead)}<span class="hl-rule-wild">*.</span>${escapeHtml(trimmed.slice(2))}`;
	}
	if (trimmed.startsWith('.')) {
		const lead = line.slice(0, line.length - trimmed.length);
		return `${escapeHtml(lead)}<span class="hl-rule-dot">.</span>${escapeHtml(trimmed.slice(1))}`;
	}

	return escapeHtml(line);
}

function highlightInlineRuleLine(line: string): string {
	const { main, tail } = splitInlineComment(line);
	let out = highlightInlineRuleMain(main);
	if (tail) {
		out += `<span class="hl-rule-comment">${escapeHtml(tail)}</span>`;
	}
	return out;
}

export function highlightInlineRuleListContent(raw: string): string {
	if (!raw) return '';
	const lines = raw.split('\n');
	const parts: string[] = [];
	for (let i = 0; i < lines.length; i++) {
		parts.push(highlightInlineRuleLine(lines[i]));
		if (i < lines.length - 1) parts.push('\n');
	}
	return parts.join('');
}
