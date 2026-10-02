<!-- frontend/src/lib/components/connections/ConnectionsBreakdown.svelte -->
<script lang="ts">
	import ConnectionsBreakdownPanel, { type PanelBucket } from './ConnectionsBreakdownPanel.svelte';

	interface Props {
		byTunnel: PanelBucket[];
		byDst: PanelBucket[];
		byClient: PanelBucket[];
		activeTunnelKey: string;
		activeSearch: string;
		onTunnelToggle: (key: string) => void;
		onSearchToggle: (key: string) => void;
	}

	let { byTunnel, byDst, byClient, activeTunnelKey, activeSearch, onTunnelToggle, onSearchToggle }: Props =
		$props();
</script>

<div class="grid">
	<ConnectionsBreakdownPanel title="По туннелям" buckets={byTunnel} activeKey={activeTunnelKey} onSelect={onTunnelToggle} />
	<ConnectionsBreakdownPanel title="По назначению" buckets={byDst} activeKey={activeSearch} onSelect={onSearchToggle} />
	<ConnectionsBreakdownPanel title="По клиентам" buckets={byClient} activeKey={activeSearch} onSelect={onSearchToggle} />
</div>

<style>
	.grid {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 12px;
		margin-bottom: 12px;
		align-items: stretch;
	}
	@media (max-width: 900px) {
		.grid { grid-template-columns: 1fr; }
	}
</style>
