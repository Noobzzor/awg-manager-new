import { describe, it, expect } from 'vitest';
import { stripAnsi } from './ansi';

describe('stripAnsi', () => {
	it('returns empty string for null/undefined/empty', () => {
		expect(stripAnsi(null)).toBe('');
		expect(stripAnsi(undefined)).toBe('');
		expect(stripAnsi('')).toBe('');
	});

	it('passes through plain text unchanged', () => {
		expect(stripAnsi('hello world')).toBe('hello world');
	});

	it('strips SGR colour codes', () => {
		expect(stripAnsi('\x1b[36mINFO\x1b[0m message')).toBe('INFO message');
		expect(stripAnsi('\x1b[31mFATAL\x1b[0m boom')).toBe('FATAL boom');
	});

	it('strips a sing-box-shaped log line with timestamp', () => {
		const raw = '+0000 2026-05-11 16:32:25 \x1b[36mINFO\x1b[0m network: updated default interface ppp0';
		expect(stripAnsi(raw)).toBe('+0000 2026-05-11 16:32:25 INFO network: updated default interface ppp0');
	});

	it('strips the pre-init logrus FATAL shape', () => {
		const raw = '\x1b[31mFATAL\x1b[0m[0000] start service: default outbound not found: direct';
		expect(stripAnsi(raw)).toBe('FATAL[0000] start service: default outbound not found: direct');
	});

	it('strips non-SGR CSI sequences (e.g. cursor moves)', () => {
		expect(stripAnsi('\x1b[2Acursor up')).toBe('cursor up');
		expect(stripAnsi('\x1b[H\x1b[2Jcleared')).toBe('cleared');
	});

	it('preserves multi-line content', () => {
		const raw = '\x1b[36mINFO\x1b[0m a\n\x1b[37mTRACE\x1b[0m b\n\x1b[31mFATAL\x1b[0m c';
		expect(stripAnsi(raw)).toBe('INFO a\nTRACE b\nFATAL c');
	});
});
