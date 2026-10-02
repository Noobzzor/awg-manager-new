import { describe, expect, it, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
import RuleSetsTable from './RuleSetsTable.svelte';

describe('RuleSetsTable', () => {
  it('renders dat conversion rule sets as dat source instead of raw URL', () => {
    render(RuleSetsTable, {
      props: {
        ruleSets: [
          {
            tag: 'geosite-GOOGLE',
            type: 'remote',
            format: 'binary',
            url: 'http://127.0.0.1:2222/api/singbox/router/rulesets/dat-srs?kind=geosite&tag=GOOGLE&token=secret',
            update_interval: '24h',
          },
        ],
        onEdit: vi.fn(),
        onDelete: vi.fn(),
      },
    });

    expect(screen.getByText('dat')).toBeTruthy();
    expect(screen.getByText('geosite: GOOGLE')).toBeTruthy();
    expect(screen.getByText('direct')).toBeTruthy();
  });

  it('renders multi-tag dat sources with a single kind prefix', () => {
    render(RuleSetsTable, {
      props: {
        ruleSets: [
          {
            tag: 'geosite-google',
            type: 'remote',
            format: 'binary',
            url: 'http://127.0.0.1:2222/api/singbox/router/rulesets/dat-srs?kind=geosite&tag=GOOGLE-PLAY&tag=GOOGLE-DEEPMIND&tag=GOOGLE-GEMINI&token=secret',
            update_interval: '24h',
          },
        ],
        onEdit: vi.fn(),
        onDelete: vi.fn(),
      },
    });

    expect(screen.getByText('geosite: GOOGLE-PLAY, GOOGLE-DEEPMIND, GOOGLE-GEMINI')).toBeTruthy();
    expect(screen.queryByText(/geosite:GOOGLE-DEEPMIND/)).toBeNull();
  });

  it('reports only remote tags visible under the active type-filter via onSelectableChange', async () => {
    const onSelectableChange = vi.fn();
    render(RuleSetsTable, {
      props: {
        ruleSets: [
          { tag: 'remote-a', type: 'remote', url: 'https://example.com/a.srs', update_interval: '24h' },
          { tag: 'local-a', type: 'local', path: '/tmp/a.srs' },
        ],
        onEdit: vi.fn(),
        onDelete: vi.fn(),
        onSelectableChange,
      },
    });

    // Изначально фильтр «Все» — виден remote-тег.
    expect(onSelectableChange).toHaveBeenLastCalledWith(['remote-a']);

    // Переключаем на фильтр «Local» — единственный remote-тег больше не виден.
    onSelectableChange.mockClear();
    await fireEvent.click(screen.getByText('Local'));
    expect(onSelectableChange).toHaveBeenLastCalledWith([]);
  });
});
