import { afterEach, describe, expect, it, vi } from 'vitest';
import type { localStorageValues } from '@/types/localStorage-types';
import { applyStoredUpdate, storageChanged } from './useLocalStorage';

const GENERAL = 'Preferencias generales' as const;
const SECURITY = 'Preferencias de seguridad' as const;

function stubStorage(values: Partial<Record<string, unknown>>) {
  const entries = Object.fromEntries(
    Object.entries(values).map(([key, value]) => [key, JSON.stringify(value)]),
  );
  vi.stubGlobal('localStorage', { getItem: (key: string) => entries[key] ?? null });
}

afterEach(() => vi.unstubAllGlobals());

describe('applyStoredUpdate', () => {
  it('parte de lo que hay en storage, no del valor del render, tras una escritura de un efecto de layout', () => {
    // Render con dark_mode=false → efecto de layout guarda dark_mode=true → el efecto pasivo actualiza otra clave.
    stubStorage({ [GENERAL]: { dark_mode: true, show_location_in_searchBar: false } });

    const next = applyStoredUpdate(GENERAL, (prev) => ({ ...prev, show_location_in_searchBar: true }));

    expect(next).toEqual({ dark_mode: true, show_location_in_searchBar: true });
  });

  it('relee la clave activa en cada escritura, incluso yendo A→B→A sin escrituras intermedias', () => {
    const general: localStorageValues<typeof GENERAL> = { dark_mode: false, show_location_in_searchBar: true };
    const security: localStorageValues<typeof SECURITY> = { remember_in_this_device: false, two_steps_verification: true };
    stubStorage({ [GENERAL]: general, [SECURITY]: security });

    expect(applyStoredUpdate(SECURITY, (prev) => prev)).toEqual(security);
    expect(applyStoredUpdate(GENERAL, (prev) => prev)).toEqual(general);
  });
});

describe('storageChanged', () => {
  it('detecta una escritura que otra instancia hizo antes de subscribirse', () => {
    const rendered: localStorageValues<typeof GENERAL> = { dark_mode: false, show_location_in_searchBar: false };
    const stored: localStorageValues<typeof GENERAL> = { dark_mode: true, show_location_in_searchBar: false };

    expect(storageChanged(rendered, stored)).toBe(true);
    expect(storageChanged(stored, stored)).toBe(false);
  });
});
