/**
 * Session TTL (issue #441) — bounds shared by the «Время жизни сессии»
 * input in the «Доступ» card. Mirrors backend validation (1..720, default 24).
 */
export const SESSION_TTL_MIN_HOURS = 1;
export const SESSION_TTL_MAX_HOURS = 720;
export const SESSION_TTL_DEFAULT_HOURS = 24;

/**
 * Normalize a user-typed / backend-provided TTL to a valid whole number
 * of hours. Non-numeric or absent values (legacy backends omit the field)
 * fall back to the default of 24.
 */
export function clampSessionTtlHours(value: unknown): number {
	// null/undefined/'' means "no value" (legacy backend or cleared input),
	// not zero — Number() would silently coerce those to 0.
	if (value === null || value === undefined || value === '') {
		return SESSION_TTL_DEFAULT_HOURS;
	}
	const n = typeof value === 'number' ? value : Number(value);
	if (!Number.isFinite(n)) return SESSION_TTL_DEFAULT_HOURS;
	const rounded = Math.round(n);
	if (rounded < SESSION_TTL_MIN_HOURS) return SESSION_TTL_MIN_HOURS;
	if (rounded > SESSION_TTL_MAX_HOURS) return SESSION_TTL_MAX_HOURS;
	return rounded;
}
