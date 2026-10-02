import { render, screen, fireEvent, waitFor, within } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import GatewayPage from './+page.svelte';
import { api } from '$lib/api/client';

vi.mock('$lib/api/client', () => ({
	api: {
		getCapabilities: vi.fn(),
		getGatewayClients: vi.fn(),
		getGatewaySubscriptions: vi.fn(),
		getGatewayGroups: vi.fn(),
		awg3List: vi.fn(),
		getGatewayPolicyProfiles: vi.fn(),
		createGatewayPolicyProfile: vi.fn(),
		updateGatewayPolicyProfile: vi.fn(),
		previewGatewayPolicyProfile: vi.fn(),
		applyGatewayPolicyProfile: vi.fn(),
		deleteGatewayPolicyProfile: vi.fn(),
	},
}));

const mockedApi = vi.mocked(api);

beforeEach(() => {
	vi.clearAllMocks();
	mockedApi.getCapabilities.mockResolvedValue({ gateway: true } as never);
	mockedApi.getGatewayClients.mockResolvedValue([]);
	mockedApi.getGatewaySubscriptions.mockResolvedValue([]);
	mockedApi.getGatewayGroups.mockResolvedValue([]);
	mockedApi.awg3List.mockResolvedValue([]);
	mockedApi.getGatewayPolicyProfiles.mockResolvedValue([]);
});

describe('Gateway policy profiles', () => {
	it('creates, lists, applies, and deletes a default-action profile', async () => {
		const profile = { id: 'home', name: 'Home', defaultAction: 'direct' as const };
		mockedApi.createGatewayPolicyProfile.mockResolvedValue(profile);
		mockedApi.getGatewayPolicyProfiles
			.mockResolvedValueOnce([])
			.mockResolvedValueOnce([profile])
			.mockResolvedValueOnce([]);

		render(GatewayPage);

		const idField = await screen.findByLabelText('ID профиля');
		await fireEvent.input(idField, { target: { value: 'home' } });
		await fireEvent.input(screen.getByLabelText('Название профиля'), { target: { value: 'Home' } });
		await fireEvent.change(screen.getByLabelText('Действие по умолчанию'), { target: { value: 'direct' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Создать профиль' }));

		await screen.findByText('Home');
		expect(mockedApi.createGatewayPolicyProfile).toHaveBeenCalledWith(profile);
		expect(mockedApi.applyGatewayPolicyProfile).not.toHaveBeenCalled();

		await fireEvent.click(screen.getByRole('button', { name: 'Применить Home' }));
		await waitFor(() => expect(mockedApi.applyGatewayPolicyProfile).toHaveBeenCalledWith('home'));
		expect((await screen.findByRole('status')).textContent).toContain('Профиль «Home» применён.');

		await fireEvent.click(screen.getByRole('button', { name: 'Удалить Home' }));
		await waitFor(() => expect(mockedApi.deleteGatewayPolicyProfile).toHaveBeenCalledWith('home'));
	});

	it('disables WARP choices until a WARP outbound is configured', async () => {
		const profile = { id: 'home', name: 'Home', defaultAction: 'direct' as const, rules: [] };
		mockedApi.getGatewayPolicyProfiles.mockResolvedValueOnce([profile]);

		render(GatewayPage);
		await screen.findByText('Home');

		const defaultWarpOption = screen
			.getByLabelText('Действие по умолчанию')
			.querySelector('option[value="warp"]') as HTMLOptionElement | null;
		expect(defaultWarpOption?.disabled).toBe(true);

		await fireEvent.click(screen.getByRole('button', { name: 'Изменить Home' }));
		const ruleWarpOption = screen
			.getByLabelText('Действие правила')
			.querySelector('option[value="warp"]') as HTMLOptionElement | null;
		expect(ruleWarpOption?.disabled).toBe(true);
	});

	it('shows available outbound choices without exposing endpoint addresses', async () => {
		mockedApi.awg3List.mockResolvedValueOnce([
			{ id: 'vpn-1', tag: 'vpn-1', host: '198.51.100.10:51820', headerProtection: false },
		]);

		render(GatewayPage);
		const summary = await screen.findByRole('region', { name: 'Исходящие маршруты' });

		await within(summary).findByText('vpn-1');
		expect(within(summary).getByText('DIRECT')).toBeTruthy();
		expect(within(summary).getByText('BLOCK')).toBeTruthy();
		expect(within(summary).getByText('WARP')).toBeTruthy();
		expect(within(summary).getByText(/не подключён/i)).toBeTruthy();
		expect(within(summary).queryByText('198.51.100.10:51820')).toBeNull();
	});

	it('does not request profiles when gateway capability is disabled', async () => {
		mockedApi.getCapabilities.mockResolvedValue({ gateway: false } as never);
		render(GatewayPage);
		await screen.findByText('Gateway выключен');
		expect(mockedApi.getGatewayPolicyProfiles).not.toHaveBeenCalled();
	});

	it('edits a profile without dropping its existing rules', async () => {
		const profile = {
			id: 'home',
			name: 'Home',
			defaultAction: 'direct' as const,
			rules: [{ id: 'block-https', action: 'block' as const, clientIds: ['phone'], ports: [443], enabled: true }],
		};
		const updated = { ...profile, defaultAction: 'block' as const };
		mockedApi.getGatewayPolicyProfiles.mockResolvedValueOnce([profile]).mockResolvedValueOnce([updated]);
		mockedApi.updateGatewayPolicyProfile.mockResolvedValue(updated);

		render(GatewayPage);
		await screen.findByText('Home');
		await fireEvent.click(screen.getByRole('button', { name: 'Изменить Home' }));
		await fireEvent.change(screen.getByLabelText('Действие по умолчанию'), { target: { value: 'block' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Сохранить профиль' }));

		await waitFor(() => expect(mockedApi.updateGatewayPolicyProfile).toHaveBeenCalledWith(updated));
	});

	it('adds a domain-suffix rule to an existing profile', async () => {
		const profile = { id: 'home', name: 'Home', defaultAction: 'direct' as const, rules: [] };
		const updated = {
			...profile,
			rules: [{ id: 'video', action: 'direct' as const, outbound: 'direct', priority: 10, domainSuffixes: ['example.com'], enabled: true }],
		};
		mockedApi.getGatewayPolicyProfiles.mockResolvedValueOnce([profile]).mockResolvedValueOnce([updated]);
		mockedApi.updateGatewayPolicyProfile.mockResolvedValue(updated);

		render(GatewayPage);
		await screen.findByText('Home');
		await fireEvent.click(screen.getByRole('button', { name: 'Изменить Home' }));
		await fireEvent.input(screen.getByLabelText('ID правила'), { target: { value: 'video' } });
		await fireEvent.input(screen.getByLabelText('Доменные суффиксы'), { target: { value: 'example.com' } });
		await fireEvent.input(screen.getByLabelText('Приоритет правила'), { target: { value: '10' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Добавить правило' }));

		await waitFor(() => expect(mockedApi.updateGatewayPolicyProfile).toHaveBeenCalledWith(updated));
	});

	it('shows a compiled preview before applying a profile', async () => {
		const profile = {
			id: 'home',
			name: 'Home',
			defaultAction: 'direct' as const,
			rules: [{ id: 'video', action: 'direct' as const, outbound: 'direct', domainSuffixes: ['example.com'], enabled: true }],
		};
		mockedApi.getGatewayPolicyProfiles.mockResolvedValueOnce([profile]);
		mockedApi.previewGatewayPolicyProfile.mockResolvedValue({
			final: 'direct',
			rules: [{ domain_suffix: ['example.com'], action: 'route', outbound: 'direct' }],
		});

		render(GatewayPage);
		await screen.findByText('Home');
		await fireEvent.click(screen.getByRole('button', { name: 'Предпросмотр Home' }));

		await screen.findByText('direct', { selector: '.policy-preview code' });
		expect(screen.getByText(/example\.com/, { selector: '.policy-preview li' })).toBeTruthy();
		expect(mockedApi.applyGatewayPolicyProfile).not.toHaveBeenCalled();
	});

	it('can disable a saved rule without deleting it', async () => {
		const profile = {
			id: 'home',
			name: 'Home',
			defaultAction: 'direct' as const,
			rules: [{ id: 'block-https', action: 'block' as const, sourceCidrs: ['10.66.0.2/32'], ports: [443], protocols: ['tcp'], enabled: true }],
		};
		const updated = { ...profile, rules: [{ ...profile.rules[0], priority: 100, enabled: false }] };
		mockedApi.getGatewayPolicyProfiles.mockResolvedValueOnce([profile]).mockResolvedValueOnce([updated]);
		mockedApi.updateGatewayPolicyProfile.mockResolvedValue(updated);

		render(GatewayPage);
		await screen.findByText('Home');
		await fireEvent.click(screen.getByRole('button', { name: 'Изменить Home' }));
		await fireEvent.click(screen.getByRole('button', { name: 'Изменить правило block-https' }));
		await fireEvent.click(screen.getByLabelText('Правило включено'));
		await fireEvent.click(screen.getByRole('button', { name: 'Сохранить правило' }));

		await waitFor(() => expect(mockedApi.updateGatewayPolicyProfile).toHaveBeenCalledWith(updated));
	});

	it('does not route a VPN rule through DIRECT when its outbound is missing', async () => {
		const profile = { id: 'home', name: 'Home', defaultAction: 'direct' as const, rules: [] };
		mockedApi.getGatewayPolicyProfiles.mockResolvedValueOnce([profile]);

		render(GatewayPage);
		await screen.findByText('Home');
		await fireEvent.click(screen.getByRole('button', { name: 'Изменить Home' }));
		await fireEvent.input(screen.getByLabelText('ID правила'), { target: { value: 'video' } });
		await fireEvent.change(screen.getByLabelText('Действие правила'), { target: { value: 'vpn' } });
		await fireEvent.input(screen.getByLabelText('Доменные суффиксы'), { target: { value: 'example.com' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Добавить правило' }));

		await screen.findByText('Укажи тег исходящего маршрута для этого действия.');
		expect(mockedApi.updateGatewayPolicyProfile).not.toHaveBeenCalled();
	});

	it('offers configured AWG3 outbounds for VPN rules and persists the selected tag', async () => {
		const profile = { id: 'home', name: 'Home', defaultAction: 'direct' as const, rules: [] };
		const updated = {
			...profile,
			rules: [{
				id: 'video',
				action: 'vpn' as const,
				outbound: 'awg-de-1',
				priority: 100,
				domainSuffixes: ['example.com'],
				enabled: true,
			}],
		};
		mockedApi.getGatewayPolicyProfiles.mockResolvedValueOnce([profile]);
		mockedApi.awg3List.mockResolvedValue([{ id: 'awg-de-1', tag: 'awg-de-1', host: '198.51.100.10', headerProtection: false }]);
		mockedApi.updateGatewayPolicyProfile.mockResolvedValue(updated);

		render(GatewayPage);
		await screen.findByText('Home');
		await fireEvent.click(screen.getByRole('button', { name: 'Изменить Home' }));
		await fireEvent.input(screen.getByLabelText('ID правила'), { target: { value: 'video' } });
		await fireEvent.change(screen.getByLabelText('Действие правила'), { target: { value: 'vpn' } });
		await fireEvent.change(screen.getByLabelText('Тег AWG3'), { target: { value: 'awg-de-1' } });
		await fireEvent.input(screen.getByLabelText('Доменные суффиксы'), { target: { value: 'example.com' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Добавить правило' }));

		await waitFor(() => expect(mockedApi.updateGatewayPolicyProfile).toHaveBeenCalledWith(updated));
	});
});
