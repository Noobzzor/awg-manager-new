export type NatMode = 'full' | 'internet-only' | 'none';

/** Resolve tri-state NAT from API fields (natMode wins over legacy natEnabled). */
export function resolveNatMode(natMode?: NatMode, natEnabled?: boolean): NatMode {
	if (natMode === 'full' || natMode === 'internet-only' || natMode === 'none') {
		return natMode;
	}
	return natEnabled ? 'full' : 'none';
}

export function maskToPrefix(mask: string): string {
	if (/^\d+$/.test(mask)) return mask;
	const parts = mask.split('.').map(Number);
	let bits = 0;
	for (const p of parts) bits += (p >>> 0).toString(2).split('1').length - 1;
	return String(bits);
}

/** Subnet like 10.0.0.X/24 — host octets replaced with X per mask. */
export function formatSubnetPlaceholder(address: string, mask: string): string {
	const prefix = Number(maskToPrefix(mask));
	const octets = address.split('.').map((p) => parseInt(p, 10));
	if (octets.length !== 4 || octets.some((n) => Number.isNaN(n)) || Number.isNaN(prefix)) {
		return `${address}/${maskToPrefix(mask)}`;
	}
	const parts = octets.map((value, i) => {
		const bitsBefore = i * 8;
		if (prefix <= bitsBefore) return 'X';
		if (prefix >= bitsBefore + 8) return String(value);
		return 'X';
	});
	return `${parts.join('.')}/${prefix}`;
}
