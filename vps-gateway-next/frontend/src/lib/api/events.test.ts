import { describe, expect, it } from 'vitest';
import { parseConnectedEvent } from './events';

describe('parseConnectedEvent', () => {
	it('parses valid connected payload', () => {
		const got = parseConnectedEvent('{"ok":true,"instanceId":"abc"}');
		expect(got).toEqual({ ok: true, instanceId: 'abc' });
	});

	it('returns undefined for invalid JSON', () => {
		const got = parseConnectedEvent('{invalid');
		expect(got).toBeUndefined();
	});

	it('parses JSON without instanceId', () => {
		const got = parseConnectedEvent('{"ok":true}');
		expect(got).toEqual({ ok: true });
	});
});

