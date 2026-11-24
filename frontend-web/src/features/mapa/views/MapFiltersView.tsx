import { useState } from "react";
import ColSection from "@/components/layout/ColSection";
import CategoryChip from "@/components/ui/CategoryChip";
import Button from "@/components/ui/Button";
import { SegmentedTabs, TabItem } from "@/components/ui/SegmentedTabs";
import { useCommonPanel } from "@/panels/useCommonPanel";
import { useReportes } from "@/features/reportes/useReportes";
import {
    ALL_STATUSES, DEFAULT_FILTERS, filterReportes, OFFICIAL_STATUSES, OPEN_STATUSES, toggled, type MapPeriod, type ReportFilters,
} from "@/features/reportes/reportFilters";
import {
    DEFAULT_CENTRO_FILTERS,
    filterCentros,
    type CentroFilters,
    type CentroTab,
} from "@/features/centros/centroFormat";
import { useCentros } from "@/features/centros/useCentros";
import { esRefugio, tipoStyle } from "@/features/centros/tiposCentro";

// shortcut: falta "Rango" del Figma, que necesita un selector de fechas; añadirlo cuando el periodo se active.
const PERIODS: { key: MapPeriod; label: string }[] = [
    { key: "ahora", label: "Ahora" },
    { key: "24h", label: "24 horas" },
    { key: "7d", label: "7 días" },
];

const CENTRO_TABS: { key: CentroTab; label: string }[] = [
    { key: "activos", label: "Activos" },
    { key: "ocupados", label: "Ocupados" },
    { key: "todos", label: "Todos" },
];

const sectionTitleStyle = "text-xs font-semibold tracking-[0.72px] text-ink-muted";

// "7. Mapa - Filtros" del Figma. Edita el filtro que comparten el mapa y la lista de "Incidentes"; los cambios se
// aplican con "Mostrar N resultados", y regresar o cerrar los descarta. Sin la sección "Capas" del Figma: sus filas
// repetían los estados y confundían, y "Avisos oficiales" es el chip "Solo oficiales", igual que en la lista: va en su
// propia sección porque, a diferencia de los estados, restringe lo que se ve al marcarlo.
export default function MapFiltersView() {
    const { closePanel } = useCommonPanel();
    const { reportes, categories, filters, setFilters } = useReportes();
    const { centros, tipos, filters: centroFilters, setFilters: setCentroFilters } = useCentros();
    const [draft, setDraft] = useState<ReportFilters>(filters);
    const [centroDraft, setCentroDraft] = useState<CentroFilters>(centroFilters);


    // Las activas y las desactivadas que aún tienen reportes, para poder ocultarlos también.
    const categoryList = [...categories.values()].filter((category) =>
        category.esta_activo || reportes?.some((reporte) => reporte.incidente_id === category.id));
    const hasDates = !!reportes?.some((reporte) => reporte.created_at);
    // El periodo necesita created_at, que el listado aún no envía: queda desactivado hasta que llegue.
    const reportCount = reportes ? filterReportes(reportes, draft).length : null;
    const centroCount = centros ? filterCentros(centros, centroDraft, tipos).length : null;
    // Suma incidentes y centros; null mientras no haya llegado ninguno de los dos.
    const count = reportCount === null && centroCount === null ? null : (reportCount ?? 0) + (centroCount ?? 0);

    const update = (changes: Partial<ReportFilters>) => setDraft((prev) => ({ ...prev, ...changes }));
    const updateCentros = (changes: Partial<CentroFilters>) => setCentroDraft((prev) => ({ ...prev, ...changes }));

    const apply = () => {
        setFilters(draft);
        setCentroFilters(centroDraft);
        closePanel();
    };

    return (
        <ColSection
            title="Filtros del mapa"
            subtitle="Elige qué se muestra en el mapa"
            arrangementSubsections="toUp"
            className="min-h-0 overflow-visible"
        >
            {/* Mismo ancho fijo que el resto de paneles; el scroll llega al borde y el pie queda abajo, como en el Figma.
                ColSection va con overflow-visible: con scroll propio, lo que el scroll ocupa del relleno le daría una
                barra horizontal. */}
            <div className="flex min-h-0 w-full flex-1 flex-col font-ui sm:w-[max(25rem,calc(25vw-2rem))]">
                <div className="-mr-3 flex min-h-0 flex-1 flex-col gap-3.5 overflow-y-auto pb-2 pr-3 sm:-mr-4 sm:pr-4 [scrollbar-width:thin] [&>*]:shrink-0">
                    <span className={sectionTitleStyle}>CATEGORÍAS</span>
                    <div className="flex flex-wrap gap-2">
                        {categoryList.map((category) =>
                            <CategoryChip
                                key={category.id}
                                label={category.etiqueta}
                                color={category.color}
                                isSelected={!draft.hiddenCategories.includes(category.id)}
                                onPress={() => update({ hiddenCategories: toggled(draft.hiddenCategories, category.id) })}
                            />
                        )}
                    </div>

                    <span className={sectionTitleStyle}>ESTADO</span>
                    <div className="flex flex-wrap gap-2">
                        {ALL_STATUSES.map((status) =>
                            <CategoryChip
                                key={status}
                                label={status}
                                color="var(--color-brand)"
                                isSelected={draft.statuses.includes(status)}
                                onPress={() => update({ statuses: toggled(draft.statuses, status) })}
                            />
                        )}
                    </div>

                    <span className={sectionTitleStyle}>ORIGEN</span>
                    <div className="flex flex-wrap gap-2">
                        {/* Sin pestañas que decidan, como en la lista (ver toggledOfficial): al quitarlo vuelven los de entrada. */}
                        <CategoryChip
                            label="Solo oficiales"
                            color="var(--color-brand)"
                            isSelected={draft.onlyOfficial}
                            onPress={() => update({ statuses: draft.onlyOfficial ? OPEN_STATUSES : OFFICIAL_STATUSES, onlyOfficial: !draft.onlyOfficial })}
                        />
                    </div>

                    <span className={sectionTitleStyle}>PERIODO</span>
                    {/* fieldset desactiva de una vez todos los botones de las pestañas. */}
                    <fieldset disabled={!hasDates} className="flex min-w-0 flex-col gap-1.5 disabled:opacity-60">
                        <legend className="sr-only">Periodo</legend>
                        <SegmentedTabs
                            format="row"
                            activeTab={PERIODS.findIndex(({ key }) => key === draft.period)}
                            onChange={(index) => update({ period: PERIODS[index].key })}
                        >
                            {PERIODS.map(({ key, label }) => <TabItem key={key} tabTitle={label} />)}
                        </SegmentedTabs>
                        {!hasDates && <span className="text-xs text-ink-muted">Disponible cuando los reportes incluyan su fecha.</span>}
                    </fieldset>

                    <span className={sectionTitleStyle}>CENTROS DE AYUDA</span>
                    <SegmentedTabs
                        format="row"
                        activeTab={CENTRO_TABS.findIndex(({ key }) => key === centroDraft.tab)}
                        onChange={(index) => updateCentros({ tab: CENTRO_TABS[index].key })}
                    >
                        {CENTRO_TABS.map(({ key, label }) => <TabItem key={key} tabTitle={label} />)}
                    </SegmentedTabs>
                    {tipos.size > 0 &&
                        <div className="flex flex-wrap gap-2">
                            {/* En "Ocupados" solo puede haber refugios, igual que en la lista de centros. */}
                            {[...tipos.values()].filter((tipo) => centroDraft.tab !== "ocupados" || esRefugio(tipo)).map((tipo) =>
                                <CategoryChip
                                    key={tipo.id}
                                    label={tipo.nombre}
                                    color={tipoStyle(tipo).color}
                                    isSelected={!centroDraft.hiddenTipos.includes(tipo.id)}
                                    onPress={() => updateCentros({ hiddenTipos: toggled(centroDraft.hiddenTipos, tipo.id) })}
                                />
                            )}
                        </div>
                    }
                </div>

                {/* flex-wrap: en los teléfonos más estrechos los dos botones no caben en una fila y cada uno pasa a la suya. */}
                <div className="flex flex-wrap gap-2.5 border-t border-line pt-3.5">
                    {/* El área dibujada no es una opción de este panel: se conserva al restablecer los demás filtros. */}
                    <Button
                        variant="subtle"
                        size="lg"
                        title="Restablecer"
                        onClick={() => {
                            setDraft({ ...DEFAULT_FILTERS, polygon: draft.polygon });
                            setCentroDraft({ ...DEFAULT_CENTRO_FILTERS, polygon: centroDraft.polygon });
                        }}
                        className="flex-1 max-sm:px-4"
                    />
                    <Button
                        variant="primary"
                        size="lg"
                        title={count === null ? "Mostrar resultados" : `Mostrar ${count} ${count === 1 ? "resultado" : "resultados"}`}
                        onClick={apply}
                        className="flex-1 drop-shadow-button-brand max-sm:px-4"
                    />
                </div>
            </div>
        </ColSection>
    );
}
