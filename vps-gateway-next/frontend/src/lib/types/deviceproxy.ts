// ─────────────────────────────────────────────
// #region Device Proxy
// ─────────────────────────────────────────────

export interface DeviceProxyAuth {
	enabled: boolean;
	username: string;
	password: string;
}

export interface DeviceProxyConfig {
	enabled: boolean;
	listenAll: boolean;
	listenInterface: string;
	port: number;
	auth: DeviceProxyAuth;
	selectedOutbound: string;
}

export interface DeviceProxyInstance extends DeviceProxyConfig {
	id: string;
	name: string;
}

export type DeviceProxyOutboundKind = 'direct' | 'singbox' | 'subscription' | 'awg' | 'router';

export interface DeviceProxyOutbound {
	tag: string;
	kind: DeviceProxyOutboundKind;
	label: string;
	detail: string;
}

export interface DeviceProxyRuntime {
	alive: boolean;
	activeTag: string;
	defaultTag: string;
	/**
	 * Выбранный outbound (== defaultTag) сейчас отсутствует в merged-конфиге
	 * (слот-источник выключен — например, движок маршрутизации). Пусто или
	 * undefined — деградации нет (issue #465).
	 */
	degradedOutbound?: string;
	/** Через какой тег фактически идёт трафик, пока деградация активна. */
	fallbackTag?: string;
}

// #endregion
