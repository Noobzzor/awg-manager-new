import { fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api, type GatewayClient, type GatewayGroup } from '$lib/api/client';
import Page from '../../../routes/gateway/+page.svelte';

const client: GatewayClient = {
	id: 'phone-1',
	label: 'Тестовый телефон',
	address: '10.66.0.2',
	publicKey: 'public-test-key',
	status: 'active',
	createdAt: '2026-09-23T00:00:00Z',
	updatedAt: '2026-09-23T00:00:00Z',
};

const capabilities = (gateway: boolean) => ({
	singboxStatus: false,
	awg3: false,
	singboxConfigEditor: false,
	subscriptions: false,
	inbounds: false,
	dnsRoutes: false,
	dnsRouteBackend: null,
	proxyInbound: false,
	logs: false,
	ndms: false,
	gateway,
});

afterEach(() => vi.restoreAllMocks());

describe('VPS Gateway page', () => {
	it('generates a QR in memory from the selected client profile', async () => {
		vi.spyOn(api, 'getCapabilities').mockResolvedValue(capabilities(true));
		vi.spyOn(api, 'getGatewayClients').mockResolvedValue([client]);
		vi.spyOn(api, 'getGatewaySubscriptions').mockResolvedValue([]);
		vi.spyOn(api, 'getGatewayGroups').mockResolvedValue([]);
		vi.spyOn(api, 'getGatewayClientConfig').mockResolvedValue('fixture profile text');

		render(Page);
		await fireEvent.click(await screen.findByRole('button', { name: 'Профиль / QR' }));

		const qr = await screen.findByRole('img', { name: 'QR-код конфигурации для Тестовый телефон' });
		expect(qr.getAttribute('src')).toMatch(/^data:image\/png;base64,/);
		expect(api.getGatewayClientConfig).toHaveBeenCalledWith('phone-1');
		expect(screen.getByRole('button', { name: 'Скачать .conf' })).toBeTruthy();
	});

	it('creates and displays a one-time subscription URL', async () => {
		const issue = {
			subscription: {
				id: 'subscription-1',
				clientId: 'phone-1',
				createdAt: '2026-09-23T00:00:00Z',
				expiresAt: '2026-10-23T00:00:00Z',
			},
			url: 'https://gateway.example.com/s/test-capability-token',
		};
		vi.spyOn(api, 'getCapabilities').mockResolvedValue(capabilities(true));
		vi.spyOn(api, 'getGatewayClients').mockResolvedValue([client]);
		vi.spyOn(api, 'getGatewaySubscriptions').mockResolvedValue([]);
		vi.spyOn(api, 'getGatewayGroups').mockResolvedValue([]);
		vi.spyOn(api, 'createGatewaySubscription').mockResolvedValue(issue);

		render(Page);
		await fireEvent.click(await screen.findByRole('button', { name: 'Создать ссылку' }));

		const url = await screen.findByLabelText('Ссылка для подключения');
		expect((url as HTMLInputElement).value).toBe(issue.url);
		expect(screen.getByText(/Секретная ссылка показывается только при создании/)).toBeTruthy();
		expect(api.createGatewaySubscription).toHaveBeenCalledWith('phone-1', 30);
	});

	it('shows a disabled-state message when the runtime capability is absent', async () => {
		vi.spyOn(api, 'getCapabilities').mockResolvedValue(capabilities(false));
		vi.spyOn(api, 'getGatewayClients').mockResolvedValue([]);

		render(Page);

		expect(await screen.findByRole('heading', { name: 'Gateway выключен' })).toBeTruthy();
		expect(api.getGatewayClients).not.toHaveBeenCalled();
	});

	it('creates, edits, and deletes a client group', async () => {
		const group: GatewayGroup = {
			id: 'group-1',
			name: 'Семья',
			clientIds: ['phone-1'],
			createdAt: '2026-09-23T00:00:00Z',
			updatedAt: '2026-09-23T00:00:00Z',
		};
		let storedGroups: GatewayGroup[] = [];
		vi.spyOn(api, 'getCapabilities').mockResolvedValue(capabilities(true));
		vi.spyOn(api, 'getGatewayClients').mockResolvedValue([client]);
		vi.spyOn(api, 'getGatewaySubscriptions').mockResolvedValue([]);
		vi.spyOn(api, 'getGatewayGroups').mockImplementation(async () => storedGroups.map((item) => ({ ...item, clientIds: [...item.clientIds] })));
		vi.spyOn(api, 'createGatewayGroup').mockImplementation(async (name, clientIds) => {
			const created = { ...group, name, clientIds };
			storedGroups = [...storedGroups, created];
			return created;
		});
		const update = vi.spyOn(api, 'updateGatewayGroup').mockImplementation(async (id, name, clientIds) => {
			const updated = { ...group, id, name, clientIds };
			storedGroups = storedGroups.map((item) => item.id === id ? updated : item);
			return updated;
		});
		const remove = vi.spyOn(api, 'deleteGatewayGroup').mockImplementation(async (id) => {
			storedGroups = storedGroups.filter((item) => item.id !== id);
			return { deleted: true };
		});

		render(Page);
		const heading = await screen.findByRole('heading', { name: /Группы клиентов/ });
		expect(heading).toBeTruthy();
		await screen.findByText('Групп пока нет.');
		await fireEvent.input(screen.getByLabelText('Название новой группы'), { target: { value: 'Семья' } });
		await fireEvent.click(screen.getByRole('checkbox', { name: 'Добавить Тестовый телефон в новую группу' }));
		await fireEvent.click(screen.getByRole('button', { name: 'Создать группу' }));
		await screen.findByRole('button', { name: 'Изменить группу Семья' });
		expect(api.createGatewayGroup).toHaveBeenCalledWith('Семья', ['phone-1']);
		await fireEvent.click(screen.getByRole('button', { name: 'Изменить группу Семья' }));
		const name = screen.getByLabelText('Название группы') as HTMLInputElement;
		await fireEvent.input(name, { target: { value: 'Дом' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Сохранить группу' }));
		expect(update).toHaveBeenCalledWith('group-1', 'Дом', ['phone-1']);
		await fireEvent.click(await screen.findByRole('button', { name: 'Удалить группу Дом' }));
		expect(remove).toHaveBeenCalledWith('group-1');
		expect(await screen.findByText('Групп пока нет.')).toBeTruthy();
	});
});
