import { describe, it, expect } from 'vitest';
import { parsePortEntry, parsePortsString, serializePorts, parseDraftEntries, portKey, portLabel } from './ports';

describe('parsePortEntry — single ports', () => {
  it('accepts "993 tcp"', () => {
    expect(parsePortEntry('993 tcp')).toEqual({ ok: true, entry: { from: 993, to: 993, proto: 'TCP' } });
  });
  it('accepts forgiving forms tcp:993, 993/tcp, tcp25', () => {
    expect(parsePortEntry('tcp:993')).toEqual({ ok: true, entry: { from: 993, to: 993, proto: 'TCP' } });
    expect(parsePortEntry('993/tcp')).toEqual({ ok: true, entry: { from: 993, to: 993, proto: 'TCP' } });
    expect(parsePortEntry('tcp25')).toEqual({ ok: true, entry: { from: 25, to: 25, proto: 'TCP' } });
  });
  it('normalizes proto to uppercase (udp)', () => {
    expect(parsePortEntry('53 udp')).toEqual({ ok: true, entry: { from: 53, to: 53, proto: 'UDP' } });
  });
  it('rejects missing proto', () => {
    expect(parsePortEntry('5001')).toMatchObject({ ok: false });
  });
  it('rejects missing port', () => {
    expect(parsePortEntry('tcp')).toMatchObject({ ok: false });
  });
  it('rejects out-of-range ports', () => {
    expect(parsePortEntry('70000 tcp')).toMatchObject({ ok: false });
    expect(parsePortEntry('0 tcp')).toMatchObject({ ok: false });
  });
  it('accepts boundary ports 1 and 65535', () => {
    expect(parsePortEntry('1 tcp').ok).toBe(true);
    expect(parsePortEntry('65535 udp').ok).toBe(true);
  });
});

describe('parsePortEntry — ranges', () => {
  it('accepts "5000-5500 UDP"', () => {
    expect(parsePortEntry('5000-5500 UDP')).toEqual({
      ok: true,
      entry: { from: 5000, to: 5500, proto: 'UDP' },
    });
  });
  it('accepts "8000-9000 TCP"', () => {
    expect(parsePortEntry('8000-9000 TCP')).toEqual({
      ok: true,
      entry: { from: 8000, to: 9000, proto: 'TCP' },
    });
  });
  it('accepts single-value range "443-443 TCP" (from === to)', () => {
    expect(parsePortEntry('443-443 TCP')).toEqual({
      ok: true,
      entry: { from: 443, to: 443, proto: 'TCP' },
    });
  });
  it('rejects reversed range "5500-5000 UDP"', () => {
    expect(parsePortEntry('5500-5000 UDP')).toMatchObject({ ok: false });
  });
  it('rejects range with out-of-range end port', () => {
    expect(parsePortEntry('5000-99999 UDP')).toMatchObject({ ok: false });
  });
  it('rejects range with out-of-range start port', () => {
    expect(parsePortEntry('0-100 UDP')).toMatchObject({ ok: false });
  });
});

describe('portKey', () => {
  it('single port key format', () => {
    expect(portKey({ from: 443, to: 443, proto: 'TCP' })).toBe('443-443/TCP');
  });
  it('range key format', () => {
    expect(portKey({ from: 5000, to: 5500, proto: 'UDP' })).toBe('5000-5500/UDP');
  });
  it('deduplication works via key equality', () => {
    expect(parsePortsString('443 TCP, 443 tcp')).toHaveLength(1);
  });
});

describe('portLabel', () => {
  it('single port shows "443/TCP"', () => {
    expect(portLabel({ from: 443, to: 443, proto: 'TCP' })).toBe('443/TCP');
  });
  it('range shows "5000–5500/UDP" with en-dash', () => {
    expect(portLabel({ from: 5000, to: 5500, proto: 'UDP' })).toBe('5000–5500/UDP');
  });
});

describe('parsePortsString / serializePorts', () => {
  it('parses backend single-port format', () => {
    expect(parsePortsString('443 TCP, 53 UDP')).toEqual([
      { from: 443, to: 443, proto: 'TCP' },
      { from: 53, to: 53, proto: 'UDP' },
    ]);
  });
  it('parses backend range format', () => {
    expect(parsePortsString('5000-5500 UDP')).toEqual([
      { from: 5000, to: 5500, proto: 'UDP' },
    ]);
  });
  it('parses mixed single and range', () => {
    expect(parsePortsString('443 TCP, 5000-5500 UDP')).toEqual([
      { from: 443, to: 443, proto: 'TCP' },
      { from: 5000, to: 5500, proto: 'UDP' },
    ]);
  });
  it('skips invalid entries', () => {
    expect(parsePortsString('443 TCP, garbage, 53 UDP')).toEqual([
      { from: 443, to: 443, proto: 'TCP' },
      { from: 53, to: 53, proto: 'UDP' },
    ]);
  });
  it('dedups (case-insensitive proto)', () => {
    expect(parsePortsString('443 TCP, 443 tcp')).toEqual([{ from: 443, to: 443, proto: 'TCP' }]);
  });
  it('empty/blank → empty array', () => {
    expect(parsePortsString('')).toEqual([]);
    expect(parsePortsString('   ')).toEqual([]);
  });
  it('serializes single port to backend format', () => {
    expect(serializePorts([{ from: 443, to: 443, proto: 'TCP' }, { from: 53, to: 53, proto: 'UDP' }])).toBe('443 TCP, 53 UDP');
  });
  it('serializes range to backend format', () => {
    expect(serializePorts([{ from: 5000, to: 5500, proto: 'UDP' }])).toBe('5000-5500 UDP');
  });
  it('round-trip single port parse↔serialize is stable', () => {
    expect(serializePorts(parsePortsString('443 TCP, 53 UDP'))).toBe('443 TCP, 53 UDP');
  });
  it('round-trip range parse↔serialize is stable', () => {
    expect(serializePorts(parsePortsString('5000-5500 UDP, 8000-9000 TCP'))).toBe('5000-5500 UDP, 8000-9000 TCP');
  });
  it('serialize output matches backend grammar PORT UDP|TCP', () => {
    expect(serializePorts([{ from: 1194, to: 1194, proto: 'UDP' }])).toMatch(/^\d+ (TCP|UDP)(, \d+ (TCP|UDP))*$/);
  });
  it('serialize range output matches backend grammar PORT-PORT UDP|TCP', () => {
    expect(serializePorts([{ from: 5000, to: 5500, proto: 'UDP' }])).toMatch(/^\d+-\d+ (TCP|UDP)$/);
  });
});

describe('parseDraftEntries', () => {
  it('parses a single entry', () => {
    expect(parseDraftEntries('443 tcp')).toEqual({ ok: true, entries: [{ from: 443, to: 443, proto: 'TCP' }] });
  });
  it('parses a range entry', () => {
    expect(parseDraftEntries('5000-5500 udp')).toEqual({
      ok: true,
      entries: [{ from: 5000, to: 5500, proto: 'UDP' }],
    });
  });
  it('parses pasted multi-entry "443 tcp, 53 udp"', () => {
    expect(parseDraftEntries('443 tcp, 53 udp')).toEqual({
      ok: true,
      entries: [{ from: 443, to: 443, proto: 'TCP' }, { from: 53, to: 53, proto: 'UDP' }],
    });
  });
  it('parses mixed single and range', () => {
    expect(parseDraftEntries('51820 udp, 5000-5500 tcp')).toEqual({
      ok: true,
      entries: [{ from: 51820, to: 51820, proto: 'UDP' }, { from: 5000, to: 5500, proto: 'TCP' }],
    });
  });
  it('all-or-nothing: any invalid part aborts with first error, no partial accept', () => {
    const r = parseDraftEntries('443 tcp, 99999 tcp');
    expect(r.ok).toBe(false);
    if (!r.ok) expect(r.error).toContain('1–65535');
  });
  it('ignores blank parts / trailing comma', () => {
    expect(parseDraftEntries('443 tcp, , 53 udp,')).toEqual({
      ok: true,
      entries: [{ from: 443, to: 443, proto: 'TCP' }, { from: 53, to: 53, proto: 'UDP' }],
    });
  });
  it('empty draft → ok with no entries', () => {
    expect(parseDraftEntries('   ')).toEqual({ ok: true, entries: [] });
  });
});
