/** One level of indent in JSON editor (matches `tab-size: 4` in CSS). */
export const JSON_EDITOR_TAB = '\t';

export type CodeTextareaKeyContext = {
	getValue: () => string;
	setValue: (v: string) => void;
	getSelection: () => { start: number; end: number };
	setSelection: (start: number, end: number) => void;
};

/** Extra indent after Enter based on current line ending (JSON). */
export function jsonLineContinuationIndent(line: string): string {
	const base = line.match(/^(\s*)/)?.[1] ?? '';
	const trimmed = line.trimEnd();
	if (/[{[,]\s*$/.test(trimmed)) return base + JSON_EDITOR_TAB;
	if (/:\s*$/.test(trimmed)) return base + JSON_EDITOR_TAB;
	return base;
}

function lineRangeAt(value: string, index: number): { lineStart: number; lineEnd: number } {
	const before = value.slice(0, index);
	const lineStart = before.lastIndexOf('\n') + 1;
	const lineEnd = value.indexOf('\n', index);
	return { lineStart, lineEnd: lineEnd === -1 ? value.length : lineEnd };
}

function selectedLineRanges(value: string, start: number, end: number): Array<{ from: number; to: number }> {
	const ranges: Array<{ from: number; to: number }> = [];
	let pos = start;
	const first = lineRangeAt(value, start);
	pos = first.lineStart;
	const last = lineRangeAt(value, Math.max(start, end - 1));
	let cursor = pos;
	while (cursor <= last.lineStart) {
		const { lineStart, lineEnd } = lineRangeAt(value, cursor);
		ranges.push({ from: lineStart, to: lineEnd });
		cursor = lineEnd + 1;
	}
	return ranges;
}

function stripOneIndentLevel(line: string): { line: string; removed: number } {
	if (line.startsWith('\t')) return { line: line.slice(1), removed: 1 };
	if (line.startsWith('    ')) return { line: line.slice(4), removed: 4 };
	return { line, removed: 0 };
}

function applyLineEdits(
	value: string,
	edits: Array<{ from: number; to: number; insert: string }>,
): string {
	const sorted = [...edits].sort((a, b) => b.from - a.from);
	let out = value;
	for (const e of sorted) {
		out = out.slice(0, e.from) + e.insert + out.slice(e.to);
	}
	return out;
}

function indentSelection(ctx: CodeTextareaKeyContext): void {
	const { start, end } = ctx.getSelection();
	const value = ctx.getValue();
	const ranges = selectedLineRanges(value, start, end);
	const edits = ranges.map((r) => ({
		from: r.from,
		to: r.from,
		insert: JSON_EDITOR_TAB,
	}));
	const next = applyLineEdits(value, edits);
	let deltaStart = 0;
	let deltaEnd = 0;
	for (const r of ranges) {
		if (r.from < start) deltaStart += JSON_EDITOR_TAB.length;
		if (r.from < end) deltaEnd += JSON_EDITOR_TAB.length;
	}
	ctx.setValue(next);
	ctx.setSelection(start + deltaStart, end + deltaEnd);
}

function outdentSelection(ctx: CodeTextareaKeyContext): void {
	const { start, end } = ctx.getSelection();
	let value = ctx.getValue();
	const ranges = selectedLineRanges(value, start, end);
	let deltaStart = 0;
	let deltaEnd = 0;
	for (let i = ranges.length - 1; i >= 0; i--) {
		const r = ranges[i];
		const line = value.slice(r.from, r.to);
		const { line: next, removed } = stripOneIndentLevel(line);
		if (removed === 0) continue;
		value = value.slice(0, r.from) + next + value.slice(r.to);
		if (r.from < start) deltaStart += removed;
		if (r.from < end) deltaEnd += removed;
	}
	ctx.setValue(value);
	ctx.setSelection(Math.max(0, start - deltaStart), Math.max(0, end - deltaEnd));
}

function insertTab(ctx: CodeTextareaKeyContext): void {
	const { start, end } = ctx.getSelection();
	const value = ctx.getValue();
	const next = value.slice(0, start) + JSON_EDITOR_TAB + value.slice(end);
	ctx.setValue(next);
	const pos = start + JSON_EDITOR_TAB.length;
	ctx.setSelection(pos, pos);
}

function handleEnterJson(e: KeyboardEvent, ctx: CodeTextareaKeyContext): boolean {
	if (e.key !== 'Enter' || e.shiftKey || e.ctrlKey || e.metaKey || e.altKey) return false;
	const { start, end } = ctx.getSelection();
	if (start !== end) return false;

	const value = ctx.getValue();
	const { lineStart } = lineRangeAt(value, start);
	const currentLine = value.slice(lineStart, start);
	const continuation = jsonLineContinuationIndent(currentLine);
	const insert = '\n' + continuation;

	ctx.setValue(value.slice(0, start) + insert + value.slice(start));
	const pos = start + insert.length;
	ctx.setSelection(pos, pos);
	return true;
}

/** Tab / Shift+Tab / Enter (JSON indent). Returns true if handled. */
export function handleJsonEditorKeydown(e: KeyboardEvent, ctx: CodeTextareaKeyContext): boolean {
	if (e.key === 'Tab') {
		e.preventDefault();
		if (e.shiftKey) {
			outdentSelection(ctx);
		} else if (ctx.getSelection().start !== ctx.getSelection().end) {
			indentSelection(ctx);
		} else {
			insertTab(ctx);
		}
		return true;
	}
	if (handleEnterJson(e, ctx)) {
		e.preventDefault();
		return true;
	}
	return false;
}
