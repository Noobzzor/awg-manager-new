<!-- frontend/src/lib/components/routing/singboxRouter/ConnectionsBreakdown.svelte -->
<script lang="ts">
	import type { ConnectionBucket, ConnectionFilters } from '$lib/types/singboxConnections';
	import ConnectionsBreakdownPanel from './ConnectionsBreakdownPanel.svelte';

	interface Props {
		byOutbound: ConnectionBucket[];
		byHost: ConnectionBucket[];
		byClient: ConnectionBucket[];
		filters: ConnectionFilters;
		onFilterToggle: (kind: 'outbound' | 'host' | 'client', key: string) => void;
	}

	let { byOutbound, byHost, byClient, filters, onFilterToggle }: Props = $props();
</script>

<div class="grid">
	<ConnectionsBreakdownPanel
		title="By Outbound"
		buckets={byOutbound}
		activeKey={filters.outbound}
		onSelect={(k) => onFilterToggle('outbound', k)}
	/>
	<ConnectionsBreakdownPanel
		title="By Host"
		buckets={byHost}
		activeKey={filters.search}
		onSelect={(k) => onFilterToggle('host', k)}
	/>
	<ConnectionsBreakdownPanel
		title="By Client"
		buckets={byClient}
		activeKey={filters.search}
		onSelect={(k) => onFilterToggle('client', k)}
	/>
</div>

<style>
	.grid {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 12px;
		margin-bottom: 16px;
		align-items: stretch;
	}
	@media (max-width: 900px) {
		.grid { grid-template-columns: 1fr; }
	}
</style>
