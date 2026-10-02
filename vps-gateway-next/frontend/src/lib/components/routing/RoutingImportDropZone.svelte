<script lang="ts">
	import { Upload } from 'lucide-svelte';

	interface Props {
		/** Чем наполнен файл — подставляется в стандартное описание
		 * «Файл .json с {subject}, экспортированными ранее из AWG Manager». */
		subject?: string;
		/** Полностью заменяет стандартное описание (когда файл — не экспорт
		 * AWG Manager). Пустая строка при пустом subject скрывает описание. */
		description?: string;
		/** Заголовок дроп-зоны. */
		dropTitle?: string;
		/** Значение accept для file input, по умолчанию только .json. */
		accept?: string;
		parseError?: string;
		onfile: (file: File) => void;
	}

	let {
		subject = '',
		description = '',
		dropTitle = 'Перетащите .json файл сюда',
		accept = '.json',
		parseError = '',
		onfile,
	}: Props = $props();

	let dragging = $state(false);
	let fileInput = $state<HTMLInputElement>(null!);

	function processFile(file: File) {
		onfile(file);
	}

	function handleFile(e: Event) {
		const file = (e.target as HTMLInputElement).files?.[0];
		if (file) processFile(file);
	}

	function handleDrop(e: DragEvent) {
		e.preventDefault();
		dragging = false;
		const file = e.dataTransfer?.files?.[0];
		if (file) processFile(file);
	}
</script>

<div class="import-upload">
	{#if description}
		<p class="import-description">{description}</p>
	{:else if subject}
		<p class="import-description">
			Файл <b class="import-accent">.json</b> с {subject}, экспортированными ранее из AWG Manager.
		</p>
	{/if}
	<div
		class="drop-zone"
		class:dragging
		role="button"
		tabindex="0"
		onclick={() => fileInput.click()}
		onkeydown={(e) => {
			if (e.key === 'Enter' || e.key === ' ') fileInput.click();
		}}
		ondrop={handleDrop}
		ondragover={(e) => {
			e.preventDefault();
			dragging = true;
		}}
		ondragleave={() => {
			dragging = false;
		}}
	>
		<Upload size={24} class="drop-icon" strokeWidth={1.5} aria-hidden="true" />
		<p class="drop-title">
			{dropTitle}<br />
			<span class="drop-hint">или нажмите для выбора</span>
		</p>
	</div>
	<input type="file" {accept} onchange={handleFile} bind:this={fileInput} class="hidden-input" />
	{#if parseError}
		<p class="import-error">{parseError}</p>
	{/if}
</div>
