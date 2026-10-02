import { describe, expect, it } from 'vitest';
import {
	supportsDaemonRestart,
	supportsHydraroute,
	supportsNDMSProxy,
	supportsRouterDiagnostics,
	supportsSingboxRouter,
	supportsTunnelDiagnostics,
	supportsUpdates,
	type RuntimeCapabilities,
} from './runtimeCapabilities';

function capabilities(ndms: boolean): RuntimeCapabilities {
	return {
		singboxStatus: !ndms,
		awg3: !ndms,
		singboxConfigEditor: false,
		subscriptions: false,
		inbounds: false,
		dnsRoutes: true,
		dnsRouteBackend: ndms ? 'ndms' : 'singbox',
		proxyInbound: false,
		logs: false,
		ndms,
		singboxRouter: ndms,
		hydraroute: ndms,
		tunnelDiagnostics: ndms,
		updates: ndms,
		daemonRestart: ndms,
		ndmsProxy: ndms,
	};
}

describe('supportsRouterDiagnostics', () => {
	it('disables router-only tools in local Docker mode', () => {
		expect(supportsRouterDiagnostics(capabilities(false))).toBe(false);
	});

	it('keeps router tools for NDMS deployments', () => {
		expect(supportsRouterDiagnostics(capabilities(true))).toBe(true);
	});

	it('does not infer router-only surfaces from singboxStatus', () => {
		const local = capabilities(false);
		expect(supportsSingboxRouter(local)).toBe(false);
		expect(supportsHydraroute(local)).toBe(false);
		expect(supportsTunnelDiagnostics(local)).toBe(false);
		expect(supportsUpdates(local)).toBe(false);
		expect(supportsDaemonRestart(local)).toBe(false);
		expect(supportsNDMSProxy(local)).toBe(false);
	});

	it('keeps legacy backends compatible when new fields are absent', () => {
		const legacy = { ...capabilities(true) };
		delete legacy.singboxRouter;
		delete legacy.hydraroute;
		delete legacy.tunnelDiagnostics;
		delete legacy.updates;
		delete legacy.daemonRestart;
		delete legacy.ndmsProxy;
		expect(supportsSingboxRouter(legacy)).toBe(true);
		expect(supportsHydraroute(legacy)).toBe(true);
		expect(supportsTunnelDiagnostics(legacy)).toBe(true);
		expect(supportsUpdates(legacy)).toBe(true);
		expect(supportsDaemonRestart(legacy)).toBe(true);
		expect(supportsNDMSProxy(legacy)).toBe(true);
	});
});
