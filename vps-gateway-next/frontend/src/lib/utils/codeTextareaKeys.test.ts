import { describe, expect, it } from 'vitest';
import { jsonLineContinuationIndent } from './codeTextareaKeys';

describe('jsonLineContinuationIndent', () => {
	it('keeps base indent on plain line', () => {
		expect(jsonLineContinuationIndent('  "a": 1')).toBe('  ');
	});

	it('adds tab after opening brace or bracket', () => {
		expect(jsonLineContinuationIndent('\t{')).toBe('\t\t');
		expect(jsonLineContinuationIndent('  [')).toBe('  \t');
	});

	it('adds tab after trailing colon', () => {
		expect(jsonLineContinuationIndent('\t"rules":')).toBe('\t\t');
	});
});
