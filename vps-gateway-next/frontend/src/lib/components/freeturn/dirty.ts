/** Ключи конфига, отличающиеся от сохранённого снапшота (shallow, поля примитивные). */
export function changedKeys<T extends object>(cur: T, saved: T | null): (keyof T)[] {
	if (!saved) return [];
	return (Object.keys(cur) as (keyof T)[]).filter((k) => cur[k] !== saved[k]);
}
