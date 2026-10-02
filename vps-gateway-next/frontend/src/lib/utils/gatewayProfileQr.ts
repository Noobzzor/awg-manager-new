import QRCode from 'qrcode';

/** Create an in-memory PNG QR for a client profile. Never persist or log the profile. */
export async function createGatewayProfileQr(profile: string): Promise<string> {
	if (!profile.trim()) throw new Error('Профиль пуст');
	return QRCode.toDataURL(profile, {
		type: 'image/png',
		errorCorrectionLevel: 'M',
		margin: 2,
		width: 360,
		color: { dark: '#111827', light: '#FFFFFF' },
	});
}
