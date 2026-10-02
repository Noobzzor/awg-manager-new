import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { get } from 'svelte/store';
import { singboxTrafficTotals, applyTrafficTotals } from './singbox';
import { singboxTrafficLive, type SingboxTrafficLive } from './singboxEngineStats';

describe('singboxTrafficLive', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		singboxTrafficTotals.set({ downloadBytes: 0, uploadBytes: 0 });
	});
	afterEach(() => {
		vi.useRealTimers();
	});

	it('первый снимок после подписки — объёмы есть, скорости нет', () => {
		applyTrafficTotals({ downloadTotal: 1000, uploadTotal: 100 });
		const live = get(singboxTrafficLive);
		expect(live.totals.downloadBytes).toBe(1000);
		expect(live.totals.uploadBytes).toBe(100);
		expect(live.rate.hasRate).toBe(false);
	});

	it('второй снимок через 2с — скорость = дельта/время', () => {
		let live: SingboxTrafficLive | undefined;
		const unsub = singboxTrafficLive.subscribe((v) => (live = v));
		vi.advanceTimersByTime(2000);
		applyTrafficTotals({ downloadTotal: 2000, uploadTotal: 200 });
		expect(live!.rate.hasRate).toBe(true);
		expect(live!.rate.downloadRate).toBe(1000);
		expect(live!.rate.uploadRate).toBe(100);
		unsub();
	});

	it('сброс счётчиков (рестарт движка) не даёт отрицательную скорость', () => {
		singboxTrafficTotals.set({ downloadBytes: 5000, uploadBytes: 500 });
		let live: SingboxTrafficLive | undefined;
		const unsub = singboxTrafficLive.subscribe((v) => (live = v));
		vi.advanceTimersByTime(2000);
		applyTrafficTotals({ downloadTotal: 20, uploadTotal: 10 });
		expect(live!.rate.hasRate).toBe(false);
		expect(live!.rate.downloadRate).toBe(0);
		unsub();
	});

	it('после отписки всех снимок сбрасывается — новая подписка не даёт «среднее за простой»', () => {
		const unsub1 = singboxTrafficLive.subscribe(() => {});
		vi.advanceTimersByTime(2000);
		applyTrafficTotals({ downloadTotal: 1000, uploadTotal: 100 });
		unsub1();

		// Пока никто не подписан, счётчики растут (SSE продолжает работать).
		vi.advanceTimersByTime(600_000);
		applyTrafficTotals({ downloadTotal: 500_000_000, uploadTotal: 100 });

		let live: SingboxTrafficLive | undefined;
		const unsub2 = singboxTrafficLive.subscribe((v) => (live = v));
		expect(live!.rate.hasRate).toBe(false);
		expect(live!.totals.downloadBytes).toBe(500_000_000);
		unsub2();
	});

	it('разрыв потока дольше MAX_RATE_INTERVAL — скорость не считается по гигантскому окну', () => {
		let live: SingboxTrafficLive | undefined;
		const unsub = singboxTrafficLive.subscribe((v) => (live = v));
		applyTrafficTotals({ downloadTotal: 1000, uploadTotal: 0 });
		// SSE замолчал на минуту (движок умер и перезапустился с бОльшими totals).
		vi.advanceTimersByTime(60_000);
		applyTrafficTotals({ downloadTotal: 600_000_000, uploadTotal: 0 });
		expect(live!.rate.hasRate).toBe(false);
		unsub();
	});
});
