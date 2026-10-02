import type { DnsRouteBackend } from './dnsRouteBackend';

export interface RuntimeCapabilities {
	singboxStatus: boolean;
	awg3: boolean;
	singboxConfigEditor: boolean;
	subscriptions: boolean;
	inbounds: boolean;
	dnsRoutes: boolean;
	dnsRouteBackend: DnsRouteBackend | null;
	proxyInbound: boolean;
	logs: boolean;
	ndms: boolean;
	/** Router-specific Sing-box control surface; absent means legacy backend. */
	singboxRouter?: boolean;
	hydraroute?: boolean;
	tunnelDiagnostics?: boolean;
	updates?: boolean;
	daemonRestart?: boolean;
	ndmsProxy?: boolean;
	/** VPS gateway client management is mounted only in gateway-enabled mode. */
	gateway?: boolean;
}

export function supportsRouterDiagnostics(capabilities: RuntimeCapabilities): boolean {
	return capabilities.ndms;
}

function legacyFallback(capabilities: RuntimeCapabilities, value: boolean | undefined): boolean {
	return value ?? capabilities.ndms;
}

export function supportsSingboxRouter(capabilities: RuntimeCapabilities): boolean {
	return legacyFallback(capabilities, capabilities.singboxRouter);
}

export function supportsHydraroute(capabilities: RuntimeCapabilities): boolean {
	return legacyFallback(capabilities, capabilities.hydraroute);
}

export function supportsTunnelDiagnostics(capabilities: RuntimeCapabilities): boolean {
	return legacyFallback(capabilities, capabilities.tunnelDiagnostics);
}

export function supportsUpdates(capabilities: RuntimeCapabilities): boolean {
	return legacyFallback(capabilities, capabilities.updates);
}

export function supportsDaemonRestart(capabilities: RuntimeCapabilities): boolean {
	return legacyFallback(capabilities, capabilities.daemonRestart);
}

export function supportsNDMSProxy(capabilities: RuntimeCapabilities): boolean {
	return legacyFallback(capabilities, capabilities.ndmsProxy);
}
