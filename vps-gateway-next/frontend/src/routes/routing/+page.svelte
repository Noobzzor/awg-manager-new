<script lang="ts">
    import { onMount, onDestroy } from 'svelte';
    import { get } from 'svelte/store';
    import { goto } from '$app/navigation';
    import { browser } from '$app/environment';
    import { page } from '$app/stores';
    import {
        routing,
        subscribeRouting,
        invalidateAllRouting,
        routingDnsNdmsTabReady,
        routingIpTabReady,
        routingClientVpnTabReady,
        hydrarouteStatusStore,
    } from '$lib/stores/routing';
    import { singboxRouter as singboxRouterStore } from '$lib/stores/singboxRouter';
    import { systemInfo } from '$lib/stores/system';
    import { api } from '$lib/api/client';
    import { notifications } from '$lib/stores/notifications';
    import { PageContainer, PageHeader } from '$lib/components/layout';
    import { Search } from 'lucide-svelte';
    import { Tabs, Button, Modal } from '$lib/components/ui';
    import { RoutingSearch } from '$lib/components/routing';
    import DnsRoutesTab from './DnsRoutesTab.svelte';
    import IpRoutesTab from './IpRoutesTab.svelte';
    import AccessPoliciesTab from './AccessPoliciesTab.svelte';
    import ClientRoutesTab from './ClientRoutesTab.svelte';
    import { HrNeoTab } from '$lib/components/hrneo';
    import { SingboxRouterRedesignPage } from '$lib/components/sb-router';
    import FakeIPTab from '$lib/components/fakeip/FakeIPTab.svelte';
    import ModeSwitchHost from '$lib/components/routing/ModeSwitchHost.svelte';
    import { modeSwitch, modeSwitchBusy } from '$lib/stores/modeSwitch';
    import GeoDataTab from './GeoDataTab.svelte';
    import { isRoutingSubTabVisible, type RoutingSubTab, type UsageLevel } from '$lib/types/usageLevel';
    import { usageLevel } from '$lib/stores/settings';
    import { dnsRouteTabCapability, type DnsRouteBackend } from '$lib/utils/dnsRouteBackend';
    import type { RuntimeCapabilities } from '$lib/utils/runtimeCapabilities';

    // Per-section polling stores — subscribe here so all 8 fetch while
    // the routing page is open. Unsubscribed on destroy to stop polling.
    let unsubRouting: (() => void) | null = null;
    let advertisedDnsBackend = $state<DnsRouteBackend | null>(null);
    let advertisedNdms = $state<boolean | null>(null);
    let advertisedSingboxRouter = $state<boolean | null>(null);
    let advertisedHydraroute = $state<boolean | null>(null);

    async function loadCapabilities(): Promise<RuntimeCapabilities | null> {
        try {
            const response = await fetch('/api/capabilities');
            if (!response.ok) return null;
            const body = await response.json();
            const capabilities = body.data as RuntimeCapabilities;
            const backend = body.data?.dnsRouteBackend;
            if (typeof body.data?.ndms === 'boolean') advertisedNdms = body.data.ndms;
            if (typeof body.data?.singboxRouter === 'boolean') advertisedSingboxRouter = body.data.singboxRouter;
            if (typeof body.data?.hydraroute === 'boolean') advertisedHydraroute = body.data.hydraroute;
            if (backend === 'ndms' || backend === 'hydraroute' || backend === 'singbox') {
                advertisedDnsBackend = backend;
            }
            return capabilities;
        } catch {
            // Older Keenetic builds have no capability endpoint; the OS5 fallback below preserves them.
            return null;
        }
    }

    async function initializeRouting(): Promise<void> {
        const capabilities = await loadCapabilities();
        unsubRouting = subscribeRouting({
            ndms: capabilities?.ndms ?? true,
            hydraroute: capabilities?.hydraroute ?? capabilities?.ndms ?? true,
        });
        const singboxRouterSupported = capabilities?.singboxRouter ?? capabilities?.ndms ?? true;
        if (singboxRouterSupported) {
            // Prime router-only state only when the backend mounted that API.
            void singboxRouterStore.reloadStatus();
            void singboxRouterStore.reloadSettings();
        }
    }

    onMount(() => {
        // Legacy URL: standalone «Прокси для устройств» → Expert Inbounds в Sing-box Router.
        const sp = new URLSearchParams($page.url.search);
        if (sp.get('tab') === 'deviceproxy') {
            sp.set('tab', 'singbox');
            sp.set('mode', 'expert');
            sp.delete('sub');
            goto(`?${sp.toString()}`, { replaceState: true });
        }
        void initializeRouting();
    });
    onDestroy(() => {
        unsubRouting?.();
    });

    let activeTab = $state<'hrneo' | 'geodata' | 'dns' | 'ip' | 'policy' | 'clientvpn' | 'singbox' | 'fakeip'>('dns');

    // ?policy=Policy1 — прямой переход из настроек sing-box в редактор
    // конкретной политики (#573).
    let deepLinkPolicy = $derived($page.url.searchParams.get('policy'));

    let isOS5 = $derived($systemInfo.data?.isOS5 ?? false);
    let dnsRouteBackend = $derived<DnsRouteBackend | null>(advertisedDnsBackend ?? (isOS5 ? 'ndms' : null));
    let dnsTab = $derived(dnsRouteTabCapability(dnsRouteBackend));
    let showLegacyRoutingTabs = $derived(advertisedNdms !== false);
    let hydrarouteInstalled = $derived(
        advertisedHydraroute !== false && ($routing.hydrarouteStatus?.installed ?? false),
    );
    let hasDnsEngine = $derived(dnsTab.visible || hydrarouteInstalled);
    let singboxInstalled = $derived(
        advertisedSingboxRouter !== false && ($systemInfo.data?.singbox?.installed ?? false),
    );

    let pendingTab = $state<string | null>(null);

    function requestTab(id: string): void {
        if (modeSwitchBusy(get(modeSwitch))) return;
        const hasDraft = get(singboxRouterStore.staging)?.hasDraft ?? false;
        if (activeTab === 'singbox' && id !== 'singbox' && hasDraft) {
            pendingTab = id;
            return;
        }
        activeTab = id as typeof activeTab;
    }
    function confirmLeave(): void {
        if (pendingTab) activeTab = pendingTab as typeof activeTab;
        pendingTab = null;
    }

    // Search → edit rule integration
    let editRuleId = $state('');
    let editRuleCounter = $state(0);
    let searchOpen = $state(false);

    function handleSearchRuleClick(id: string, type: 'dns' | 'ip') {
        if (type === 'dns') {
            // dnsRoutes mixes NDMS and hydraroute backends in one array;
            // route hydraroute hits to the HR Neo tab so the edit modal
            // actually opens (DnsRoutesTab filters those out).
            const route = dnsRoutes.find(r => r.id === id);
            activeTab = route?.backend === 'hydraroute' ? 'hrneo' : 'dns';
        } else {
            activeTab = 'ip';
        }
        editRuleId = id;
        editRuleCounter++;
        searchOpen = false;
    }

    // NDMS tab is OS5-only (see tabItems gate). On OS4, bounce off `dns`
    // to HR Neo when hydraroute is installed, otherwise IP.
    $effect(() => {
        if (!$systemInfo.data) return;
        const hr = $hydrarouteStatusStore;
        if (hr.lastFetchedAt === 0 && hr.status !== 'error') return;

        if (!dnsTab.visible && activeTab === 'dns') {
            activeTab = hydrarouteInstalled ? 'hrneo' : 'ip';
        }
    });

    // In fakeip-tun mode, land on the FakeIP tab instead of the tproxy-
    // oriented default — the tproxy view would show the engine as "running"
    // while the tproxy slot is disabled, which is misleading. The FakeIP UI
    // now lives as a tab on THIS page, so we just select it (activeTab is
    // the page's tab source-of-truth; the Tabs component syncs ?tab=fakeip
    // outbound). We deliberately do NOT goto('/fakeip') — that route now
    // bounces back to /routing?tab=fakeip and would create an infinite loop.
    //
    // One-shot (fakeipAutoSelected) so a manual switch to another tab sticks,
    // and skipped when the URL already carries an explicit ?tab= (deep-link)
    // so we never override a user's chosen tab. Guarded on singboxInstalled
    // (the same condition that renders the tab) so we never select a tab that
    // isn't there — fakeip-tun implies sing-box installed, but this keeps the
    // selection from racing ahead of systemInfo arriving.
    const singboxInitializedStore = singboxRouterStore.initialized;
    const singboxSettings = singboxRouterStore.settings;
    let fakeipAutoSelected = false;
    $effect(() => {
        if (!browser) return;
        if (!$singboxInitializedStore) return;
        if (!singboxInstalled) return;
        if (fakeipAutoSelected) return;
        if ($singboxSettings?.routingMode === 'fakeip-tun') {
            fakeipAutoSelected = true;
            const explicitTab = new URL(window.location.href).searchParams.get('tab');
            if (!explicitTab) {
                activeTab = 'fakeip';
            }
        }
    });

    // Data from SSE-driven store
    let dnsRoutes = $derived($routing.dnsRoutes);
    let ipRoutes = $derived($routing.staticRoutes);
    let accessPolicies = $derived($routing.accessPolicies);
    let policyDevices = $derived($routing.policyDevices);
    let policyInterfaces = $derived($routing.policyInterfaces);
    let clientRoutes = $derived($routing.clientRoutes);
    let routingTunnels = $derived($routing.tunnels);
    let missing = $derived($routing.missing);

    let refreshing = $state(false);
    async function handleRefresh() {
        if (refreshing) return;
        refreshing = true;
        try {
            const res = await api.refreshRouting();
            // Force every section store to refetch now (the backend also
            // posts resource:invalidated hints, but a local kick keeps the
            // UI responsive even if SSE happens to be lagging).
            invalidateAllRouting();
            if (res.missing.length === 0) {
                notifications.success('Данные получены');
            } else {
                notifications.warning(`Не удалось загрузить: ${res.missing.join(', ')}`);
            }
        } catch (e) {
            notifications.error(`Ошибка обновления: ${(e as Error).message}`);
        } finally {
            refreshing = false;
        }
    }

    // Derived: tab badges
    let hrRuleCount = $derived(dnsRoutes.filter(r => r.backend === 'hydraroute').length);
    let geoFileCount = $state(0);

    async function loadGeoFileCount() {
        if (advertisedHydraroute === false && advertisedSingboxRouter === false) {
            geoFileCount = 0;
            return;
        }
        try {
            const files = await api.getGeoFiles();
            geoFileCount = files?.length ?? 0;
        } catch {
            geoFileCount = 0;
        }
    }

    $effect(() => {
        if (hydrarouteInstalled || singboxInstalled) void loadGeoFileCount();
        else geoFileCount = 0;
    });
    let dnsActiveCount = $derived(dnsRoutes.filter(r => r.enabled && r.backend !== 'hydraroute').length);
    let ipActiveCount = $derived(ipRoutes.filter(r => r.enabled).length);
    let clientActiveCount = $derived(clientRoutes.filter(r => r.enabled).length);
    let policyCount = $derived(accessPolicies.length);

    type TabChildItem = {
        id: string;
        label: string;
        badge?: number | string;
        badgeTone?: 'default' | 'success' | 'warning' | 'muted';
    };

    type TabItem = {
        id: string;
        label: string;
        badge?: number | string;
        badgeTone?: 'default' | 'success' | 'warning' | 'muted';
        separatorBefore?: boolean;
        muted?: boolean;
        children?: TabChildItem[];
    };

    const TAB_TO_SUBTAB: Record<string, RoutingSubTab> = {
        policy: 'accessPolicies',
        clientvpn: 'clientRoutes',
        dns: 'dnsRoutes',
        ip: 'ipRoutes',
        hrneo: 'hrNeo',
        geodata: 'geoData',
        singbox: 'singboxRouter',
    };

    function tabVisible(localId: string, level?: UsageLevel): boolean {
        const sub = TAB_TO_SUBTAB[localId];
        const lvl = level ?? $usageLevel;
        return sub ? isRoutingSubTabVisible(lvl, sub) : true;
    }

    function tabLeafIds(tab: TabItem): string[] {
        return tab.children?.map((c) => c.id) ?? [tab.id];
    }

    function tabsInclude(items: TabItem[], id: string): boolean {
        return items.some((it) => tabLeafIds(it).includes(id));
    }

    const singboxRouterStatus = singboxRouterStore.status;
    let singboxRuleCount = $derived($singboxRouterStatus?.ruleCount ?? 0);

    const showSingboxTproxy = $derived(
        advertisedSingboxRouter !== false && singboxInstalled && tabVisible('singbox'),
    );
    // FakeIP is expert-gated (mirrors the 'singbox' tab's 'expert' level) BUT
    // stays visible whenever the engine is actually in fakeip-tun mode — that's
    // the in-use case the auto-select effect lands on, and hiding the chip there
    // would strand activeTab on a tab with no chip to navigate back from.
    const showSingboxFakeip = $derived(
        advertisedSingboxRouter !== false &&
            singboxInstalled &&
            (tabVisible('singbox') || $singboxSettings?.routingMode === 'fakeip-tun'),
    );
    const singboxMenuChildren = $derived(
        (
            [
                showSingboxTproxy
                    ? { id: 'singbox', label: 'TProxy', badge: singboxRuleCount }
                    : null,
                showSingboxFakeip ? { id: 'fakeip', label: 'FakeIP' } : null,
            ] as (TabChildItem | null)[]
        ).filter((c): c is TabChildItem => c !== null),
    );

    let tabItems = $derived(
        ([
            // NDMS dns-proxy with object-group fqdn is OS5-only — gate the
            // tab on isOS5 so OS4 routers don't see an unusable NDMS tab
            // (hydraroute users on OS4 use the HR Neo tab instead).
            dnsTab.visible ? { id: 'dns', label: dnsTab.label, badge: dnsActiveCount } : null,
            showLegacyRoutingTabs ? { id: 'ip', label: 'IP-адреса', badge: ipActiveCount } : null,
            showLegacyRoutingTabs ? { id: 'clientvpn', label: 'VPN для устройств', badge: clientActiveCount } : null,
            showLegacyRoutingTabs ? { id: 'policy', label: 'Политики доступа', badge: policyCount } : null,
            // Sing-box modes as one dropdown chip (same pattern as tunnels page).
            singboxMenuChildren.length > 0
                ? {
                        id: singboxMenuChildren[0].id,
                        label: 'Sing-box',
                        separatorBefore: true,
                        children: singboxMenuChildren,
                    }
                : null,
            // HR Neo is a separate routing engine (not sing-box) — divider before it.
            hydrarouteInstalled ? { id: 'hrneo', label: 'HR Neo', badge: hrRuleCount, separatorBefore: true } : null,
            (hydrarouteInstalled || singboxInstalled)
                ? { id: 'geodata', label: 'Гео-данные', badge: geoFileCount, separatorBefore: true }
                : null,
        ] as (TabItem | null)[])
            .filter((t): t is TabItem => t !== null)
            .filter((t) => (t.children ? true : tabVisible(t.id)))
    );

    // If the user deep-linked / had the tab active and sing-box disappeared
    // (uninstall while the page is open), bounce them off.
    $effect(() => {
        if (!$systemInfo.data) return;
        if (!singboxInstalled && (activeTab === 'singbox' || activeTab === 'fakeip')) {
            activeTab = 'dns';
        }
    });

    // Пока список вкладок меняется (systemInfo, HR, уровень), не держим
    // active на id, которого ещё нет в tabItems — иначе пустой контент.
    // Не сбрасываем NDMS/sing-box до прихода systemInfo: до fetch
    // isOS5=false и вкладки dns ещё нет в списке — иначе F5 с NDMS
    // уводил на IP. Аналогично HR Neo — ждём hydraroute-status.
    $effect(() => {
        const items = tabItems;
        if (items.length === 0) return;

        const si = $systemInfo;
        const systemKnown = si.lastFetchedAt > 0 || si.status === 'error';
        const hr = $hydrarouteStatusStore;
        const hrKnown = hr.lastFetchedAt > 0 || hr.status === 'error';

        if (
            !systemKnown &&
            (activeTab === 'dns' || activeTab === 'singbox' || activeTab === 'fakeip') &&
            !tabsInclude(items, activeTab)
        ) {
            return;
        }
        if (
            !hrKnown &&
            (activeTab === 'hrneo' || activeTab === 'geodata') &&
            !tabsInclude(items, activeTab)
        ) {
            return;
        }

        if (!tabsInclude(items, activeTab)) {
            const first = items[0];
            activeTab = (first.children?.[0]?.id ?? first.id) as typeof activeTab;
        }
    });

</script>

<svelte:head>
    <title>Маршрутизация - AWG Manager</title>
</svelte:head>

<PageContainer width="full">
    <div class="routing-page">
    <PageHeader title="Маршрутизация">
        {#snippet actions()}
            <Button
                variant="secondary"
                size="md"
                onclick={() => (searchOpen = true)}
                iconBefore={searchIcon}
            >
                Поиск
            </Button>
            <!-- TODO Phase 1: warning variant for missing>0 -->
            <Button
                variant="secondary"
                size="md"
                onclick={handleRefresh}
                disabled={refreshing}
                loading={refreshing}
            >
                {#if missing.length > 0}
                    Загрузить недостающее ({missing.length})
                {:else}
                    Обновить
                {/if}
            </Button>
        {/snippet}
    </PageHeader>

    <Tabs
        tabs={tabItems}
        active={activeTab}
        onchange={(id) => requestTab(id)}
        urlParam="tab"
        defaultTab="dns"
    />

    {#if activeTab === 'hrneo'}
        <HrNeoTab
            {dnsRoutes}
            tunnels={routingTunnels}
            policies={accessPolicies}
            {policyInterfaces}
            {editRuleId}
            {editRuleCounter}
        />
    {:else if activeTab === 'dns'}
        <DnsRoutesTab
            {dnsRoutes}
            {routingTunnels}
            {editRuleId}
            {editRuleCounter}
            {isOS5}
            dnsBackend={dnsRouteBackend ?? 'ndms'}
            {hasDnsEngine}
            bodyLoading={!$routingDnsNdmsTabReady}
        />
    {:else if activeTab === 'ip'}
        <IpRoutesTab
            {ipRoutes}
            {routingTunnels}
            {editRuleId}
            {editRuleCounter}
            bodyLoading={!$routingIpTabReady}
        />
    {:else if activeTab === 'policy'}
            <AccessPoliciesTab
                {accessPolicies}
                {policyDevices}
                {policyInterfaces}
                missing={missing.includes('accessPolicies')}
                openPolicy={deepLinkPolicy}
            />
    {:else if activeTab === 'clientvpn'}
        <ClientRoutesTab
            {clientRoutes}
            {policyDevices}
            {routingTunnels}
            bodyLoading={!$routingClientVpnTabReady}
        />
    {:else if activeTab === 'geodata'}
        <GeoDataTab />
    {:else if activeTab === 'singbox'}
        <SingboxRouterRedesignPage />
    {:else if activeTab === 'fakeip'}
        <FakeIPTab />
    {/if}
    <ModeSwitchHost />
    </div>
</PageContainer>

<Modal
    open={pendingTab !== null}
    title="Несохранённые правки маршрутизации"
    size="sm"
    onclose={() => (pendingTab = null)}
>
    <p>Правки sing-box сохранены как черновик, но <strong>ещё не применены</strong>. Если уйти с вкладки — маршрутизация не изменится, пока вы не нажмёте «Применить».</p>
    {#snippet actions()}
        <Button variant="ghost" size="md" onclick={() => (pendingTab = null)}>Остаться</Button>
        <Button variant="primary" size="md" onclick={confirmLeave}>Уйти всё равно</Button>
    {/snippet}
</Modal>

<Modal
    open={searchOpen}
    onclose={() => (searchOpen = false)}
    title={dnsRouteBackend === 'ndms' ? 'Поиск по правилам маршрутизации NDMS' : 'Поиск по правилам DNS-маршрутизации'}
    size="xl"
>
    <RoutingSearch
        {dnsRoutes}
        staticRoutes={ipRoutes}
        tunnels={routingTunnels}
        onRuleClick={handleSearchRuleClick}
    />
</Modal>

{#snippet searchIcon()}
    <Search size={14} strokeWidth={2} aria-hidden="true" />
{/snippet}

<style>
	@media (max-width: 640px) {
		.routing-page :global(.page-header .actions) {
			display: grid;
			grid-template-columns: repeat(2, minmax(0, 1fr));
			align-items: stretch;
			gap: 0.5rem;
			width: 100%;
		}

		.routing-page :global(.page-header .actions .btn) {
			width: 100%;
			min-width: 0;
			justify-content: center;
		}
	}
</style>
