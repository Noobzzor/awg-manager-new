/**
 * Разбор позиции ошибки из сообщений JSON.parse разных движков в
 * 1-базные line/column для гаттера редактора конфигурации.
 *
 * Поддерживаемые форматы:
 *  - V8 (новый):  «… in JSON at position 42 (line 3 column 5)»
 *  - Firefox:     «… at line 3 column 5 of the JSON data»
 *  - V8 (старый): «… in JSON at position 42» → пересчёт по тексту
 */

export interface JsonErrorPos {
	/** 1-базная строка. */
	line: number;
	/** 1-базная колонка. */
	column: number;
}

/** Пересчёт байтового (точнее, code-unit) смещения в line/column (1-базные). */
export function positionToLineCol(text: string, position: number): JsonErrorPos {
	const pos = Math.max(0, Math.min(position, text.length));
	let line = 1;
	let lineStart = 0;
	for (let i = 0; i < pos; i++) {
		if (text[i] === '\n') {
			line++;
			lineStart = i + 1;
		}
	}
	return { line, column: pos - lineStart + 1 };
}

const LINE_COL_RE = /line (\d+),? column (\d+)/i;
const POSITION_RE = /at position (\d+)/i;

/**
 * Достаёт line/column из сообщения ошибки JSON.parse. text нужен для
 * пересчёта «at position N» (старый V8). null — когда движок позицию
 * не сообщил (Safari) или сообщение нераспознано.
 */
export function parseJsonErrorPosition(message: string, text: string): JsonErrorPos | null {
	const lc = LINE_COL_RE.exec(message);
	if (lc) {
		const line = Number.parseInt(lc[1], 10);
		const column = Number.parseInt(lc[2], 10);
		if (Number.isFinite(line) && Number.isFinite(column) && line >= 1 && column >= 1) {
			return { line, column };
		}
	}
	const p = POSITION_RE.exec(message);
	if (p) {
		const pos = Number.parseInt(p[1], 10);
		if (Number.isFinite(pos)) {
			return positionToLineCol(text, pos);
		}
	}
	return null;
}
