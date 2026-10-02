<script lang="ts">
	import { Stat, StatStrip } from '$lib/components/ui';

	interface Props {
		stats: Array<{ value: string; label: string; sub?: string }>;
	}

	let { stats }: Props = $props();
</script>

{#if stats.length > 0}
	<div class="dashboard-summary">
		<StatStrip>
			{#each stats as stat (stat.label)}
				<Stat value={stat.value} label={stat.label} sub={stat.sub} />
			{/each}
		</StatStrip>
	</div>
{/if}

<style>
	.dashboard-summary {
		min-width: 0;
	}
</style>
