<script lang="ts">
	import { latencyTier } from '$lib/utils/latencyTier';

	interface Props {
		/** Latency datapoints in ms, oldest → newest. */
		history: number[];
		width?: number;
		height?: number;
	}

	let { history, width = 60, height = 16 }: Props = $props();

	const strokeColor = $derived.by(() => {
		const last = history.length > 0 ? history[history.length - 1] : 0;
		switch (latencyTier(last)) {
			case 'success': return 'var(--color-success)';
			case 'warning': return 'var(--color-warning)';
			case 'error':   return 'var(--color-error)';
			default:        return 'var(--color-text-muted)';
		}
	});

	// Build polyline points scaled to the box. Treats 0 (timeout) as the
	// max value visually so it still draws inside the chart.
	const points = $derived.by(() => {
		const n = history.length;
		if (n < 2) return '';
		const positiveValues = history.filter((v) => v > 0);
		if (positiveValues.length === 0) return '';
		const maxRaw = Math.max(...positiveValues, 1);
		const min = Math.min(...positiveValues, maxRaw);
		const range = Math.max(1, maxRaw - min);
		const stepX = width / Math.max(1, n - 1);
		return history
			.map((v, i) => {
				const display = v === 0 ? maxRaw : v;
				const y = height - 1 - ((display - min) / range) * (height - 2);
				return `${(i * stepX).toFixed(1)},${y.toFixed(1)}`;
			})
			.join(' ');
	});
</script>

{#if history.length >= 2}
	<svg viewBox="0 0 {width} {height}" {width} {height} class="sparkline" aria-hidden="true">
		<polyline {points} fill="none" stroke={strokeColor} stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round" />
	</svg>
{/if}

<style>
	.sparkline {
		display: inline-block;
		vertical-align: middle;
	}
</style>
