import { describe, expect, it, vi } from 'vitest';
import { render, fireEvent, screen } from '@testing-library/svelte';
import BulkSelectBar from './BulkSelectBar.svelte';

describe('BulkSelectBar', () => {
	it('disables Apply at value="" by default (outbound bars have no empty option)', () => {
		render(BulkSelectBar, {
			props: {
				count: 2,
				options: [
					{ value: '', label: '— сбросить —' },
					{ value: 'direct', label: 'direct' },
				],
				onapply: vi.fn(),
				oncancel: vi.fn(),
			},
		});

		const applyBtn = screen.getByText('Применить').closest('button') as HTMLButtonElement;
		expect(applyBtn.disabled).toBe(true);
	});

	it('allowEmpty=true: Apply is enabled at value="" when count>0, and calls onapply("")', async () => {
		const onapply = vi.fn();
		render(BulkSelectBar, {
			props: {
				count: 2,
				options: [
					{ value: '', label: '— сбросить —' },
					{ value: 'direct', label: 'direct' },
				],
				onapply,
				oncancel: vi.fn(),
				allowEmpty: true,
			},
		});

		const applyBtn = screen.getByText('Применить').closest('button') as HTMLButtonElement;
		expect(applyBtn.disabled).toBe(false);

		await fireEvent.click(applyBtn);
		expect(onapply).toHaveBeenCalledWith('');
	});

	it('allowEmpty=true but count=0: Apply stays disabled', () => {
		render(BulkSelectBar, {
			props: {
				count: 0,
				options: [{ value: '', label: '— сбросить —' }],
				onapply: vi.fn(),
				oncancel: vi.fn(),
				allowEmpty: true,
			},
		});

		const applyBtn = screen.getByText('Применить').closest('button') as HTMLButtonElement;
		expect(applyBtn.disabled).toBe(true);
	});
});
