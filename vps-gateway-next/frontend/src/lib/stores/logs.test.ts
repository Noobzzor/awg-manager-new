import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { appLogEntries } from './logs';
import type { LogEntryEvent } from '$lib/api/events';
import type { LogEntry } from '$lib/types';

function ev(over: Partial<LogEntryEvent> = {}): LogEntryEvent {
	return {
		timestamp: '2026-01-01T10:00:00Z',
		level: 'warn',
		group: 'tunnel',
		subgroup: 'test',
		action: 'http-check',
		target: 'awg10',
		message: 'Connectivity check failed: timeout',
		bucket: 'app',
		...over,
	};
}

describe('logs store: схлопнутые повторы', () => {
	beforeEach(() => appLogEntries.clear());

	it('append с repeats обновляет существующую строку вместо новой', () => {
		appLogEntries.append(ev());
		appLogEntries.append(ev({ repeats: 1, lastSeen: '2026-01-01T10:00:30Z' }));

		const entries = get(appLogEntries);
		expect(entries).toHaveLength(1);
		expect(entries[0].repeats).toBe(1);
		expect(entries[0].lastSeen).toBe('2026-01-01T10:00:30Z');
		expect(get(appLogEntries.total)).toBe(1);
	});

	it('append с repeats без базовой строки вставляет её как обычную', () => {
		appLogEntries.append(ev({ repeats: 3, lastSeen: '2026-01-01T10:02:00Z' }));
		const entries = get(appLogEntries);
		expect(entries).toHaveLength(1);
		expect(entries[0].repeats).toBe(3);
	});

	it('append без repeats добавляет новую строку как раньше', () => {
		appLogEntries.append(ev());
		appLogEntries.append(ev({ message: 'другое сообщение' }));
		expect(get(appLogEntries)).toHaveLength(2);
	});

	it('appendMany освежает счётчик у существующей строки при бОльшем repeats', () => {
		appLogEntries.append(ev());
		const rest: LogEntry[] = [
			{
				timestamp: '2026-01-01T10:00:00Z',
				level: 'warn',
				group: 'tunnel',
				subgroup: 'test',
				action: 'http-check',
				target: 'awg10',
				message: 'Connectivity check failed: timeout',
				repeats: 5,
				lastSeen: '2026-01-01T10:04:00Z',
			},
		];
		appLogEntries.appendMany(rest);
		const entries = get(appLogEntries);
		expect(entries).toHaveLength(1);
		expect(entries[0].repeats).toBe(5);
		expect(entries[0].lastSeen).toBe('2026-01-01T10:04:00Z');
	});

	it('appendMany не откатывает счётчик при меньшем repeats', () => {
		appLogEntries.append(ev({ repeats: 4, lastSeen: '2026-01-01T10:03:00Z' }));
		appLogEntries.appendMany([
			{
				timestamp: '2026-01-01T10:00:00Z',
				level: 'warn',
				group: 'tunnel',
				subgroup: 'test',
				action: 'http-check',
				target: 'awg10',
				message: 'Connectivity check failed: timeout',
				repeats: 2,
			},
		]);
		const entries = get(appLogEntries);
		expect(entries).toHaveLength(1);
		expect(entries[0].repeats).toBe(4);
	});
});
