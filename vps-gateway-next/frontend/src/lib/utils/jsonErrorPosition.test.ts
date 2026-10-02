import { describe, expect, it } from 'vitest';
import { parseJsonErrorPosition, positionToLineCol } from './jsonErrorPosition';

describe('positionToLineCol', () => {
	it('первая строка, начало текста', () => {
		expect(positionToLineCol('{"a":1}', 0)).toEqual({ line: 1, column: 1 });
	});

	it('середина первой строки', () => {
		expect(positionToLineCol('{"a":1}', 5)).toEqual({ line: 1, column: 6 });
	});

	it('позиция на следующих строках', () => {
		const text = '{\n  "a": 1,\n  "b": }\n}';
		// index первого '}' — третья строка, восьмая колонка
		const pos = text.indexOf('}');
		expect(positionToLineCol(text, pos)).toEqual({ line: 3, column: 8 });
	});

	it('клампит выход за границы текста', () => {
		expect(positionToLineCol('ab\ncd', 999)).toEqual({ line: 2, column: 3 });
		expect(positionToLineCol('ab\ncd', -5)).toEqual({ line: 1, column: 1 });
	});

	it('позиция сразу после \\n — первая колонка новой строки', () => {
		expect(positionToLineCol('ab\ncd', 3)).toEqual({ line: 2, column: 1 });
	});
});

describe('parseJsonErrorPosition', () => {
	const text = '{\n  "a": ,\n}';

	it('новый V8: line/column прямо в сообщении', () => {
		const msg = "Unexpected token ',', ...\"a\": ,\" is not valid JSON at position 9 (line 2 column 8)";
		expect(parseJsonErrorPosition(msg, text)).toEqual({ line: 2, column: 8 });
	});

	it('Firefox: at line N column M', () => {
		const msg = 'JSON.parse: unexpected character at line 2 column 8 of the JSON data';
		expect(parseJsonErrorPosition(msg, text)).toEqual({ line: 2, column: 8 });
	});

	it('старый V8: at position N → пересчёт по тексту', () => {
		const msg = 'Unexpected token , in JSON at position 9';
		expect(parseJsonErrorPosition(msg, text)).toEqual({ line: 2, column: 8 });
	});

	it('реальная ошибка JSON.parse текущего движка распознаётся', () => {
		// «Expected …»-ошибки V8 включают «at position N (line X column Y)»;
		// формат «Unexpected token 'x', …context…» позицию не сообщает — для
		// него util честно возвращает null (маркер в гаттере не ставится).
		const bad = '{\n  "a": 1,\n  "b": 2,\n}';
		let message = '';
		try {
			JSON.parse(bad);
		} catch (e) {
			message = e instanceof Error ? e.message : String(e);
		}
		const pos = parseJsonErrorPosition(message, bad);
		expect(pos).not.toBeNull();
		expect(pos!.line).toBe(4);
	});

	it('сообщение без позиции → null', () => {
		expect(parseJsonErrorPosition('Something went wrong', text)).toBeNull();
	});
});
