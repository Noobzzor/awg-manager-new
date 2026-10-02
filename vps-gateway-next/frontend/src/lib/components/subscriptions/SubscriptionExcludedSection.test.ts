import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent } from '@testing-library/svelte';
import SubscriptionExcludedSection from './SubscriptionExcludedSection.svelte';
import type { SubscriptionMember } from '$lib/types';

const members: SubscriptionMember[] = [
	{ tag: 'tag-alpha-1234', label: 'Alpha NL', protocol: 'vless', server: 'nl.example.com', port: 443 },
	{ tag: 'tag-bravo-5678', label: 'Bravo DE', protocol: 'trojan', server: 'de.example.com', port: 8443 },
];

function cards(container: HTMLElement): HTMLElement[] {
	return Array.from(container.querySelectorAll('.excluded-card'));
}
function cardCheckboxes(container: HTMLElement): HTMLInputElement[] {
	return Array.from(container.querySelectorAll('.excluded-sel input[type="checkbox"]'));
}

describe('SubscriptionExcludedSection', () => {
	it('renders nothing when members is empty', () => {
		const { container } = render(SubscriptionExcludedSection, {
			props: { members: [], restoring: false, onrestore: vi.fn() },
		});
		expect(container.querySelector('.excluded')).toBeNull();
	});

	it('non-empty → hint + a card per member', () => {
		const { container, getByText } = render(SubscriptionExcludedSection, {
			props: { members, restoring: false, onrestore: vi.fn() },
		});
		expect(getByText(/исключили вручную/i)).toBeTruthy();
		expect(cards(container)).toHaveLength(2);
		expect(container.querySelector('.excluded-toggle')).toBeNull();
	});

	it('per-card «Вернуть» calls onrestore([member.tag])', async () => {
		const onrestore = vi.fn();
		const { container } = render(SubscriptionExcludedSection, {
			props: { members, restoring: false, onrestore },
		});
		const buttons = Array.from(container.querySelectorAll('.excluded-card')).map((c) =>
			c.querySelector('button'),
		) as HTMLButtonElement[];
		await fireEvent.click(buttons[1]); // bravo
		expect(onrestore).toHaveBeenCalledTimes(1);
		expect(onrestore).toHaveBeenCalledWith(['tag-bravo-5678']);
	});

	it('batch: select cards then «Вернуть выбранные» calls onrestore with selected tags', async () => {
		const onrestore = vi.fn();
		const { container, getByText } = render(SubscriptionExcludedSection, {
			props: { members, restoring: false, onrestore },
		});
		// no batch button until something selected
		expect(() => getByText(/Вернуть выбранные/)).toThrow();

		const boxes = cardCheckboxes(container);
		await fireEvent.click(boxes[0]); // alpha
		await fireEvent.click(boxes[1]); // bravo

		const batchBtn = getByText(/Вернуть выбранные \(2\)/);
		await fireEvent.click(batchBtn);

		expect(onrestore).toHaveBeenCalledTimes(1);
		expect(onrestore).toHaveBeenCalledWith(['tag-alpha-1234', 'tag-bravo-5678']);
	});
});
