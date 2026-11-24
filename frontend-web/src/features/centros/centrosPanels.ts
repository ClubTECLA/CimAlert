import type { PanelViewKey } from "@/panels/panelTypes";

// Paneles del módulo de centros: mientras uno está abierto, el mapa oculta los incidentes.
const CENTRO_PANELS: PanelViewKey[] = ["Centros", "Detalle Centro", "Nuevo Centro"];
// Paneles del módulo de incidentes: mientras uno está abierto, el mapa oculta los centros.
const INCIDENT_PANELS: PanelViewKey[] = ["Incidentes", "Detalle Incidente", "Nuevo Incidente"];
// El área seleccionada no pertenece a ninguno: el mapa muestra juntos incidentes y centros filtrados por el polígono.
const AREA_PANEL: PanelViewKey = "Área seleccionada";

export const isCentroPanel = (view?: PanelViewKey | null) => !!view && CENTRO_PANELS.includes(view);
export const isIncidentPanel = (view?: PanelViewKey | null) => !!view && INCIDENT_PANELS.includes(view);
export const isAreaPanel = (view?: PanelViewKey | null) => !!view && view === AREA_PANEL;
