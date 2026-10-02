import { describe, it, expect, vi, beforeEach } from 'vitest';
import type { SingboxRouterSettings } from '$lib/types';

vi.mock('$lib/api/client', () => ({
  api: {
    singboxRouterGetSettings: vi.fn(),
    singboxRouterPutSettings: vi.fn(),
  },
}));

vi.mock('$lib/stores/singboxRouter', () => ({
  singboxRouter: {
    loadAll: vi.fn(async () => {}),
  },
}));

import { api } from '$lib/api/client';
import { singboxRouter } from '$lib/stores/singboxRouter';
import { BYPASS_PRESETS, mergeAndSaveSettings } from './settingsActions';

const baseSettings: SingboxRouterSettings = {
  enabled: true,
  policyName: 'awgm-router',
  snifferEnabled: false,
  wanAutoDetect: true,
};

function mockCurrent(settings: SingboxRouterSettings): void {
  (api.singboxRouterGetSettings as ReturnType<typeof vi.fn>).mockResolvedValue(settings);
}

describe('settingsActions', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCurrent(baseSettings);
  });

  it('BYPASS_PRESETS has 4 entries with correct ids', () => {
    expect(BYPASS_PRESETS).toHaveLength(4);
    const ids = BYPASS_PRESETS.map((p) => p.id).sort();
    expect(ids).toEqual(['keendns', 'l2tp', 'netbios-smb', 'ntp']);
    for (const p of BYPASS_PRESETS) {
      expect(p.label).toBeTruthy();
      expect(p.desc).toBeTruthy();
    }
  });

  it('mergeAndSaveSettings: merges patch over FRESH settings from the API', async () => {
    await mergeAndSaveSettings({ snifferEnabled: true });
    expect(api.singboxRouterGetSettings).toHaveBeenCalled();
    expect(api.singboxRouterPutSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        enabled: true,
        policyName: 'awgm-router',
        snifferEnabled: true,
        wanAutoDetect: true,
      }),
    );
    expect(singboxRouter.loadAll).toHaveBeenCalled();
  });

  // Регрессия #487/#494-ревью: поля, изменённые ВНЕ settings-форм (бэкенд пишет
  // fakeipRealServer при правке адреса DNS-сервера «real»), не должны
  // откатываться эхом устаревшего стора — база merge берётся свежим GET'ом.
  it('mergeAndSaveSettings: carries server-side field changes (fakeipRealServer)', async () => {
    mockCurrent({ ...baseSettings, fakeipRealServer: '9.9.9.9' });
    await mergeAndSaveSettings({ snifferEnabled: true });
    expect(api.singboxRouterPutSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        fakeipRealServer: '9.9.9.9',
        snifferEnabled: true,
      }),
    );
  });

  it('mergeAndSaveSettings: patch overrides existing field', async () => {
    mockCurrent({ ...baseSettings, wanInterface: '' });
    await mergeAndSaveSettings({ wanAutoDetect: false, wanInterface: 'ppp0' });
    expect(api.singboxRouterPutSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        wanAutoDetect: false,
        wanInterface: 'ppp0',
      }),
    );
  });

  it('mergeAndSaveSettings: bypass presets array', async () => {
    await mergeAndSaveSettings({ bypassPresets: ['l2tp', 'ntp'] });
    expect(api.singboxRouterPutSettings).toHaveBeenCalledWith(
      expect.objectContaining({ bypassPresets: ['l2tp', 'ntp'] }),
    );
  });

  it('mergeAndSaveSettings: re-throws PUT error', async () => {
    (api.singboxRouterPutSettings as ReturnType<typeof vi.fn>).mockRejectedValueOnce(new Error('boom'));
    await expect(mergeAndSaveSettings({ snifferEnabled: true })).rejects.toThrow('boom');
  });

  it('mergeAndSaveSettings: re-throws GET error without attempting PUT', async () => {
    (api.singboxRouterGetSettings as ReturnType<typeof vi.fn>).mockRejectedValueOnce(new Error('offline'));
    await expect(mergeAndSaveSettings({ snifferEnabled: true })).rejects.toThrow('offline');
    expect(api.singboxRouterPutSettings).not.toHaveBeenCalled();
  });

  it('mergeAndSaveSettings: empty patch → put with current', async () => {
    await mergeAndSaveSettings({});
    expect(api.singboxRouterPutSettings).toHaveBeenCalledWith(
      expect.objectContaining({ enabled: true }),
    );
  });
});
