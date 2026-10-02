<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type GatewayClient, type GatewayGroup, type GatewayPolicyAction, type GatewayPolicyPreviewRule, type GatewayPolicyProfile, type GatewayPolicyRule, type GatewaySubscription } from '$lib/api/client';
	import { createGatewayProfileQr } from '$lib/utils/gatewayProfileQr';
	import { PageContainer, PageHeader } from '$lib/components/layout';

	type PageState = 'loading' | 'disabled' | 'ready' | 'error';

	let pageState = $state<PageState>('loading');
	let clients = $state<GatewayClient[]>([]);
	let loadingClients = $state(false);
	let errorMessage = $state('');
	let clientId = $state('');
	let clientLabel = $state('');
	let creating = $state(false);
	let subscriptions = $state<GatewaySubscription[]>([]);
	let loadingSubscriptions = $state(false);
	let subscriptionClientId = $state('');
	let subscriptionDays = $state(30);
	let subscriptionCreating = $state(false);
	let subscriptionActionId = $state('');
	let oneTimeSubscriptionURL = $state('');
	let oneTimeSubscriptionId = $state('');
	let subscriptionURLCopied = $state(false);
	let groups = $state<GatewayGroup[]>([]);
	let loadingGroups = $state(false);
	let groupName = $state('');
	let newGroupClientIds = $state<string[]>([]);
	let groupCreating = $state(false);
	let editingGroupId = $state('');
	let editingGroupName = $state('');
	let editingGroupClientIds = $state<string[]>([]);
	let groupActionId = $state('');
	let policyProfiles = $state<GatewayPolicyProfile[]>([]);
	let policyOutbounds = $state<{ tag: string }[]>([]);
	let policyOutboundsLoading = $state(false);
	let policyOutboundsError = $state('');
	let policyProfileId = $state('');
	let policyProfileName = $state('');
	let policyEditingId = $state('');
	let policyDefaultAction = $state<GatewayPolicyAction>('direct');
	let policyProfilesLoading = $state(false);
	let policyProfileBusy = $state(false);
	let policyMessage = $state('');
	let policyRuleEditingId = $state('');
	let policyRuleId = $state('');
	let policyRuleAction = $state<GatewayPolicyAction>('direct');
	let policyRuleEnabled = $state(true);
	let policyRuleOutbound = $state('direct');
	let policyRulePriority = $state('100');
	let policyRuleClientIds = $state<string[]>([]);
	let policyRuleGroupIds = $state<string[]>([]);
	let policyRuleDomains = $state('');
	let policyRuleDomainSuffixes = $state('');
	let policyRuleCIDRs = $state('');
	let policyRuleSourceCIDRs = $state('');
	let policyRulePorts = $state('');
	let policyRuleProtocols = $state('');
	let policyRuleSets = $state('');
	let policyPreview = $state<{ profileName: string; preview: { final: string; rules: GatewayPolicyPreviewRule[] } } | null>(null);
	let policyPreviewingId = $state('');
	let profileClient = $state<GatewayClient | null>(null);
	let profileText = $state('');
	let profileQr = $state('');
	let profileLoading = $state(false);
	let revokeTarget = $state<GatewayClient | null>(null);
	let actionBusy = $state(false);

	async function loadClients() {
		loadingClients = true;
		errorMessage = '';
		try {
			clients = await api.getGatewayClients();
			const activeClients = clients.filter((client) => client.status === 'active');
			if (!activeClients.some((client) => client.id === subscriptionClientId)) {
				subscriptionClientId = activeClients[0]?.id ?? '';
			}
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : 'Не удалось загрузить клиентов';
		} finally {
			loadingClients = false;
		}
	}

	async function loadSubscriptions() {
		loadingSubscriptions = true;
		try {
			subscriptions = await api.getGatewaySubscriptions();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : 'Не удалось загрузить подписки';
		} finally {
			loadingSubscriptions = false;
		}
	}

	async function loadGroups() {
		loadingGroups = true;
		try {
			groups = await api.getGatewayGroups();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : 'Не удалось загрузить группы клиентов';
		} finally {
			loadingGroups = false;
		}
	}

	async function loadPolicyProfiles() {
		policyProfilesLoading = true;
		try {
			policyProfiles = await api.getGatewayPolicyProfiles();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : 'Не удалось загрузить профили политик';
		} finally {
			policyProfilesLoading = false;
		}
	}

	async function loadPolicyOutbounds() {
		policyOutboundsLoading = true;
		policyOutboundsError = '';
		try {
			policyOutbounds = (await api.awg3List()).map(({ tag }) => ({ tag }));
		} catch (error) {
			policyOutbounds = [];
			policyOutboundsError = error instanceof Error ? error.message : 'Не удалось загрузить AWG3 endpoints';
		} finally {
			policyOutboundsLoading = false;
		}
	}

	async function initialize() {
		try {
			const capabilities = await api.getCapabilities();
			if (!capabilities.gateway) {
				pageState = 'disabled';
				return;
			}
			pageState = 'ready';
			await Promise.all([loadClients(), loadSubscriptions(), loadGroups(), loadPolicyProfiles(), loadPolicyOutbounds()]);
		} catch (error) {
			pageState = 'error';
			errorMessage = error instanceof Error ? error.message : 'Не удалось проверить режим gateway';
		}
	}

	onMount(() => {
		void initialize();
		return () => {
			clearProfile();
			oneTimeSubscriptionURL = '';
		};
	});

	async function createClient(event: SubmitEvent) {
		event.preventDefault();
		if (creating || !clientId.trim() || !clientLabel.trim()) return;
		creating = true;
		errorMessage = '';
		try {
			await api.createGatewayClient(clientId, clientLabel);
			clientId = '';
			clientLabel = '';
			await loadClients();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : 'Не удалось создать клиента';
		} finally {
			creating = false;
		}
	}

	async function runAction(client: GatewayClient, action: 'disable' | 'enable' | 'revoke') {
		if (actionBusy) return;
		actionBusy = true;
		errorMessage = '';
		try {
			await api.gatewayClientAction(client.id, action);
			revokeTarget = null;
			clearProfile();
			await loadClients();
			await loadSubscriptions();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : 'Не удалось изменить состояние клиента';
		} finally {
			actionBusy = false;
		}
	}

	async function createSubscription(event: SubmitEvent) {
		event.preventDefault();
		if (subscriptionCreating || !subscriptionClientId || subscriptionDays < 1 || subscriptionDays > 365) return;
		subscriptionCreating = true;
		errorMessage = '';
		subscriptionURLCopied = false;
		try {
			const issue = await api.createGatewaySubscription(subscriptionClientId, subscriptionDays);
			oneTimeSubscriptionURL = issue.url;
			oneTimeSubscriptionId = issue.subscription.id;
			await loadSubscriptions();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : 'Не удалось создать ссылку подписки';
		} finally {
			subscriptionCreating = false;
		}
	}

	async function revokeSubscription(subscription: GatewaySubscription) {
		if (subscriptionActionId) return;
		subscriptionActionId = subscription.id;
		errorMessage = '';
		try {
			await api.revokeGatewaySubscription(subscription.id);
			if (oneTimeSubscriptionId === subscription.id) {
				oneTimeSubscriptionURL = '';
				oneTimeSubscriptionId = '';
			}
			await loadSubscriptions();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : 'Не удалось отозвать ссылку';
		} finally {
			subscriptionActionId = '';
		}
	}

	async function createPolicyProfile(event: SubmitEvent) {
		event.preventDefault();
		if (policyProfileBusy || !policyProfileId.trim() || !policyProfileName.trim()) return;
		policyProfileBusy = true;
		errorMessage = '';
		policyMessage = '';
		try {
			const profile = {
				id: policyProfileId.trim(),
				name: policyProfileName.trim(),
				defaultAction: policyDefaultAction,
			};
			if (policyEditingId) {
				const existing = policyProfiles.find((item) => item.id === policyEditingId);
				if (!existing) throw new Error('Профиль не найден. Обнови список перед повторной попыткой.');
				await api.updateGatewayPolicyProfile({ ...existing, ...profile });
				policyPreview = null;
				policyMessage = `Профиль «${profile.name}» сохранён.`;
			} else {
				await api.createGatewayPolicyProfile(profile);
				policyPreview = null;
				policyMessage = `Профиль «${profile.name}» создан.`;
			}
			policyProfileId = '';
			policyProfileName = '';
			policyEditingId = '';
			await loadPolicyProfiles();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : 'Не удалось создать профиль политики';
		} finally {
			policyProfileBusy = false;
		}
	}

	function editPolicyProfile(profile: GatewayPolicyProfile) {
		policyPreview = null;
		policyEditingId = profile.id;
		policyProfileId = profile.id;
		policyProfileName = profile.name;
		policyDefaultAction = profile.defaultAction;
		policyMessage = '';
		errorMessage = '';
	}

	function cancelPolicyEdit() {
		policyEditingId = '';
		policyProfileId = '';
		policyProfileName = '';
		policyDefaultAction = 'direct';
		resetPolicyRuleDraft();
	}

	function splitPolicyValues(value: string): string[] {
		return value.split(',').map((item) => item.trim()).filter(Boolean);
	}

	function resetPolicyRuleDraft() {
		policyRuleEditingId = '';
		policyRuleId = '';
		policyRuleAction = 'direct';
		policyRuleEnabled = true;
		policyRuleOutbound = 'direct';
		policyRulePriority = '100';
		policyRuleClientIds = [];
		policyRuleGroupIds = [];
		policyRuleDomains = '';
		policyRuleDomainSuffixes = '';
		policyRuleCIDRs = '';
		policyRuleSourceCIDRs = '';
		policyRulePorts = '';
		policyRuleProtocols = '';
		policyRuleSets = '';
	}

	function editPolicyRule(rule: GatewayPolicyRule) {
		policyRuleEditingId = rule.id;
		policyRuleId = rule.id;
		policyRuleAction = rule.action;
		policyRuleEnabled = rule.enabled ?? true;
		policyRuleOutbound = rule.outbound ?? '';
		policyRulePriority = String(rule.priority ?? 100);
		policyRuleClientIds = [...(rule.clientIds ?? [])];
		policyRuleGroupIds = [...(rule.groupIds ?? [])];
		policyRuleDomains = (rule.domains ?? []).join(', ');
		policyRuleDomainSuffixes = (rule.domainSuffixes ?? []).join(', ');
		policyRuleCIDRs = (rule.cidrs ?? []).join(', ');
		policyRuleSourceCIDRs = (rule.sourceCidrs ?? []).join(', ');
		policyRulePorts = (rule.ports ?? []).join(', ');
		policyRuleProtocols = (rule.protocols ?? []).join(', ');
		policyRuleSets = (rule.ruleSets ?? []).join(', ');
	}

	function setPolicyRuleAction(action: GatewayPolicyAction) {
		policyRuleAction = action;
		policyRuleOutbound = action === 'direct' ? 'direct' : action === 'vpn' ? policyOutbounds[0]?.tag ?? '' : '';
	}

	async function savePolicyRule(event: SubmitEvent) {
		event.preventDefault();
		if (!policyEditingId || policyProfileBusy || !policyRuleId.trim()) return;
		const existing = policyProfiles.find((profile) => profile.id === policyEditingId);
		if (!existing) {
			errorMessage = 'Профиль не найден. Обнови список перед повторной попыткой.';
			return;
		}
		const priority = Number(policyRulePriority);
		if (!Number.isInteger(priority) || priority < 0) {
			errorMessage = 'Приоритет должен быть целым числом не меньше 0.';
			return;
		}
		const ports = splitPolicyValues(policyRulePorts).map(Number);
		if (ports.some((port) => !Number.isInteger(port) || port < 1 || port > 65535)) {
			errorMessage = 'Порты должны быть целыми числами от 1 до 65535.';
			return;
		}
		const clientIds = [...policyRuleClientIds];
		const groupIds = [...policyRuleGroupIds];
		const domains = splitPolicyValues(policyRuleDomains);
		const domainSuffixes = splitPolicyValues(policyRuleDomainSuffixes);
		const cidrs = splitPolicyValues(policyRuleCIDRs);
		const sourceCidrs = splitPolicyValues(policyRuleSourceCIDRs);
		const protocols = splitPolicyValues(policyRuleProtocols).map((protocol) => protocol.toLowerCase());
		const ruleSets = splitPolicyValues(policyRuleSets);
		if (![clientIds, groupIds, domains, domainSuffixes, cidrs, sourceCidrs, ports, protocols, ruleSets].some((values) => values.length > 0)) {
			errorMessage = 'Добавь хотя бы одно условие совпадения правила.';
			return;
		}
		if (policyRuleAction !== 'block' && !policyRuleOutbound.trim()) {
			errorMessage = 'Укажи тег исходящего маршрута для этого действия.';
			return;
		}
		if ((policyRuleAction === 'vpn' || policyRuleAction === 'warp') && policyRuleOutbound.trim().toLowerCase() === 'direct') {
			errorMessage = 'Для VPN/WARP выбери соответствующий upstream, а не DIRECT.';
			return;
		}
		if (policyRuleAction === 'vpn' && !policyOutbounds.some(({ tag }) => tag === policyRuleOutbound.trim())) {
			errorMessage = 'Выбери доступный AWG3 endpoint для VPN-маршрута.';
			return;
		}
		const rule: GatewayPolicyRule = {
			id: policyRuleId.trim(),
			action: policyRuleAction,
			...(policyRuleAction === 'block' ? {} : { outbound: policyRuleAction === 'direct' ? 'direct' : policyRuleOutbound.trim() }),
			priority,
			...(clientIds.length ? { clientIds } : {}),
			...(groupIds.length ? { groupIds } : {}),
			...(domains.length ? { domains } : {}),
			...(domainSuffixes.length ? { domainSuffixes } : {}),
			...(cidrs.length ? { cidrs } : {}),
			...(sourceCidrs.length ? { sourceCidrs } : {}),
			...(ports.length ? { ports } : {}),
			...(protocols.length ? { protocols } : {}),
			...(ruleSets.length ? { ruleSets } : {}),
			enabled: policyRuleEnabled,
		};
		const rules = existing.rules ?? [];
		if (!policyRuleEditingId && rules.some((item) => item.id === rule.id)) {
			errorMessage = 'ID такого правила уже используется в профиле.';
			return;
		}
		const nextRules = policyRuleEditingId
			? rules.map((item) => item.id === policyRuleEditingId ? rule : item)
			: [...rules, rule];
		policyProfileBusy = true;
		errorMessage = '';
		try {
			await api.updateGatewayPolicyProfile({ ...existing, rules: nextRules });
			policyPreview = null;
			policyMessage = `Правило «${rule.id}» ${policyRuleEditingId ? 'сохранено' : 'добавлено'}.`;
			resetPolicyRuleDraft();
			await loadPolicyProfiles();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : 'Не удалось сохранить правило политики';
		} finally {
			policyProfileBusy = false;
		}
	}

	async function deletePolicyRule(profile: GatewayPolicyProfile, rule: GatewayPolicyRule) {
		if (policyProfileBusy) return;
		policyProfileBusy = true;
		errorMessage = '';
		try {
			await api.updateGatewayPolicyProfile({ ...profile, rules: (profile.rules ?? []).filter((item) => item.id !== rule.id) });
			policyPreview = null;
			if (policyRuleEditingId === rule.id) resetPolicyRuleDraft();
			policyMessage = `Правило «${rule.id}» удалено.`;
			await loadPolicyProfiles();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : 'Не удалось удалить правило политики';
		} finally {
			policyProfileBusy = false;
		}
	}

	async function previewPolicyProfile(profile: GatewayPolicyProfile) {
		if (policyProfileBusy || policyPreviewingId) return;
		policyPreviewingId = profile.id;
		policyPreview = null;
		errorMessage = '';
		try {
			const preview = await api.previewGatewayPolicyProfile(profile);
			policyPreview = { profileName: profile.name, preview };
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : 'Не удалось подготовить предпросмотр политики';
		} finally {
			policyPreviewingId = '';
		}
	}

	function describePolicyPreviewRule(rule: GatewayPolicyPreviewRule): string {
		const matches = [
			...(rule.domain ?? []).map((value) => `домен ${value}`),
			...(rule.domain_suffix ?? []).map((value) => `суффикс ${value}`),
			...(rule.rule_set ?? []).map((value) => `набор ${value}`),
			...(rule.ip_cidr ?? []).map((value) => `IP ${value}`),
			...(rule.source_ip_cidr ?? []).map((value) => `клиент ${value}`),
			...(rule.port ?? []).map((value) => `порт ${value}`),
			...(rule.protocol ?? []).map((value) => `протокол ${value}`),
		].join(' · ') || 'Все подключения';
		const action = rule.action === 'reject' ? 'BLOCK' : rule.outbound ? `→ ${rule.outbound}` : rule.action ?? 'маршрут';
		return `${matches} — ${action}`;
	}

	async function applyPolicyProfile(profile: GatewayPolicyProfile) {
		if (policyProfileBusy) return;
		policyProfileBusy = true;
		errorMessage = '';
		policyMessage = '';
		try {
			await api.applyGatewayPolicyProfile(profile.id);
			policyMessage = `Профиль «${profile.name}» применён.`;
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : 'Не удалось применить профиль политики';
		} finally {
			policyProfileBusy = false;
		}
	}

	async function deletePolicyProfile(profile: GatewayPolicyProfile) {
		if (policyProfileBusy) return;
		policyProfileBusy = true;
		errorMessage = '';
		try {
			await api.deleteGatewayPolicyProfile(profile.id);
			policyPreview = null;
			await loadPolicyProfiles();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : 'Не удалось удалить профиль политики';
		} finally {
			policyProfileBusy = false;
		}
	}

	async function createGroup(event: SubmitEvent) {
		event.preventDefault();
		if (groupCreating || !groupName.trim()) return;
		groupCreating = true;
		errorMessage = '';
		try {
			await api.createGatewayGroup(groupName, newGroupClientIds);
			groupName = '';
			newGroupClientIds = [];
			await loadGroups();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : 'Не удалось создать группу';
		} finally {
			groupCreating = false;
		}
	}

	function startEditGroup(group: GatewayGroup) {
		editingGroupId = group.id;
		editingGroupName = group.name;
		editingGroupClientIds = [...group.clientIds];
	}

	function toggleMember(memberIds: string[], clientId: string, checked: boolean): string[] {
		if (checked) return memberIds.includes(clientId) ? memberIds : [...memberIds, clientId];
		return memberIds.filter((id) => id !== clientId);
	}

	async function saveGroup(event: SubmitEvent) {
		event.preventDefault();
		if (!editingGroupId || groupActionId) return;
		groupActionId = editingGroupId;
		errorMessage = '';
		try {
			await api.updateGatewayGroup(editingGroupId, editingGroupName, editingGroupClientIds);
			editingGroupId = '';
			await loadGroups();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : 'Не удалось сохранить группу';
		} finally {
			groupActionId = '';
		}
	}

	async function deleteGroup(group: GatewayGroup) {
		if (groupActionId) return;
		groupActionId = group.id;
		errorMessage = '';
		try {
			await api.deleteGatewayGroup(group.id);
			if (editingGroupId === group.id) editingGroupId = '';
			await loadGroups();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : 'Не удалось удалить группу';
		} finally {
			groupActionId = '';
		}
	}

	function groupClientLabels(group: GatewayGroup): string {
		if (group.clientIds.length === 0) return 'Нет устройств';
		return group.clientIds.map((id) => clients.find((client) => client.id === id)?.label ?? id).join(', ');
	}

	async function copySubscriptionURL() {
		if (!oneTimeSubscriptionURL) return;
		try {
			await navigator.clipboard.writeText(oneTimeSubscriptionURL);
			subscriptionURLCopied = true;
		} catch {
			errorMessage = 'Не удалось скопировать ссылку. Выдели её и скопируй вручную.';
		}
	}

	async function openProfile(client: GatewayClient) {
		if (profileLoading) return;
		profileLoading = true;
		errorMessage = '';
		try {
			const config = await api.getGatewayClientConfig(client.id);
			const qr = await createGatewayProfileQr(config);
			profileClient = client;
			profileText = config;
			profileQr = qr;
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : 'Не удалось подготовить профиль';
		} finally {
			profileLoading = false;
		}
	}

	function clearProfile() {
		profileText = '';
		profileQr = '';
		profileClient = null;
	}

	function downloadProfile() {
		if (!profileClient || !profileText) return;
		const blobUrl = URL.createObjectURL(new Blob([profileText], { type: 'text/plain;charset=utf-8' }));
		const anchor = document.createElement('a');
		anchor.href = blobUrl;
		anchor.download = `awg-${profileClient.id.replace(/[^a-zA-Z0-9._-]/g, '_')}.conf`;
		anchor.click();
		window.setTimeout(() => URL.revokeObjectURL(blobUrl), 0);
	}

	function clientStatusLabel(status: GatewayClient['status']): string {
		if (status === 'active') return 'Активен';
		if (status === 'disabled') return 'Отключён';
		return 'Отозван';
	}

	function subscriptionStatusLabel(subscription: GatewaySubscription): string {
		if (subscription.revokedAt) return 'Отозвана';
		if (Date.parse(subscription.expiresAt) <= Date.now()) return 'Истекла';
		const client = clients.find((item) => item.id === subscription.clientId);
		if (!client || client.status !== 'active') return 'Клиент отключён';
		return 'Активна';
	}

	function subscriptionClientLabel(subscription: GatewaySubscription): string {
		return clients.find((client) => client.id === subscription.clientId)?.label ?? subscription.clientId;
	}

	function formatExpiry(value: string): string {
		return new Intl.DateTimeFormat('ru-RU', { dateStyle: 'medium' }).format(new Date(value));
	}
</script>

<svelte:head>
	<title>VPS Gateway — AWG Manager</title>
</svelte:head>

<PageContainer width="wide">
	<PageHeader
		title="VPS Gateway"
		description="Управление клиентами WireGuard/AWG и их доступом к gateway."
	/>

	{#if pageState === 'loading'}
		<p class="notice">Проверяю режим gateway…</p>
	{:else if pageState === 'disabled'}
		<section class="notice">
			<h2>Gateway выключен</h2>
			<p>Для этого Docker-контейнера включи <code>AWG_GATEWAY_ENABLE=true</code> и настрой внешний endpoint.</p>
		</section>
	{:else if pageState === 'error'}
		<section class="notice error"><p>{errorMessage}</p></section>
	{:else}
		{#if errorMessage}
			<div class="notice error" role="alert">{errorMessage}</div>
		{/if}

		<section class="panel create-panel" aria-labelledby="create-title">
			<div class="panel-heading">
				<div>
					<p class="eyebrow">НОВОЕ УСТРОЙСТВО</p>
					<h2 id="create-title">Добавить клиента</h2>
				</div>
				<span class="pool-note">Адрес назначит gateway</span>
			</div>
			<form class="create-form" onsubmit={createClient}>
				<label>
					<span>ID клиента</span>
					<input bind:value={clientId} required maxlength="64" autocomplete="off" placeholder="phone-anna" />
				</label>
				<label>
					<span>Название</span>
					<input bind:value={clientLabel} required maxlength="80" autocomplete="off" placeholder="Телефон Анны" />
				</label>
				<button class="primary" type="submit" disabled={creating || !clientId.trim() || !clientLabel.trim()}>
					{creating ? 'Создаю…' : 'Создать клиента'}
				</button>
			</form>
		</section>

		<section class="panel subscriptions-panel" aria-labelledby="subscriptions-title">
			<div class="panel-heading">
				<div>
					<p class="eyebrow">ПОДПИСКИ</p>
					<h2 id="subscriptions-title">Ссылки на профиль <span class="count">{subscriptions.length}</span></h2>
				</div>
				<button class="quiet" type="button" onclick={() => void loadSubscriptions()} disabled={loadingSubscriptions}>
					{loadingSubscriptions ? 'Обновляю…' : 'Обновить'}
				</button>
			</div>
			<p class="subscription-intro">Создай временную HTTPS-ссылку на профиль клиента. Её можно отозвать отдельно от самого клиента.</p>
			<form class="subscription-form" onsubmit={createSubscription}>
				<label>
					<span>Клиент</span>
					<select aria-label="Клиент" bind:value={subscriptionClientId} required>
						{#each clients.filter((client) => client.status === 'active') as client (client.id)}
							<option value={client.id}>{client.label} ({client.id})</option>
						{/each}
					</select>
				</label>
				<label>
					<span>Срок, дней</span>
					<input type="number" min="1" max="365" bind:value={subscriptionDays} required />
				</label>
				<button class="primary" type="submit" disabled={subscriptionCreating || !subscriptionClientId || subscriptionDays < 1 || subscriptionDays > 365}>
					{subscriptionCreating ? 'Создаю…' : 'Создать ссылку'}
				</button>
			</form>
			{#if oneTimeSubscriptionURL}
				<div class="one-time-link" role="status">
					<p class="subscription-secret-note">Секретная ссылка показывается только при создании. Сохрани её: повторно получить эту ссылку нельзя.</p>
					<label>
						<span>Ссылка для подключения</span>
						<input aria-label="Ссылка для подключения" value={oneTimeSubscriptionURL} readonly autocomplete="off" spellcheck="false" />
					</label>
					<div class="dialog-actions">
						<button class="primary" type="button" onclick={() => void copySubscriptionURL()}>{subscriptionURLCopied ? 'Скопировано' : 'Скопировать ссылку'}</button>
						<button class="quiet" type="button" onclick={() => { oneTimeSubscriptionURL = ''; oneTimeSubscriptionId = ''; }}>Скрыть</button>
					</div>
				</div>
			{/if}
			{#if loadingSubscriptions && subscriptions.length === 0}
				<p class="notice">Загружаю ссылки…</p>
			{:else if subscriptions.length === 0}
				<p class="notice">Ссылок пока нет. Профиль клиента можно выдать напрямую через QR или .conf.</p>
			{:else}
				<div class="subscription-list">
					{#each subscriptions as subscription (subscription.id)}
						<article class="subscription-card">
							<div class="subscription-main">
								<div class="client-title-row">
									<h3>{subscriptionClientLabel(subscription)}</h3>
									<span class="status" class:active={subscriptionStatusLabel(subscription) === 'Активна'} class:disabled={subscriptionStatusLabel(subscription) === 'Клиент отключён'} class:revoked={subscriptionStatusLabel(subscription) === 'Отозвана' || subscriptionStatusLabel(subscription) === 'Истекла'}>
										{subscriptionStatusLabel(subscription)}
									</span>
								</div>
								<p class="client-meta"><code>{subscription.clientId}</code><span>до {formatExpiry(subscription.expiresAt)}</span></p>
							</div>
							{#if !subscription.revokedAt}
								<button class="danger" type="button" aria-label={`Отозвать ссылку ${subscriptionClientLabel(subscription)}`} disabled={subscriptionActionId !== ''} onclick={() => void revokeSubscription(subscription)}>
									{subscriptionActionId === subscription.id ? 'Отзываю…' : 'Отозвать ссылку'}
								</button>
							{/if}
						</article>
					{/each}
				</div>
			{/if}
		</section>

		<section class="panel policy-panel" aria-labelledby="policy-profiles-title">
			<div class="panel-heading">
				<div>
					<p class="eyebrow">МАРШРУТИЗАЦИЯ</p>
					<h2 id="policy-profiles-title">Профили политик <span class="count">{policyProfiles.length}</span></h2>
				</div>
				<button class="quiet" type="button" onclick={() => void loadPolicyProfiles()} disabled={policyProfileBusy || policyProfilesLoading}>
					{policyProfilesLoading ? 'Обновляю…' : 'Обновить'}
				</button>
			</div>
			<p class="policy-intro">Правила применяются по приоритету: меньшее число — раньше. Каждое правило должно иметь хотя бы одно условие совпадения.</p>
			{#if policyMessage}
				<p class="policy-status" role="status">{policyMessage}</p>
			{/if}
			<form class="policy-form" onsubmit={createPolicyProfile}>
				<label>
					<span>ID профиля</span>
					<input bind:value={policyProfileId} required maxlength="64" autocomplete="off" disabled={policyProfileBusy || !!policyEditingId} />
				</label>
				<label>
					<span>Название профиля</span>
					<input bind:value={policyProfileName} required maxlength="80" autocomplete="off" disabled={policyProfileBusy} />
				</label>
				<label>
					<span>Действие по умолчанию</span>
					<select bind:value={policyDefaultAction} disabled={policyProfileBusy}>
						<option value="vpn">VPN</option>
						<option value="warp" disabled>WARP</option>
						<option value="direct">Напрямую</option>
						<option value="block">Блокировать</option>
					</select>
				</label>
				<button class="primary" type="submit" disabled={policyProfileBusy || !policyProfileId.trim() || !policyProfileName.trim()}>
					{policyProfileBusy ? 'Сохраняю…' : policyEditingId ? 'Сохранить профиль' : 'Создать профиль'}
				</button>
				{#if policyEditingId}
					<button class="quiet" type="button" onclick={cancelPolicyEdit} disabled={policyProfileBusy}>Отмена</button>
				{/if}
			</form>
			{#if policyEditingId}
				<section class="policy-rule-editor" aria-label="Редактор правил политики">
					<h3>Правила профиля</h3>
					<p class="rule-help">Списки значений вводи через запятую. Для VPN укажи точный тег AWG3-маршрута; DIRECT задаётся автоматически. WARP пока не подключён и будет отклонён предпросмотром до настройки outbound.</p>
					{#if policyProfiles.find((item) => item.id === policyEditingId)?.rules?.length}
						<ul class="policy-rule-list" aria-label="Правила профиля">
							{#each policyProfiles.find((item) => item.id === policyEditingId)?.rules ?? [] as rule (rule.id)}
								<li class="policy-rule-card">
									<div>
										<strong>{rule.id}</strong>
										<span>Приоритет {rule.priority ?? 0} · {rule.action}{rule.outbound ? ` → ${rule.outbound}` : ''} · {rule.enabled === false ? 'Выключено' : 'Включено'}</span>
										<span>{[...(rule.domainSuffixes ?? []), ...(rule.domains ?? []), ...(rule.ruleSets ?? []).map((item) => `набор ${item}`), ...(rule.cidrs ?? []).map((item) => `IP ${item}`), ...(rule.sourceCidrs ?? []).map((item) => `источник ${item}`), ...(rule.ports ?? []).map((item) => `порт ${item}`), ...(rule.protocols ?? []).map((item) => `протокол ${item}`), ...(rule.clientIds ?? []).map((item) => `клиент ${item}`), ...(rule.groupIds ?? []).map((item) => `группа ${item}`)].join(', ')}</span>
									</div>
									<div class="policy-actions">
										<button class="quiet" type="button" aria-label={`Изменить правило ${rule.id}`} disabled={policyProfileBusy} onclick={() => editPolicyRule(rule)}>Изменить</button>
										<button class="danger" type="button" aria-label={`Удалить правило ${rule.id}`} disabled={policyProfileBusy} onclick={() => void deletePolicyRule(policyProfiles.find((item) => item.id === policyEditingId)!, rule)}>Удалить</button>
									</div>
								</li>
							{/each}
						</ul>
					{:else}
						<p class="notice">В этом профиле пока нет правил.</p>
					{/if}
					<form class="rule-form" onsubmit={savePolicyRule}>
						<div class="rule-fields">
							<label><span>ID правила</span><input bind:value={policyRuleId} required maxlength="64" autocomplete="off" disabled={policyProfileBusy || !!policyRuleEditingId} /></label>
							<label><span>Действие правила</span><select value={policyRuleAction} onchange={(event) => setPolicyRuleAction(event.currentTarget.value as GatewayPolicyAction)} disabled={policyProfileBusy}><option value="vpn">VPN</option><option value="warp" disabled>WARP</option><option value="direct">Напрямую</option><option value="block">Блокировать</option></select></label>
							{#if policyRuleAction === 'vpn'}
								<label><span>Тег AWG3</span><select aria-label="Тег AWG3" bind:value={policyRuleOutbound} required disabled={policyProfileBusy || policyOutboundsLoading || policyOutbounds.length === 0}>
									<option value="" disabled>{policyOutboundsLoading ? 'Загружаю endpoints…' : 'Выбери endpoint'}</option>
									{#if policyRuleOutbound && !policyOutbounds.some(({ tag }) => tag === policyRuleOutbound)}<option value={policyRuleOutbound} disabled>{policyRuleOutbound} (не найден)</option>{/if}
									{#each policyOutbounds as outbound (outbound.tag)}<option value={outbound.tag}>{outbound.tag}</option>{/each}
								</select></label>
							{:else}
								<label><span>Тег исходящего маршрута</span><input bind:value={policyRuleOutbound} placeholder="тег WARP upstream" disabled={policyProfileBusy || policyRuleAction === 'block' || policyRuleAction === 'direct'} /></label>
							{/if}
							{#if policyRuleAction === 'vpn' && policyOutboundsError}
								<p class="rule-help">Не удалось загрузить AWG3 endpoints: {policyOutboundsError}</p>
							{:else if policyRuleAction === 'vpn' && !policyOutboundsLoading && policyOutbounds.length === 0}
								<p class="rule-help">Нет доступных AWG3 endpoints. Добавь endpoint перед настройкой VPN-маршрута.</p>
							{/if}
							<label><span>Приоритет правила</span><input aria-label="Приоритет правила" type="number" min="0" step="1" bind:value={policyRulePriority} required disabled={policyProfileBusy} /></label>
							<label><span>Домены</span><input bind:value={policyRuleDomains} placeholder="example.com, video.example.net" disabled={policyProfileBusy} /></label>
							<label><span>Доменные суффиксы</span><input bind:value={policyRuleDomainSuffixes} placeholder="example.com" disabled={policyProfileBusy} /></label>
							<label><span>CIDR назначения</span><input bind:value={policyRuleCIDRs} placeholder="203.0.113.0/24" disabled={policyProfileBusy} /></label>
							<label><span>CIDR источника</span><input bind:value={policyRuleSourceCIDRs} placeholder="10.66.0.2/32" disabled={policyProfileBusy} /></label>
							<label><span>Порты</span><input bind:value={policyRulePorts} placeholder="443, 8443" disabled={policyProfileBusy} /></label>
							<label><span>Протоколы</span><input bind:value={policyRuleProtocols} placeholder="tcp, udp" disabled={policyProfileBusy} /></label>
							<label><span>Наборы доменов</span><input bind:value={policyRuleSets} placeholder="streaming, ru-domains" disabled={policyProfileBusy} /></label>
						</div>
						<label class="member-option rule-enabled-toggle"><input type="checkbox" bind:checked={policyRuleEnabled} disabled={policyProfileBusy} /><span>Правило включено</span></label>
						<fieldset class="member-picker">
							<legend>Клиенты</legend>
							{#each clients as client (client.id)}
								<label class="member-option"><input type="checkbox" checked={policyRuleClientIds.includes(client.id)} disabled={policyProfileBusy} onchange={(event) => { policyRuleClientIds = toggleMember(policyRuleClientIds, client.id, event.currentTarget.checked); }} /><span>{client.label} <code>{client.id}</code></span></label>
							{:else}
								<p class="member-empty">Клиентов пока нет.</p>
							{/each}
						</fieldset>
						<fieldset class="member-picker">
							<legend>Группы</legend>
							{#each groups as group (group.id)}
								<label class="member-option"><input type="checkbox" checked={policyRuleGroupIds.includes(group.id)} disabled={policyProfileBusy} onchange={(event) => { policyRuleGroupIds = toggleMember(policyRuleGroupIds, group.id, event.currentTarget.checked); }} /><span>{group.name} <code>{group.id}</code></span></label>
							{:else}
								<p class="member-empty">Групп пока нет.</p>
							{/each}
						</fieldset>
						<div class="rule-form-actions">
							<button class="primary" type="submit" disabled={policyProfileBusy || !policyRuleId.trim()}>{policyProfileBusy ? 'Сохраняю…' : policyRuleEditingId ? 'Сохранить правило' : 'Добавить правило'}</button>
							{#if policyRuleEditingId}<button class="quiet" type="button" onclick={resetPolicyRuleDraft} disabled={policyProfileBusy}>Отмена</button>{/if}
						</div>
					</form>
				</section>
			{/if}
			{#if policyProfilesLoading && policyProfiles.length === 0}
				<p class="notice">Загружаю профили…</p>
			{:else if policyProfiles.length === 0}
				<p class="notice">Профилей политик пока нет.</p>
			{:else}
				<ul class="policy-list" aria-label="Сохранённые профили политик">
					{#each policyProfiles as profile (profile.id)}
						<li class="policy-card">
							<div class="policy-summary">
								<h3>{profile.name}</h3>
								<p><code>{profile.id}</code><span>По умолчанию: {profile.defaultAction}</span><span>Правил: {profile.rules?.length ?? 0}</span></p>
							</div>
							<div class="policy-actions">
								<button class="quiet" type="button" aria-label={`Изменить ${profile.name}`} disabled={policyProfileBusy} onclick={() => editPolicyProfile(profile)}>Изменить</button>
								<button class="quiet" type="button" aria-label={`Предпросмотр ${profile.name}`} disabled={policyProfileBusy || !!policyPreviewingId} onclick={() => void previewPolicyProfile(profile)}>{policyPreviewingId === profile.id ? 'Собираю…' : 'Предпросмотр'}</button>
								<button class="primary" type="button" aria-label={`Применить ${profile.name}`} disabled={policyProfileBusy} onclick={() => void applyPolicyProfile(profile)}>
									{policyProfileBusy ? 'Выполняю…' : 'Применить'}
								</button>
								<button class="danger" type="button" aria-label={`Удалить ${profile.name}`} disabled={policyProfileBusy} onclick={() => void deletePolicyProfile(profile)}>
									{policyProfileBusy ? 'Выполняю…' : 'Удалить'}
								</button>
							</div>
						</li>
					{/each}
				</ul>
			{/if}
			{#if policyPreview}
				<section class="policy-preview" aria-label="Предпросмотр политики">
					<h3>Предпросмотр: {policyPreview.profileName}</h3>
					<p>Итоговый маршрут: <code>{policyPreview.preview.final}</code></p>
					{#if policyPreview.preview.rules.length}
						<ul>{#each policyPreview.preview.rules as rule, index (`${policyPreview.profileName}-${index}`)}<li>{describePolicyPreviewRule(rule)}</li>{/each}</ul>
					{:else}
						<p class="rule-help">Дополнительных правил нет.</p>
					{/if}
					<details><summary>Показать техническое представление</summary><pre>{JSON.stringify(policyPreview.preview, null, 2)}</pre></details>
				</section>
			{/if}
		</section>

		<section class="panel outbounds-panel" aria-labelledby="outbounds-title">
			<div class="panel-heading">
				<div>
					<p class="eyebrow">ВЫХОДЫ</p>
					<h2 id="outbounds-title">Исходящие маршруты</h2>
				</div>
				<button class="quiet" type="button" onclick={() => void loadPolicyOutbounds()} disabled={policyOutboundsLoading}>
					{policyOutboundsLoading ? 'Обновляю…' : 'Обновить'}
				</button>
			</div>
			<ul class="outbounds-list">
				<li class="outbounds-row">
					<strong>VPN</strong>
					{#if policyOutboundsLoading}
						<p class="rule-help">Загружаю доступные AWG3 endpoints…</p>
					{:else if policyOutboundsError}
						<p class="rule-help">Не удалось загрузить VPN endpoints: {policyOutboundsError}</p>
					{:else if policyOutbounds.length === 0}
						<p class="rule-help">Нет доступных AWG3 endpoints. Добавь их через «Туннели → WG endpoints».</p>
					{:else}
						<div class="outbound-tags">{#each policyOutbounds as outbound (outbound.tag)}<code>{outbound.tag}</code>{/each}</div>
					{/if}
				</li>
				<li class="outbounds-row"><strong>DIRECT</strong><p class="rule-help">Напрямую через внешний интерфейс VPS.</p></li>
				<li class="outbounds-row"><strong>BLOCK</strong><p class="rule-help">Отбрасывает совпавший трафик.</p></li>
				<li class="outbounds-row"><strong>WARP</strong><p class="rule-help">Не подключён; действие отключено до появления настроенного outbound.</p></li>
			</ul>
		</section>

		<section class="clients-section" aria-labelledby="clients-title">
			<div class="section-heading">
				<div>
					<p class="eyebrow">PEERS</p>
					<h2 id="clients-title">Клиенты <span class="count">{clients.length}</span></h2>
				</div>
				<button class="quiet" type="button" onclick={() => void loadClients()} disabled={loadingClients}>
					{loadingClients ? 'Обновляю…' : 'Обновить'}
				</button>
			</div>

			{#if loadingClients && clients.length === 0}
				<p class="notice">Загружаю список клиентов…</p>
			{:else if clients.length === 0}
				<p class="notice">Пока нет клиентов. Создай первое устройство выше.</p>
			{:else}
				<div class="client-list">
					{#each clients as client (client.id)}
						<article class="client-card">
							<div class="client-main">
								<div class="client-title-row">
									<h3>{client.label}</h3>
									<span class="status" class:active={client.status === 'active'} class:disabled={client.status === 'disabled'} class:revoked={client.status === 'revoked'}>
										{clientStatusLabel(client.status)}
									</span>
								</div>
								<p class="client-meta"><code>{client.id}</code><span>{client.address}</span></p>
							</div>
							<div class="client-actions">
								{#if client.status === 'active'}
									<button class="quiet" type="button" onclick={() => void openProfile(client)} disabled={profileLoading}>
										{profileLoading ? 'Готовлю…' : 'Профиль / QR'}
									</button>
									<button class="quiet" type="button" onclick={() => void runAction(client, 'disable')} disabled={actionBusy}>Отключить</button>
								{:else if client.status === 'disabled'}
									<button class="quiet" type="button" onclick={() => void runAction(client, 'enable')} disabled={actionBusy}>Включить</button>
								{/if}
								{#if client.status !== 'revoked'}
									<button class="danger" type="button" onclick={() => (revokeTarget = client)} disabled={actionBusy}>Отозвать</button>
								{/if}
							</div>
						</article>
					{/each}
				</div>
			{/if}
		</section>

		<section class="panel groups-panel" aria-labelledby="groups-title">
			<div class="panel-heading">
				<div>
					<p class="eyebrow">ГРУППЫ</p>
					<h2 id="groups-title">Группы клиентов <span class="count">{groups.length}</span></h2>
				</div>
				<button class="quiet" type="button" onclick={() => void loadGroups()} disabled={loadingGroups}>
					{loadingGroups ? 'Обновляю…' : 'Обновить'}
				</button>
			</div>
			<p class="group-intro">Сохраняет состав устройств. Привязка правил маршрутизации к группам пока не подключена.</p>
			<form class="group-form" onsubmit={createGroup}>
				<label>
					<span>Название новой группы</span>
					<input bind:value={groupName} required maxlength="80" autocomplete="off" />
				</label>
				<fieldset class="member-picker">
					<legend>Устройства</legend>
					{#if clients.length === 0}
						<p class="member-empty">Сначала добавь клиента.</p>
					{:else}
						{#each clients as client (client.id)}
							<label class="member-option">
								<input
									type="checkbox"
									checked={newGroupClientIds.includes(client.id)}
									aria-label={`Добавить ${client.label} в новую группу`}
									onchange={(event) => { newGroupClientIds = toggleMember(newGroupClientIds, client.id, (event.currentTarget as HTMLInputElement).checked); }}
								/>
								<span>{client.label} <code>{client.id}</code></span>
							</label>
						{/each}
					{/if}
				</fieldset>
				<button class="primary" type="submit" disabled={groupCreating || !groupName.trim()}>
					{groupCreating ? 'Создаю…' : 'Создать группу'}
				</button>
			</form>

			{#if loadingGroups && groups.length === 0}
				<p class="notice">Загружаю группы…</p>
			{:else if groups.length === 0}
				<p class="notice">Групп пока нет.</p>
			{:else}
				<div class="group-list">
					{#each groups as group (group.id)}
						<article class="group-card">
							{#if editingGroupId === group.id}
								<form class="group-edit-form" onsubmit={saveGroup}>
									<label>
										<span>Название группы</span>
										<input bind:value={editingGroupName} required maxlength="80" autocomplete="off" />
									</label>
									<fieldset class="member-picker">
										<legend>Устройства</legend>
										{#each clients as client (client.id)}
											<label class="member-option">
												<input
													type="checkbox"
													checked={editingGroupClientIds.includes(client.id)}
													aria-label={`Устройство ${client.label}`}
													onchange={(event) => { editingGroupClientIds = toggleMember(editingGroupClientIds, client.id, (event.currentTarget as HTMLInputElement).checked); }}
												/>
												<span>{client.label} <code>{client.id}</code></span>
											</label>
										{/each}
									</fieldset>
									<div class="group-actions">
										<button class="primary" type="submit" disabled={groupActionId !== '' || !editingGroupName.trim()}>
											{groupActionId === group.id ? 'Сохраняю…' : 'Сохранить группу'}
										</button>
										<button class="quiet" type="button" onclick={() => { editingGroupId = ''; }}>Отмена</button>
									</div>
								</form>
							{:else}
								<div class="group-summary">
									<h3>{group.name}</h3>
									<p class="group-members">{groupClientLabels(group)}</p>
								</div>
								<div class="group-actions">
									<button class="quiet" type="button" aria-label={`Изменить группу ${group.name}`} onclick={() => startEditGroup(group)}>Изменить</button>
									<button class="danger" type="button" aria-label={`Удалить группу ${group.name}`} disabled={groupActionId !== ''} onclick={() => void deleteGroup(group)}>
										{groupActionId === group.id ? 'Удаляю…' : 'Удалить'}
									</button>
								</div>
							{/if}
						</article>
					{/each}
				</div>
			{/if}
		</section>
	{/if}
</PageContainer>

{#if profileClient}
	<div class="overlay" role="presentation" onclick={(event) => { if (event.target === event.currentTarget) clearProfile(); }}>
		<div class="dialog profile-dialog" role="dialog" aria-modal="true" aria-labelledby="profile-title" tabindex="-1">
			<button class="close" type="button" aria-label="Закрыть профиль" onclick={clearProfile}>×</button>
			<p class="eyebrow">КЛИЕНТСКИЙ ПРОФИЛЬ</p>
			<h2 id="profile-title">{profileClient.label}</h2>
			<p class="dialog-copy">Отсканируй QR в WireGuard/AWG или скачай файл. Профиль содержит приватный ключ этого устройства.</p>
			{#if profileQr}
				<img class="qr" src={profileQr} alt="QR-код конфигурации для {profileClient.label}" width="360" height="360" />
			{/if}
			<div class="dialog-actions">
				<button class="primary" type="button" onclick={downloadProfile}>Скачать .conf</button>
				<button class="quiet" type="button" onclick={clearProfile}>Закрыть</button>
			</div>
		</div>
	</div>
{/if}

{#if revokeTarget}
	<div class="overlay" role="presentation" onclick={(event) => { if (event.target === event.currentTarget) revokeTarget = null; }}>
		<div class="dialog" role="dialog" aria-modal="true" aria-labelledby="revoke-title" tabindex="-1">
			<p class="eyebrow">НЕОБРАТИМОЕ ДЕЙСТВИЕ</p>
			<h2 id="revoke-title">Отозвать доступ?</h2>
			<p class="dialog-copy">Устройство «{revokeTarget.label}» будет отключено. Отозванный клиент не сможет быть включён снова — для возврата доступа создай новый профиль.</p>
			<div class="dialog-actions">
				<button class="danger" type="button" onclick={() => void runAction(revokeTarget!, 'revoke')} disabled={actionBusy}>
					{actionBusy ? 'Отзываю…' : 'Отозвать навсегда'}
				</button>
				<button class="quiet" type="button" onclick={() => (revokeTarget = null)} disabled={actionBusy}>Отмена</button>
			</div>
		</div>
	</div>
{/if}

<style>
	:global(.main) { min-width: 0; }
	.panel, .client-card, .subscription-card, .notice, .dialog {
		border: 1px solid var(--border);
		border-radius: 16px;
		background: var(--card);
		color: var(--foreground);
	}
	.panel { padding: 1.25rem; margin-bottom: 2rem; }
	.panel-heading, .section-heading, .client-title-row, .client-meta, .client-actions, .dialog-actions {
		display: flex; align-items: center; justify-content: space-between; gap: .75rem;
	}
	.panel-heading, .section-heading { margin-bottom: 1rem; }
	h2, h3, p { margin: 0; }
	h2 { font-size: 1.25rem; letter-spacing: -.02em; }
	h3 { font-size: 1rem; font-weight: 600; }
	.eyebrow { color: var(--muted-foreground); font: 600 .68rem/1.3 var(--font-mono, monospace); letter-spacing: .12em; margin-bottom: .35rem; }
	.pool-note, .client-meta, .dialog-copy { color: var(--muted-foreground); font-size: .875rem; }
	.create-form { display: grid; grid-template-columns: minmax(150px, .8fr) minmax(180px, 1.2fr) auto; align-items: end; gap: .8rem; }
	.subscription-intro { color: var(--muted-foreground); font-size: .88rem; margin: -.35rem 0 1rem; }
	.subscription-form { display: grid; grid-template-columns: minmax(180px, 1.4fr) minmax(110px, .55fr) auto; align-items: end; gap: .8rem; }
	.group-intro { color: var(--muted-foreground); font-size: .88rem; margin: -.35rem 0 1rem; }
	.policy-intro { color: var(--muted-foreground); font-size: .88rem; margin: -.35rem 0 1rem; }
	.policy-status { color: #23a67a; font-size: .85rem; margin: -.45rem 0 1rem; }
	.policy-form { display: grid; grid-template-columns: minmax(130px, .75fr) minmax(160px, 1fr) minmax(150px, .8fr) auto; align-items: end; gap: .8rem; }
	.policy-list { display: grid; gap: .65rem; margin: 1rem 0 0; padding: 0; list-style: none; }
	.policy-card { display: flex; align-items: center; justify-content: space-between; gap: 1rem; padding: .9rem 1rem; border: 1px solid var(--border); border-radius: 12px; }
	.outbounds-list { display: grid; gap: .55rem; margin: 0; padding: 0; list-style: none; }
	.outbounds-row { display: grid; grid-template-columns: minmax(5rem, .25fr) minmax(0, 1fr); align-items: start; gap: .8rem; padding: .75rem; border: 1px solid var(--border); border-radius: 10px; }
	.outbounds-row > strong { font-size: .82rem; }
	.outbound-tags { display: flex; flex-wrap: wrap; gap: .4rem; }
	.outbound-tags code { padding: .2rem .45rem; border: 1px solid var(--border); border-radius: 6px; font-size: .78rem; overflow-wrap: anywhere; }
	.policy-rule-editor { display: grid; gap: .8rem; margin-top: 1.1rem; padding: 1rem; border: 1px solid var(--border); border-radius: 12px; }
	.rule-help { color: var(--muted-foreground); font-size: .82rem; line-height: 1.45; }
	.policy-rule-list { display: grid; gap: .55rem; margin: 0; padding: 0; list-style: none; }
	.policy-rule-card { display: flex; align-items: center; justify-content: space-between; gap: 1rem; padding: .75rem; border: 1px solid var(--border); border-radius: 10px; }
	.policy-rule-card > div:first-child { display: grid; gap: .25rem; min-width: 0; }
	.policy-rule-card span { color: var(--muted-foreground); font-size: .78rem; overflow-wrap: anywhere; }
	.policy-preview { display: grid; gap: .65rem; margin-top: 1rem; padding: 1rem; border: 1px solid color-mix(in srgb, var(--accent) 45%, var(--border)); border-radius: 12px; }
	.policy-preview > p, .policy-preview li { color: var(--muted-foreground); font-size: .84rem; line-height: 1.5; overflow-wrap: anywhere; }
	.policy-preview ul { display: grid; gap: .35rem; margin: 0; padding-left: 1.25rem; }
	.policy-preview code { color: var(--foreground); }
	.policy-preview details { color: var(--muted-foreground); font-size: .8rem; }
	.policy-preview pre { max-height: 18rem; overflow: auto; padding: .8rem; border: 1px solid var(--border); border-radius: 9px; font: .75rem/1.45 var(--font-mono, monospace); }
	.rule-form { display: grid; gap: .8rem; padding-top: .5rem; }
	.rule-fields { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: .75rem; }
	.rule-form-actions { display: flex; flex-wrap: wrap; gap: .6rem; }
	.policy-summary { min-width: 0; }
	.policy-summary p { display: flex; flex-wrap: wrap; gap: .75rem; margin-top: .35rem; color: var(--muted-foreground); font-size: .8rem; }
	.policy-summary code { overflow-wrap: anywhere; }
	.policy-actions { display: flex; align-items: center; gap: .55rem; flex-wrap: wrap; }
	.group-form { display: grid; grid-template-columns: minmax(180px, .8fr) minmax(220px, 1.2fr) auto; align-items: end; gap: .8rem; }
	.group-list { display: grid; gap: .7rem; margin-top: 1rem; }
	.group-card { display: flex; align-items: center; justify-content: space-between; gap: 1rem; padding: 1rem 1.1rem; border: 1px solid var(--border); border-radius: 12px; }
	.group-summary { min-width: 0; }
	.group-members { margin-top: .4rem; color: var(--muted-foreground); font-size: .82rem; overflow-wrap: anywhere; }
	.group-edit-form { display: grid; gap: .75rem; width: 100%; }
	.group-actions { display: flex; align-items: center; justify-content: flex-end; gap: .6rem; flex-wrap: wrap; }
	.member-picker { display: grid; gap: .45rem; min-width: 0; max-height: 12rem; overflow: auto; margin: 0; padding: .65rem .75rem; border: 1px solid var(--border); border-radius: 10px; }
	.member-picker legend { padding: 0 .2rem; color: var(--muted-foreground); font-size: .78rem; }
	.member-option { display: flex; align-items: center; gap: .55rem; min-width: 0; color: var(--foreground); font-size: .8rem; cursor: pointer; }
	.member-option input { width: 1rem; height: 1rem; margin: 0; padding: 0; accent-color: var(--accent); }
	.member-option span { min-width: 0; overflow-wrap: anywhere; }
	.member-option code { color: var(--muted-foreground); font-size: .75rem; }
	.member-empty { color: var(--muted-foreground); font-size: .8rem; }
	label { display: grid; gap: .4rem; font-size: .8rem; color: var(--muted-foreground); }
	input, select { min-width: 0; border: 1px solid var(--border); border-radius: 9px; padding: .68rem .75rem; color: var(--foreground); background: var(--card); font: inherit; }
	.one-time-link { display: grid; gap: .75rem; margin-top: 1rem; padding: 1rem; border: 1px solid color-mix(in srgb, var(--accent) 45%, var(--border)); border-radius: 12px; }
	.subscription-secret-note { color: var(--muted-foreground); font-size: .85rem; line-height: 1.45; }
	.one-time-link input { width: 100%; font-family: var(--font-mono, monospace); font-size: .76rem; }
	button { border: 1px solid var(--border); border-radius: 9px; padding: .62rem .85rem; color: var(--foreground); background: transparent; font: inherit; font-size: .82rem; font-weight: 600; line-height: 1.2; cursor: pointer; transition: border-color .15s, background .15s, opacity .15s; }
	button:hover:not(:disabled) { border-color: var(--accent); }
	button:disabled { opacity: .55; cursor: wait; }
	button.primary { background: var(--accent); border-color: var(--accent); color: #fff; }
	button.danger { color: #e06a68; }
	button.quiet { color: var(--muted-foreground); }
	.client-list { display: grid; gap: .7rem; }
	.subscription-list { display: grid; gap: .7rem; margin-top: 1rem; }
	.subscription-card { display: flex; align-items: center; justify-content: space-between; gap: 1rem; padding: 1rem 1.1rem; }
	.subscription-main { min-width: 0; }
	.client-card { display: flex; align-items: center; justify-content: space-between; gap: 1rem; padding: 1rem 1.1rem; }
	.client-main { min-width: 0; }
	.client-title-row { justify-content: flex-start; flex-wrap: wrap; }
	.client-meta { justify-content: flex-start; gap: 1rem; margin-top: .45rem; font-size: .78rem; }
	.client-meta code { color: var(--muted-foreground); }
	.count { color: var(--muted-foreground); font-size: .85rem; font-weight: 500; }
	.status { border-radius: 99px; padding: .22rem .55rem; font-size: .72rem; background: color-mix(in srgb, var(--muted-foreground) 13%, transparent); }
	.status.active { color: #23a67a; background: color-mix(in srgb, #23a67a 14%, transparent); }
	.status.disabled { color: #c58b27; background: color-mix(in srgb, #c58b27 14%, transparent); }
	.status.revoked { color: #e06a68; background: color-mix(in srgb, #e06a68 14%, transparent); }
	.client-actions { justify-content: flex-end; flex-wrap: wrap; }
	.notice { padding: 1.1rem 1.25rem; color: var(--muted-foreground); }
	.notice h2 { color: var(--foreground); margin-bottom: .45rem; }
	.notice.error { border-color: color-mix(in srgb, #e06a68 60%, var(--border)); color: #e06a68; margin-bottom: 1rem; }
	.notice code { color: var(--foreground); }
	.overlay { position: fixed; inset: 0; z-index: 1000; display: grid; place-items: center; padding: 1rem; background: rgb(0 0 0 / 58%); }
	.dialog { position: relative; width: min(100%, 480px); padding: 1.4rem; box-shadow: 0 24px 80px rgb(0 0 0 / 30%); }
	.dialog h2 { margin-bottom: .55rem; }
	.dialog-copy { line-height: 1.5; }
	.profile-dialog { text-align: center; }
	.profile-dialog .eyebrow { text-align: left; }
	.profile-dialog h2 { text-align: left; }
	.profile-dialog .dialog-copy { text-align: left; }
	.qr { display: block; width: min(100%, 360px); height: auto; aspect-ratio: 1; object-fit: contain; margin: 1rem auto; padding: .5rem; border-radius: 12px; background: #fff; }
	.dialog-actions { justify-content: flex-end; margin-top: 1rem; }
	.close { position: absolute; top: .7rem; right: .7rem; width: 2rem; height: 2rem; padding: 0; font-size: 1.35rem; }
	@media (max-width: 680px) {
		.create-form { grid-template-columns: 1fr; }
		.subscription-form { grid-template-columns: 1fr; }
		.group-form { grid-template-columns: 1fr; }
		.policy-form { grid-template-columns: 1fr; }
		.policy-card { align-items: flex-start; flex-direction: column; }
		.policy-rule-card { align-items: flex-start; flex-direction: column; }
		.rule-fields { grid-template-columns: 1fr; }
		.client-card { align-items: flex-start; flex-direction: column; }
		.subscription-card { align-items: flex-start; flex-direction: column; }
		.group-card { align-items: stretch; flex-direction: column; }
		.client-actions { justify-content: flex-start; }
		.group-actions { justify-content: flex-start; }
		.panel-heading { align-items: flex-start; flex-direction: column; }
	}
</style>
