import type { CSSProperties } from "react";
import { LuFilter, LuX } from "react-icons/lu";
import { useCommonPanel } from "@/panels/useCommonPanel";
import { usePanelRight } from "@/panels/usePanelRight";
import { useReportes } from "@/features/reportes/useReportes";
import { activeFilterLabels, DEFAULT_FILTERS } from "@/features/reportes/reportFilters";
import { useCentros } from "@/features/centros/useCentros";
import { DEFAULT_CENTRO_FILTERS, activeCentroFilterLabels } from "@/features/centros/centroFormat";
import { isCentroPanel, isIncidentPanel } from "@/features/centros/centrosPanels";
import { useRiesgo } from "@/features/riesgo/useRiesgo";

// Aviso sobre el mapa de que los pines están filtrados (incidentes y centros): el filtro sigue aplicándose al cerrar los
// paneles. Con el estilo de "Capas activas" del Figma, justo encima del dock y en su mismo eje (el mapa libre a la
// derecha del panel); la píldora de capas, si se muestra, va en la fila del dock. Por debajo de 1024 px, con un panel
// abierto, no cabe en el mapa libre: se oculta.
// Cada grupo se avisa solo si su capa se ve en el mapa y ningún panel abierto ya enseña su filtro: no con el riesgo
// (oculta los dos), ni con los paneles de centros o de incidentes (ocultan la otra capa y muestran su propio filtro).
// Con "Área seleccionada" sí se muestra: el polígono sigue filtrando al cerrar el panel y el badge es la forma de volver
// a él o de quitarlo.
const PANELS_SHOWING_FILTERS = ["Incidentes", "Capas y filtros"];
const AREA_LABEL = "Área seleccionada";

export default function ActiveFiltersBadge() {
    const { currentPanelView, openPanel } = useCommonPanel();
    const { reportes, filteredReportes, filters, setFilters } = useReportes();
    const { centros, filteredCentros, filters: centroFilters, setFilters: setCentroFilters } = useCentros();
    const { activo: riesgo } = useRiesgo();
    const panelRight = usePanelRight();

    const view = currentPanelView ?? "";
    const muestraFiltros = !riesgo && !PANELS_SHOWING_FILTERS.includes(view);
    const incidentLabels = muestraFiltros && reportes && !isCentroPanel(currentPanelView)
        ? activeFilterLabels(filters)
        : [];
    // Los paneles de centros también avisan del área: sigue filtrando el mapa aunque no vean la lista de filtros.
    const centroLabels = muestraFiltros && centros && !isIncidentPanel(currentPanelView)
        ? activeCentroFilterLabels(centroFilters)
        : [];
    // El área se anuncia aunque falte alguna lista por cargar y en los paneles de centros. Los dos módulos pueden
    // declararla, así que se deduplica; el X solo cuenta los filtros que sí se pueden quitar desde aquí.
    const polygon = filters.polygon ?? centroFilters.polygon;
    const labels = [...new Set([...incidentLabels, ...centroLabels, ...(muestraFiltros && polygon ? [AREA_LABEL] : [])])];
    if (!labels.length) return null;

    const removiblesIncidentes = incidentLabels.filter((label) => label !== AREA_LABEL);
    const removiblesCentros = centroLabels.filter((label) => label !== AREA_LABEL);
    const hayRemovibles = removiblesIncidentes.length > 0 || removiblesCentros.length > 0;

    const counts = [
        incidentLabels.length > 0 && reportes ? `${filteredReportes.length} de ${reportes.length} incidentes` : null,
        centroLabels.length > 0 && centros ? `${filteredCentros.length} de ${centros.length} centros` : null,
    ].filter(Boolean);

    // El área dibujada no se quita aquí: sigue filtrando hasta que se borre desde el panel del área.
    const quitarFiltros = () => {
        if (removiblesIncidentes.length) setFilters((prev) => ({ ...DEFAULT_FILTERS, polygon: prev.polygon }));
        if (removiblesCentros.length) setCentroFilters((prev) => ({ ...DEFAULT_CENTRO_FILTERS, polygon: prev.polygon }));
    };

    return (
        <div
            style={{ "--panel-right": `${panelRight}px` } as CSSProperties}
            className={`
                absolute bottom-24 left-1/2 z-10 flex max-w-[calc(100%-2rem)] -translate-x-1/2 items-center gap-1
                ${panelRight ? "max-lg:hidden" : ""}
                sm:left-[calc(var(--panel-right)+(100%-var(--panel-right))/2)] sm:max-w-[calc(100%-var(--panel-right)-2rem)]
                rounded-full border border-glass-line bg-glass p-1.5 font-ui backdrop-blur-glass drop-shadow-floating
            `}
        >
            <button
                type="button"
                onClick={() => openPanel(polygon ? "Área seleccionada" : "Capas y filtros")}
                title={polygon ? "Ver el área seleccionada" : "Ver o cambiar los filtros"}
                className="flex min-w-0 items-center gap-2.5 rounded-full py-0.5 pl-0.5 pr-2 text-left transition-colors hover:bg-subtle"
            >
                <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-brand text-white" aria-hidden>
                    <LuFilter className="size-4.5" />
                </span>
                <span className="flex min-w-0 flex-col">
                    <span className="truncate text-sm font-semibold text-ink">Filtro activo: {labels.join(" · ")}</span>
                    {/* role="status": al cambiar el filtro o llegar datos nuevos, el lector de pantalla lee el nuevo total. */}
                    <span role="status" className="truncate text-xs text-ink-muted">
                        El mapa muestra {counts.join(" · ")}
                    </span>
                </span>
            </button>
            {hayRemovibles && (
                <button
                    type="button"
                    onClick={quitarFiltros}
                    aria-label="Quitar filtros"
                    title="Quitar filtros"
                    className="flex size-8 shrink-0 items-center justify-center rounded-full text-icon transition-colors hover:bg-subtle"
                >
                    <LuX className="size-4.5" />
                </button>
            )}
        </div>
    );
}