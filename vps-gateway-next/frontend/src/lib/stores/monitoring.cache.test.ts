// @vitest-environment jsdom
import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { monitoringStore as monitoring } from './monitoring';
import type { MonitoringSnapshot } from '$lib/types';

const CACHE_KEY = 'awgm_monitoring_snapshot_v1';

const validSnapshot: MonitoringSnapshot = {
	targets: [{ id: 't1', host: '1.1.1.1', name: 'Cloudflare' }],
	tunnels: [
		{
			id: 'tun1',
			name: 'Home',
			ifaceName: 'nwg0',
			pingcheckTarget: '1.1.1.1',
			selfTarget: '10.0.0.1',
			selfMethod: 'ping',
		},
	],
	cells: [
		{
			targetId: 't1',
			tunnelId: 'tun1',
			latencyMs: 12,
			ok: true,
			activeForRestart: false,
			isSelf: false,
			ts: '2026-07-08T10:00:00Z',
		},
	],
	updatedAt: '2026-07-08T10:00:00Z',
};

beforeEach(() => {
	localStorage.clear();
	// Сбрасываем состояние стора между тестами (loadCached — единственный вход).
	monitoring.reset();
});

describe('monitoring.loadCached — недоверенный localStorage', () => {
	it('валидный кэш загружается как stale-снапшот', () => {
		localStorage.setItem(CACHE_KEY, JSON.stringify(validSnapshot));
		monitoring.loadCached();
		const s = get(monitoring);
		expect(s.snapshot?.targets).toHaveLength(1);
		expect(s.stale).toBe(true);
	});

	it('кэш от старой версии с неверной формой молча игнорируется', () => {
		// cells: объект вместо массива — форма из гипотетической прошлой версии
		localStorage.setItem(
			CACHE_KEY,
			JSON.stringify({ ...validSnapshot, cells: { legacy: true } }),
		);
		monitoring.loadCached();
		expect(get(monitoring).snapshot).toBeNull();
	});

	it('запись с неверным типом поля внутри массива игнорируется', () => {
		const bad = structuredClone(validSnapshot) as unknown as {
			cells: Array<Record<string, unknown>>;
		};
		bad.cells[0].latencyMs = 'fast'; // должен быть number | null
		localStorage.setItem(CACHE_KEY, JSON.stringify(bad));
		monitoring.loadCached();
		expect(get(monitoring).snapshot).toBeNull();
	});

	it('лишние поля (эволюция формата вперёд) не мешают загрузке', () => {
		localStorage.setItem(
			CACHE_KEY,
			JSON.stringify({ ...validSnapshot, futureField: { anything: 1 } }),
		);
		monitoring.loadCached();
		expect(get(monitoring).snapshot).not.toBeNull();
	});

	it('не-JSON мусор игнорируется без исключений', () => {
		localStorage.setItem(CACHE_KEY, '{broken json');
		expect(() => monitoring.loadCached()).not.toThrow();
		expect(get(monitoring).snapshot).toBeNull();
	});

	it('null-слайсы (Go nil без omitempty, пустой роутер) валидны и нормализуются в []', () => {
		// Бэкенд-DTO маршалит nil-слайсы в null: {"targets":null,...} — это
		// легитимный кэш текущей версии, а не мусор.
		localStorage.setItem(
			CACHE_KEY,
			JSON.stringify({ targets: null, tunnels: null, cells: null, updatedAt: '2026-07-08T10:00:00Z' }),
		);
		monitoring.loadCached();
		const s = get(monitoring);
		expect(s.snapshot).not.toBeNull();
		expect(s.snapshot?.targets).toEqual([]);
		expect(s.snapshot?.tunnels).toEqual([]);
		expect(s.snapshot?.cells).toEqual([]);
		expect(s.stale).toBe(true);
	});
});
