import { afterEach, describe, expect, it, vi } from 'vitest'
import { AREA_FILTER_STORAGE_KEY, type AreaFilterPreferencesType } from '@/types/localStorage-types'
import {
    AREA_THRESHOLD_MAX,
    AREA_THRESHOLD_MIN,
    DEFAULT_AREA_PREFERENCES,
    normalizeAreaPreferences,
    readAreaPreferences,
    resolveAreaMode,
} from './areaPreferences'

function memoryStorage(initial: Record<string, string> = {}) {
    const data = new Map<string, string>(Object.entries(initial))
    return {
        getItem: (key: string) => data.get(key) ?? null,
        setItem: (key: string, value: string) => { data.set(key, value) },
    }
}

afterEach(() => vi.unstubAllGlobals())

describe('preferencias de área', () => {
    it('el registro arranca con el modo backend activo', () => {
        expect(DEFAULT_AREA_PREFERENCES).toEqual({ mode: 'backend', threshold: 500 })
        vi.stubGlobal('localStorage', memoryStorage())
        expect(readAreaPreferences()).toEqual(DEFAULT_AREA_PREFERENCES)
    })

    it('lee la clave registrada a través del hook', () => {
        const saved: AreaFilterPreferencesType = { mode: 'auto', threshold: 120 }
        vi.stubGlobal('localStorage', memoryStorage({ [AREA_FILTER_STORAGE_KEY]: JSON.stringify(saved) }))
        expect(readAreaPreferences()).toEqual(saved)
    })

    it('acota el umbral al rango permitido', () => {
        expect(normalizeAreaPreferences({ threshold: 1 }).threshold).toBe(AREA_THRESHOLD_MIN)
        expect(normalizeAreaPreferences({ threshold: 99999 }).threshold).toBe(AREA_THRESHOLD_MAX)
        expect(normalizeAreaPreferences({ threshold: 120.6 }).threshold).toBe(121)
    })

    it('ignora valores corruptos o incompletos', () => {
        expect(normalizeAreaPreferences(null)).toEqual(DEFAULT_AREA_PREFERENCES)
        expect(normalizeAreaPreferences({ mode: 'otro', threshold: 20 })).toEqual({ mode: 'backend', threshold: 20 })
        expect(normalizeAreaPreferences({ mode: 'auto', threshold: 'muchos' })).toEqual({ mode: 'auto', threshold: 500 })
    })
})

describe('modo efectivo', () => {
    const auto = { mode: 'auto' as const, threshold: 100 }

    it('los modos forzados no dependen del número de reportes', () => {
        expect(resolveAreaMode({ mode: 'frontend', threshold: 100 }, 10_000)).toBe('frontend')
        expect(resolveAreaMode({ mode: 'backend', threshold: 100 }, 1)).toBe('backend')
    })

    it('auto usa el frontend con pocos reportes y el backend al superar el límite', () => {
        expect(resolveAreaMode(auto, 0)).toBe('frontend')
        expect(resolveAreaMode(auto, 100)).toBe('frontend')
        expect(resolveAreaMode(auto, 101)).toBe('backend')
    })
})
