import type { Polygon, Position } from "geojson";
import { createContext } from "react";

// Contrato del dibujo de áreas. El polígono aplicado se lee de ReportFilters (fuente canónica); el provider solo
// guarda el dibujo en curso y coordina los filtros de reportes y centros.
export interface PolygonSelectionContextValue {
    /** Área aplicada al mapa y a los filtros; null si no hay selección. */
    polygon: Polygon | null
    /** true mientras el usuario está colocando vértices. */
    isDrawing: boolean
    /** Vértices ya colocados, en coordenadas GeoJSON [longitud, latitud]. */
    draft: Position[]
    /** Empieza un dibujo nuevo: cierra el panel activo, previa confirmación si hay un borrador pendiente. */
    start: () => void
    /** Añade el vértice donde se hizo clic. */
    addPoint: (point: Position) => void
    /** Cierra el anillo, aplica el área y abre el panel "Área seleccionada". */
    finish: () => void
    /** Abandona el dibujo sin cambiar el área ya aplicada. */
    cancel: () => void
    /** Quita el área de reportes y centros. No se usa al cerrar el panel. */
    clear: () => void
}

export const PolygonSelectionContext = createContext<PolygonSelectionContextValue | null>(null);
