// Живой агрегатный трафик движка sing-box (скорость + объём за сессию).
//
// Источник — singboxTrafficTotals: кумулятивные счётчики Clash за всю жизнь
// процесса (включая закрытые соединения), монотонные до рестарта движка.
// Per-tag карта singboxTraffic для агрегатов непригодна: она пересобирается
// из ОТКРЫТЫХ соединений (суммы падают при каждом закрытии) и считает каждое
// звено chain'а по разу.
//
// readable с closure-состоянием, а не module-scope prev: снимок «прошлого
// тика» живёт ровно столько, сколько есть подписчики. После отписки всех
// (уход со вкладки) и повторной подписки первый пересчёт честно отдаёт
// hasRate:false, а не среднюю скорость за всё время отсутствия.
import { readable } from 'svelte/store';
import { singboxTrafficTotals, type SingboxTrafficTotals } from './singbox';
import {
	computeRate,
	type RateSnapshot,
	type TrafficRate,
} from '$lib/utils/singboxTrafficRate';

export interface SingboxTrafficLive {
	totals: SingboxTrafficTotals;
	rate: TrafficRate;
}

const IDLE: SingboxTrafficLive = {
	totals: { downloadBytes: 0, uploadBytes: 0 },
	rate: { downloadRate: 0, uploadRate: 0, hasRate: false },
};

// SSE публикует totals каждые ~2 с; дельта старше нескольких тиков означает
// разрыв потока (движок умер, вкладка спала) — скорость по такому интервалу
// была бы «средним за простой», а не текущим значением.
const MAX_RATE_INTERVAL_MS = 10_000;

export const singboxTrafficLive = readable<SingboxTrafficLive>(IDLE, (set) => {
	let prev: RateSnapshot | null = null;
	const unsubscribe = singboxTrafficTotals.subscribe((totals) => {
		const next: RateSnapshot = {
			timestamp: Date.now(),
			downloadBytes: totals.downloadBytes,
			uploadBytes: totals.uploadBytes,
		};
		let rate = computeRate(prev, next);
		if (rate.hasRate && prev && next.timestamp - prev.timestamp > MAX_RATE_INTERVAL_MS) {
			rate = { downloadRate: 0, uploadRate: 0, hasRate: false };
		}
		prev = next;
		set({ totals, rate });
	});
	return () => {
		unsubscribe();
	};
});
