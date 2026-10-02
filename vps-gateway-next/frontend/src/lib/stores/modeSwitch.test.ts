import { describe, it, expect, vi, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import type { SingboxRouterStatus, SingboxRouterSettings } from '$lib/types';

// Тесты кормят currentMode и стор частичными фикстурами (читаются только
// enabled/routingMode); один явный каст в хелпере вместо рассыпанных as any.
const asStatus = (p: Partial<SingboxRouterStatus>) => p as SingboxRouterStatus;
const asSettings = (p: Partial<SingboxRouterSettings>) => p as SingboxRouterSettings;

// vi.hoisted: vi.mock factories are hoisted above these declarations, so the
// shared spy fns must be hoisted too (vitest only auto-hoists `mock*`-prefixed
// vars). Keeps the spies in lexical scope for both the factories and the tests.
const { switchMode, begin, fail, reset, loadAll, refs } = vi.hoisted(() => ({
	switchMode: vi.fn().mockResolvedValue(undefined),
	begin: vi.fn(),
	fail: vi.fn(),
	reset: vi.fn(),
	loadAll: vi.fn().mockResolvedValue(undefined),
	refs: {
		statusVal: null as Partial<SingboxRouterStatus> | null,
		settingsVal: null as Partial<SingboxRouterSettings> | null,
	},
}));
vi.mock('$lib/api/client', () => ({ api: { singboxRouterSwitchMode: (m: string) => switchMode(m) } }));
vi.mock('$lib/stores/fakeipTransition', () => ({ fakeipTransition: { begin, fail, reset } }));
vi.mock('$lib/stores/singboxRouter', () => ({
	singboxRouter: {
		status: {
			subscribe: (run: (v: Partial<SingboxRouterStatus> | null) => void) => {
				run(refs.statusVal);
				return () => {};
			},
		},
		settings: {
			subscribe: (run: (v: Partial<SingboxRouterSettings> | null) => void) => {
				run(refs.settingsVal);
				return () => {};
			},
		},
		loadAll,
	},
}));

import { modeSwitch, currentMode, modeSwitchBusy } from './modeSwitch';

beforeEach(() => {
	// Reset store state FIRST (these call mocked reset/loadAll), THEN clear counters,
	// so the reset/loadAll from teardown don't pollute the next test's call counts.
	modeSwitch.closeProgress();
	modeSwitch.cancel();
	switchMode.mockClear(); begin.mockClear(); fail.mockClear(); reset.mockClear(); loadAll.mockClear();
});

describe('currentMode', () => {
	it('off when not enabled (ignores stale routingMode)', () => {
		expect(
			currentMode(asStatus({ enabled: false }), asSettings({ routingMode: 'fakeip-tun' })),
		).toBe('off');
	});
	it('routingMode when enabled', () => {
		expect(currentMode(asStatus({ enabled: true }), asSettings({ routingMode: 'tproxy' }))).toBe(
			'tproxy',
		);
	});
	it('defaults to tproxy when enabled but routingMode missing', () => {
		expect(currentMode(asStatus({ enabled: true }), asSettings({}))).toBe('tproxy');
	});
});

describe('modeSwitch store', () => {
	it('request → confirming with computed from/target', () => {
		refs.statusVal = { enabled: true }; refs.settingsVal = { routingMode: 'fakeip-tun' };
		modeSwitch.request('off');
		const s = get(modeSwitch);
		expect(s.phase).toBe('confirming');
		expect(s.from).toBe('fakeip-tun');
		expect(s.target).toBe('off');
		expect(modeSwitchBusy(s)).toBe(true);
	});
	it('request is a no-op when target === current mode', () => {
		refs.statusVal = { enabled: true }; refs.settingsVal = { routingMode: 'tproxy' };
		modeSwitch.request('tproxy');
		expect(get(modeSwitch).phase).toBe('idle');
	});
	it('confirm → running, begins transition, posts SwitchMode(target)', async () => {
		refs.statusVal = { enabled: true }; refs.settingsVal = { routingMode: 'tproxy' };
		modeSwitch.request('fakeip-tun');
		await modeSwitch.confirm();
		expect(begin).toHaveBeenCalledWith('tproxy', 'fakeip-tun');
		expect(switchMode).toHaveBeenCalledWith('fakeip-tun');
		expect(get(modeSwitch).phase).toBe('running');
	});
	it('confirm failure folds error into transition', async () => {
		switchMode.mockRejectedValueOnce(new Error('boom'));
		refs.statusVal = { enabled: false }; refs.settingsVal = {};
		modeSwitch.request('tproxy');
		await modeSwitch.confirm();
		expect(fail).toHaveBeenCalledWith('boom');
	});
	it('closeProgress → idle, resets transition + reloads', () => {
		refs.statusVal = { enabled: true }; refs.settingsVal = { routingMode: 'tproxy' };
		modeSwitch.request('off');
		modeSwitch.closeProgress();
		expect(reset).toHaveBeenCalled();
		expect(loadAll).toHaveBeenCalled();
		expect(get(modeSwitch).phase).toBe('idle');
	});
	it('cancel only acts from confirming — no-op during running', async () => {
		refs.statusVal = { enabled: true }; refs.settingsVal = { routingMode: 'tproxy' };
		modeSwitch.request('fakeip-tun');
		await modeSwitch.confirm();
		expect(get(modeSwitch).phase).toBe('running');
		modeSwitch.cancel(); // stray cancel mid-transition must not abandon it
		expect(get(modeSwitch).phase).toBe('running');
	});
});
