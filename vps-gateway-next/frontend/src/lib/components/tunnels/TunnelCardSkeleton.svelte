<script lang="ts">
	// Скелет-плейсхолдер карточки туннеля (TunnelCard): шапка со статус-дотом,
	// строки метрик, футер. compact — уменьшенная геометрия компакт-режима.
	interface Props {
		compact?: boolean;
	}
	let { compact = false }: Props = $props();
</script>

<div class="card" class:compact aria-hidden="true">
	<div class="head">
		<span class="skeleton-circle dot"></span>
		<span class="skeleton bar" style="width: 45%"></span>
		<span class="skeleton bar" style="width: 3.5rem; margin-left: auto;"></span>
	</div>
	{#if !compact}
		<span class="skeleton bar" style="width: 72%"></span>
		<span class="skeleton bar" style="width: 55%"></span>
	{/if}
	<div class="foot">
		<span class="skeleton bar" style="width: 30%"></span>
		<span class="skeleton bar" style="width: 2rem"></span>
	</div>
</div>

<style>
	.card {
		display: flex;
		flex-direction: column;
		gap: 10px;
		border: 1px solid var(--color-border);
		border-radius: 12px;
		padding: 14px;
		min-height: 148px;
		cursor: default;
	}
	.card.compact {
		min-height: 96px;
		padding: 10px 12px;
		gap: 8px;
	}
	.head {
		display: flex;
		align-items: center;
		gap: 8px;
	}
	.dot {
		width: 10px;
		height: 10px;
		flex: none;
	}
	.bar {
		height: 0.75rem;
	}
	.foot {
		margin-top: auto;
		display: flex;
		justify-content: space-between;
	}
</style>
