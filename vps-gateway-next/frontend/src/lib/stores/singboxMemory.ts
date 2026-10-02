// singboxMemory — память Go-рантайма sing-box (bytes) из поля `memory`
// Clash /connections WebSocket, доставляется SSE-событием `singbox:memory`.
// Это НЕ RSS процесса: sing-box сам репортит runtime-статистику (heap/stack
// in-use), фактический RSS в top заметно выше. Fed by +layout
// onSingboxMemory handler. Value is 0 before the first push.

import { writable } from 'svelte/store';

export const singboxMemory = writable<number>(0);

export function applySingboxMemory(data: { memory: number }): void {
	singboxMemory.set(data.memory ?? 0);
}
