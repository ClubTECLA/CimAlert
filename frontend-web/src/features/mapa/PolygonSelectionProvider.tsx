import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import type { Position } from "geojson";
import { useCentros } from "@/features/centros/useCentros";
import { useReportes } from "@/features/reportes/useReportes";
import { useCommonPanel } from "@/panels/useCommonPanel";
import { polygonFromPoints } from "@/lib/geo";
import { PolygonSelectionContext, type PolygonSelectionContextValue } from "./PolygonContext.ts";
import { polygonDrawingStore } from "./polygonDrawingStore";

/**
 * Estado del área dibujada. El polígono VIVE en ReportFilters: así el mapa, la lista de incidentes, el badge y
 * "Capas y filtros" lo ven sin un estado paralelo. Aquí solo se guarda el dibujo en curso y se replica el polígono
 * a CentroFilters para que los pins de centros se filtren con el mismo criterio.
 */
export function PolygonSelectionProvider({ children }: { children: ReactNode }) {
    const { filters, setFilters } = useReportes();
    const { setFilters: setCentroFilters } = useCentros();
    const { currentPanelView, closePanel, openPanel } = useCommonPanel();

    const [draft, setDraft] = useState<Position[]>([]);
    const [isDrawing, setDrawing] = useState(false);
    // finish() lo llama el mapa desde un listener registrado una sola vez: lee los vértices del ref, no del render.
    const draftRef = useRef<Position[]>([]);

    const polygon = filters.polygon ?? null;

    // Un solo sitio que escribe en los filtros de centros: cualquier cambio del área canónica (dibujo, "Limpiar
    // selección" o resetFilters) llega aquí. Devolver `prev` cuando no cambia evita renders y bucles.
    useEffect(() => {
        setCentroFilters((prev) => prev.polygon === polygon ? prev : { ...prev, polygon });
    }, [polygon, setCentroFilters]);

    const setDrawingState = useCallback((next: boolean) => {
        setDrawing(next);
        // Los providers de ubicación y de dibujo son hermanos: esta señal evita que un clic del dibujo mueva el
        // marcador compartido de "Nuevo incidente" / "Nuevo centro" si aún no se ha desmontado su panel.
        polygonDrawingStore.set(next);
        if (!next) {
            draftRef.current = [];
            setDraft([]);
        }
    }, []);

    const start = useCallback(() => {
        // Cerrar el panel desmonta LocationPickerProvider's handler global de clic.
        closePanel();
        draftRef.current = [];
        setDraft([]);
        setDrawing(true);
        polygonDrawingStore.set(true);
    }, [closePanel]);

    const addPoint = useCallback((point: Position) => {
        const points = draftRef.current;
        const last = points[points.length - 1];
        // El doble clic dispara dos clics seguidos en el mismo sitio: no queremos dos vértices iguales.
        if (last && last[0] === point[0] && last[1] === point[1]) return;
        draftRef.current = [...points, point];
        setDraft(draftRef.current);
    }, []);

    const finish = useCallback(() => {
        const next = polygonFromPoints(draftRef.current);
        setDrawingState(false);
        if (!next) return; // dibujo incompleto: se conserva el área anterior
        setFilters((prev) => ({ ...prev, polygon: next }));
        openPanel("Área seleccionada");
    }, [openPanel, setDrawingState, setFilters]);

    const cancel = useCallback(() => {
        setDrawingState(false);
    }, [setDrawingState]);

    const clear = useCallback(() => {
        setFilters((prev) => prev.polygon ? { ...prev, polygon: null } : prev);
        // El panel del área sin área no dice nada: solo lo cerramos si es el que está abierto.
        if (currentPanelView === "Área seleccionada") closePanel();
    }, [closePanel, currentPanelView, setFilters]);

    const value = useMemo<PolygonSelectionContextValue>(() => ({
        polygon, isDrawing, draft, start, addPoint, finish, cancel, clear,
    }), [polygon, isDrawing, draft, start, addPoint, finish, cancel, clear]);

    return (
        <PolygonSelectionContext.Provider value={value}>
            {children}
        </PolygonSelectionContext.Provider>
    );
}
