<script lang="ts">
	interface Props {
		title: string;
		count?: number;
		countLabel?: string;
		/** Subsection inside a parent block — no outer wrapper, equal vertical rhythm. */
		nested?: boolean;
		/** Table row variant — spans full table width. */
		variant?: 'block' | 'table-row';
		colspan?: number;
	}

	let {
		title,
		count,
		countLabel,
		nested = false,
		variant = 'block',
		colspan = 6,
	}: Props = $props();

	const meta = $derived.by(() => {
		if (count == null) return '';
		const unit = countLabel ?? '';
		return unit ? `${count} ${unit}` : String(count);
	});
</script>

{#if variant === 'table-row'}
	<tr class="tunnel-dashboard-section-row">
		<td {colspan}>
			<span class="tunnel-dashboard-section-title">{title}</span>
			{#if meta}
				<span class="tunnel-dashboard-section-meta">· {meta}</span>
			{/if}
		</td>
	</tr>
{:else if nested}
	<h2 class="tunnel-dashboard-section-title tunnel-dashboard-section-title--nested">
		{title}
		{#if meta}
			<span class="tunnel-dashboard-section-meta">· {meta}</span>
		{/if}
	</h2>
{:else}
	<div class="tunnel-dashboard-section">
		<h2 class="tunnel-dashboard-section-title">
			{title}
			{#if meta}
				<span class="tunnel-dashboard-section-meta">· {meta}</span>
			{/if}
		</h2>
	</div>
{/if}

<style>
	.tunnel-dashboard-section {
		margin-block: 0.75rem;
	}

	.tunnel-dashboard-section-title {
		margin: 0;
		font-size: 0.75rem;
		font-weight: 700;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		color: var(--color-text-muted);
	}

	.tunnel-dashboard-section-title--nested {
		margin-block: 0.75rem;
	}

	.tunnel-dashboard-section-meta {
		font-weight: 600;
		color: var(--color-text-secondary);
		text-transform: none;
		letter-spacing: normal;
	}

	:global(.tunnel-dashboard-section-row td) {
		padding-block: 0.75rem;
		padding-inline: 0.75rem;
		border-top: none;
		border-bottom: none;
		background: transparent;
	}

	:global(tr:not(.tunnel-dashboard-section-row):has(+ tr.tunnel-dashboard-section-row) td) {
		border-bottom: none;
	}

	:global(.tunnel-dashboard-section-row .tunnel-dashboard-section-title) {
		font-size: 0.75rem;
		font-weight: 700;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		color: var(--color-text-muted);
	}
</style>
