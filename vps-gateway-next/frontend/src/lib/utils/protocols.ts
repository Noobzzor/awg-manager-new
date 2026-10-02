// ---------------------------------------------------------------------------
// AWG Signature Profile Catalog
//
// Каталог профилей имитации (CONTEXT.md «Профиль имитации»). Сигнатуру
// собирает бэкенд: POST /api/signature/generate. Ключи = signature.Profiles.
//
// Суммарная ДЛИНА СТРОК I1-I5 ограничена буфером awg-tools, см.
// signature.MaxSignatureChars — там же замеры со стенда. Полезная нагрузка
// (`<r 1000>` = 1000 байт в 8 символах) на лимит не влияет.
// ---------------------------------------------------------------------------

export const MAX_SIGNATURE_CHARS = 3500;

export type ProtocolKey = 'quic_initial' | 'stun' | 'dns' | 'dtls' | 'sip';

export const protocols: Record<ProtocolKey, { name: string; description: string }> = {
	quic_initial: { name: 'QUIC Initial', description: 'HTTP/3 — валидный ClientHello с шифрованием по RFC 9001' },
	stun: { name: 'STUN / TURN', description: 'WebRTC ICE — Binding или Allocate с FINGERPRINT' },
	dns: { name: 'DNS Query', description: 'UDP DNS-запрос A/AAAA/HTTPS с EDNS0' },
	dtls: { name: 'DTLS (WebRTC)', description: 'DTLS 1.2 ClientHello с use_srtp' },
	sip: { name: 'SIP', description: 'VoIP — REGISTER и повтор с Digest-авторизацией (I1, I2)' },
};

export interface SignaturePackets {
	i1: string;
	i2: string;
	i3: string;
	i4: string;
	i5: string;
}

/** Суммарная длина строк I1-I5 — величина, которую ограничивает буфер awg-tools. */
export function calcTotalChars(packets: SignaturePackets): number {
	return (packets.i1 + packets.i2 + packets.i3 + packets.i4 + packets.i5).length;
}
