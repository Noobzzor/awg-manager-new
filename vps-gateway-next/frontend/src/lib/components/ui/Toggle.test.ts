import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent } from '@testing-library/svelte';
import Toggle from './Toggle.svelte';

describe('Toggle', () => {
	it('controlled: click fires onchange but reverts DOM to the prop (parent owns state)', async () => {
		const onchange = vi.fn();
		const { container } = render(Toggle, {
			props: { checked: false, controlled: true, onchange },
		});
		const input = container.querySelector('input[type="checkbox"]') as HTMLInputElement;
		expect(input.checked).toBe(false);

		await fireEvent.click(input);

		// Parent is notified with the intended value...
		expect(onchange).toHaveBeenCalledWith(true);
		// ...but since the parent did NOT change `checked`, the toggle reverts
		// (no optimistic self-commit). This is what lets a cancelled confirm
		// modal leave the toggle in its original state.
		expect(input.checked).toBe(false);
	});

	it('default (uncontrolled): click commits checked optimistically', async () => {
		const onchange = vi.fn();
		const { container } = render(Toggle, {
			props: { checked: false, onchange },
		});
		const input = container.querySelector('input[type="checkbox"]') as HTMLInputElement;

		await fireEvent.click(input);

		expect(onchange).toHaveBeenCalledWith(true);
		expect(input.checked).toBe(true);
	});
});
