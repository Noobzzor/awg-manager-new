import { describe, it, expect } from 'vitest';
import { errorMessage } from './errorMessage';

describe('errorMessage', () => {
	it('returns the message of a real Error', () => {
		expect(errorMessage(new Error('boom'))).toBe('boom');
	});

	it('falls back for an Error with an empty message', () => {
		expect(errorMessage(new Error(''), 'fallback')).toBe('fallback');
	});

	it('returns a non-empty string throw verbatim', () => {
		expect(errorMessage('plain failure')).toBe('plain failure');
	});

	it('uses the default fallback when none is given', () => {
		expect(errorMessage(null)).toBe('Ошибка');
	});

	it('falls back for non-Error objects', () => {
		expect(errorMessage({ message: 'nope' }, 'fallback')).toBe('fallback');
		expect(errorMessage(undefined, 'fallback')).toBe('fallback');
		expect(errorMessage(42, 'fallback')).toBe('fallback');
	});
});
