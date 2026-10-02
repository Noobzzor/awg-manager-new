<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Button, type ButtonSize } from '$lib/components/ui';

	interface Props {
		label: string;
		size?: ButtonSize;
		disabled?: boolean;
		iconBefore?: Snippet;
		/** Тело меню; получает close(), чтобы пункт закрыл меню до вызова действия. */
		menu: Snippet<[() => void]>;
	}

	let { label, size = 'sm', disabled = false, iconBefore, menu }: Props = $props();

	let open = $state(false);
	let rootEl = $state<HTMLDivElement | null>(null);

	function close(): void {
		open = false;
	}

	function toggle(event: MouseEvent): void {
		// stopPropagation: иначе открывающий клик дойдёт до document и сразу закроет меню.
		event.stopPropagation();
		open = !open;
	}

	// Слушатели живут только пока меню открыто.
	$effect(() => {
		if (!open) return;

		function handleDocumentClick(event: MouseEvent): void {
			if (!rootEl?.contains(event.target as Node)) close();
		}

		function handleDocumentKeydown(event: KeyboardEvent): void {
			if (event.key === 'Escape') close();
		}

		document.addEventListener('click', handleDocumentClick);
		document.addEventListener('keydown', handleDocumentKeydown);
		return () => {
			document.removeEventListener('click', handleDocumentClick);
			document.removeEventListener('keydown', handleDocumentKeydown);
		};
	});
</script>

<div class="dropdown-wrapper" bind:this={rootEl} aria-haspopup="menu" aria-expanded={open}>
	<Button variant="primary" {size} {disabled} onclick={toggle} {iconBefore}>
		{label}
		{#snippet iconAfter()}
			<svg width="10" height="10" viewBox="0 0 10 10" fill="currentColor" aria-hidden="true">
				<path d="M2 4l3 3 3-3" />
			</svg>
		{/snippet}
	</Button>

	{#if open}
		<div class="dropdown-menu" role="menu">
			{@render menu(close)}
		</div>
	{/if}
</div>

<style>
	.dropdown-wrapper {
		position: relative;
		display: inline-block;
	}

	.dropdown-menu {
		position: absolute;
		top: calc(100% + 4px);
		right: 0;
		z-index: var(--dropdown-menu-z-index, 10);
		min-width: var(--dropdown-menu-min-width, 210px);
		padding: 4px;
		border: 1px solid var(--border);
		border-radius: var(--dropdown-menu-radius, 8px);
		background: var(--bg-secondary, var(--bg-card, #1a1b2e));
		box-shadow: var(--dropdown-menu-shadow, 0 8px 24px rgba(0, 0, 0, 0.4));
	}

	@media (max-width: 640px) {
		.dropdown-wrapper {
			display: block;
			width: 100%;
		}

		.dropdown-wrapper :global(.btn) {
			width: 100%;
			justify-content: center;
		}
	}
</style>
