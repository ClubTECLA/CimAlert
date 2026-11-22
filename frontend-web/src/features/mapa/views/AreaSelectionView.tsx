import { LuPentagon, LuX } from "react-icons/lu";
import ColSection from "@/components/layout/ColSection";
import Button from "@/components/ui/Button";
import CentroRow from "@/features/centros/components/CentroRow";
import { useCentros } from "@/features/centros/useCentros";
import IncidentRow from "@/features/reportes/components/IncidentRow";
import { useReportes } from "@/features/reportes/useReportes";
import { masRecientes } from "@/features/resumen/resumen";
import { useCommonPanel } from "@/panels/useCommonPanel";
import { usePolygon } from "../usePolygon";

// Mismo tope que otras listas resumidas del panel: el detalle completo sigue en "Incidentes" y "Centros".
const MAX_ROWS = 20;
const sectionTitleStyle = "text-xs font-semibold tracking-[0.72px] text-ink-muted";
const noteStyle = "py-4 text-center text-sm text-ink-muted";

/**
 * Lo que cae dentro del polígono dibujado. Reportes y centros llegan ya filtrados de sus contextos, así que esta
 * vista solo presenta el resultado: no repite la geometría ni decide qué se ve en el mapa.
 */
export default function AreaSelectionView() {
    const { openPanel } = useCommonPanel();
    const { reportes, categories, filteredReportes } = useReportes();
    const { centros, tipos, filteredCentros } = useCentros();
    const { polygon, start, clear } = usePolygon();

    const incidentes = masRecientes(filteredReportes, MAX_ROWS);
    const centrosVisibles = filteredCentros.slice(0, MAX_ROWS);
    const subtitle = !reportes && !centros
        ? "Cargando…"
        : `${filteredReportes.length} ${filteredReportes.length === 1 ? "incidente" : "incidentes"} · ${filteredCentros.length} ${filteredCentros.length === 1 ? "centro" : "centros"}`;

    return (
        <ColSection
            title="Área seleccionada"
            subtitle={subtitle}
            arrangementSubsections="toUp"
            className="min-h-0 overflow-visible"
        >
            <div className="flex min-h-0 w-full flex-1 flex-col font-ui sm:w-[max(25rem,calc(25vw-2rem))]">
                <div className="-mr-3 flex min-h-0 flex-1 flex-col gap-3.5 overflow-y-auto pb-5 pr-3 sm:-mr-4 sm:pr-4">
                    {!polygon && <span className={noteStyle}>No hay un área seleccionada.</span>}

                    {polygon && (
                        <>
                            <span className={sectionTitleStyle}>INCIDENTES · {filteredReportes.length}</span>
                            {!reportes && <span className={noteStyle}>Cargando incidentes…</span>}
                            {reportes && filteredReportes.length === 0 && <span className={noteStyle}>Ningún incidente en esta área.</span>}
                            {incidentes.map((reporte) =>
                                <IncidentRow
                                    key={reporte.id}
                                    reporte={reporte}
                                    incidente={categories.get(reporte.incidente_id)}
                                    onClick={() => openPanel("Detalle Incidente", { reporteId: reporte.id })}
                                />
                            )}
                            {filteredReportes.length > incidentes.length &&
                                <button
                                    type="button"
                                    onClick={() => openPanel("Incidentes")}
                                    className="w-full rounded-full bg-canvas py-2 text-sm font-semibold text-ink transition-colors hover:bg-line-soft"
                                >
                                    Ver los {filteredReportes.length} incidentes en la lista
                                </button>
                            }

                            <span className={sectionTitleStyle}>CENTROS · {filteredCentros.length}</span>
                            {!centros && <span className={noteStyle}>Cargando centros…</span>}
                            {centros && filteredCentros.length === 0 && <span className={noteStyle}>Ningún centro en esta área.</span>}
                            {centrosVisibles.map((centro) =>
                                <CentroRow
                                    key={centro.id}
                                    centro={centro}
                                    tipo={tipos.get(centro.tipo_centro)}
                                    onClick={() => openPanel("Detalle Centro", { centroId: centro.id })}
                                />
                            )}
                            {filteredCentros.length > centrosVisibles.length &&
                                <button
                                    type="button"
                                    onClick={() => openPanel("Centros")}
                                    className="w-full rounded-full bg-canvas py-2 text-sm font-semibold text-ink transition-colors hover:bg-line-soft"
                                >
                                    Ver los {filteredCentros.length} centros en la lista
                                </button>
                            }
                        </>
                    )}
                </div>

                {/* Dibujar de nuevo sustituye el área al terminar. Limpiar quita el filtro de los dos módulos; cerrar el
                    panel, en cambio, no toca nada (el polígono sigue visible en el mapa y en el badge). */}
                <div className="flex flex-wrap gap-2.5 border-t border-line pt-3.5">
                    <Button
                        variant="secondary"
                        size="lg"
                        icon={<LuPentagon />}
                        title="Dibujar otra área"
                        onClick={start}
                        className="flex-1 max-sm:px-4"
                    />
                    <Button
                        variant="danger"
                        size="lg"
                        icon={<LuX />}
                        title="Limpiar selección"
                        onClick={clear}
                        className="flex-1 max-sm:px-4"
                    />
                </div>
            </div>
        </ColSection>
    );
}
