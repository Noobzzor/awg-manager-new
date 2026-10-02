import { describe, it, expect } from 'vitest';
import {
	normalizeTag,
	getItemTags,
	collectAllTags,
	groupFlatItemsByTag,
	filterItemsByTag,
} from './tunnelDashboardTags';

const item = (key: string) => ({ key, name: key });

describe('normalizeTag', () => {
	it('trims and collapses inner whitespace', () => {
		expect(normalizeTag('  дом \t  офис  ')).toBe('дом офис');
	});

	it('returns null for empty or whitespace-only input', () => {
		expect(normalizeTag('')).toBeNull();
		expect(normalizeTag('   \t ')).toBeNull();
	});

	it('caps length at 24 characters', () => {
		const result = normalizeTag('a'.repeat(40));
		expect(result).toBe('a'.repeat(24));
	});

	it('does not leave a trailing space after the cap', () => {
		const result = normalizeTag(`${'a'.repeat(23)} bcd`);
		expect(result).toBe('a'.repeat(23));
	});
});

describe('getItemTags', () => {
	it('returns tags for a key and [] for unknown keys', () => {
		const tags = { 'awg:1': ['дом'] };
		expect(getItemTags(tags, 'awg:1')).toEqual(['дом']);
		expect(getItemTags(tags, 'awg:2')).toEqual([]);
	});
});

describe('collectAllTags', () => {
	it('returns unique tags sorted alphabetically (ru locale)', () => {
		const tags = {
			'awg:1': ['офис', 'Дом'],
			'singbox:x': ['архив', 'офис'],
		};
		expect(collectAllTags(tags)).toEqual(['архив', 'Дом', 'офис']);
	});

	it('returns [] for empty map', () => {
		expect(collectAllTags({})).toEqual([]);
	});

	it('dedupes case-insensitively keeping the first-encountered casing', () => {
		const tags = {
			'awg:1': ['Home'],
			'awg:2': ['home'],
			'awg:3': ['Дом', 'дом'],
		};
		expect(collectAllTags(tags)).toEqual(['Дом', 'Home']);
	});
});

describe('groupFlatItemsByTag', () => {
	it('puts an item into every group it is tagged with', () => {
		const items = [item('awg:1'), item('singbox:x')];
		const tags = {
			'awg:1': ['дом', 'офис'],
			'singbox:x': ['офис'],
		};
		const groups = groupFlatItemsByTag(items, tags);
		expect(groups.map((g) => g.tag)).toEqual(['дом', 'офис']);
		expect(groups[0].items.map((i) => i.key)).toEqual(['awg:1']);
		expect(groups[1].items.map((i) => i.key)).toEqual(['awg:1', 'singbox:x']);
	});

	it('puts untagged items into a trailing null group', () => {
		const items = [item('awg:1'), item('awg:2')];
		const groups = groupFlatItemsByTag(items, { 'awg:1': ['дом'] });
		expect(groups.map((g) => g.tag)).toEqual(['дом', null]);
		expect(groups[1].items.map((i) => i.key)).toEqual(['awg:2']);
	});

	it('omits empty groups including the null group', () => {
		const items = [item('awg:1')];
		const tags = {
			'awg:1': ['дом'],
			'awg:gone': ['архив'],
		};
		const groups = groupFlatItemsByTag(items, tags);
		expect(groups.map((g) => g.tag)).toEqual(['дом']);
	});

	it('returns a single null group when nothing is tagged', () => {
		const items = [item('a'), item('b')];
		const groups = groupFlatItemsByTag(items, {});
		expect(groups).toEqual([{ tag: null, items }]);
	});

	it('returns [] for empty items', () => {
		expect(groupFlatItemsByTag([], { 'awg:1': ['дом'] })).toEqual([]);
	});

	it('merges differently-cased tags into one group', () => {
		const items = [item('awg:1'), item('awg:2')];
		const tags = {
			'awg:1': ['Home'],
			'awg:2': ['home'],
		};
		const groups = groupFlatItemsByTag(items, tags);
		expect(groups.map((g) => g.tag)).toEqual(['Home']);
		expect(groups[0].items.map((i) => i.key)).toEqual(['awg:1', 'awg:2']);
	});
});

describe('filterItemsByTag', () => {
	it('keeps only items carrying the tag', () => {
		const items = [item('awg:1'), item('awg:2'), item('singbox:x')];
		const tags = {
			'awg:1': ['дом'],
			'singbox:x': ['дом', 'офис'],
		};
		const result = filterItemsByTag(items, tags, 'дом');
		expect(result.map((i) => i.key)).toEqual(['awg:1', 'singbox:x']);
	});

	it('returns [] when no item has the tag', () => {
		expect(filterItemsByTag([item('a')], {}, 'дом')).toEqual([]);
	});

	it('matches tags case-insensitively', () => {
		const items = [item('awg:1'), item('awg:2')];
		const tags = {
			'awg:1': ['Home'],
			'awg:2': ['home'],
		};
		expect(filterItemsByTag(items, tags, 'HOME').map((i) => i.key)).toEqual(['awg:1', 'awg:2']);
	});
});
