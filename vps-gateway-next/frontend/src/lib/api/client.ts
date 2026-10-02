// Фасад API-клиента. Доменные методы разнесены по слоям client*.ts
// (цепочка наследования от CoreClient); публичная поверхность не менялась:
// `api` и сопутствующие экспорты доступны по прежнему пути $lib/api/client.
import { Awg3Client } from './clientAwg3';
import type { AwgAnalyzeData } from '$lib/types';
import type { RuntimeCapabilities } from '$lib/utils/runtimeCapabilities';

export { ApiGatewayError } from './clientCore';
export type { TrafficPeriod } from './clientCore';
// Реэкспорт для существующих импортов из '$lib/api/client'; сами типы
// объявлены в $lib/types/systemTools.
export type {
	SystemFileRoot,
	SystemFileEntry,
	FileSystemScriptStatus,
	SystemServiceItem,
	SystemOpkgPackage,
	SystemPortBinding,
	SystemProcSnapshot,
	SystemProcessItem,
} from '$lib/types';

export type GatewayClientStatus = 'active' | 'disabled' | 'revoked';
export interface GatewayClient {
	id: string;
	label: string;
	address: string;
	publicKey: string;
	status: GatewayClientStatus;
	createdAt: string;
	updatedAt: string;
}

export interface GatewaySubscription {
	id: string;
	clientId: string;
	createdAt: string;
	expiresAt: string;
	revokedAt?: string;
}

export interface GatewaySubscriptionIssue {
	subscription: GatewaySubscription;
	url: string;
}

export interface GatewayGroup {
	id: string;
	name: string;
	clientIds: string[];
	createdAt: string;
	updatedAt: string;
}

export type GatewayPolicyAction = 'vpn' | 'warp' | 'direct' | 'block';

export interface GatewayPolicyRule {
	id: string;
	action: GatewayPolicyAction;
	outbound?: string;
	priority?: number;
	clientIds?: string[];
	groupIds?: string[];
	sourceCidrs?: string[];
	domains?: string[];
	domainSuffixes?: string[];
	ruleSets?: string[];
	cidrs?: string[];
	ports?: number[];
	protocols?: string[];
	enabled?: boolean;
}

export interface GatewayPolicyProfile {
	id: string;
	name: string;
	defaultAction: GatewayPolicyAction;
	rules?: GatewayPolicyRule[];
}

export interface GatewayPolicyPreviewRule {
	domain?: string[];
	domain_suffix?: string[];
	rule_set?: string[];
	ip_cidr?: string[];
	source_ip_cidr?: string[];
	port?: number[];
	protocol?: string[];
	action?: string;
	outbound?: string;
}

export interface GatewayPolicyPreview {
	final: string;
	rules: GatewayPolicyPreviewRule[];
}

class ApiClient extends Awg3Client {
	async getCapabilities(): Promise<RuntimeCapabilities> {
		return this.request<RuntimeCapabilities>('/capabilities');
	}

	// Анализ .conf на бэкенде: версия, поля без ключей, ошибки совместимости.
	// tunnelId подставляет ключи из хранилища, если их нет в тексте (#865).
	async analyzeAwgConf(conf: string, tunnelId?: string): Promise<AwgAnalyzeData> {
		return this.request<AwgAnalyzeData>('/awg/analyze', {
			method: 'POST',
			body: JSON.stringify(tunnelId ? { conf, tunnelId } : { conf }),
		});
	}

	async getGatewayClients(): Promise<GatewayClient[]> {
		return this.request<GatewayClient[]>('/gateway/clients');
	}

	async createGatewayClient(id: string, label: string): Promise<GatewayClient> {
		return this.request<GatewayClient>('/gateway/clients/create', {
			method: 'POST',
			body: JSON.stringify({ id, label }),
		});
	}

	async gatewayClientAction(id: string, action: 'disable' | 'enable' | 'revoke'): Promise<GatewayClient> {
		return this.request<GatewayClient>(`/gateway/clients/${encodeURIComponent(id)}/${action}`, {
			method: 'POST',
		});
	}

	async getGatewayClientConfig(id: string): Promise<string> {
		return this.requestText(`/gateway/clients/${encodeURIComponent(id)}/config`);
	}

	async getGatewaySubscriptions(): Promise<GatewaySubscription[]> {
		return this.request<GatewaySubscription[]>('/gateway/subscriptions');
	}

	async createGatewaySubscription(clientId: string, expiresInDays: number): Promise<GatewaySubscriptionIssue> {
		return this.request<GatewaySubscriptionIssue>('/gateway/subscriptions/create', {
			method: 'POST',
			body: JSON.stringify({ clientId, expiresInDays }),
		});
	}

	async revokeGatewaySubscription(id: string): Promise<{ revoked: boolean }> {
		return this.request<{ revoked: boolean }>(`/gateway/subscriptions/${encodeURIComponent(id)}/revoke`, {
			method: 'POST',
		});
	}

	async getGatewayGroups(): Promise<GatewayGroup[]> {
		return this.request<GatewayGroup[]>('/gateway/groups');
	}

	async createGatewayGroup(name: string, clientIds: string[]): Promise<GatewayGroup> {
		return this.request<GatewayGroup>('/gateway/groups/create', {
			method: 'POST',
			body: JSON.stringify({ name, clientIds }),
		});
	}

	async updateGatewayGroup(id: string, name: string, clientIds: string[]): Promise<GatewayGroup> {
		return this.request<GatewayGroup>(`/gateway/groups/${encodeURIComponent(id)}`, {
			method: 'PUT',
			body: JSON.stringify({ name, clientIds }),
		});
	}

	async deleteGatewayGroup(id: string): Promise<{ deleted: boolean }> {
		return this.request<{ deleted: boolean }>(`/gateway/groups/${encodeURIComponent(id)}`, {
			method: 'DELETE',
		});
	}

	async getGatewayPolicyProfiles(): Promise<GatewayPolicyProfile[]> {
		return this.request<GatewayPolicyProfile[]>('/gateway/policies/profiles');
	}

	async createGatewayPolicyProfile(profile: GatewayPolicyProfile): Promise<GatewayPolicyProfile> {
		return this.request<GatewayPolicyProfile>('/gateway/policies/profiles/create', {
			method: 'POST',
			body: JSON.stringify(profile),
		});
	}

	async updateGatewayPolicyProfile(profile: GatewayPolicyProfile): Promise<GatewayPolicyProfile> {
		return this.request<GatewayPolicyProfile>(`/gateway/policies/profiles/${encodeURIComponent(profile.id)}`, {
			method: 'PUT',
			body: JSON.stringify(profile),
		});
	}

	async previewGatewayPolicyProfile(profile: GatewayPolicyProfile): Promise<GatewayPolicyPreview> {
		return this.request<GatewayPolicyPreview>('/gateway/policies/profiles/preview', {
			method: 'POST',
			body: JSON.stringify(profile),
		});
	}

	async applyGatewayPolicyProfile(id: string): Promise<{ applied: boolean }> {
		return this.request<{ applied: boolean }>(`/gateway/policies/profiles/${encodeURIComponent(id)}/apply`, {
			method: 'POST',
		});
	}

	async deleteGatewayPolicyProfile(id: string): Promise<{ deleted: boolean }> {
		return this.request<{ deleted: boolean }>(`/gateway/policies/profiles/${encodeURIComponent(id)}`, {
			method: 'DELETE',
		});
	}
}

export const api = new ApiClient();
