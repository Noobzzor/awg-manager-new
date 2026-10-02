import type { SystemInfo } from '$lib/types';

/**
 * Portable local mode has no router/NDMS system-info endpoint. Keep the
 * settings page usable with explicit non-router defaults instead of hiding it.
 */
export function createLocalSystemInfo(version: string): SystemInfo {
	return {
		version,
		goVersion: '',
		goArch: '',
		goOS: 'linux',
		keeneticOS: '',
		isOS5: false,
		firmwareVersion: '',
		supportsExtendedASC: false,
		supportsHRanges: false,
		supportsPingCheck: false,
		totalMemoryMB: 0,
		isLowMemory: false,
		gcMemLimit: '',
		gogc: '',
		disableMemorySaving: false,
		kernelModuleExists: false,
		kernelModuleLoaded: false,
		kernelModuleModel: '',
		kernelModuleVersion: '',
		isAarch64: false,
		activeBackend: 'singbox',
		routerIP: '',
		bootInProgress: false,
		backendAvailability: { nativewg: false, kernel: false },
		singbox: { installed: true, version: '' },
	};
}
