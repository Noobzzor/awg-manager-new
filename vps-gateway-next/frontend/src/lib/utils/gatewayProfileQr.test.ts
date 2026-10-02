import { describe, expect, it } from 'vitest';
import { createGatewayProfileQr } from './gatewayProfileQr';

describe('createGatewayProfileQr', () => {
	it('encodes profile text into a PNG data URL', async () => {
		const image = await createGatewayProfileQr('[Interface]\nPrivateKey = test-only\n');
		expect(image).toMatch(/^data:image\/png;base64,/);
		const bytes = Buffer.from(image.split(',')[1], 'base64');
		expect(bytes.subarray(0, 8)).toEqual(Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]));
	});

	it('rejects an empty profile rather than creating a misleading QR', async () => {
		await expect(createGatewayProfileQr('  \n')).rejects.toThrow('Профиль пуст');
	});
});
