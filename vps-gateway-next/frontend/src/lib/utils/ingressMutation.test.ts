import { describe, expect, it } from 'vitest';
import { createIngressMutationLock } from './ingressMutation';

describe('createIngressMutationLock', () => {
	it('runs mutations sequentially', async () => {
		const withLock = createIngressMutationLock();
		const order: number[] = [];

		const first = withLock(async () => {
			order.push(1);
			await new Promise((r) => setTimeout(r, 20));
			order.push(2);
		});

		const second = withLock(async () => {
			order.push(3);
		});

		await Promise.all([first, second]);
		expect(order).toEqual([1, 2, 3]);
	});

	it('continues chain after failure', async () => {
		const withLock = createIngressMutationLock();

		await expect(
			withLock(async () => {
				throw new Error('fail');
			}),
		).rejects.toThrow('fail');

		await expect(withLock(async () => 'ok')).resolves.toBe('ok');
	});
});
