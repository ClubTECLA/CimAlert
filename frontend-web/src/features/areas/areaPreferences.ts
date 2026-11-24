// Preferencias del filtrado por área: tipos y valor predeterminado viven en el registro de localStorage,
// la lectura la hace el hook y aquí solo queda la validación y la decisión del modo (sin DOM).

import { AREA_FILTER_STORAGE_KEY, type AreaFilterMode, type AreaFilterPreferencesType } from '@/types/localStorage-types';
import { localStorageBaseRegistry } from '@/lib/LocalStorageRegistry';
import { readStoredValue } from '@/lib/useLocalStorage';

export type AreaMode = AreaFilterMode
export type AreaPreferences = AreaFilterPreferencesType
export type EffectiveAreaMode = Exclude<AreaMode, 'auto'>

export const AREA_THRESHOLD_MIN = 10
export const AREA_THRESHOLD_MAX = 5000
export const AREA_THRESHOLD_STEP = 10

export const DEFAULT_AREA_PREFERENCES: AreaPreferences = localStorageBaseRegistry[AREA_FILTER_STORAGE_KEY]

const MODES: AreaMode[] = ['auto', 'frontend', 'backend']

export function normalizeAreaPreferences(value: unknown): AreaPreferences {
    if (typeof value !== 'object' || value === null) return { ...DEFAULT_AREA_PREFERENCES }
    const candidate = value as Partial<AreaPreferences>
    const mode = MODES.includes(candidate.mode as AreaMode) ? candidate.mode as AreaMode : DEFAULT_AREA_PREFERENCES.mode
    const threshold = typeof candidate.threshold === 'number' && Number.isFinite(candidate.threshold)
        ? Math.min(AREA_THRESHOLD_MAX, Math.max(AREA_THRESHOLD_MIN, Math.round(candidate.threshold)))
        : DEFAULT_AREA_PREFERENCES.threshold
    return { mode, threshold }
}

/** Lee la clave registrada con el mismo parseo que usa el hook, aplicando además el rango del umbral. */
export function readAreaPreferences(): AreaPreferences {
    return normalizeAreaPreferences(readStoredValue(AREA_FILTER_STORAGE_KEY))
}

/** Modo efectivo: en 'auto', el número de reportes decide. */
export function resolveAreaMode(preferences: AreaPreferences, totalReports: number): EffectiveAreaMode {
    if (preferences.mode !== 'auto') return preferences.mode
    return totalReports > preferences.threshold ? 'backend' : 'frontend'
}
