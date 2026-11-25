import { readAreaPreferences, resolveAreaMode, type EffectiveAreaMode } from './areaPreferences'

// Dato volátil compartido entre ReportesProvider y CentrosProvider: el total de
// reportes sin filtrar que sirve para decidir el modo en 'auto'. Igual que
// polygonDrawingStore, evita un contexto nuevo y el acoplamiento entre ambos
// providers. null = todavía no se ha descargado ninguna lista completa.

let totalReportes: number | null = null

export const areaFilterStore = {
    /** Lo escribe ReportesProvider cuando descarga la lista completa (sin área o en modo automático). */
    setTotalReportes(total: number | null) {
        totalReportes = total
    },

    getTotalReportes() {
        return totalReportes
    },

    /** Misma decisión para ambos providers, con el total conocido hasta ahora. */
    resolveMode(): EffectiveAreaMode {
        return resolveAreaMode(readAreaPreferences(), totalReportes ?? 0)
    },
}
