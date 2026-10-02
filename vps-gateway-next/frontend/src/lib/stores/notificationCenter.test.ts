import { describe, it, expect, beforeEach, vi } from 'vitest';
import { get } from 'svelte/store';
import { createNotificationCenterStore, dayBucket } from './notificationCenter';

function createLocalStorageMock(): Storage {
  const data = new Map<string, string>();
  return {
    get length() { return data.size; },
    clear: () => data.clear(),
    getItem: (k: string) => data.get(k) ?? null,
    key: (i: number) => Array.from(data.keys())[i] ?? null,
    removeItem: (k: string) => { data.delete(k); },
    setItem: (k: string, v: string) => { data.set(k, v); },
  };
}

const MIN = 60_000;
const DAY = 24 * 60 * 60 * 1000;

beforeEach(() => {
  vi.stubGlobal('localStorage', createLocalStorageMock());
});

describe('notificationCenter: record + coalesce', () => {
  it('adds a new entry at the head', () => {
    const nc = createNotificationCenterStore();
    nc.record({ type: 'error', message: 'boom', ts: 0 });
    const list = get(nc);
    expect(list).toHaveLength(1);
    expect(list[0]).toMatchObject({ type: 'error', message: 'boom', count: 1, read: false });
  });

  it('coalesces same type+message within 5 min: bumps count, refreshes ts, re-marks unread', () => {
    const nc = createNotificationCenterStore();
    nc.record({ type: 'error', message: 'boom', ts: 0 });
    nc.markRead(get(nc)[0].id);
    nc.record({ type: 'error', message: 'boom', ts: 1 * MIN });
    const list = get(nc);
    expect(list).toHaveLength(1);
    expect(list[0].count).toBe(2);
    expect(list[0].lastTs).toBe(1 * MIN);
    expect(list[0].read).toBe(false);
  });

  it('does NOT coalesce outside the 5 min window', () => {
    const nc = createNotificationCenterStore();
    nc.record({ type: 'error', message: 'boom', ts: 0 });
    nc.record({ type: 'error', message: 'boom', ts: 6 * MIN });
    expect(get(nc)).toHaveLength(2);
  });

  it('does NOT coalesce different type or message', () => {
    const nc = createNotificationCenterStore();
    nc.record({ type: 'error', message: 'boom', ts: 0 });
    nc.record({ type: 'warning', message: 'boom', ts: 1000 });
    nc.record({ type: 'error', message: 'other', ts: 1000 });
    expect(get(nc)).toHaveLength(3);
  });
});

describe('notificationCenter: read ops', () => {
  it('markRead flips one entry', () => {
    const nc = createNotificationCenterStore();
    nc.record({ type: 'error', message: 'a', ts: 0 });
    const id = get(nc)[0].id;
    nc.markRead(id);
    expect(get(nc)[0].read).toBe(true);
  });

  it('markAllRead flips all', () => {
    const nc = createNotificationCenterStore();
    nc.record({ type: 'error', message: 'a', ts: 0 });
    nc.record({ type: 'warning', message: 'b', ts: 1 });
    nc.markAllRead();
    expect(get(nc).every((e) => e.read)).toBe(true);
  });

  it('remove drops one entry; clearAll empties', () => {
    const nc = createNotificationCenterStore();
    nc.record({ type: 'error', message: 'a', ts: 0 });
    nc.record({ type: 'warning', message: 'b', ts: 1 });
    const id = get(nc)[0].id;
    nc.remove(id);
    expect(get(nc)).toHaveLength(1);
    nc.clearAll();
    expect(get(nc)).toHaveLength(0);
  });
});

describe('notificationCenter: persistence', () => {
  it('round-trips through localStorage: a fresh store loads prior entries', () => {
    const now = Date.now();
    const a = createNotificationCenterStore();
    a.record({ type: 'error', message: 'persisted', ts: now });
    const b = createNotificationCenterStore();
    const list = get(b);
    expect(list).toHaveLength(1);
    expect(list[0].message).toBe('persisted');
  });
});

describe('notificationCenter: id collision after reload', () => {
  it('does not reissue an id that survived in localStorage (no duplicate keys)', () => {
    const now = Date.now();
    const a = createNotificationCenterStore();
    a.record({ type: 'error', message: 'before reload', ts: now });
    const survivingId = get(a)[0].id;

    // Fresh store = page reload: counter resets, but the entry is still stored.
    const b = createNotificationCenterStore();
    b.record({ type: 'error', message: 'after reload', ts: now + 1 });

    const list = get(b);
    const ids = list.map((e) => e.id);
    expect(new Set(ids).size).toBe(ids.length); // all unique
    // Both must survive: a colliding id would let prune's dedup drop a real entry.
    expect(list.map((e) => e.message).sort()).toEqual(['after reload', 'before reload']);
    expect(ids.filter((id) => id === survivingId)).toHaveLength(1);
  });

  it('prune dedups entries that already share an id', () => {
    const now = Date.now();
    localStorage.setItem(
      'awg-manager-notification-center',
      JSON.stringify([
        { id: 'nc-1', type: 'error', message: 'a', firstTs: now, lastTs: now, count: 1, read: false },
        { id: 'nc-1', type: 'error', message: 'b', firstTs: now, lastTs: now, count: 1, read: false },
      ]),
    );
    const nc = createNotificationCenterStore();
    expect(get(nc)).toHaveLength(1);
  });
});

describe('notificationCenter: retention', () => {
  it('keeps at most 100 newest entries', () => {
    const nc = createNotificationCenterStore();
    for (let i = 0; i < 105; i++) {
      nc.record({ type: 'error', message: `m${i}`, ts: i });
    }
    const list = get(nc);
    expect(list).toHaveLength(100);
    expect(list[0].message).toBe('m104');
    expect(list.some((e) => e.message === 'm0')).toBe(false);
  });

  it('drops entries older than 7 days relative to the newest record ts', () => {
    const nc = createNotificationCenterStore();
    nc.record({ type: 'error', message: 'old', ts: 0 });
    nc.record({ type: 'warning', message: 'new', ts: 8 * DAY });
    const list = get(nc);
    expect(list.some((e) => e.message === 'old')).toBe(false);
    expect(list.some((e) => e.message === 'new')).toBe(true);
  });

  it('prunes >7-day-old entries on load using current time', () => {
    vi.useFakeTimers();
    try {
      vi.setSystemTime(0);
      const a = createNotificationCenterStore();
      a.record({ type: 'error', message: 'stale', ts: 0 });
      vi.setSystemTime(8 * DAY);
      const b = createNotificationCenterStore();
      expect(get(b)).toHaveLength(0);
    } finally {
      vi.useRealTimers();
    }
  });
});

describe('dayBucket', () => {
  const base = new Date(2026, 5, 13, 12, 0, 0).getTime();

  it('classifies same calendar day as today', () => {
    const morning = new Date(2026, 5, 13, 1, 0, 0).getTime();
    expect(dayBucket(morning, base)).toBe('today');
  });

  it('classifies previous calendar day as yesterday', () => {
    const y = new Date(2026, 5, 12, 23, 0, 0).getTime();
    expect(dayBucket(y, base)).toBe('yesterday');
  });

  it('classifies older as earlier', () => {
    expect(dayBucket(base - 3 * DAY, base)).toBe('earlier');
  });
});
