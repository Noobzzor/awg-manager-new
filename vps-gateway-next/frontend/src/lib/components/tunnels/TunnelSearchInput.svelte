<script lang="ts">
	interface Props {
		value: string;
		onInput: (value: string) => void;
		placeholder?: string;
	}

	let { value, onInput, placeholder = 'Поиск...' }: Props = $props();
</script>

<input
	class="tunnel-search"
	type="text"
	{placeholder}
	{value}
	oninput={(e) => onInput((e.currentTarget as HTMLInputElement).value)}
/>

<style>
	/* Ширину задаёт обёртка потребителя — инпут растягивается на 100%. */
	.tunnel-search {
		width: 100%;
		min-width: 0;
		height: 32px;
		box-sizing: border-box;
		padding: 0 0.5rem;
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		background: var(--bg-primary);
		color: var(--text-primary);
		font-size: 0.6875rem;
	}

	.tunnel-search::placeholder {
		color: var(--text-muted);
	}
</style>
